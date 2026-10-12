import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { nextTick } from 'vue'

vi.hoisted(() => {
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })) })
})
vi.mock('../api/session', () => ({ signOut: vi.fn(), watchSession: () => vi.fn() }))
import App from '../App.vue'

describe('application navigation', () => {
  it('opens the mobile navigation without changing the current resource', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/domain/:id/:section', component: { template: '<p>Resource content</p>' } }] })
    await router.push('/domain/test/devices')
    const wrapper = mount(App, { global: { plugins: [router] } })
    await wrapper.get('.icon-button').trigger('click')
    await nextTick()
    expect(wrapper.get('.app-shell').classes()).toContain('mobile-nav-open')
    expect(router.currentRoute.value.path).toBe('/domain/test/devices')
    expect(wrapper.text()).toContain('Resource content')
    wrapper.unmount()
  })
})
