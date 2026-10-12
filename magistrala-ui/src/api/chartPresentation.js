export function unitText(unit) {
  return typeof unit === 'object' && unit
    ? String(unit.value || unit.label || '')
    : String(unit || '')
}
export function messageTime(value) {
  if (typeof value === 'string' && !Number.isFinite(Number(value))) return Date.parse(value)
  const n = Number(value)
  return !Number.isFinite(n) ? NaN : n > 1e16 ? n / 1e6 : n > 1e11 ? n : n * 1000
}
function key(row) {
  return row._source !== undefined
    ? String(row._source)
    : `${row.name || ''}|${row.publisher || ''}`
}
export function telemetrySeries(rows) {
  const series = new Map()
  for (const row of rows) {
    const time = messageTime(row.time)
    if (typeof row.value !== 'number' || !Number.isFinite(row.value) || !Number.isFinite(time))
      continue
    const id = key(row)
    if (!series.has(id))
      series.set(id, {
        id,
        name: row._label || `${row.name || ''} ${row.publisher || ''}`.trim(),
        color: row._color,
        unit: unitText(row._unit || row.unit),
        points: new Map(),
      })
    // Reader is newest first; keep its first sample when timestamps collide.
    const group = series.get(id)
    if (!group.points.has(time)) group.points.set(time, row.value)
  }
  return [...series.values()].map(({ points, ...s }) => ({
    ...s,
    data: [...points].sort((a, b) => a[0] - b[0]),
  }))
}
export function chartOptions(widget, rows, size = {}) {
  const chart = widget.chart || {},
    type = chart.chartType,
    groups = telemetrySeries(rows)
  const base = {
    tooltip: { trigger: 'axis', renderMode: 'richText' },
    legend: { type: 'scroll', bottom: 0 },
    grid: { left: 60, right: 25, top: 35, bottom: 70, containLabel: true },
  }
  if (type === 'pie_chart')
    return {
      ...base,
      tooltip: { trigger: 'item', renderMode: 'richText' },
      series: [
        {
          type: 'pie',
          radius: '65%',
          data: groups.map((s) => ({
            name: s.name,
            value: s.data.at(-1)[1],
            itemStyle: { color: s.color },
          })),
        },
      ],
    }
  if (type === 'gauge_chart') {
    const source = chart.dataSources?.[0] || {},
      latest = [...rows].find((r) => typeof r.value === 'number' && Number.isFinite(r.value)),
      unit = unitText(source.unit || latest?.unit)
    const temperature = chart.gaugeType === 'Temperature Gauge',
      speed = chart.gaugeType === 'Speed Gauge',
      color = source.color || '#020780',
      compact = Math.min(size.width || Infinity, size.height || Infinity) < 300
    return {
      series: [
        {
          type: 'gauge',
          min: source.minValue ?? chart.min ?? 0,
          max: source.maxValue ?? chart.max ?? 100,
          center: ['50%', temperature ? '63%' : '55%'],
          radius: temperature ? '70%' : speed ? '95%' : '100%',
          startAngle: temperature ? 200 : 225,
          endAngle: temperature ? -20 : -45,
          data: [{ value: latest?.value ?? null, name: source.label || widget.title }],
          title: { show: false },
          splitNumber: compact ? 5 : 10,
          detail: {
            formatter: (v) => (latest ? `${v}${unit ? ' ' + unit : ''}` : '—'),
            valueAnimation: true,
            fontSize: compact ? 22 : temperature ? 40 : speed ? 30 : 22,
            offsetCenter: [0, temperature ? '5%' : '40%'],
            color,
          },
          progress: { show: !speed, width: temperature ? 30 : 10 },
          pointer: { show: !temperature, itemStyle: { color } },
          axisLine: { lineStyle: { width: temperature || speed ? 30 : 10 } },
          axisTick: {
            distance: temperature ? -45 : speed ? -30 : -5,
            length: 8,
            lineStyle: { color, width: 2 },
          },
          splitLine: {
            distance: temperature ? -52 : speed ? -30 : -5,
            length: speed ? 30 : 14,
            lineStyle: { color, width: 3 },
          },
          axisLabel: {
            distance: temperature ? -15 : speed ? (compact ? 25 : 40) : 15,
            fontSize: compact ? 12 : speed ? 20 : 15,
            color,
          },
          itemStyle: { color },
        },
      ],
    }
  }
  return {
    ...base,
    xAxis: { type: 'time', name: chart.xLabel || '', nameLocation: 'middle', nameGap: 35 },
    yAxis: { type: 'value', name: chart.yLabel || '', nameLocation: 'middle', nameGap: 45 },
    series: groups.map((s) => ({
      name: s.name,
      type: type === 'bar_chart' ? 'bar' : 'line',
      data: s.data,
      itemStyle: { color: s.color },
      lineStyle: { color: s.color, width: Number(chart.width) || 1 },
      areaStyle: type === 'area_chart' ? {} : undefined,
      smooth: type !== 'bar_chart',
      showSymbol: false,
      connectNulls: true,
      tooltip: { valueFormatter: (value) => `${value}${s.unit ? ' ' + s.unit : ''}` },
    })),
  }
}
