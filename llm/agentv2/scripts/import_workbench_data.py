"""Build a read-only, workspace-scoped replay dataset from existing receiver logs.

Reuses the validated protocol parser. Never sends network requests or modifies
the platform, original logs, device settings, or existing runtime registries.
"""
import argparse
import hashlib
import importlib.util
import json
import pathlib
import tempfile
import os


def build(root):
    resources_path = root / 'output/weather-platform/resources.json'
    resources = json.loads(resources_path.read_text(encoding='utf-8-sig'))
    parser_path = root / 'output/weather-receiver/rk_protocol.py'
    spec = importlib.util.spec_from_file_location('workbench_rk_protocol', parser_path)
    protocol = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(protocol)
    metrics = []
    mappings = []
    for device in resources['devices'].values():
        for m in device['metrics']:
            metrics.append(dict(key=m['key'], label=m['label'], unit=m.get('unit', ''),
                                admission=m['ingestion_recommendation'],
                                unit_confirmation=('confirmed' if m['key'] == 'air_pressure' and m.get('unit') == 'kPa'
                                                   else m.get('unit_confirmation', 'not_flagged')),
                                publisher_id=device['id']))
            mappings.append(m)
    frames = []
    sources = {}
    seen = set()
    for path in sorted((root / 'output/weather-receiver/logs').glob('received-*.jsonl')):
        sources[path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
        for line_number, line in enumerate(path.open(encoding='utf-8-sig'), 1):
            item = json.loads(line)
            if item.get('event') != 'frame':
                continue
            raw = bytes.fromhex(item['hex'])
            parsed = protocol.parse_packet(raw)
            if parsed.get('type_code') != 'EA03' or parsed['device_id'] != resources['station_id']:
                continue
            identity = hashlib.sha256((item['time'] + ':' + raw.hex()).encode()).hexdigest()
            if identity in seen:
                continue
            seen.add(identity)
            nodes = {n['node_id']: n for n in parsed['nodes']}
            values = {m['key']: round(nodes[m['node_id']][m['channel']] * m['scale'], 6)
                      for m in mappings if m['node_id'] in nodes}
            frames.append(dict(id=identity, received_at=item['time'],
                               source=f'{path.name}:{line_number}', values=values))
    frames.sort(key=lambda f: f['received_at'])
    sources['resources.json'] = hashlib.sha256(resources_path.read_bytes()).hexdigest()
    sources['rk_protocol.py'] = hashlib.sha256(parser_path.read_bytes()).hexdigest()
    version = hashlib.sha256(json.dumps(dict(sources=sources, metrics=metrics), sort_keys=True).encode()).hexdigest()
    return dict(version=version, workspace_id=resources['workspace_id'], station_id=resources['station_id'],
                channel_id=resources['channel_id'], time_kind='receiver_time_not_sensor_sampling_time',
                metrics=metrics, frames=frames, sources=sources)


def main():
    cli = argparse.ArgumentParser()
    cli.add_argument('--root', type=pathlib.Path, required=True)
    cli.add_argument('--output', type=pathlib.Path, required=True)
    args = cli.parse_args()
    data = build(args.root.resolve())
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', dir=args.output.parent,
                                     prefix='.import-', suffix='.tmp', delete=False) as f:
        temp = pathlib.Path(f.name)
        json.dump(data, f, ensure_ascii=False, separators=(',', ':'))
    try:
        os.replace(temp, args.output)
    finally:
        temp.unlink(missing_ok=True)
    print(json.dumps(dict(frames=len(data['frames']), metrics=len(data['metrics']),
                          version=data['version'], output=str(args.output)), ensure_ascii=False))


if __name__ == '__main__':
    main()
