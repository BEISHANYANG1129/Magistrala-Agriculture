import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SimpleNativeEditor from '../components/SimpleNativeEditor.vue'
import { request, listAll } from '../api/platform'
import { dashboardImport } from '../api/dashboardLayout'

vi.mock('../api/platform', () => ({
  request: vi.fn(), listAll: vi.fn(), errorText: (e) => e.message,
}))

beforeEach(() => {
  vi.resetAllMocks()
  listAll.mockResolvedValue([])
  request.mockResolvedValue({ id: 'created-dashboard' })
})

describe('imported dashboard creation', () => {
  it('persists imported widgets together with their layout on the first save', async () => {
    const widget = {
      id: 'temperature', title: '空气温度', type: 'chart',
      chart: { chartType: 'value_card', valueName: 'air_temperature',
        dataSources: [{ channel: { id: 'weather-channel' }, client: { id: 'environment-box' } }] },
    }
    const imported = dashboardImport(JSON.stringify({
      name: '气象站', metadata: { widget: { temperature: widget }, station_id: '40366226' },
      layout: { lg: [{ i: 'temperature', x: 0, y: 0, w: 4, h: 4 }] },
    }))
    const wrapper = mount(SimpleNativeEditor, {
      props: { section: 'dashboards', workspace: 'weather', record: imported },
      global: { stubs: { DashboardEditor: true, ModalDialog: true } },
    })
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(request).toHaveBeenCalledWith('/api/platform/ui/weather/dashboards', {
      method: 'POST', body: expect.objectContaining({
        metadata: imported.metadata, layout: imported.layout,
      }),
    })
    expect(wrapper.emitted('saved')).toEqual([[{ id: 'created-dashboard' }]])
    wrapper.unmount()
  })
})
