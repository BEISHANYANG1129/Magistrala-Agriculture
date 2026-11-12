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
  if (chart.presentation === 'weather' || type === 'compass_chart') return weatherOptions(widget, rows, groups)
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

function weatherOptions(widget, rows, groups) {
  const c = widget.chart, latest = rows.find((r) => Number.isFinite(r.value))
  const ink = '#263c46', muted = '#81909a', line = '#eaf0ef'
  if (c.chartType === 'compass_chart') {
    return {
      animationDuration: 500,
      series: [{ type: 'gauge', min: 0, max: 360, startAngle: 90, endAngle: -270,
        center: ['50%', '48%'], radius: '76%', splitNumber: 8,
        axisLine: { lineStyle: { width: 12, color: [[1, '#edf4f2']] } },
        axisTick: { show: false }, splitLine: { length: 12, distance: -13, lineStyle: { color: '#b7cbc5', width: 1 } },
        axisLabel: { distance: 20, color: muted, fontSize: 12,
          formatter: (v) => ({ 0: '北 N', 90: '东 E', 180: '南 S', 270: '西 W' }[v] || '') },
        pointer: { show: !!latest, length: '63%', width: 5, itemStyle: { color: '#208b77' } },
        anchor: { show: true, size: 12, itemStyle: { color: '#208b77', borderColor: '#fff', borderWidth: 3 } },
        title: { show: false }, detail: { offsetCenter: [0, '74%'], fontSize: 23, fontWeight: 600, color: ink,
          formatter: () => latest ? `${latest.value}°` : '暂无近期数据' },
        data: [{ value: latest?.value ?? null }],
      }],
    }
  }
  const bound = (name) => Number.isFinite(c[name]) ? c[name] : undefined
  return {
    animationDuration: 450, color: groups.map((s) => s.color || '#208b77'),
    textStyle: { fontFamily: 'Microsoft YaHei, sans-serif', color: muted, fontSize: 11 },
    tooltip: { trigger: 'axis', renderMode: 'richText', backgroundColor: '#fff', borderColor: '#dce9e4',
      padding: [12, 16], textStyle: { color: ink }, axisPointer: { type: 'line', lineStyle: { color: '#b7cbc5', type: 'dashed' } } },
    legend: { show: groups.length > 1, bottom: 0, left: 14, icon: 'roundRect', itemWidth: 14, itemHeight: 3,
      textStyle: { color: muted, fontSize: 11 } },
    grid: { left: 12, right: 18, top: 30, bottom: groups.length > 1 ? 38 : 14, containLabel: true },
    xAxis: { type: 'time', axisLine: { lineStyle: { color: line } }, axisTick: { show: false },
      splitLine: { show: false }, axisLabel: { color: muted, hideOverlap: true, margin: 12,
        formatter: (v) => new Date(v).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) } },
    yAxis: { type: 'value', name: c.yLabel || '', nameTextStyle: { color: muted, padding: [0, 0, 0, 12] },
      min: bound('min'), max: bound('max'), scale: c.scale === true, minInterval: c.minInterval,
      axisLine: { show: false }, axisTick: { show: false },
      axisLabel: { color: muted, hideOverlap: true }, splitLine: { lineStyle: { color: line, type: 'dashed' } } },
    series: groups.map((s, i) => ({ name: s.name, type: 'line', data: weatherPoints(s.data, c.gapSeconds || 90),
      smooth: false, showSymbol: false, connectNulls: false, sampling: 'lttb',
      itemStyle: { color: s.color }, lineStyle: { color: s.color, width: i ? 2 : 2.5, type: i ? 'dashed' : 'solid' },
      areaStyle: c.chartType === 'area_chart' ? { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [{ offset: 0, color: s.color || '#208b77' }, { offset: 1, color: '#ffffff' }] }, opacity: 0.18 } : undefined,
      tooltip: { valueFormatter: (v) => v == null ? '数据中断' : `${v}${s.unit ? ' ' + s.unit : ''}` },
      markLine: i === 0 && Number.isFinite(c.referenceValue) ? { silent: true, symbol: 'none',
        lineStyle: { color: '#c19a5c', type: 'dashed', width: 1 },
        label: { formatter: c.referenceLabel || String(c.referenceValue), color: '#a1814c', position: 'insideEndTop', fontSize: 10 },
        data: [{ yAxis: c.referenceValue }] } : undefined,
    })),
  }
}

// A missing reporting interval must not become an invented continuous curve.
export function weatherPoints(points, gapSeconds = 90) {
  const result = []
  for (const point of points) {
    const previous = result.at(-1)
    if (previous && point[0] - previous[0] > gapSeconds * 1000)
      result.push([previous[0] + 1, null])
    result.push(point)
  }
  return result
}
