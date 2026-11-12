import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import MessagesPage from '../components/MessagesPage.vue'
import { runOperation, request } from '../api/platform'

vi.mock('../api/platform', () => ({ runOperation: vi.fn(), request: vi.fn(), errorText: (e) => e.message }))
vi.mock('../api/resourceWorkflow', () => ({ deviceKey: vi.fn() }))
const views = {
  c1: [
    { id: 'environment', name: '多要素百叶盒', format: 'senml', publishers: ['box'], position: 0 },
    { id: 'wind', name: '风速传感器', format: 'senml', publishers: ['wind-sensor'], position: 1 },
  ],
  c2: [{ id: 'soil', name: '土壤', format: 'senml', publishers: ['soil-sensor'], position: 0 }],
}
beforeEach(() => {
  vi.resetAllMocks()
  runOperation.mockImplementation(async (op) => ({ items: op === 'resources'
    ? [{ id: 'c1', name: '气象站' }, { id: 'c2', name: '土壤通道' }]
    : [{ id: 'box', name: '百叶盒设备' }, { id: 'wind-sensor', name: '风速设备' }] }))
  request.mockImplementation(async (path) => {
    const channel = path.match(/channels\/([^/]+)/)?.[1]
    if (path.endsWith('message-views')) return { message_views: views[channel] || [] }
    const publisher = new URL(path, 'http://local').searchParams.get('publishers')
    return { total: 1, messages: [{ name: publisher === 'box' ? 'air_temperature' : 'reading', value: 29, publisher: publisher || 'box', time: 1793018200 }] }
  })
})
const button = (wrapper, text) => wrapper.findAll('button').find((b) => b.text() === text)
const readerCalls = () => request.mock.calls.filter(([url]) => url.includes('/messages?'))
async function setup(query = {}, props = {}) {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/domain/:id/messages', component: MessagesPage, props: () => props },
  ] })
  await router.push({ path: '/domain/weather/messages', query })
  await router.isReady()
  const wrapper = mount({ template: '<router-view />' }, { global: { plugins: [router], stubs: { MessageViewEditor: true, ModalDialog: true, MessageImage: true } } })
  await flushPromises()
  return { wrapper, router }
}

describe('message view navigation', () => {
  it('retains a manually selected channel when applying a device view', async () => {
    const { wrapper, router } = await setup()
    await wrapper.find('select').setValue('c1'); await flushPromises()
    await button(wrapper, '多要素百叶盒').trigger('click'); await flushPromises()
    expect(wrapper.find('select').element.value).toBe('c1')
    expect(router.currentRoute.value.query).toMatchObject({ channel: 'c1', view: 'environment' })
    expect(new URL(readerCalls().at(-1)[0], 'http://local').searchParams.getAll('publishers')).toEqual(['box'])
    expect(wrapper.text()).toContain('air_temperature')
    expect(runOperation.mock.calls.filter(([op]) => op === 'resources')).toHaveLength(1)
    wrapper.unmount()
  })
  it('switches and resets views without clearing the channel or retaining the old device filter', async () => {
    const { wrapper, router } = await setup({ channel: 'c1', view: 'environment' })
    await button(wrapper, '风速传感器').trigger('click'); await flushPromises()
    expect(wrapper.find('select').element.value).toBe('c1')
    expect(new URL(readerCalls().at(-1)[0], 'http://local').searchParams.getAll('publishers')).toEqual(['wind-sensor'])
    await button(wrapper, '全部消息').trigger('click'); await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ channel: 'c1' })
    expect(new URL(readerCalls().at(-1)[0], 'http://local').searchParams.has('publishers')).toBe(false)
    await wrapper.find('select').setValue('c2'); await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ channel: 'c2' })
    expect(wrapper.text()).toContain('土壤')
    expect(button(wrapper, '多要素百叶盒')).toBeUndefined()
    wrapper.unmount()
  })
  it('restores channel and view from navigation without reloading the workspace catalog', async () => {
    const { wrapper, router } = await setup({ channel: 'c1', view: 'environment' })
    await router.push({ query: { channel: 'c2', view: 'soil' } }); await flushPromises()
    expect(wrapper.find('select').element.value).toBe('c2')
    expect(new URL(readerCalls().at(-1)[0], 'http://local').searchParams.getAll('publishers')).toEqual(['soil-sensor'])
    await router.push({ query: { channel: 'c1', view: 'wind' } }); await flushPromises()
    expect(wrapper.find('select').element.value).toBe('c1')
    expect(button(wrapper, '风速传感器').classes()).toContain('active')
    expect(runOperation.mock.calls.filter(([op]) => op === 'resources')).toHaveLength(1)
    wrapper.unmount()
  })
  it('keeps the supplied channel for embedded message views', async () => {
    const { wrapper, router } = await setup({}, { channelId: 'c1', embedded: true })
    await button(wrapper, '多要素百叶盒').trigger('click'); await flushPromises()
    expect(button(wrapper, '多要素百叶盒').classes()).toContain('active')
    expect(readerCalls().at(-1)[0]).toContain('/channels/c1/messages?')
    expect(router.currentRoute.value.query.view).toBe('environment')
    expect(runOperation.mock.calls.filter(([op]) => op === 'resources')).toHaveLength(1)
    wrapper.unmount()
  })
  it('discards an outstanding response when the channel is cleared', async () => {
    const { wrapper } = await setup({ channel: 'c1' })
    let finish
    request.mockImplementation((path) => path.includes('/messages?')
      ? new Promise((resolve) => { finish = resolve }) : Promise.resolve({ message_views: views.c1 }))
    await button(wrapper, '刷新').trigger('click'); await flushPromises()
    await wrapper.find('select').setValue(''); await flushPromises()
    finish({ total: 1, messages: [{ name: 'late-reading', value: 999 }] }); await flushPromises()
    expect(wrapper.text()).not.toContain('late-reading')
    expect(wrapper.find('select').element.value).toBe('')
    wrapper.unmount()
  })
})
