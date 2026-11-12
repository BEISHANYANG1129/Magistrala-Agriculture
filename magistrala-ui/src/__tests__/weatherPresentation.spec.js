import { describe, it, expect } from 'vitest'
import { chartOptions, weatherPoints } from '../api/chartPresentation'

describe('weather telemetry presentation', () => {
  it('preserves zero readings and breaks the line at a reporting gap', () => {
    expect(weatherPoints([[100000, 0], [120000, 0], [300000, 2]])).toEqual([
      [100000, 0], [120000, 0], [120001, null], [300000, 2],
    ])
  })
  it('shows temperature and dew point as separate series with exact samples', () => {
    const rows = [
      { _source: 0, _label: '气温', _color: '#aa7755', time: 1793018000, value: 29 },
      { _source: 1, _label: '露点', _color: '#33aa99', time: 1793018000, value: 21 },
    ]
    const options = chartOptions({ chart: { presentation: 'weather', chartType: 'line_chart', scale: true } }, rows)
    expect(options.series).toHaveLength(2)
    expect(options.series[0].data[0][1]).toBe(29)
    expect(options.series[1].data[0][1]).toBe(21)
    expect(options.series.every((s) => !s.smooth && !s.connectNulls)).toBe(true)
    expect(options.yAxis.scale).toBe(true)
  })
  it('keeps a manufacturer range reference distinct from a measured CO2 series', () => {
    const options = chartOptions({ chart: { presentation: 'weather', chartType: 'line_chart', referenceValue: 400, referenceLabel: '有效量程下限' } },
      [{ time: 1793018000, value: 352 }])
    expect(options.series[0].data[0][1]).toBe(352)
    expect(options.series[0].markLine.data[0].yAxis).toBe(400)
    expect(options.series[0].markLine.label.formatter).toBe('有效量程下限')
  })
  it('puts north at zero clockwise, and never renders missing direction as zero', () => {
    const widget = { chart: { presentation: 'weather', chartType: 'compass_chart' } }
    const empty = chartOptions(widget, []).series[0]
    expect(empty.pointer.show).toBe(false)
    expect(empty.data[0].value).toBeNull()
    const north = chartOptions(widget, [{ time: 1793018000, value: 0 }]).series[0]
    expect(north.pointer.show).toBe(true)
    expect(north.startAngle).toBe(90)
    expect(north.endAngle).toBe(-270)
    expect(north.detail.formatter()).toBe('0°')
  })
})
