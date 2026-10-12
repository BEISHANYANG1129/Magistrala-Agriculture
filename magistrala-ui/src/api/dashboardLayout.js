export const gridColumns = { lg: 12, md: 10, sm: 6, xs: 4, xxs: 2 }
export function normalizeLayout(layout = {}, widgets = []) {
  const ids = widgets.map((w) => w.id)
  const result = { ...layout }
  for (const [breakpoint, cols] of Object.entries(gridColumns)) {
    const existing = (layout[breakpoint] || []).filter((c) => ids.includes(c.i))
    result[breakpoint] = existing
      .map((c) => ({
        ...c,
        x: Math.max(0, Math.min(cols - 1, Math.round(c.x || 0))),
        y: Math.max(0, Math.round(c.y || 0)),
        w: Math.max(1, Math.min(cols, Math.round(c.w || Math.min(6, cols)))),
        h: Math.max(2, Math.round(c.h || 5)),
      }))
      .map((c) => ({ ...c, x: Math.min(c.x, cols - c.w) }))
    for (const id of ids)
      if (!existing.some((c) => c.i === id)) {
        const y = Math.max(0, ...result[breakpoint].map((c) => c.y + c.h))
        result[breakpoint].push({ i: id, x: 0, y, w: Math.min(6, cols), h: 6 })
      }
  }
  return result
}
export function moveGridCell(cells, id, patch, cols) {
  const result = cells.map((c) => ({ ...c })),
    target = result.find((c) => c.i === id)
  if (!target) return result
  Object.assign(target, patch)
  target.w = Math.max(1, Math.min(cols, Math.round(target.w)))
  target.h = Math.max(2, Math.round(target.h))
  target.x = Math.max(0, Math.min(cols - target.w, Math.round(target.x)))
  target.y = Math.max(0, Math.round(target.y))
  const overlap = (a, b) => a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y
  const placed = [target]
  for (const c of result.filter((c) => c !== target).sort((a, b) => a.y - b.y || a.x - b.x)) {
    let hits
    while ((hits = placed.filter((p) => overlap(c, p))).length)
      c.y = Math.max(...hits.map((p) => p.y + p.h))
    placed.push(c)
  }
  return result
}
export function dashboardImport(text) {
  const d = JSON.parse(text)
  if (!d || typeof d !== 'object' || !d.metadata?.widget || Array.isArray(d.metadata.widget))
    throw new Error('Invalid dashboard JSON / 仪表盘 JSON 无效')
  const widgets = Object.entries(d.metadata.widget).map(([id, w]) => {
    if (!w || typeof w !== 'object' || !w.chart || typeof w.chart !== 'object')
      throw new Error('Invalid widget / 组件配置无效')
    return { ...w, id: w.id || id }
  })
  if (new Set(widgets.map((w) => w.id)).size !== widgets.length)
    throw new Error('Duplicate widget IDs / 组件 ID 重复')
  return {
    name: d.name || '',
    description: d.description || '',
    tags: d.tags || [],
    metadata: { ...d.metadata, widget: Object.fromEntries(widgets.map((w) => [w.id, w])) },
    layout: normalizeLayout(d.layout, widgets),
  }
}
export function downloadDashboard(dashboard) {
  const url = URL.createObjectURL(
      new Blob([JSON.stringify(dashboard, null, 2)], { type: 'application/json' }),
    ),
    a = document.createElement('a')
  a.href = url
  a.download = (dashboard.name || 'dashboard') + '.json'
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
