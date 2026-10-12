export function alarmCause(record) {
  return record.cause ?? record.message ?? record.description ?? ''
}
export function alarmAcknowledged(record) {
  return typeof record.acknowledged === 'boolean'
    ? record.acknowledged
    : Boolean(record.acknowledged_at && !record.acknowledged_at.startsWith('0001-'))
}
export const alarmColumnKeys = [
  'id',
  'status',
  'severity',
  'cause',
  'measurement',
  'value',
  'unit',
  'threshold',
  'rule_id',
  'channel_id',
  'device_id',
  'subtopic',
  'assignee_id',
  'created_at',
  'updated_at',
  'updated_by',
  'assigned_at',
  'assigned_by',
  'acknowledged_at',
  'acknowledged_by',
  'resolved_at',
  'resolved_by',
]
export function completeAlarmColumns(columns) {
  if (!columns)
    return alarmColumnKeys.map((key, order) => ({
      key,
      label: key,
      order,
      visible: ['status', 'severity', 'cause', 'created_at'].includes(key),
    }))
  const known = new Set(columns.map((c) => c.key))
  return [
    ...columns,
    ...alarmColumnKeys
      .filter((key) => !known.has(key))
      .map((key, i) => ({ key, label: key, visible: false, order: columns.length + i })),
  ]
}

export function alarmTimestamp(value) {
  return value && !String(value).startsWith('0001-') ? value : '—'
}

export const alarmColors = ['red', 'orange', 'yellow', 'green', 'blue', 'purple', 'gray']
export const alarmIcons = [
  'bell',
  'alert-triangle',
  'check-circle',
  'clock',
  'user',
  'shield',
  'zap',
  'activity',
]
