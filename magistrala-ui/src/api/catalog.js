import { contracts, operationSchema, operation, runOperation, request } from './platform'

export const pages = {
  info: { list: 'tenant', single: true, edit: 'updateTenant', scope: 'id' },
  devices: {
    list: 'entities',
    create: 'createEntity',
    edit: 'updateEntity',
    detail: 'entity',
    defaults: { kind: 'device' },
  },
  gateways: {
    list: 'entities',
    create: 'createEntity',
    edit: 'updateEntity',
    detail: 'entity',
    defaults: { kind: 'device', attributesContains: { is_gateway: true } },
    createDefaults: { attributes: { is_gateway: true } },
  },
  channels: {
    list: 'resources',
    create: 'createResource',
    edit: 'updateResource',
    detail: 'resource',
    defaults: { kind: 'channel' },
  },
  groups: { list: 'groups', create: 'createGroup', edit: 'updateGroup', detail: 'group' },
  users: { list: 'tenantMembers', create: 'addTenantMember', detail: 'entity' },
  'device-types': {
    list: 'profiles',
    create: 'createProfile',
    edit: 'updateProfile',
    detail: 'profile',
    defaults: { objectKind: 'entity', kind: 'device' },
  },
  'role-management': { list: 'roles', create: 'createRole', edit: 'updateRole', detail: 'role' },
  invitations: { list: 'tenantInvitations', create: 'createTenantInvitation' },
  account: { list: 'accessTokens', create: 'createAccessToken', unscoped: true },
  platform: { list: 'auditLogs' },
  dashboards: { rest: true, service: 'ui', collection: 'dashboards', key: 'dashboards' },
  'custom-nodes': { rest: true, service: 'ui', collection: 'custom-nodes', key: 'custom_nodes' },
  rules: { rest: true, service: 'rules', collection: 'rules', key: 'rules' },
  alarms: { rest: true, service: 'alarms', collection: 'alarms', key: 'alarms' },
  reports: { rest: true, service: 'reports', collection: 'reports/configs', key: 'report_configs' },
  bootstraps: { rest: true, service: 'bootstrap', collection: 'devices/configs', key: 'configs' },
}

const groupMatches = {
  info: /Tenant$|TenantMember|TenantRoles/,
  devices: /Entity|Entities|Ownership|SharedKey|Credential|Certificate/,
  gateways: /Entity|Entities|Ownership|SharedKey|Credential/,
  channels: /Resource|DirectPolic|PermissionBlock|RoleAssignment|Authz/i,
  groups: /Group|ObjectGroup|PrincipalGroup/,
  users: /TenantMember|TenantAssignable|Entity|Password/,
  'device-types': /Profile/,
  'role-management': /Role|Permission|Action|Polic|Authz/i,
  invitations: /Invitation/,
  account: /AccessToken|Credential|Password|Session|Token|Invitation|^entity$|^updateEntity$/,
  platform: /.*/,
}

const string = { type: 'string' },
  object = { type: 'json' }
const uiDefinitions = [
  [
    'dashboards',
    'dashboards',
    {
      name: string,
      description: string,
      metadata: object,
      layout: object,
      tags: { type: 'array', items: string },
    },
  ],
  [
    'custom-nodes',
    'custom_nodes',
    {
      name: string,
      description: string,
      logic: {
        type: 'object',
        properties: { type: { type: 'integer' }, value: string, mode: string },
      },
      metadata: object,
    },
  ],
  [
    'channels/{channelID}/message-views',
    'message_views',
    {
      name: string,
      format: string,
      subtopic: string,
      publishers: { type: 'array', items: string },
      columns: { type: 'array', items: object },
      position: { type: 'number' },
    },
  ],
]
const uiOperations = []
for (const [path, key, properties] of uiDefinitions) {
  for (const [method, suffix, verb] of [
    ['GET', '', 'list'],
    ['POST', '', 'create'],
    ['GET', '/{id}', 'get'],
    [path.includes('message-views') ? 'PUT' : 'PATCH', '/{id}', 'update'],
    ['DELETE', '/{id}', 'delete'],
  ]) {
    if (path.includes('message-views') && method === 'GET' && suffix) continue
    uiOperations.push({
      id: `ui:${verb}:${path}`,
      name: `${verb} ${key}`,
      service: 'ui',
      path: `/{workspaceID}/${path}${suffix}`,
      method,
      parameters: [
        { name: 'workspaceID', in: 'path', required: true, schema: string },
        ...(path.includes('{channelID}')
          ? [{ name: 'channelID', in: 'path', required: true, schema: string }]
          : []),
        ...(suffix ? [{ name: 'id', in: 'path', required: true, schema: string }] : []),
        ...(method === 'GET' && !suffix
          ? [
              { name: 'limit', in: 'query', schema: { type: 'integer', minimum: 1, maximum: 100 } },
              { name: 'offset', in: 'query', schema: { type: 'integer', minimum: 0 } },
            ]
          : []),
      ],
      mime: 'application/json',
      body: ['POST', 'PATCH', 'PUT'].includes(method)
        ? { type: 'object', properties, required: ['name'] }
        : null,
    })
  }
}
for (const [method, suffix] of [
  ['GET', 'share'],
  ['POST', 'share'],
  ['PATCH', 'share'],
  ['DELETE', 'unshare'],
])
  uiOperations.push({
    id: `ui:${method}:${suffix}`,
    name: `${method === 'GET' ? 'get' : suffix} dashboard`,
    service: 'ui',
    path: `/{workspaceID}/dashboards/{dashboardID}/${suffix}`,
    method,
    mime: 'application/json',
    parameters: ['workspaceID', 'dashboardID'].map((name) => ({
      name,
      in: 'path',
      required: true,
      schema: string,
    })),
    body:
      method === 'GET'
        ? null
        : {
            type: 'object',
            properties: { user_ids: { type: 'array', items: string }, public: { type: 'boolean' } },
          },
  })
