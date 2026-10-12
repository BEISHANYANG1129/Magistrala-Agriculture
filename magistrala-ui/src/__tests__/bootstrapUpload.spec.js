// Modal keyboard/focus behavior is exercised with real Teleport in interactionParity.spec.js.
vi.mock('../components/ModalDialog.vue', () => ({ default: { template: '<div class="modal"><slot /></div>' } }))
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import BootstrapProfileUpload from '../components/BootstrapProfileUpload.vue'
import { request } from '../api/platform'

vi.mock('../api/platform', () => ({ request: vi.fn(), errorText: (e) => e.message }))
beforeEach(() => vi.resetAllMocks())
async function choose(w, name, text, size = text.length) {
  const input = w.find('input[type=file]')
  Object.defineProperty(input.element, 'files', {
    configurable: true,
    value: [{ name, size, text: async () => text }],
  })
  await input.trigger('change')
  await flushPromises()
}
describe('Bootstrap profile upload', () => {
  it('previews without writes, then preserves YAML and sends its content type to the workspace endpoint', async () => {
    const w = mount(BootstrapProfileUpload, { props: { workspace: 'workspace-a' } })
    const raw = 'name: sample\ncontent_template: |\n  {{ .Device.ID }}\n'
    await choose(w, 'profile.yaml', raw)
    expect(request).not.toHaveBeenCalled()
    expect(w.find('pre').text()).toContain('{{ .Device.ID }}')
    request.mockResolvedValue({ id: 'profile-1' })
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).toHaveBeenCalledWith(
      '/api/platform/bootstrap/workspace-a/devices/bootstrap/profiles/upload',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/yaml' },
        body: raw,
      },
    )
    expect(w.emitted('uploaded')[0][0]).toEqual({ id: 'profile-1' })
    w.unmount()
  })
  it('rejects oversized and invalid JSON files before any write', async () => {
    const w = mount(BootstrapProfileUpload, { props: { workspace: 'a' } })
    await choose(w, 'profile.json', '{}', 1024 * 1024 + 1)
    expect(w.find('[role=alert]').text()).toContain('1 MB')
    await choose(w, 'profile.json', '[]')
    await w.find('form').trigger('submit')
    expect(request).not.toHaveBeenCalled()
    expect(w.find('pre').exists()).toBe(false)
    w.unmount()
  })
  it('prevents duplicate creation after an unknown outcome but permits correction after validation errors', async () => {
    const w = mount(BootstrapProfileUpload, { props: { workspace: 'a' } })
    await choose(w, 'profile.toml', 'name = "sample"')
    request.mockRejectedValueOnce(Object.assign(new Error('Invalid profile'), { status: 400 }))
    await w.find('form').trigger('submit')
    await flushPromises()
    request.mockRejectedValueOnce(Object.assign(new Error('Timed out'), { status: 0 }))
    await w.find('form').trigger('submit')
    await flushPromises()
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).toHaveBeenCalledTimes(2)
    expect(w.text()).toContain('避免重复创建')
    w.unmount()
  })
})
