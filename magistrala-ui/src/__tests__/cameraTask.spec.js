import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
const mocks = vi.hoisted(() => ({ request: vi.fn() }))
vi.mock('../api/platform', () => ({ ...mocks, errorText: (e) => e.message }))
import CameraTaskPanel from '../components/CameraTaskPanel.vue'

function task(status = 'waiting_camera_confirmation') {
  return {
    id: 'task-a',
    status,
    automation: {
      camera: {
        source: { name: '乔安 G4S4' },
        request: {
          operations: [
            { action: 'move', direction: 'left', duration_ms: 200, view: 1 },
            { action: 'snapshot', view: 1 },
          ],
          analyze: false,
        },
        revision: 'revision-a',
        expires_at: '2026-11-10T23:00:00Z',
        receipts: [],
        captures: [],
      },
    },
  }
}
beforeEach(() => {
  localStorage.setItem('agri-language', 'zh')
  mocks.request.mockReset().mockResolvedValue({})
})
describe('camera task plan', () => {
  it('requires one explicit whole-plan confirmation and shows exact timed movement', async () => {
    const wrapper = mount(CameraTaskPanel, { props: { task: task(), workspace: 'w' } })
    const button = wrapper.findAll('button').find((b) => b.text().includes('确认整条'))
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('200 ms')
    expect(mocks.request).not.toHaveBeenCalled()
    await wrapper.get('input[type=checkbox]').setValue(true)
    await button.trigger('click')
    await flushPromises()
    expect(mocks.request).toHaveBeenCalledWith(
      expect.stringContaining('/task-a/camera/confirm'),
      expect.objectContaining({
        method: 'POST',
        body: { revision: 'revision-a', confirmed: true },
        headers: { 'X-Workspace-ID': 'w' },
      }),
    )
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    wrapper.unmount()
  })
  it('stops a running task without another checkbox or confirmation', async () => {
    const wrapper = mount(CameraTaskPanel, {
      props: { task: task('camera_running'), workspace: 'w' },
    })
    expect(wrapper.find('input[type=checkbox]').exists()).toBe(false)
    await wrapper.get('.stop-camera').trigger('click')
    await flushPromises()
    expect(mocks.request).toHaveBeenCalledWith(
      expect.stringContaining('/camera/stop'),
      expect.objectContaining({ method: 'POST', body: {} }),
    )
    wrapper.unmount()
  })
  it('loads saved captures on request with workspace authentication and exposes download', async () => {
    const value = task('completed')
    value.automation.camera.captures = [
      { id: 'capture-a', view: 1, received_at: '2026-11-10T13:00:00Z' },
    ]
    mocks.request.mockResolvedValue({ mime: 'image/jpeg', image: 'aGVsbG8=' })
    const wrapper = mount(CameraTaskPanel, { props: { task: value, workspace: 'w' } })
    expect(wrapper.find('img').exists()).toBe(false)
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '查看截图')
      .trigger('click')
    await flushPromises()
    expect(mocks.request).toHaveBeenCalledWith(
      expect.stringContaining('/camera/captures/capture-a'),
      expect.objectContaining({ method: 'GET', headers: { 'X-Workspace-ID': 'w' } }),
    )
    expect(wrapper.get('img').attributes('src')).toBe('data:image/jpeg;base64,aGVsbG8=')
    expect(wrapper.get('a[download]').attributes('download')).toContain('capture-a')
    wrapper.unmount()
  })
})
