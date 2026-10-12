import { alarmColors, alarmIcons } from './alarmPresentation'
// Wire formats verified against the locally deployed Magistrala dashboard.
export const cardTypes = [
  'count_card',
  'table_card',
  'map',
  'polygon_map',
  'alarm_count',
  'alarm_table',
  'building_overview',
]
export function widgetType(widget) {
  const c = widget?.chart || {}
  return (
    c.cardType ||
    c.chartType ||
    (c.type === 'switch' ? 'switch_control' : c.type === 'slider' ? 'slider_control' : '')
  )
}
export function setWidgetType(widget, type) {
  const c = (widget.chart ||= {})
  delete c.chartType
  delete c.cardType
  delete c.type
  if (cardTypes.includes(type)) {
    widget.type = 'card'
    c.cardType = type
  } else if (type.endsWith('_control')) {
    widget.type = 'control'
    c.type = type.replace('_control', '')
  } else {
    widget.type = 'chart'
    c.chartType = type
  }
  c.dataSources ||= []
  for (const source of c.dataSources) {
    source.channel ||= { id: '' }
    source.client ||= { id: '' }
    if (type === 'map') {
      source.entityDetails ||= source.entity || { id: '' }
      delete source.entity
    } else if (type === 'polygon_map') source.entity ||= source.entityDetails || { id: '' }
  }
  c.dataSource ||= {
    channel: { id: '' },
    client: { id: '' },
    label: 'command',
    entity: 'device',
    status: 'enabled',
  }
  if (['count_card', 'table_card'].includes(type)) {
    c.dataSource.entity ||= 'device'
    c.dataSource.status ??= 'enabled'
  }
  if (type === 'building_overview') c.buildings ||= []
  return widget
}
export function metadataValue(value) {
  return value && typeof value === 'object' && 'value' in value ? value.value : value
}
export function atPath(value, key) {
  return String(key)
    .split('.')
    .reduce((o, k) => metadataValue(o)?.[k], value)
}
export function entitySection(type = '') {
  const t = type.toLowerCase()
  return t.includes('channel') ? 'channels' : t === 'group' ? 'groups' : 'devices'
}
export function entityMetadata(record) {
  return record.attributes?.metadata || record.metadata || record.attributes || {}
}
export function locationOf(record, chart = {}) {
  const location = metadataValue(atPath(entityMetadata(record), chart.locationKey || 'location'))
  const lat = metadataValue(atPath(location, chart.latitudeKey || 'latitude'))
  const lng = metadataValue(atPath(location, chart.longitudeKey || 'longitude'))
  if (
    lat === undefined ||
    lng === undefined ||
    lat === null ||
    lng === null ||
    lat === '' ||
    lng === ''
  )
    return null
  return Number.isFinite(Number(lat)) &&
    Number.isFinite(Number(lng)) &&
    Math.abs(Number(lat)) <= 90 &&
    Math.abs(Number(lng)) <= 180
    ? [Number(lat), Number(lng)]
    : null
}
export function perimeterOf(record, chart = {}) {
  const p = metadataValue(atPath(entityMetadata(record), chart.polygonKey || 'perimeter'))
  if (!p) return null
  const valid = (v) =>
    Array.isArray(v) &&
    v.length === 2 &&
    v.every(Number.isFinite) &&
    Math.abs(v[0]) <= 90 &&
    Math.abs(v[1]) <= 180
  if (p.type === 'polygon' && p.coordinates?.length >= 3 && p.coordinates.every(valid)) return p
  if (p.type === 'rectangle' && p.bounds?.length === 2 && p.bounds.every(valid)) return p
  if (p.type === 'circle' && valid(p.center) && Number.isFinite(p.radius) && p.radius > 0) return p
  return null
}
export function validateWidget(widget) {
  const c = widget.chart,
    type = widgetType(widget)
  if (!c || !type) throw new Error('请选择组件类型 / Select a widget type')
  if (!widget.title?.trim()) throw new Error('请输入标题 / Enter a title')
  if (
    type === 'alarm_count' &&
    ((c.color && !alarmColors.includes(c.color)) || (c.icon && !alarmIcons.includes(c.icon)))
  )
    throw new Error('请选择支持的告警颜色和图标 / Select a supported alarm color and icon')
  if (type === 'building_overview') {
    const devices = new Set(),
      gateways = new Set(),
      ids = new Set()
    for (const b of c.buildings || []) {
      if (!b.name?.trim() || !b.id || ids.has(b.id))
        throw new Error('建筑名称和唯一编号必填 / Buildings require names and unique IDs')
      ids.add(b.id)
      const names = new Set()
      for (const f of b.floors || []) {
        if (!f.name?.trim() || names.has(f.name))
          throw new Error('楼层名称必填且不能重复 / Floor names must be unique')
        names.add(f.name)
        for (const d of f.devices || []) {
          if (!d.id?.trim() || devices.has(d.id))
            throw new Error('设备编号必填且不能重复 / Device IDs must be unique')
          devices.add(d.id)
          d.floor = f.name
          d.buildingId = b.id
        }
      }
      for (const g of b.gateways || []) {
        if (!g.id?.trim() || gateways.has(g.id) || !names.has(g.floor))
          throw new Error('网关编号或楼层无效 / Invalid gateway ID or floor')
        gateways.add(g.id)
        g.buildingId = b.id
      }
    }
  }
  if (
    c.timeWindowFormat === 'history' &&
    (!Number.isFinite(Date.parse(c.startTime)) ||
      !Number.isFinite(Date.parse(c.stopTime)) ||
      Date.parse(c.startTime) >= Date.parse(c.stopTime))
  )
    throw new Error('历史时间范围无效 / Invalid historical time range')
  if (c.dataType?.type === 'aggregation' && !c.dataType.interval)
    throw new Error('请填写聚合间隔 / Enter an aggregation interval')
  for (const date of [c.createdFrom, c.createdTo])
    if (date && !Number.isFinite(Date.parse(date)))
      throw new Error('告警日期无效 / Invalid alarm date')
  if (c.createdFrom && c.createdTo && Date.parse(c.createdFrom) >= Date.parse(c.createdTo))
    throw new Error('告警时间范围无效 / Invalid alarm date range')
  if (type.endsWith('_control') && (!c.dataSource?.channel?.id || !c.dataSource?.client?.id))
    throw new Error('控制组件需要通道和设备 / Controls require a channel and device')
  if (
    type === 'slider_control' &&
    !(Number(c.dataSource.maxValue ?? 100) > Number(c.dataSource.minValue ?? 0))
  )
    throw new Error('最大值必须大于最小值 / Maximum must exceed minimum')
  // The deployed official Reader prioritizes `last` even in history mode.
  // Persist only the active window so both interfaces request the same data.
  if (c.timeWindowFormat === 'history') {
    delete c.last
    c.isLive = false
    c.startTime = new Date(c.startTime).toISOString()
    c.stopTime = new Date(c.stopTime).toISOString()
  } else if (c.timeWindowFormat === 'realtime') {
    delete c.startTime
    delete c.stopTime
    c.isLive = true
  }
  if (c.width !== undefined && Number.isFinite(Number(c.width)))
    c.width = Number(c.width).toFixed(1)
  return widget
}
