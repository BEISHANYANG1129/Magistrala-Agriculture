import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick, defineComponent, ref } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useListLocation } from '../api/listLocation'
import ModalDialog from '../components/ModalDialog.vue'
import PopoverMenu from '../components/PopoverMenu.vue'
import TablePagination from '../components/TablePagination.vue'
import DashboardEditor from '../components/DashboardEditor.vue'
import { request, listAll } from '../api/platform'

vi.mock('../api/platform', () => ({ request: vi.fn(), listAll: vi.fn(), errorText: e => e.message }))
const mounted = []
const render = (component, options = {}) => {
  const wrapper = mount(component, { attachTo: document.body, ...options })
  mounted.push(wrapper)
  return wrapper
}
beforeEach(() => { vi.resetAllMocks(); listAll.mockResolvedValue([]) })
afterEach(() => { mounted.reverse().forEach(w => w.unmount()); mounted.length = 0; document.body.innerHTML = '' })
const key = async (name, options = {}) => {
  document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: name, bubbles: true, cancelable: true, ...options }))
  await nextTick()
}

describe('shared dialog interactions', () => {
  it('focuses the first field, traps Tab, closes with Escape and restores the trigger', async () => {
    const trigger = document.createElement('button'); document.body.append(trigger); trigger.focus()
    const w = render(ModalDialog, { slots: { default: '<form><h2>Edit</h2><input /><button>Cancel</button></form>' } })
    await flushPromises()
    const input = document.querySelector('[role="dialog"] input')
    expect(document.activeElement).toBe(input)
    expect(trigger.hasAttribute('inert')).toBe(true)
    await key('Tab', { shiftKey: true })
    expect(document.activeElement.classList.contains('dialog-close')).toBe(true)
    await key('Tab')
    expect(document.activeElement).toBe(input)
    await key('Escape')
    expect(w.emitted('close')).toHaveLength(1)
    w.unmount(); mounted.pop(); await nextTick()
    expect(document.activeElement).toBe(trigger)
    expect(trigger.hasAttribute('inert')).toBe(false)
  })
  it('does not dismiss a submitting dialog or a confirmation through the backdrop', async () => {
    const w = render(ModalDialog, { props: { busy: true, confirmation: true }, slots: { default: '<div><h2>Confirm</h2><button>Cancel</button></div>' } })
    await flushPromises(); await key('Escape')
    expect(w.emitted('close')).toBeUndefined()
    await w.setProps({ busy: false })
    const overlay = document.querySelector('.modal')
    overlay.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    overlay.click(); await nextTick()
    expect(w.emitted('close')).toBeUndefined()
    await key('Escape'); expect(w.emitted('close')).toHaveLength(1)
  })
  it('only dismisses the top dialog and retains the outer focus trap and scroll lock', async () => {
    const outer = render(ModalDialog, { slots: { default: '<div><h2>Outer</h2><input id="outer-input" /></div>' } })
    await flushPromises()
    const inner = render(ModalDialog, { slots: { default: '<div><h2>Inner</h2><input /></div>' } })
    await flushPromises(); await key('Escape')
    expect(inner.emitted('close')).toHaveLength(1)
    expect(outer.emitted('close')).toBeUndefined()
    inner.unmount(); mounted.pop(); await nextTick()
    expect(document.activeElement.id).toBe('outer-input')
    expect(document.body.style.overflow).toBe('hidden')
  })
})
describe('shared menus and pagination', () => {
  it('returns to the stable menu trigger when its action opens and closes a dialog', async () => {
    const Host = defineComponent({
      components: { PopoverMenu, ModalDialog }, setup: () => ({ opened: ref(false) }),
      template: '<PopoverMenu label="Actions"><template #trigger>Actions</template><button @click="opened = true">Edit</button></PopoverMenu><ModalDialog v-if="opened" @close="opened = false"><div><h2>Edit</h2><input /></div></ModalDialog>',
    })
    const w = render(Host)
    await w.get('button').trigger('click'); await flushPromises()
    document.querySelector('.popover-panel button').click(); await flushPromises()
    await key('Escape'); await flushPromises()
    expect(document.activeElement).toBe(w.get('button').element)
  })
  it('supports arrow navigation, retains checkbox selections, and returns focus on Escape', async () => {
    const w = render(PopoverMenu, { props: { label: 'View' }, slots: { trigger: 'View', default: '<label><input type="checkbox" />Name</label><button>Open</button>' } })
    await w.get('button').trigger('click'); await flushPromises()
    const checkbox = document.querySelector('.popover-panel input')
    checkbox.click(); await nextTick()
    expect(document.querySelector('.popover-panel')).not.toBeNull()
    await key('ArrowDown'); expect(document.activeElement.textContent).toBe('Open')
    await key('Escape'); expect(document.querySelector('.popover-panel')).toBeNull()
    expect(document.activeElement).toBe(w.get('button').element)
  })
  it('closes on outside clicks without stealing focus and on chosen actions', async () => {
    const w = render(PopoverMenu, { slots: { trigger: 'Menu', default: '<button>View</button>' } })
    await w.get('button').trigger('click'); await flushPromises()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true })); await nextTick()
    expect(document.querySelector('.popover-panel')).toBeNull()
    await w.get('button').trigger('click'); await flushPromises()
    document.querySelector('.popover-panel button').click(); await nextTick()
    expect(document.querySelector('.popover-panel')).toBeNull()
  })
  it('provides all official page sizes and clamps the page after records disappear', async () => {
    const w = render(TablePagination, { props: { page: 3, size: 10, total: 25 } })
    expect(w.findAll('option').map(n => n.text())).toEqual(['5','10','20','30','40','50'])
    await w.setProps({ total: 5 })
    expect(w.emitted('update:page').at(-1)).toEqual([1])
  })
})
describe('list location state', () => {
  it('restores a deep link and history filters without losing the requested page', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/list', component: { template: '<div />' } }] })
    await router.push('/list?limit=5&offset=10&q=soil')
    const Host = defineComponent({ setup() {
      const page = ref(1), size = ref(10), search = ref('')
      useListLocation({ page, size, filters: { q: [search, ''] } })
      return { page, size, search }
    }, template: '<div>{{ page }} {{ search }}</div>' })
    const w = render(Host, { global: { plugins: [router] } }); await flushPromises()
    expect([w.vm.page, w.vm.size, w.vm.search]).toEqual([3, 5, 'soil'])
    await router.push('/list?limit=20&offset=40&q=north'); await flushPromises()
    expect([w.vm.page, w.vm.size, w.vm.search]).toEqual([3, 20, 'north'])
    w.vm.search = 'south'; await flushPromises()
    expect(w.vm.page).toBe(1)
    expect(router.currentRoute.value.query.offset).toBe('0')
    expect(router.currentRoute.value.query.q).toBe('south')
  })
})
describe('dashboard edit lifecycle', () => {
  const record = { id: 'dashboard', name: 'Test', metadata: { widget: {} }, layout: {} }
  const mountEditor = () => render(DashboardEditor, { props: { record, workspace: 'workspace' }, global: { stubs: { DashboardWidget: true, DashboardGrid: true, WidgetSettings: true } } })
  it('saves on leaving Edit Mode, retaining the draft on failure and allowing retry', async () => {
    request.mockResolvedValueOnce(record).mockRejectedValueOnce(new Error('save failed'))
    const w = mountEditor(); await flushPromises()
    await w.get('[role="switch"]').setValue(true)
    await w.get('[role="switch"]').setValue(false); await flushPromises()
    expect(w.get('[role="switch"]').element.checked).toBe(true)
    expect(w.text()).toContain('save failed')
    request.mockResolvedValueOnce(record).mockResolvedValueOnce(record)
    await w.get('[role="switch"]').setValue(false); await flushPromises()
    expect(w.get('[role="switch"]').element.checked).toBe(false)
    expect(request.mock.calls.filter(([,opts])=>opts?.method==='PATCH')).toHaveLength(2)
  })
  it('cancels editing without saving', async () => {
    const w = mountEditor(); await flushPromises()
    await w.get('[role="switch"]').setValue(true)
    await w.findAll('button').find(b => /取消编辑|Cancel editing/.test(b.text())).trigger('click')
    expect(w.get('[role="switch"]').element.checked).toBe(false)
    expect(request).not.toHaveBeenCalled()
  })
})
