import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import DashboardWidget from '../components/DashboardWidget.vue'
import DashboardEditor from '../components/DashboardEditor.vue'
import { loadWidgetData } from '../api/widgetData'
import { request, listAll } from '../api/platform'

const runtime = vi.hoisted(() => ({ init: vi.fn(), charts: [], observers: [] }))
vi.mock('echarts', () => ({ init: runtime.init }))
vi.mock('../api/widgetData', () => ({ loadWidgetData: vi.fn() }))
vi.mock('../api/platform', () => ({ request: vi.fn(), listAll: vi.fn(), runOperation: vi.fn(), errorText: (e) => e.message }))
vi.mock('../api/resourceWorkflow', () => ({ readResource: vi.fn() }))

function data(value) {
  return { rows: [{ name: 'temperature', publisher: 'box', time: 1793089700, value, _source: 0 }], shapes: [], value }
}
function widget(type = 'line_chart') {
  return { id: 'temperature', title: '温度', chart: { presentation: 'weather', chartType: type,
    dataRefreshInterval: 20, decimals: 1,
    dataSources: [{ channel: { id: 'channel' }, client: { id: 'box' }, valueName: 'temperature' }] } }
}
async function settle() { await flushPromises(); await flushPromises() }
function mountWidget(type = 'line_chart') {
  return mount(DashboardWidget, {
    props: { widget: widget(type), workspace: 'weather', refreshToken: 0 },
    global: { stubs: { RouterLink: true } },
  })
}
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  vi.clearAllMocks(); runtime.charts.length = 0; runtime.observers.length = 0
  runtime.init.mockImplementation(() => {
    const chart = { setOption: vi.fn(), resize: vi.fn(), dispose: vi.fn() }
    runtime.charts.push(chart); return chart
  })
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback) { this.callback = callback; this.observe = vi.fn(); this.disconnect = vi.fn(); runtime.observers.push(this) }
  })
  loadWidgetData.mockResolvedValue(data(28))
  request.mockResolvedValue({})
  listAll.mockResolvedValue([])
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('stable dashboard refresh', () => {
  it('updates the existing chart without a loading paragraph, a new canvas or repeated entry animation', async () => {
    const wrapper = mountWidget()
    await settle()
    expect(runtime.init).toHaveBeenCalledTimes(1)
    const canvas = wrapper.get('.chart-canvas').element
    const chart = runtime.charts[0]
    let finish
    loadWidgetData.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve }))
    vi.advanceTimersByTime(20000); await settle()
    expect(wrapper.text()).not.toContain('正在加载')
    expect(wrapper.get('.chart-canvas').element).toBe(canvas)
    expect(chart.dispose).not.toHaveBeenCalled()
    finish(data(29)); await settle()
    expect(runtime.init).toHaveBeenCalledTimes(1)
    expect(runtime.observers).toHaveLength(1)
    expect(chart.setOption).toHaveBeenCalledTimes(2)
    expect(chart.setOption.mock.calls[1][0].animation).toBe(false)
    wrapper.unmount()
    expect(chart.dispose).toHaveBeenCalledTimes(1)
    expect(runtime.observers[0].disconnect).toHaveBeenCalledTimes(1)
  })

  it('keeps a value card visible during background refresh and displays zero as data', async () => {
    const wrapper = mountWidget('value_card')
    await settle()
    expect(wrapper.get('.metric').text()).toContain('28.0')
    let finish
    loadWidgetData.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve }))
    vi.advanceTimersByTime(20000); await settle()
    expect(wrapper.text()).not.toContain('正在加载')
    expect(wrapper.get('.metric').text()).toContain('28.0')
    finish(data(0)); await settle()
    expect(wrapper.get('.metric').text()).toContain('0.0')
    wrapper.unmount()
  })

  it('retains the last successful chart when a refresh fails and updates an empty result without rebuilding', async () => {
    const wrapper = mountWidget()
    await settle()
    const chart = runtime.charts[0]
    loadWidgetData.mockRejectedValueOnce(new Error('offline'))
    vi.advanceTimersByTime(20000); await settle()
    expect(wrapper.text()).toContain('offline')
    expect(runtime.init).toHaveBeenCalledTimes(1)
    expect(chart.dispose).not.toHaveBeenCalled()
    expect(chart.setOption).toHaveBeenCalledTimes(1)
    loadWidgetData.mockResolvedValueOnce({ rows: [], shapes: [], value: '—' })
    vi.advanceTimersByTime(20000); await settle()
    expect(chart.setOption.mock.calls.at(-1)[0].series).toEqual([])
    expect(wrapper.text()).not.toContain('offline')
    expect(runtime.init).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('reloads on a manual refresh token and ignores an older response that arrives later', async () => {
    const wrapper = mountWidget('value_card')
    await settle()
    let finishOlder
    loadWidgetData.mockImplementationOnce(() => new Promise((resolve) => { finishOlder = resolve }))
    vi.advanceTimersByTime(20000); await settle()
    loadWidgetData.mockResolvedValueOnce(data(30))
    await wrapper.setProps({ refreshToken: 1 }); await settle()
    expect(wrapper.get('.metric').text()).toContain('30.0')
    finishOlder(data(29)); await settle()
    expect(wrapper.get('.metric').text()).toContain('30.0')
    expect(loadWidgetData).toHaveBeenCalledTimes(3)
    wrapper.unmount()
  })

  it('does not recreate a chart after unmounting with a refresh in flight', async () => {
    const wrapper = mountWidget()
    await settle()
    let finish
    loadWidgetData.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve }))
    vi.advanceTimersByTime(20000); await settle()
    wrapper.unmount()
    finish(data(29)); await settle()
    expect(runtime.init).toHaveBeenCalledTimes(1)
    expect(runtime.charts[0].setOption).toHaveBeenCalledTimes(1)
    expect(runtime.charts[0].dispose).toHaveBeenCalledTimes(1)
  })

  it('does not remount dashboard widgets when the dashboard refresh button is clicked', async () => {
    const w = widget('value_card')
    const wrapper = mount(DashboardEditor, {
      props: { workspace: 'weather', record: { id: 'dashboard', name: 'Weather', metadata: { widget: { [w.id]: w } } } },
      global: { stubs: {
        DashboardWidget: true,
        DashboardGrid: { props: ['widgets'], template: '<div><slot v-for="item in widgets" :id="item.id" /></div>' },
      } },
    })
    await settle()
    const first = wrapper.findComponent({ name: 'DashboardWidget' }).vm
    await wrapper.findAll('button').find((b) => b.text() === '刷新仪表盘').trigger('click')
    await settle()
    expect(wrapper.findComponent({ name: 'DashboardWidget' }).vm).toBe(first)
    expect(wrapper.findComponent({ name: 'DashboardWidget' }).props('refreshToken')).toBe(1)
    wrapper.unmount()
  })
})
