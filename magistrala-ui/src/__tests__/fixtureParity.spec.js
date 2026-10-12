import { describe, it, expect, vi } from 'vitest'
import {
  alarmCause,
  alarmAcknowledged,
  alarmTimestamp,
  completeAlarmColumns,
} from '../api/alarmPresentation'
import { loadWidgetData } from '../api/widgetData'
import { setWidgetType, validateWidget } from '../api/widgetConfig'

describe('real fixture parity regressions', () => {
  it('reads native cause and preserves acknowledgement after API reload', () => {
    expect(alarmCause({ cause: 'dry soil', message: 'legacy' })).toBe('dry soil')
    expect(alarmAcknowledged({ acknowledged_at: '2026-10-11T00:00:00Z' })).toBe(true)
    expect(alarmAcknowledged({ acknowledged_at: '0001-01-01T00:00:00Z' })).toBe(false)
    expect(
      alarmAcknowledged({ acknowledged: false, acknowledged_at: '2026-10-11T00:00:00Z' }),
    ).toBe(false)
    expect(alarmTimestamp('0001-01-01T00:00:00Z')).toBe('—')
  })
  it('retains custom column order and labels while exposing omitted native fields', () => {
    const columns = [{ key: 'message', label: 'Custom cause', visible: true, order: 0 }]
    const result = completeAlarmColumns(columns)
    expect(result[0]).toEqual(columns[0])
    expect(result.find((c) => c.key === 'acknowledged_at').visible).toBe(false)
    expect(new Set(result.map((c) => c.key)).size).toBe(result.length)
  })
  it('loads official location sources and migrates the earlier agricultural shape', async () => {
    const readResource = vi.fn(async () => ({
      id: 'g',
      metadata: { location: { value: { latitude: 1, longitude: 2 } } },
    }))
    const chart = {
      cardType: 'map',
      dataSources: [{ entityType: 'group', entityDetails: { id: 'g', name: 'Field' } }],
    }
    const result = await loadWidgetData({ chart }, 'w', { readResource })
    expect(readResource).toHaveBeenCalledWith('groups', 'g', 'w')
    expect(result.rows[0]).toMatchObject({ latitude: 1, longitude: 2 })
    const widget = { chart: { dataSources: [{ entity: { id: 'legacy' } }] } }
    setWidgetType(widget, 'map')
    expect(widget.chart.dataSources[0].entityDetails.id).toBe('legacy')
    expect(widget.chart.dataSources[0].entity).toBeUndefined()
  })
  it('keeps the selected route metric to exclude unrelated messages', async () => {
    const request = vi.fn(async () => ({ messages: [] }))
    await loadWidgetData(
      {
        chart: {
          chartType: 'route_map',
          dataSources: [{ channel: { id: 'c' }, valueName: 'position' }],
        },
      },
      'w',
      { request },
    )
    const url = new URL(request.mock.calls[0][0], 'http://localhost')
    expect(url.searchParams.get('name')).toBe('position')
  })
  it('rejects alarm styles that crash the deployed official renderer', () => {
    const widget = {
      title: 'Alarm',
      chart: { cardType: 'alarm_count', color: '#19806c', icon: 'bell' },
    }
    expect(() => validateWidget(widget)).toThrow(/alarm color/)
    widget.chart.color = 'green'
    expect(() => validateWidget(widget)).not.toThrow()
  })
})
