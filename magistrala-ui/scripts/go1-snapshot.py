"""Capture cam3 once and publish through the registered Magistrala device.

Requires requests, websocket-client, asyncssh and Pillow (already in local Python).
Secrets are read from GO1_DEVICE_KEY and GO1_SSH_PASSWORD, never saved or logged.
AGRI_PLATFORM_TOKEN is optional and enables Reader/hash verification.
Uses the existing FFmpeg on the Go1 side board for one-frame decoding.
"""
import argparse
import asyncio
import base64
import hashlib
import io
import json
import os
from pathlib import Path
import struct
import time
import uuid

import asyncssh
import requests
import websocket
from PIL import Image


async def capture(known_hosts, password):
    ws = websocket.create_connection(
        'ws://192.168.12.1/cam3', timeout=4, origin='http://192.168.12.1',
        http_no_proxy=['192.168.12.1'],
    )
    chunks, count = [], 0
    started = last_ack = time.monotonic()
    try:
        ws.send_binary(struct.pack('<i', 0))
        ws.settimeout(0.5)
        while time.monotonic() - started < 8 and count < 30:
            if time.monotonic() - last_ack >= 0.5:
                ws.send_binary(struct.pack('<i', count))
                last_ack = time.monotonic()
            try:
                data = ws.recv()
            except websocket.WebSocketTimeoutException:
                continue
            if not data:
                raise RuntimeError('Camera stream closed before capture completed')
            if isinstance(data, bytes):
                chunks.append(data)
                count += 1
    finally:
        ws.close()
    if not chunks:
        raise RuntimeError('No fresh camera data received')
    # The decoded first frame belongs to this newly opened capture interval.
    async with asyncssh.connect(
        '192.168.12.1', username='pi', password=password,
        known_hosts=known_hosts, client_keys=[], login_timeout=8,
    ) as pi:
        async with asyncssh.connect(
            '192.168.123.14', username='unitree', password=password, tunnel=pi,
            known_hosts=known_hosts, client_keys=[], login_timeout=8,
        ) as nano:
            result = await nano.run(
                'ffmpeg -v error -f h264 -i pipe:0 -frames:v 1 -f image2pipe -vcodec png pipe:1',
                input=b''.join(chunks), encoding=None, timeout=15,
            )
            if result.exit_status or not result.stdout:
                raise RuntimeError('Camera frame decoding failed')
    image = Image.open(io.BytesIO(result.stdout)).convert('RGB')
    buffer = io.BytesIO()
    image.save(buffer, 'JPEG', quality=85)
    return buffer.getvalue(), image.size


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', required=True, type=Path)
    parser.add_argument('--known-hosts', required=True, type=Path)
    parser.add_argument('--output-dir', required=True, type=Path)
    parser.add_argument('--platform', default='http://127.0.0.1:5173')
    args = parser.parse_args()
    key, password = os.environ.get('GO1_DEVICE_KEY'), os.environ.get('GO1_SSH_PASSWORD')
    if not key or not password:
        parser.error('Set GO1_DEVICE_KEY and GO1_SSH_PASSWORD in the process environment')
    config = json.loads(args.config.read_text(encoding='utf-8-sig'))
    captured_at = time.time()
    jpeg, size = asyncio.run(capture(str(args.known_hosts.resolve()), password))
    sha = hashlib.sha256(jpeg).hexdigest()
    capture_id = str(uuid.uuid4())
    records = [
        {'n': 'go1/cam3/image/jpeg', 't': captured_at, 'vd': base64.b64encode(jpeg).decode()},
        {'n': 'go1/cam3/capture_id', 't': captured_at, 'vs': capture_id},
        {'n': 'go1/cam3/sha256', 't': captured_at, 'vs': sha},
    ]
    payload = json.dumps(records).encode()
    if len(payload) > 900000:
        raise RuntimeError('Image message exceeds the pilot size limit')
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / (capture_id + '.jpg')).write_bytes(jpeg)
    workspace, channel, device = (config[k] for k in ['workspace_id', 'channel_id', 'device_id'])
    session = requests.Session()
    session.trust_env = False
    # Existing test5 rule matches the channel root, not a camera subtopic.
    response = session.post(
        args.platform.rstrip('/') + f'/api/platform/http/http/m/{workspace}/c/{channel}',
        data=payload, headers={
            'Authorization': 'Basic ' + base64.b64encode((device + ':' + key).encode()).decode(),
            'Content-Type': 'application/senml+json',
        }, timeout=25,
    )
    response.raise_for_status()
    result = {'capture_id': capture_id, 'captured_at_unix': captured_at, 'dimensions': size,
              'jpeg_bytes': len(jpeg), 'sha256': sha, 'publish_http_status': response.status_code,
              'device_id': device, 'channel_id': channel, 'reader_verified': False}
    token = os.environ.get('AGRI_PLATFORM_TOKEN')
    if token:
        for _ in range(15):
            read = session.get(
                args.platform.rstrip('/') + f'/api/platform/readers/{workspace}/channels/{channel}/messages',
                params={'publisher': device, 'name': 'go1/cam3/image/jpeg', 'limit': 20, 'order': 'time', 'dir': 'desc'},
                headers={'Authorization': 'Bearer ' + token}, timeout=20,
            )
            read.raise_for_status()
            for row in read.json().get('messages', []):
                data = row.get('data_value')
                if data and hashlib.sha256(base64.b64decode(data)).hexdigest() == sha:
                    result['reader_verified'] = True
                    result['reader_time'] = row['time']
                    break
            if result['reader_verified']:
                break
            time.sleep(1)
    (args.output_dir / (capture_id + '.json')).write_text(json.dumps(result, indent=2), encoding='utf8')
    print(json.dumps(result))
    if token and not result['reader_verified']:
        raise RuntimeError('Published, but Reader verification failed; inspect the existing rule/writer')


if __name__ == '__main__':
    main()
