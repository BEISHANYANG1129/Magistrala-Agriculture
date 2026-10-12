import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import BuildingOverview from '../components/BuildingOverview.vue'
import AlarmFilters from '../components/AlarmFilters.vue'
import { alarmQuery } from '../api/alarmFilters'
import { chartOptions } from '../api/chartPresentation'
vi.mock('../api/platform', () => ({
  request: vi.fn(async () => ({ rules: [{ id: 'rule1', name: 'Irrigation' }], total: 1 })),
  listAll: vi.fn(async () => [{ id: 'member1', name: 'Operator' }]),
  errorText: (e) => e.message,
}))
describe('remaining parity audit', () => {
  it('filters alarms with zero severity, actors, dates and server sorting', () => {
    const q = alarmQuery({
      severity: 0,
      status: 'all',
      rule: { id: 'r' },
      updatedBy: { id: 'u' },
      createdFrom: '2026-10-11T00:00:00Z',
      order: 'created_at',
      dir: 'asc',
    })
    expect(Object.fromEntries(q)).toEqual({
      severity: '0',
      rule_id: 'r',
      updated_by: 'u',
      created_from: '2026-10-11T00:00:00.000Z',
      order: 'created_at',
      dir: 'asc',
    })
    expect(() => alarmQuery({ severity: -1 })).toThrow()
    expect(() => alarmQuery({ createdFrom: '2026-10-12', createdTo: '2026-10-11' })).toThrow()
  })
  it('retains unavailable saved references and lets users choose named resources', async () => {
    const filters = { rule: { id: 'old', name: 'Archived rule' } }
    const view = mount(AlarmFilters, {
      props: { filters, workspace: 'w', devices: [], channels: [] },
    })
    await flushPromises()
    expect(view.text()).toContain('Archived rule')
    expect(view.text()).toContain('Irrigation')
    const rules = view.findAll('select').find((s) => s.text().includes('Irrigation'))
    await rules.setValue('rule1')
    expect(filters.rule).toEqual({ id: 'rule1', name: 'Irrigation' })
  })
  it('protects occupied floors, updates references on rename and filters read devices', async () => {
    const buildings = [
      {
        id: 'b',
        name: 'North',
        floors: [
          { name: 'Floor 1', devices: [{ id: 'd', read: true, floor: 'Floor 1' }] },
          { name: 'Floor 2', devices: [] },
        ],
        gateways: [{ id: 'g', floor: 'Floor 2' }],
      },
    ]
    const view = mount(BuildingOverview, { props: { buildings, editable: true } })
    expect(
      view
        .findAll('button')
        .filter((b) => /Delete empty floor|删除空楼层/.test(b.text()))
        .every((b) => b.element.disabled),
    ).toBe(true)
    await view
      .find('input[aria-label="Floor name"], input[aria-label="楼层名称"]')
      .setValue('Ground')
    expect(buildings[0].floors[0].devices[0].floor).toBe('Ground')
    await view.find('input[type=checkbox]').setValue(true)
    expect(view.find('.device').isVisible()).toBe(false)
    const down = view.find('button[aria-label="Move floor down"], button[aria-label="下移楼层"]')
    await down.trigger('click')
    expect(buildings[0].floors.map((f) => f.name)).toEqual(['Floor 2', 'Ground'])
    expect(buildings[0].gateways[0].floor).toBe('Floor 2')
  })
  it('gives speed and temperature gauges different scales, pointers and arcs', () => {
    const gauge = (gaugeType) =>
      chartOptions(
        {
          chart: {
            chartType: 'gauge_chart',
            gaugeType,
            dataSources: [{ minValue: -10, maxValue: 80, unit: { value: '°C' } }],
          },
        },
        [{ value: 0 }],
      ).series[0]
    const simple = gauge('Simple Gauge'),
      speed = gauge('Speed Gauge'),
      temperature = gauge('Temperature Gauge')
    expect(simple.progress.show).toBe(true)
    expect(speed.progress.show).toBe(false)
    expect(speed.axisLabel.distance).not.toBe(simple.axisLabel.distance)
    expect(temperature.pointer.show).toBe(false)
    expect(temperature.startAngle).toBe(200)
    expect(speed.min).toBe(-10)
    expect(speed.max).toBe(80)
    expect(speed.detail.formatter(0)).toBe('0 °C')
  })
  it('keeps compact gauge labels separate from the reading', () => {
    const series = chartOptions(
      { title: 'Gauge', chart: { chartType: 'gauge_chart', gaugeType: 'Speed Gauge' } },
      [{ value: 12 }],
      { width: 400, height: 220 },
    ).series[0]
    expect(series.title.show).toBe(false)
    expect(series.splitNumber).toBe(5)
    expect(series.axisLabel.fontSize).toBe(12)
    expect(series.data[0].value).toBe(12)
  })
  it('shows device details and updates local read status without an API write', async () => {
    const buildings = [
      {
        id: 'b',
        name: 'Building',
        floors: [{ name: 'Ground', devices: [{ id: 'device', read: false }] }],
        gateways: [],
      },
    ]
    const view = mount(BuildingOverview, { props: { buildings } })
    await view
      .findAll('button')
      .find((b) => b.text() === 'device')
      .trigger('click')
    expect(view.text()).toContain('Ground')
    await view
      .findAll('button')
      .find((b) => /Toggle status|切换读取状态/.test(b.text()))
      .trigger('click')
    expect(buildings[0].floors[0].devices[0].read).toBe(true)
    expect(view.text()).toContain('1 / 1')
  })
})
