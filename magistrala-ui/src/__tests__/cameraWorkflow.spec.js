import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import CameraPlayer from '../components/CameraPlayer.vue'
import { cameraRequest, cameraPtz } from '../api/camera'
import { validateWidget } from '../api/widgetConfig'

vi.mock('../api/camera', async (original) => ({ ...(await original()), cameraRequest: vi.fn(), cameraPtz: vi.fn() }))
vi.mock('hls.js', () => ({ default: class {
  static isSupported() { return true }
  static Events = { ERROR: 'error', MANIFEST_PARSED: 'parsed' }
  on() {} attachMedia() {} loadSource() {} destroy() {}
} }))
vi.mock('../api/session', () => ({ ensureSession: vi.fn(async () => ({ token: 'test-only-token' })) }))

const workspace = '11111111-1111-1111-1111-111111111111'
const deviceId = '22222222-2222-2222-2222-222222222222'
let wrapper
beforeEach(() => {
  vi.resetAllMocks()
  cameraRequest.mockResolvedValue({sessionId: 'a'.repeat(32), state: 'starting', enabled: true, channels: []})
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
})
afterEach(() => { wrapper?.unmount(); wrapper = null; vi.restoreAllMocks() })

describe('camera video lifecycle', () => {
  it('automatically attaches all three views without changing the saved playback setting', async () => {
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId } })
    await flushPromises()
    expect(wrapper.findAll('video')).toHaveLength(3)
    expect(cameraRequest).toHaveBeenCalledWith(workspace, deviceId, 'sessions', 'POST')
    expect(cameraRequest.mock.calls.some((call) => call[2].startsWith('playback/'))).toBe(false)
  })
  it('does not connect a draft preview', async () => {
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId, preview: true } })
    await flushPromises()
    expect(cameraRequest).not.toHaveBeenCalled()
  })
  it('keeps dashboard playback free of device operation buttons', async () => {
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId, compact: true } })
    await flushPromises()
    expect(wrapper.findAll('video')).toHaveLength(3)
    expect(wrapper.find('.ptz-controls').exists()).toBe(false)
    expect(wrapper.findAll('.view-tools')).toHaveLength(0)
    expect(cameraPtz).not.toHaveBeenCalled()
  })
  it('sends only an explicit bounded movement, keeps stop available, and never retries on failure', async () => {
    cameraRequest.mockResolvedValue({ sessionId:'a'.repeat(32), state:'ready', enabled:true,
      generation:'g-'+'b'.repeat(32), channels:[{id:0,label:'画面 1',playlist:`sessions/${'a'.repeat(32)}/media/g-${'b'.repeat(32)}/0/index.m3u8`}] })
    let rejectMove
    cameraPtz.mockImplementation((_workspace, _device, action) => action === 'move'
      ? new Promise((_resolve, reject) => { rejectMove = reject }) : Promise.resolve({status:'stop_sent'}))
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId, showControls:true } })
    await flushPromises()
    expect(cameraPtz).not.toHaveBeenCalled()
    await wrapper.find('[aria-label="云台向左"]').trigger('click')
    expect(cameraPtz).toHaveBeenCalledWith(workspace, deviceId, 'move', {
      requestId:expect.stringMatching(/^[a-f0-9]{32}$/), direction:'left', durationMs:400,
    })
    expect(wrapper.find('[aria-label="云台向右"]').element.disabled).toBe(true)
    expect(wrapper.find('[aria-label="停止云台"]').element.disabled).toBe(false)
    await wrapper.find('[aria-label="停止云台"]').trigger('click')
    await flushPromises()
    expect(cameraPtz.mock.calls[1][2]).toBe('stop')
    rejectMove(Object.assign(new Error('Denied'), {status:403}))
    await flushPromises()
    expect(wrapper.text()).toContain('没有这台设备的云台控制权限')
    expect(cameraPtz).toHaveBeenCalledTimes(2)
    wrapper.unmount(); wrapper = null
    expect(cameraPtz).toHaveBeenCalledTimes(2)
  })
  it('keeps a saved manual stop when the page is reopened', async () => {
    cameraRequest.mockResolvedValue({sessionId: 'a'.repeat(32), state: 'stopped', enabled: false, channels: []})
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId } })
    await flushPromises()
    expect(wrapper.find('button').text()).toContain('开启播放')
    expect(cameraRequest.mock.calls.some((call) => call[2] === 'playback/start')).toBe(false)
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(cameraRequest).toHaveBeenCalledWith(workspace, deviceId, 'playback/start', 'POST')
  })
  it('stops global playback only on an explicit stop, and releases just the viewer on unmount', async () => {
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId } })
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(cameraRequest).toHaveBeenCalledWith(workspace, deviceId, 'playback/stop', 'POST')
    const before = cameraRequest.mock.calls.filter((call) => call[2] === 'playback/stop').length
    wrapper.unmount(); wrapper = null
    await flushPromises()
    expect(cameraRequest.mock.calls.filter((call) => call[2] === 'playback/stop')).toHaveLength(before)
    expect(cameraRequest).toHaveBeenLastCalledWith(workspace, deviceId, `sessions/${'a'.repeat(32)}`, 'DELETE')
  })
  it('releases a late-created lease when the user leaves while connection is pending', async () => {
    let resolveSession
    cameraRequest.mockImplementation((_workspace, _device, tail) => tail === 'sessions'
      ? new Promise((resolve) => { resolveSession = resolve }) : Promise.resolve({ closed: true }))
    wrapper = mount(CameraPlayer, { props: { workspace, deviceId } })
    await flushPromises()
    wrapper.unmount(); wrapper = null
    resolveSession({ sessionId: 'a'.repeat(32), state: 'starting', channels: [] })
    await flushPromises()
    expect(cameraRequest).toHaveBeenLastCalledWith(workspace, deviceId, `sessions/${'a'.repeat(32)}`, 'DELETE')
  })
  it('accepts the third display view but rejects invalid device and view bindings', () => {
    const widget = { title: 'Camera', chart: { cardType: 'camera_video', cameraDeviceId: deviceId, cameraView: '2' } }
    expect(() => validateWidget(widget)).not.toThrow()
    expect(() => validateWidget({ ...widget, chart: { ...widget.chart, cameraView: '3' } })).toThrow()
    expect(() => validateWidget({ ...widget, chart: { ...widget.chart, cameraDeviceId: '-'.repeat(36) } })).toThrow()
  })
})
