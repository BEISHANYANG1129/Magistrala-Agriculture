import { listAll, runOperation } from './platform'
import { editedMetadata } from './editedMetadata'
import { tr } from '../i18n'

const clone = (value) => JSON.parse(JSON.stringify(value))
// PostgreSQL JSONB may reorder object keys on read-back; key order is not a map edit.
export function sameMapValue(left, right) {
  const normalize = (value) =>
    Array.isArray(value)
      ? value.map(normalize)
      : value && typeof value === 'object'
        ? Object.fromEntries(
            Object.keys(value)
              .sort()
              .map((key) => [key, normalize(value[key])]),
          )
        : value
  return JSON.stringify(normalize(left)) === JSON.stringify(normalize(right))
}
export function groupMap(record) {
  const value = record?.attributes?.metadata?.field_map || {}
  return clone({ version: 1, background: '', partitions: [], positions: {}, ...value })
}
export function validPosition(point) {
  return (
    point &&
    ['x', 'y'].every((key) => Number.isFinite(point[key]) && point[key] >= 0 && point[key] <= 1)
  )
}
function assertGroup(record, workspace) {
  if (record.tenantId !== workspace || record.groupType !== 'object' || record.deletedAt)
    throw new Error(
      tr('分组不可用或不属于当前工作区。', 'Group is unavailable or outside this workspace.'),
    )
}
export function belongsToMap(device, workspace, groupId) {
  return (
    device.tenantId === workspace &&
    device.kind === 'device' &&
    !device.deletedAt &&
    device.objectGroupIds?.includes(groupId)
  )
}
export async function loadGroupMap(groupId, workspace) {
  const [record, devices] = await Promise.all([
    runOperation('group', { id: groupId }),
    listAll('entities', {
      tenantId: workspace,
      kind: 'device',
      deleted: 'live',
      parentGroupId: groupId,
    }),
  ])
  assertGroup(record, workspace)
  return { record, devices: devices.filter((device) => belongsToMap(device, workspace, groupId)) }
}
export async function saveGroupMap(original, workspace, patch, deviceId) {
  assertGroup(original, workspace)
  const latest = await runOperation('group', { id: original.id })
  assertGroup(latest, workspace)
  const map = groupMap(original)
  if (deviceId) {
    const device = await runOperation('entity', { id: deviceId })
    if (!belongsToMap(device, workspace, original.id))
      throw new Error(
        tr(
          '设备已不属于此分组，请刷新后重试。',
          'Device no longer belongs to this group. Refresh and retry.',
        ),
      )
    if (!validPosition(patch.position))
      throw new Error(tr('设备位置无效。', 'Invalid device position.'))
    map.positions = { ...map.positions, [deviceId]: patch.position }
  } else {
    if (patch.background && !/^https?:\/\//i.test(patch.background))
      throw new Error(
        tr(
          '底图地址必须以 http:// 或 https:// 开头。',
          'Background URL must start with http:// or https://.',
        ),
      )
    map.background = patch.background
    map.partitions = clone(patch.partitions)
  }
  const baseline = original.attributes?.metadata || {}
  const metadata = editedMetadata(
    baseline,
    { ...baseline, field_map: map },
    latest.attributes?.metadata || {},
  )
  return runOperation('updateGroup', {
    id: original.id,
    input: { attributes: { ...latest.attributes, metadata } },
  })
}
