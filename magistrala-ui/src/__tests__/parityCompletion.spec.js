vi.mock('../components/ModalDialog.vue', () => ({ default: { template: '<div class="modal"><slot /></div>' } }))
import { comparisonScript, parseComparison } from '../api/comparisonLogic'
import { beforeEach, describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { decodeRuleFlow, encodeRuleFlow, ruleCanvasState, writeRuleFlow } from '../api/ruleFlow'
import { normalizeLayout, moveGridCell, dashboardImport } from '../api/dashboardLayout'
import { mapPoints } from '../api/mapPoints'
import { viewBody, viewSettings } from '../api/messageViews'
import { replaceDeviceKey } from '../api/deviceCredentials'
import { request, runOperation, listAll } from '../api/platform'
import DeviceTypeBinding from '../components/DeviceTypeBinding.vue'
import DashboardEditor from '../components/DashboardEditor.vue'
import SimpleNativeEditor from '../components/SimpleNativeEditor.vue'
vi.mock('../api/platform', () => ({
  runOperation: vi.fn(),
  request: vi.fn(),
  listAll: vi.fn(),
  errorText: (e) => e.message,
}))
const button = (w, text) => w.findAll('button').find((b) => b.text() === text)
beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
  vi.resetAllMocks()
  listAll.mockResolvedValue([])
  runOperation.mockResolvedValue({})
  request.mockResolvedValue({})
})
describe('official rule canvas interoperability', () => {
  const flow = {
    outputMetadata: { inputChannel: '农田北区', other: 1 },
    nodes: [
      { id: 'native-input', type: 'input', position: { x: 24, y: 45 } },
      { id: 'native-logic', type: 'editor', position: { x: 300, y: 80 } },
      { id: 'native-output', type: 'senMl', position: { x: 680, y: 90 } },
      { id: 'future', type: 'new-type', position: { x: 10, y: 400 }, data: { keep: true } },
    ],
    edges: [
      { id: 'a', source: 'native-input', target: 'native-logic' },
      { id: 'b', source: 'native-logic', target: 'native-output' },
      { id: 'foreign', source: 'native-output', target: 'future' },
    ],
  }
  const draft = () => ({
    input_channel: 'c',
    logic: { type: 0, value: 'return message.payload', mode: 'unrestricted' },
    outputs: [{ id: 'out', type: 'save_senml' }],
    metadata: { owner: 'original', flow: encodeRuleFlow(flow) },
  })
  it('round trips Unicode, native node IDs, positions and unknown nodes without altering executable fields', () => {
    const d = draft(),
      before = JSON.stringify(d)
    const state = ruleCanvasState(d)
    expect(state.positions.input).toEqual({ x: 24, y: 45 })
    state.positions.logic = { x: 420, y: 100 }
    const result = writeRuleFlow(d, state.positions, state.edges, [{ id: 'c', name: '农田北区' }]),
      decoded = decodeRuleFlow(result)
    expect(decoded.nodes.find((n) => n.id === 'native-logic').position).toEqual({ x: 420, y: 100 })
    expect(decoded.nodes.find((n) => n.id === 'future')).toEqual(flow.nodes[3])
    expect(decoded.edges).toEqual(flow.edges)
    expect(decoded.outputMetadata).toEqual({ ...flow.outputMetadata, outputChannel: '' })
    expect(result.owner).toBe('original')
    expect(JSON.stringify(d)).toBe(before)
  })
  it('preserves a deliberately disconnected canvas and removes only orphan edges', () => {
    const d = draft()
    d.metadata = writeRuleFlow(d, undefined, [])
    expect(ruleCanvasState(d).edges).toEqual([])
    d.outputs = []
    const f = decodeRuleFlow(writeRuleFlow(d))
    expect(f.nodes.some((n) => n.id === 'native-output')).toBe(false)
    expect(f.edges).toEqual([])
  })
  it('blocks malformed native flow instead of overwriting it', () =>
    expect(() => writeRuleFlow({ metadata: { flow: 'broken' }, outputs: [] })).toThrow(
      'cannot be decoded',
    ))
})
describe('dashboard layout and telemetry', () => {
  it('preserves all breakpoint positions and pushes overlapping cells without changing widths', () => {
    const original = {
        lg: [
          { i: 'a', x: 0, y: 0, w: 6, h: 4 },
          { i: 'b', x: 6, y: 0, w: 6, h: 4 },
        ],
        md: [{ i: 'a', x: 1, y: 3, w: 4, h: 8 }],
      },
      copy = JSON.stringify(original)
    const layout = normalizeLayout(original, [{ id: 'a' }, { id: 'b' }])
    expect(layout.md[0]).toEqual(original.md[0])
    layout.lg = moveGridCell(layout.lg, 'a', { x: 6, y: 0 }, 12)
    expect(layout.lg.find((c) => c.i === 'b').y).toBe(4)
    expect(layout.md[0]).toEqual(original.md[0])
    expect(JSON.stringify(original)).toBe(copy)
  })
  it('imports native widget IDs without importing shared access or an existing dashboard ID', () => {
    const result = dashboardImport(
      JSON.stringify({
        id: 'existing',
        share_state: 'public',
        metadata: {
          widget: { a: { chart: { chartType: 'line_chart' }, title: 'A' } },
          note: 'keep',
        },
        layout: { lg: [{ i: 'a', x: 3, y: 4, w: 5, h: 5 }] },
      }),
    )
    expect(result.id).toBeUndefined()
    expect(result.share_state).toBeUndefined()
    expect(result.metadata.note).toBe('keep')
    expect(result.metadata.widget.a.id).toBe('a')
    expect(result.layout.lg[0].x).toBe(3)
  })
  it('pairs simultaneous lat/lng metrics by device, sorts routes, rejects invalid locations and supports floor pixels', () => {
    const points = mapPoints([
      { publisher: 'b', time: 2, string_value: '{"latitude":22,"longitude":114}' },
      { publisher: 'a', time: 1, name: 'latitude', value: 23 },
      { publisher: 'a', time: 1, name: 'longitude', value: 113 },
      { time: 4, value: { latitude: 1000, longitude: 120 } },
      { time: 0, value: { latitude: null, longitude: 22 } },
    ])
    expect(points.map((p) => [p.publisher, p.lat, p.lng])).toEqual([
      ['a', 23, 113],
      ['b', 22, 114],
    ])
    expect(mapPoints([{ value: { x: 300, y: 500 } }], { floorPlan: true })[0]).toMatchObject({
      lat: 500,
      lng: 300,
    })
  })
  it('saves each dashboard breakpoint and keeps unrelated concurrent metadata', async () => {
    const record = {
      id: 'dash',
      name: 'D',
      metadata: {
        widget: { a: { id: 'a', title: 'A', chart: { chartType: 'value_card', dataSources: [] } } },
      },
      layout: {
        lg: [{ i: 'a', x: 3, y: 2, w: 5, h: 5 }],
        md: [{ i: 'a', x: 1, y: 1, w: 4, h: 7 }],
      },
    }
    request.mockResolvedValue({
      ...record,
      metadata: { ...record.metadata, note: 'new elsewhere' },
    })
    const w = mount(DashboardEditor, {
      props: { record, workspace: 'w' },
      global: { stubs: { DashboardWidget: true } },
    })
    await flushPromises()
    await w.get('[role="switch"]').setValue(true)
    await w.get('[role="switch"]').setValue(false)
    await flushPromises()
    const write = request.mock.calls.find(([, o]) => o?.method === 'PATCH')[1].body
    expect(write.layout.lg).toEqual(record.layout.lg)
    expect(write.layout.md).toEqual(record.layout.md)
    expect(write.metadata.note).toBe('new elsewhere')
    w.unmount()
  })
})
describe('native message views and device configuration', () => {
  it('reads official SenML views with empty columns and old local JSON settings', () => {
    expect(viewSettings({ format: 'senml', columns: [] }).columns.map((c) => c.key)).toContain(
      'value',
    )
    expect(
      viewSettings({
        config: {
          _agri_ui: { format: 'json', subtopic: 'a', columns: [{ key: 'b', enabled: true }] },
        },
      }).subtopic,
    ).toBe('a')
    expect(
      viewBody(
        { position: 2 },
        { name: ' V ', format: 'senml', subtopic: '', publishers: ['p'], columns: [] },
      ),
    ).toEqual({
      name: 'V',
      format: 'senml',
      subtopic: '',
      publishers: ['p'],
      columns: [],
      position: 2,
    })
  })
  it('retries revocation without creating another key or revoking unrelated credentials', async () => {
    let fail = true
    runOperation.mockImplementation(async (op) => {
      if (op === 'entity') return { id: 'd', tenantId: 'w' }
      if (op === 'credentials')
        return {
          items: [
            { id: 'old', kind: 'shared_key', status: 'active' },
            { id: 'unrelated', kind: 'shared_key', status: 'active' },
          ],
        }
      if (op === 'createSharedKey') return { credentialId: 'new', key: 'test' }
      if (op === 'revokeCredential' && fail) {
        fail = false
        throw Object.assign(new Error('denied'), { status: 403 })
      }
      return {}
    })
    const state = {}
    await expect(replaceDeviceKey(state, { id: 'd' }, 'w', 'test', 'old')).rejects.toThrow('denied')
    await replaceDeviceKey(state, { id: 'd' }, 'w', 'test', 'old')
    expect(runOperation.mock.calls.filter(([op]) => op === 'createSharedKey')).toHaveLength(1)
    expect(
      runOperation.mock.calls
        .filter(([op]) => op === 'revokeCredential')
        .map(([, p]) => p.credentialId),
    ).toEqual(['old', 'old'])
  })
  it('blocks key recreation when the first creation response was lost', async () => {
    runOperation.mockImplementation(async (op) => {
      if (op === 'entity') return { id: 'd', tenantId: 'w' }
      if (op === 'credentials') return { items: [] }
      throw new Error('timeout')
    })
    const state = {}
    await expect(replaceDeviceKey(state, { id: 'd' }, 'w', 'test', '')).rejects.toThrow('timeout')
    await expect(replaceDeviceKey(state, { id: 'd' }, 'w', 'test', '')).rejects.toThrow('unknown')
    expect(runOperation.mock.calls.filter(([op]) => op === 'createSharedKey')).toHaveLength(1)
  })
  it('binds the selected profile version within the current workspace', async () => {
    runOperation.mockImplementation(async (op) =>
      op === 'profileVersions'
        ? [{ id: 'v2', version: 2, status: 'active' }]
        : { id: 'd', tenantId: 'w' },
    )
    const w = mount(DeviceTypeBinding, {
      props: { record: { id: 'd' }, workspace: 'w', profiles: [{ id: 'p', displayName: 'Soil' }] },
    })
    await flushPromises()
    await button(w, '绑定设备类型').trigger('click')
    await w.find('select').setValue('p')
    await flushPromises()
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(runOperation).toHaveBeenCalledWith('updateEntity', {
      id: 'd',
      input: { profileId: 'p', profileVersionId: 'v2' },
    })
    w.unmount()
  })
  it('uses the actual share_state contract and removes only deselected users', async () => {
    listAll.mockResolvedValue([
      { id: 'u1', name: 'A' },
      { id: 'u2', name: 'B' },
    ])
    request.mockResolvedValue({ user_ids: ['u1', 'u2'] })
    const w = mount(SimpleNativeEditor, {
      props: {
        record: { id: 'dash', share_state: 'selected_users' },
        workspace: 'w',
        section: 'dashboards',
      },
      global: { stubs: { DashboardEditor: true } },
    })
    await flushPromises()
    w.findComponent({ name: 'DashboardEditor' }).vm.$emit('share')
    await flushPromises()
    await w.find('select[multiple]').setValue(['u2'])
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).toHaveBeenCalledWith('/api/platform/ui/w/dashboards/dash/unshare', {
      method: 'DELETE',
      body: { dashboard_id: 'dash', user_ids: ['u1'] },
    })
    expect(request).toHaveBeenCalledWith('/api/platform/ui/w/dashboards/dash/share', {
      method: 'PATCH',
      body: {
        dashboard_id: 'dash',
        workspace_id: 'w',
        share_state: 'selected_users',
        user_ids: [],
      },
    })
    w.unmount()
  })
})

describe('comparison editor', () => {
  it('round trips official-style numeric, boolean and quoted string comparisons without executing user input', () => {
    const blocks = [
      { input: 'message.payload[1].v', operator: '<', type: 'number', value: '20', join: 'and' },
      {
        input: 'message.publisher',
        operator: '==',
        type: 'string',
        value: 'device \"A\"',
        join: 'and',
      },
      { input: 'message.enabled', operator: '==', type: 'boolean', value: 'true', join: 'or' },
    ]
    const script = comparisonScript(blocks)
    expect(parseComparison(script)).toEqual(blocks)
    expect(
      parseComparison(
        'if message.publisher == \"other\" then return message.payload end\n' + script,
      ),
    ).toEqual([])
    expect(() => comparisonScript([{ ...blocks[0], input: 'os.execute("bad")' }])).toThrow(
      'field path',
    )
    expect(() => comparisonScript([{ ...blocks[0], value: '0/0' }])).toThrow('Invalid')
  })
})
