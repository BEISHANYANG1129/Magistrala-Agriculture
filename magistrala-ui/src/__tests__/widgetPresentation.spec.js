import { describe, it, expect, vi } from 'vitest'
import { chartOptions, telemetrySeries, unitText } from '../api/chartPresentation'
import { loadWidgetData } from '../api/widgetData'
import { validateWidget } from '../api/widgetConfig'
import { mapPoints } from '../api/mapPoints'
import { mapMarker } from '../api/mapMarker'
describe('dashboard display compatibility', () => {
  it('renders configured Lucide marker icons and treats unknown names as plain data', () => {
    const pin = mapMarker('map-pin', '#123456')
    const tractor = mapMarker('tractor', '#123456')
    expect(tractor.querySelector('svg')).not.toBeNull()
    expect(tractor.innerHTML).not.toBe(pin.innerHTML)
    expect(mapMarker('<img src=x onerror=alert(1)>').querySelector('img')).toBeNull()
    expect(mapMarker('unknown-icon').innerHTML).toBe(mapMarker('map-pin').innerHTML)
  })
  const rows = [
    {
      time: 1791600001,
      name: 'soil',
      publisher: 'north',
      value: 30,
      _source: 0,
      _label: 'North',
      _color: '#123456',
      _unit: { label: 'Percent', value: '%' },
    },
    { time: 1791600001, name: 'soil', publisher: 'south', value: 40, _source: 1, _label: 'South' },
    {
      time: 1791600000,
      name: 'soil',
      publisher: 'north',
      value: 20,
      _source: 0,
      _label: 'North',
      _color: '#123456',
    },
  ]
  it('uses one chronological time series per source without misaligning simultaneous samples', () => {
    const s = telemetrySeries(rows)
    expect(s).toHaveLength(2)
    expect(s[0].data).toEqual([
      [1791600000000, 20],
      [1791600001000, 30],
    ])
    expect(s[1].data).toEqual([[1791600001000, 40]])
    const option = chartOptions(
      { chart: { chartType: 'line_chart', width: '2.5', xLabel: 'Time', yLabel: 'Soil' } },
      rows,
    )
    expect(option.xAxis.type).toBe('time')
    expect(option.yAxis.name).toBe('Soil')
    expect(option.series[0].lineStyle).toEqual({ color: '#123456', width: 2.5 })
    expect(option.series[0].tooltip.valueFormatter(30)).toBe('30 %')
  })
  it('pie uses latest value for each series rather than one slice per historical sample', () => {
    const option = chartOptions({ chart: { chartType: 'pie_chart' } }, rows)
    expect(option.series[0].data.map((d) => [d.name, d.value])).toEqual([
      ['North', 30],
      ['South', 40],
    ])
  })
  it('renders native unit objects and keeps a no-data gauge distinct from zero', () => {
    expect(unitText({ label: 'Celsius', value: '°C' })).toBe('°C')
    const widget = {
      title: 'Temperature',
      chart: {
        chartType: 'gauge_chart',
        gaugeType: 'Temperature Gauge',
        dataSources: [{ unit: { value: '°C' } }],
      },
    }
    const empty = chartOptions(widget, []).series[0]
    expect(empty.detail.formatter(0)).toBe('—')
    expect(empty.data[0].value).toBeNull()
    expect(chartOptions(widget, rows).series[0].detail.formatter(30)).toBe('30 °C')
  })
  it('honors history even if imported isLive is stale, and rejects invalid dates', async () => {
    const request = vi.fn(async () => ({ messages: [] }))
    const widget = {
      title: 'History',
      chart: {
        chartType: 'line_chart',
        width: 3,
        timeWindowFormat: 'history',
        isLive: true,
        last: 1000,
        startTime: '2026-10-10T00:00:00Z',
        stopTime: '2026-10-11T00:00:00Z',
        dataSources: [{ channel: { id: 'c' } }],
      },
    }
    await loadWidgetData(widget, 'w', { request })
    const q = new URL(request.mock.calls[0][0], 'http://local').searchParams
    expect(q.get('from')).toBe(String(Date.parse(widget.chart.startTime) * 1e6))
    validateWidget(widget)
    expect(widget.chart.last).toBeUndefined()
    expect(widget.chart.isLive).toBe(false)
    expect(widget.chart.width).toBe('3.0')
    expect(widget.chart.startTime).toBe('2026-10-10T00:00:00.000Z')
    widget.chart.startTime = 'invalid'
    expect(() => validateWidget(widget)).toThrow('historical')
  })
  it('preserves resource and route source styling when producing map points', () => {
    const p = mapPoints([
      { latitude: 1, longitude: 2, color: '#123456', name: 'Field' },
      { publisher: 'd', time: 1, name: 'latitude', value: 3, _color: '#abcdef', _label: 'Device' },
      { publisher: 'd', time: 1, name: 'longitude', value: 4, _color: '#abcdef', _label: 'Device' },
    ])
    expect(p[0].color).toBe('#123456')
    expect(p[1].label).toBe('Device')
    expect(p[1].color).toBe('#abcdef')
  })
  it('keeps zero-valued alarm filters and native actor references in the query', async () => {
    const request = vi.fn(async () => ({ alarms: [], total: 0 }))
    await loadWidgetData(
      {
        chart: { chartType: 'alarm_table', severity: 0, status: 'all', assignee: { id: 'user-1' } },
      },
      'w',
      { request },
    )
    const q = new URL(request.mock.calls[0][0], 'http://local').searchParams
    expect(q.get('severity')).toBe('0')
    expect(q.get('assignee_id')).toBe('user-1')
    expect(q.has('status')).toBe(false)
  })
})