export const restOperations = [...contracts.rest, ...uiOperations]

export function opsFor(section) {
  if (pages[section]?.rest)
    return restOperations
      .filter(
        (o) => o.service === pages[section].service && !['/health', '/metrics'].includes(o.path),
      )
      .map((o) => ({ ...o, kind: 'rest' }))
  const regex = groupMatches[section] || /a^/
  const graph = ['query', 'mutation'].flatMap((kind) =>
    contracts.types[kind === 'query' ? 'QueryRoot' : 'MutationRoot'].fields
      .filter((o) => regex.test(o.name))
      .map((o) => ({ ...o, id: o.name, kind })),
  )
  return section === 'platform'
    ? [
        ...graph,
        ...restOperations
          .filter((o) => !['http', 'certs'].includes(o.service))
          .map((o) => ({ ...o, kind: 'rest' })),
      ]
    : graph
}
export function formSchema(op) {
  if (op.kind !== 'rest') return operationSchema(op.name)
  return {
    type: 'object',
    properties: {
      ...Object.fromEntries(
        op.parameters.map((p) => [p.name, { ...p.schema, required: p.required }]),
      ),
      ...(op.body ? { body: op.body } : {}),
    },
  }
}
export async function execute(op, values) {
  if (op.kind !== 'rest') return runOperation(op.name, values)
  let path = op.path
  const params = new URLSearchParams()
  for (const p of op.parameters) {
    const v = values[p.name]
    if (p.in === 'path') {
      if (v === undefined || v === '') throw new Error(`${p.name} required`)
      path = path.replace(`{${p.name}}`, encodeURIComponent(v))
    }
    if (p.in === 'query' && v !== undefined && v !== '')
      params.set(p.name, Array.isArray(v) ? v.join(',') : String(v))
  }
  let body = values.body
  const headers = {}
  if (op.mime === 'multipart/form-data') {
    const f = new FormData()
    for (const [key, v] of Object.entries(body || {})) {
      if (v !== undefined)
        f.append(key, v instanceof File ? v : typeof v === 'object' ? JSON.stringify(v) : String(v))
    }
    body = f
  } else if (op.mime && body !== undefined) headers['Content-Type'] = op.mime
  if (op.service === 'http') {
    headers.Authorization = `Basic ${btoa(`${values.deviceId}:${values.deviceKey}`)}`
  }
  return request(`/api/platform/${op.service}${path}${params.size ? '?' + params : ''}`, {
    method: op.method,
    body,
    headers,
    download: values.action === 'download',
  })
}
export function defaultsFor(op, workspace, record = {}, section = '') {
  const schema = formSchema(op)
  const values = {}
  for (const key of Object.keys(schema.properties)) {
    if (['tenantId', 'workspaceID', 'workspaceId', 'workspace_id'].includes(key))
      values[key] = workspace
    else if (key === 'id') values[key] = record.id || (section === 'info' ? workspace : undefined)
    else if (key === 'limit') values[key] = 20
    else if (key === 'offset') values[key] = 0
    else if (key === 'input' || key === 'body') {
      const props = schema.properties[key].properties || {}
      values[key] = Object.fromEntries(
        Object.keys(props)
          .filter((k) => record[k] !== undefined)
          .map((k) => [k, record[k]]),
      )
      if ('tenantId' in props) values[key].tenantId = workspace
      for (const [k, v] of Object.entries({
        ...pages[section]?.defaults,
        ...pages[section]?.createDefaults,
      }))
        if (k in props && !record.id) values[key][k] = v
    } else if (/ID$|Id$/.test(key) && record.id) values[key] = record.id
  }
  return values
}
export function mainRest(section, method, detail = false) {
  const p = pages[section]
  return restOperations.find(
    (o) =>
      o.service === p.service &&
      o.method === method &&
      o.path ===
        `/{workspaceID}/${p.collection}${detail ? (section === 'dashboards' || section === 'custom-nodes' ? '/{id}' : section === 'rules' ? '/{ruleID}' : section === 'alarms' ? '/{alarmID}' : section === 'reports' ? '/{reportID}' : '/{configID}') : ''}`,
  )
}
