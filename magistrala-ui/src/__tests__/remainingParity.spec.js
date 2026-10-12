import { describe, it, expect, vi } from 'vitest'
import {
  widgetType,
  setWidgetType,
  validateWidget,
  locationOf,
  perimeterOf,
} from '../api/widgetConfig'
import { loadWidgetData } from '../api/widgetData'
import {
  encodeRuleFlow,
  decodeRuleFlow,
  removeFlowNode,
  changeOutputFlowType,
  writeRuleFlow,
} from '../api/ruleFlow'

describe('local Magistrala widget contracts', () => {
  it('reads native card and control discriminators and preserves extension settings', () => {
    expect(widgetType({ type: 'card', chart: { cardType: 'building_overview' } })).toBe(
      'building_overview',
    )
    expect(widgetType({ type: 'control', chart: { type: 'slider' } })).toBe('slider_control')
    const w = { chart: { chartType: 'line_chart', custom: { keep: true } }, title: 'x' }
    setWidgetType(w, 'map')
    expect(w.type).toBe('card')
    expect(w.chart.cardType).toBe('map')
    const count = { chart: { dataSource: {} } }
    setWidgetType(count, 'count_card')
    expect(count.chart.dataSource.entity).toBe('device')
    expect(w.chart.chartType).toBeUndefined()
    expect(w.chart.custom).toEqual({ keep: true })
  })
  it('reads wrapped resource locations, validates perimeter coordinates and kilometers', () => {
    const r = {
      attributes: {
        metadata: {
          location: { type: 'location', value: { latitude: 0, longitude: 0 } },
          perimeter: { type: 'perimeter', value: { type: 'circle', center: [0, 0], radius: 1.5 } },
        },
      },
    }
    expect(locationOf(r)).toEqual([0, 0])
    expect(perimeterOf(r).radius).toBe(1.5)
    r.attributes.metadata.location.value.latitude = null
    expect(locationOf(r)).toBeNull()
    r.attributes.metadata.perimeter.value.radius = -1
    expect(perimeterOf(r)).toBeNull()
  })
  it('loads polygon parent and child entities from resource metadata, without Reader queries', async () => {
    const request = vi.fn(),
      readResource = vi.fn(async (section, id) =>
        id === 'g'
          ? {
              name: 'Field',
              metadata: {
                perimeter: {
                  value: {
                    type: 'polygon',
                    coordinates: [
                      [1, 2],
                      [2, 3],
                      [3, 4],
                    ],
                  },
                },
              },
            }
          : {
              id,
              name: 'Sensor',
              metadata: { location: { value: { latitude: 1, longitude: 2 } } },
            },
      )
    const d = await loadWidgetData(
      {
        chart: {
          cardType: 'polygon_map',
          dataSources: [
            {
              entityType: 'Group',
              entity: { id: 'g' },
              childEntityType: 'Client',
              childEntities: [{ id: 'd' }],
            },
          ],
        },
      },
      'w',
      { request, readResource },
    )
    expect(d.shapes).toHaveLength(1)
    expect(d.rows[0].publisher).toBe('d')
    expect(request).not.toHaveBeenCalled()
    expect(readResource).toHaveBeenCalledWith('devices', 'd', 'w')
  })
  it('resource tables apply state and tag filters instead of displaying telemetry', async () => {
    const listAll = vi.fn(async () => [
      { id: 'a', status: 'active', attributes: { tags: ['farm'] } },
      { id: 'b', status: 'disabled', attributes: { tags: ['farm'] } },
    ])
    const d = await loadWidgetData(
      {
        chart: {
          cardType: 'table_card',
          dataSource: { entity: 'Client', status: 'enabled', tags: 'farm' },
        },
      },
      'w',
      { listAll },
    )
    expect(d.value).toBe(1)
    expect(d.rows[0].id).toBe('a')
  })
  it('route queries retain the configured metric and history ignores stale realtime duration', async () => {
    const request = vi.fn(async () => ({ messages: [] }))
    await loadWidgetData(
      {
        chart: {
          chartType: 'route_map',
          valueName: 'position',
          timeWindowFormat: 'history',
          last: 1000,
          startTime: '2026-10-10T00:00:00Z',
          stopTime: '2026-10-11T00:00:00Z',
          dataSources: [{ channel: { id: 'c' }, client: { id: 'd' } }],
        },
      },
      'w',
      { request },
    )
    const q = new URL(request.mock.calls[0][0], 'http://local').searchParams
    expect(q.get('name')).toBe('position')
    expect(q.get('from')).toBe(String(Date.parse('2026-10-10T00:00:00Z') * 1e6))
    expect(q.get('publisher')).toBe('d')
  })
  it('normalizes building references and rejects duplicate device IDs across floors', () => {
    const w = {
      title: 'B',
      chart: {
        cardType: 'building_overview',
        buildings: [
          {
            id: 'b',
            name: 'B',
            floors: [
              { name: 'F1', devices: [{ id: 'd' }] },
              { name: 'F2', devices: [] },
            ],
            gateways: [],
          },
        ],
      },
    }
    validateWidget(w)
    expect(w.chart.buildings[0].floors[0].devices[0].buildingId).toBe('b')
    w.chart.buildings[0].floors[1].devices.push({ id: 'd' })
    expect(() => validateWidget(w)).toThrow('Device IDs')
  })
})
describe('rule output identities', () => {
  const draft = () => ({
    outputs: [
      { id: 'one', type: 'save_senml' },
      { id: 'two', type: 'save_senml' },
    ],
    metadata: {
      flow: encodeRuleFlow({
        nodes: [
          { id: 'a', type: 'senMl', position: { x: 1, y: 2 } },
          { id: 'b', type: 'senMl', position: { x: 3, y: 4 } },
        ],
        edges: [],
      }),
    },
  })
  it('deleting the first of two equal output types retains the second node identity and position', () => {
    const d = draft()
    removeFlowNode(d, 'output-one')
    d.outputs.shift()
    const f = decodeRuleFlow(writeRuleFlow(d))
    expect(f.nodes).toEqual([{ id: 'b', type: 'senMl', position: { x: 3, y: 4 } }])
  })
  it('changing output kind retains existing node id and layout', () => {
    const d = draft()
    changeOutputFlowType(d, 0, 'save_json')
    d.outputs[0].type = 'save_json'
    expect(decodeRuleFlow(writeRuleFlow(d)).nodes[0]).toEqual({
      id: 'a',
      type: 'json',
      position: { x: 1, y: 2 },
    })
  })
})
