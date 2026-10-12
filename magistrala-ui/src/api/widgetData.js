import { alarmQuery } from './alarmFilters'
import { widgetType, entitySection, locationOf, perimeterOf } from './widgetConfig'
export async function loadWidgetData(
  widget,
  workspace,
  { request, listAll, readResource },
  now = Date.now(),
) {
  const c = widget.chart || {},
    type = widgetType(widget),
    rows = [],
    shapes = []
  if ((type === 'building_overview' && !c.map?.image) || type.endsWith('_control'))
    return { rows, shapes }
  if (type === 'map' || type === 'polygon_map') {
    for (const s of c.dataSources || []) {
      const entity = type === 'map' ? s.entityDetails || s.entity : s.entity
      if (!entity?.id) continue
      const record = await readResource(entitySection(s.entityType), entity.id, workspace)
      const point = locationOf(record, c),
        shape = perimeterOf(record, c)
      if (point)
        rows.push({
          latitude: point[0],
          longitude: point[1],
          name: s.label || record.name,
          publisher: record.id,
          color: s.color,
          icon: s.icon,
        })
      if (type === 'polygon_map' && shape)
        shapes.push({ ...shape, name: s.label || record.name, color: s.color })
      for (const child of s.childEntities || []) {
        const r = await readResource(entitySection(s.childEntityType), child.id, workspace),
          p = locationOf(r, c)
        if (p)
          rows.push({
            latitude: p[0],
            longitude: p[1],
            name: r.name,
            publisher: r.id,
            color: s.color,
            icon: s.icon,
          })
      }
    }
    // Compatibility for earlier telemetry-based agricultural map widgets.
    if (!(c.dataSources || []).some((s) => s.channel?.id)) return { rows, shapes }
  }
  if (['count_card', 'table_card'].includes(type)) {
    const s = c.dataSource || {},
      section = entitySection(s.entity),
      op = section === 'channels' ? 'resources' : section === 'groups' ? 'groups' : 'entities'
    const records = await listAll(op, {
      tenantId: workspace,
      ...(section === 'devices'
        ? { kind: 'device' }
        : section === 'channels'
          ? { kind: 'channel' }
          : {}),
    })
    const tags = Array.isArray(s.tags)
      ? s.tags
      : String(s.tags || '')
          .split(',')
          .map((x) => x.trim())
          .filter(Boolean)
    const filtered = records.filter((r) => {
      const status = r.status || r.attributes?.status || 'active'
      return (
        !r.deletedAt &&
        (!s.status ||
          s.status === 'all' ||
          status === (s.status === 'enabled' ? 'active' : s.status) ||
          status === s.status) &&
        tags.every((t) => (r.attributes?.tags || r.tags || []).includes(t))
      )
    })
    return { rows: filtered, shapes, value: filtered.length }
  }
  if (type.startsWith('alarm_')) {
    const q = alarmQuery(c)
    q.set('limit', '100')
    q.set('offset', '0')
    const d = await request(`/api/platform/alarms/${workspace}/alarms?${q}`)
    return { rows: d.alarms || [], shapes, value: d.total }
  }
  for (const [i, s] of (c.dataSources || []).entries()) {
    if (!s.channel?.id) continue
    const q = new URLSearchParams({
      limit: String(Math.min(c.limit || 50, 1000)),
      offset: '0',
      order: 'time',
      dir: 'desc',
    })
    if (s.client?.id) q.set('publisher', s.client.id)
    if (s.valueName || c.valueName) q.set('name', s.valueName || c.valueName)
    const live = c.timeWindowFormat ? c.timeWindowFormat === 'realtime' : c.isLive
    if (!live && c.startTime) q.set('from', String(Date.parse(c.startTime) * 1e6))
    if (!live && c.stopTime) q.set('to', String(Date.parse(c.stopTime) * 1e6))
    if (live && c.last) q.set('from', String((now - Number(c.last)) * 1e6))
    if (c.dataType?.type === 'aggregation') {
      q.set('aggregation', c.dataType.method)
      q.set('interval', c.dataType.interval)
    }
    const d = await request(
      `/api/platform/readers/${workspace}/channels/${encodeURIComponent(s.channel.id)}/messages?${q}`,
    )
    rows.push(
      ...(d.messages || []).map((r) => ({
        ...r,
        _source: i,
        _label: s.label,
        _color: s.color,
        _unit: s.unit,
        _icon: s.icon,
      })),
    )
  }
  const time = (r) => (Number(r.time) > 1e16 ? Number(r.time) / 1e6 : Number(r.time) * 1000)
  rows.sort((a, b) => time(b) - time(a))
  return {
    rows,
    shapes,
    value: rows[0]?.value ?? rows[0]?.bool_value ?? rows[0]?.string_value ?? '—',
  }
}
