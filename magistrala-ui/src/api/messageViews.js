const defaults = [
  ['time', 'Time'],
  ['name', 'Metric'],
  ['value', 'Value'],
  ['unit', 'Unit'],
  ['publisher', 'Publisher'],
]
export function viewSettings(record = {}) {
  const legacy = record.config?._agri_ui || {}
  const s = record.format ? record : legacy
  const format = s.format || (record.content_type?.includes('senml') ? 'senml' : 'json')
  return JSON.parse(
    JSON.stringify({
      format,
      subtopic: s.subtopic || '',
      publishers: s.publishers || [],
      columns:
        format === 'json' && Array.isArray(s.columns)
          ? s.columns.map((c) => ({ ...c, enabled: c.enabled !== false }))
          : (format === 'senml'
              ? defaults
              : [
                  ['created', 'Sent at'],
                  ['publisher', 'Publisher'],
                  ['subtopic', 'Subtopic'],
                  ['protocol', 'Protocol'],
                ]
            ).map(([key, label]) => ({ key, label, enabled: true })),
    }),
  )
}
export function viewBody(record, { name, ...settings }) {
  if (!name.trim()) throw new Error('View name is required / 请填写视图名称')
  if (settings.format === 'json' && !settings.columns.some((c) => c.enabled))
    throw new Error('Select at least one column / 至少选择一列')
  if (
    settings.format === 'json' &&
    (settings.columns.filter((c) => c.enabled).some((c) => !c.key.trim()) ||
      new Set(settings.columns.filter((c) => c.enabled).map((c) => c.key.trim())).size !==
        settings.columns.filter((c) => c.enabled).length)
  )
    throw new Error('Column keys must be nonempty and unique / 列字段不能为空或重复')
  return {
    name: name.trim(),
    format: settings.format,
    publishers: settings.publishers,
    subtopic: settings.subtopic.trim(),
    columns:
      settings.format === 'json'
        ? settings.columns
            .filter((c) => c.enabled)
            .map((c) => ({ key: c.key.trim(), label: c.label.trim() || c.key.trim() }))
        : [],
    position: record.position || 0,
  }
}
export function readerViewParams(settings) {
  const q = new URLSearchParams()
  const topic = settings.subtopic.trim()
  if (settings.format === 'json') {
    // The local JSON writer uses the last subtopic segment as its format/table name.
    const format = topic.split('/').pop()
    if (!format || !/^[A-Za-z_][A-Za-z0-9_]*$/.test(format))
      throw new Error(
        'JSON view needs a subtopic ending with a valid format name / JSON 视图需要填写子主题，最后一段为字母、数字或下划线组成的格式名',
      )
    q.set('format', format)
  } else q.set('format', 'messages')
  if (topic) q.set('subtopic', topic)
  for (const publisher of settings.publishers) q.append('publishers', publisher)
  return q
}
export function viewCell(row, key) {
  if (Object.hasOwn(row, key)) return row[key]
  return key.split('.').reduce((v, k) => (v && Object.hasOwn(v, k) ? v[k] : undefined), row)
}
