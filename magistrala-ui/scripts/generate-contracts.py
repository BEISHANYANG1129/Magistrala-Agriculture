"""Generate native form contracts from the deployed Atom SDL and Magistrala OpenAPI.

No credentials or application records are included. Run with explicit source paths.
"""
import argparse
import json
import pathlib
import re
import yaml

parser = argparse.ArgumentParser()
parser.add_argument('--schema', required=True)
parser.add_argument('--openapi', required=True)
args = parser.parse_args()
source = pathlib.Path(args.schema).read_text(encoding='utf-8')
source = re.sub(r'""".*?"""', '', source, flags=re.S)
types = {}
for kind, name, body in re.findall(r'\b(type|input|enum)\s+(\w+)\s*\{(.*?)\n\}', source, re.S):
    fields = []
    for line in body.splitlines():
        match = re.match(r'\s*(\w+)(?:\((.*?)\))?\s*:\s*([\w\[\]!]+)', line)
        if match:
            field, arguments, typ = match.groups()
            fields.append({'name': field, 'type': typ, 'args': [
                {'name': n, 'type': t} for n, t in re.findall(r'(\w+)\s*:\s*([\w\[\]!]+)', arguments or '')]})
    types[name] = {'kind': kind, 'fields': fields}
    if kind == 'enum':
        types[name]['values'] = re.findall(r'^\s*(\w+)\s*$', body, re.M)

def resolve(doc, obj, depth=0):
    if not isinstance(obj, (dict, list)):
        return obj
    if depth > 40:
        return {'type': 'object'}
    if isinstance(obj, list):
        return [resolve(doc, x, depth+1) for x in obj]
    if not isinstance(obj, dict):
        return obj
    if '$ref' in obj:
        ref = obj['$ref']
        if not ref.startswith('#/'):
            return {'type': 'object'}
        value = doc
        for segment in ref[2:].split('/'):
            value = value[segment]
        return resolve(doc, value, depth+1)
    return {k: resolve(doc, v, depth+1) for k, v in obj.items()}

rest = []
for service in ['rules', 'alarms', 'reports', 'bootstrap', 'readers', 'http', 'certs', 'notifiers']:
    doc = yaml.safe_load((pathlib.Path(args.openapi)/(service+'.yaml')).read_text(encoding='utf-8'))
    for path, methods in doc.get('paths', {}).items():
        for method, op in methods.items():
            if method not in ['get', 'post', 'put', 'patch', 'delete']:
                continue
            content = resolve(doc, op.get('requestBody', {})).get('content', {})
            mime = next(iter(content), '')
            rest.append({'id': service + ':' + op.get('operationId', method+path), 'service': service,
                         'name': op.get('operationId', method+path), 'summary': op.get('summary', ''),
                         'method': method.upper(), 'path': path,
                         'parameters': resolve(doc, methods.get('parameters', [])+op.get('parameters', [])),
                         'body': content.get(mime, {}).get('schema'), 'mime': mime,
                         'security': op.get('security', doc.get('security', []))})

out = pathlib.Path(__file__).resolve().parents[1]/'src/api/contracts.json'
out.write_text(json.dumps({'atomRevision': 'ec16f6bc668ed528f26bd3d0ae5c4de297e99600', 'types': types, 'rest': rest}, ensure_ascii=False, indent=2), encoding='utf-8')
print(f'{len(types)} GraphQL types; {len(rest)} HTTP operations -> {out}')
