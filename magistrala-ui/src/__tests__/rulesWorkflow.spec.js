// Modal keyboard/focus behavior is exercised with real Teleport in interactionParity.spec.js.
vi.mock('../components/ModalDialog.vue', () => ({ default: { template: '<div class="modal"><slot /></div>' } }))
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import RulesPage from '../components/RulesPage.vue'
import { request, listAll } from '../api/platform'

const routing = vi.hoisted(() => ({ route: null, push: vi.fn() }))
vi.mock('vue-router', () => ({
  useRoute: () => routing.route,
  useRouter: () => ({ push: routing.push }),
}))
vi.mock('../api/platform', () => ({
  request: vi.fn(),
  listAll: vi.fn(),
  errorText: (e) => e.message,
}))
const rule = {
  id: 'r',
  name: 'Existing',
  input_channel: 'c',
  input_topic: 'soil',
  logic: { type: 0, value: 'return message.payload', mode: 'unrestricted' },
  outputs: [{ id: 'output-existing', type: 'save_senml' }],
  metadata: { owned_by: 'another-module' },
  tags: ['keep'],
  schedule: { start_datetime: '0001-01-01T00:00:00Z' },
}
beforeEach(() => {
  vi.resetAllMocks()
  routing.route = reactive({ params: { id: 'workspace', recordId: 'r' } })
  listAll.mockResolvedValue([{ id: 'c', name: 'Soil' }])
  request.mockImplementation(async () => structuredClone(rule))
})
async function mountRule() {
  const wrapper = mount(RulesPage, {
    global: { stubs: { RouterLink: { template: '<a><slot/></a>' } } },
  })
  await flushPromises()
  return wrapper
}
async function openSave(w) {
  await w
    .findAll('button')
    .find((b) => b.text() === '保存规则')
    .trigger('click')
}
describe('rule editing', () => {
  it('preserves output IDs, execution mode and metadata when saving', async () => {
    const w = await mountRule()
    await openSave(w)
    await w.find('form').trigger('submit')
    await flushPromises()
    const writes = request.mock.calls.filter(([, options]) => options?.method)
    expect(writes).toEqual([
      [
        '/api/platform/rules/workspace/rules/r',
        {
          method: 'PATCH',
          body: {
            name: 'Existing',
            input_channel: 'c',
            input_topic: 'soil',
            logic: rule.logic,
            outputs: rule.outputs,
            metadata: expect.objectContaining({ ...rule.metadata, flow: expect.any(String) }),
          },
        },
      ],
    ])
    w.unmount()
  })
  it('does not overwrite a rule changed by another task', async () => {
    const w = await mountRule()
    request.mockResolvedValue({ ...rule, name: 'Edited elsewhere' })
    await openSave(w)
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request.mock.calls.some(([, options]) => options?.method)).toBe(false)
    expect(w.text()).toContain('规则已被其他操作修改')
    w.unmount()
  })
  it('does not present the server zero date as an active schedule', async () => {
    const w = await mountRule()
    expect(w.text()).not.toContain('0001-01-01')
    w.unmount()
  })
})
