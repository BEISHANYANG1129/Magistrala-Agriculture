export const alarmReferences = [
  ['channel', 'channel_id', '通道', 'Channel'],
  ['client', 'device_id', '设备', 'Device'],
  ['rule', 'rule_id', '规则', 'Rule'],
  ['assignee', 'assignee_id', '负责人', 'Assignee'],
  ['updatedBy', 'updated_by', '更新人', 'Updated by'],
  ['assignedBy', 'assigned_by', '指派人', 'Assigned by'],
  ['acknowledgedBy', 'acknowledged_by', '确认人', 'Acknowledged by'],
  ['resolvedBy', 'resolved_by', '解决人', 'Resolved by'],
]
export function alarmQuery(filters = {}) {
  const q = new URLSearchParams()
  for (const [field, key] of alarmReferences) if (filters[field]?.id) q.set(key, filters[field].id)
  for (const key of ['status', 'severity', 'subtopic', 'order', 'dir'])
    if (
      filters[key] !== undefined &&
      filters[key] !== null &&
      filters[key] !== '' &&
      filters[key] !== 'all'
    )
      q.set(key, String(filters[key]))
  for (const [field, key] of [
    ['createdFrom', 'created_from'],
    ['createdTo', 'created_to'],
  ])
    if (filters[field]) {
      const date = new Date(filters[field])
      if (!Number.isFinite(date.getTime())) throw new Error('Invalid alarm date')
      q.set(key, date.toISOString())
    }
  if (
    q.has('severity') &&
    (!Number.isInteger(Number(q.get('severity'))) ||
      Number(q.get('severity')) < 0 ||
      Number(q.get('severity')) > 100)
  )
    throw new Error('Severity must be an integer from 0 to 100')
  if (
    filters.createdFrom &&
    filters.createdTo &&
    new Date(filters.createdFrom) > new Date(filters.createdTo)
  )
    throw new Error('Created from must not be after created to')
  return q
}
