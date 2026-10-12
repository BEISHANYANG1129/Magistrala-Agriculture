import { runOperation, listAll, gql } from './platform'

export const resourceConfig = {
  devices: { list: 'entities', get: 'entity', type: 'Entity', kind: 'device' },
  gateways: { list: 'entities', get: 'entity', type: 'Entity', kind: 'device' },
  channels: { list: 'resources', get: 'resource', type: 'Resource', kind: 'channel' },
  groups: { list: 'groups', get: 'group', type: 'Group' },
}
export const statusOf = (r) =>
  r.deletedAt ? 'deleted' : r.status || r.attributes?.status || 'active'
export const tagsOf = (r) => r.attributes?.tags || []
export async function readResource(section, id, workspace) {
  const value = await runOperation(resourceConfig[section].get, { id })
  if (value.tenantId !== workspace) throw new Error('Resource is outside the current workspace.')
  return value
}
export async function readResources(section, workspace, extra = {}) {
  const c = resourceConfig[section]
  return listAll(c.list, {
    tenantId: workspace,
    ...(c.kind ? { kind: c.kind } : {}),
    ...(section === 'gateways' ? { attributesContains: { is_gateway: true } } : {}),
    ...extra,
  })
}
export async function deviceKey(id) {
  const credentials = await runOperation('credentials', { entityId: id })
  const key = credentials.items.find(
    (c) =>
      c.kind === 'shared_key' &&
      !['revoked', 'expired', 'disabled'].includes(c.status) &&
      (!c.expiresAt || new Date(c.expiresAt) > new Date()),
  )
  if (!key) throw new Error('No active device key is available.')
  return (await runOperation('revealSharedKey', { entityId: id, credentialId: key.id })).key
}
export async function setResourceStatus(section, record, enabled, workspace) {
  const latest = await readResource(section, record.id, workspace)
  if (section === 'channels')
    return runOperation('updateResource', {
      id: record.id,
      input: { attributes: { ...latest.attributes, status: enabled ? 'active' : 'disabled' } },
    })
  return runOperation(`${enabled ? 'enable' : 'disable'}${resourceConfig[section].type}`, {
    id: record.id,
  })
}
export async function channelPolicies(workspace, { channel, device } = {}) {
  const items = []
  for (let offset = 0; offset < 10000; offset += 100) {
    const data = await gql(
      `query($workspace:ID!${channel ? ',$channel:ID' : ''},$device:ID,$offset:Int){directPolicies(tenantId:$workspace,${channel ? 'objectKind:"resource",objectType:"resource:channel",objectId:$channel,' : ''}subjectKind:entity,subjectId:$device,limit:100,offset:$offset){total items{id subjectId managedBy permissionBlockId permissionBlock{objectId objectKind objectType effect actions{id name}}}}}`,
      { workspace, channel, device, offset },
    )
    items.push(
      ...data.directPolicies.items.filter(
        (p) =>
          p.permissionBlock.objectKind === 'resource' &&
          p.permissionBlock.objectType === 'resource:channel',
      ),
    )
    if (
      offset + data.directPolicies.items.length >= data.directPolicies.total ||
      !data.directPolicies.items.length
    )
      return items
  }
  throw new Error('Too many connections to display.')
}
export async function connectDevice(workspace, device, channel, names) {
  const actions = await listAll('actions', {
    tenantId: workspace,
    objectKind: 'resource',
    objectType: 'resource:channel',
  })
  const actionIds = names.map((n) => actions.find((a) => a.name === n)?.id)
  if (!actionIds.length || actionIds.some((id) => !id))
    throw new Error('Publish/subscribe actions are unavailable.')
  const block = await runOperation('createPermissionBlock', {
    input: {
      tenantId: workspace,
      scopeMode: 'object',
      objectKind: 'resource',
      objectType: 'resource:channel',
      objectId: channel,
      effect: 'allow',
      actionIds,
    },
  })
  try {
    return await runOperation('createDirectPolicy', {
      input: {
        tenantId: workspace,
        subjectKind: 'entity',
        subjectId: device,
        permissionBlockId: block.id,
      },
    })
  } catch (e) {
    // Only remove the unassigned block created by this invocation.
    try {
      await runOperation('deletePermissionBlock', { id: block.id })
    } catch {
      e.message += ` (unassigned block: ${block.id})`
    }
    throw e
  }
}
