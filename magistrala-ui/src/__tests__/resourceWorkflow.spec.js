// Modal keyboard/focus behavior is exercised with real Teleport in interactionParity.spec.js.
vi.mock('../components/ModalDialog.vue', () => ({ default: { template: '<div class="modal"><slot /></div>' } }))
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { gql, listAll, runOperation } from '../api/platform'
import { channelPolicies, connectDevice, setResourceStatus } from '../api/resourceWorkflow'
import ResourceCreate from '../components/ResourceCreate.vue'
import ResourceSettings from '../components/ResourceSettings.vue'

vi.mock('../api/platform', () => ({
  gql: vi.fn(),
  listAll: vi.fn(),
  runOperation: vi.fn(),
  errorText: (e) => e.message,
}))
beforeEach(() => {
  vi.resetAllMocks()
  listAll.mockResolvedValue([])
})

describe('resource workflows', () => {
  it('queries device connections without the object filters rejected by Atom', async () => {
    gql.mockResolvedValue({
      directPolicies: {
        total: 1,
        items: [
          {
            id: 'policy',
            permissionBlock: {
              objectId: 'channel',
              objectKind: 'resource',
              objectType: 'resource:channel',
            },
          },
        ],
      },
    })
    expect(await channelPolicies('workspace', { device: 'device' })).toHaveLength(1)
    const [query, variables] = gql.mock.calls[0]
    expect(query).not.toContain('objectKind:"resource"')
    expect(variables.device).toBe('device')
  })
  it('scopes channel-side connections to the current channel', async () => {
    gql.mockResolvedValue({ directPolicies: { total: 0, items: [] } })
    await channelPolicies('workspace', { channel: 'channel' })
    expect(gql.mock.calls[0][0]).toContain('objectId:$channel')
    expect(gql.mock.calls[0][1].channel).toBe('channel')
  })
  it('removes only its own unassigned block when connecting fails', async () => {
    listAll.mockResolvedValue([{ id: 'pub', name: 'publish' }])
    runOperation.mockImplementation(async (op) => {
      if (op === 'createPermissionBlock') return { id: 'new-block' }
      if (op === 'createDirectPolicy') throw new Error('denied')
    })
    await expect(connectDevice('workspace', 'device', 'channel', ['publish'])).rejects.toThrow(
      'denied',
    )
    expect(runOperation.mock.calls.filter(([op]) => op.startsWith('delete'))).toEqual([
      ['deletePermissionBlock', { id: 'new-block' }],
    ])
  })
  it('keeps current channel metadata when toggling status', async () => {
    runOperation.mockResolvedValueOnce({
      id: 'c',
      tenantId: 'w',
      attributes: { status: 'active', metadata: { region_id: 'north' }, route: 'soil' },
    })
    await setResourceStatus('channels', { id: 'c' }, false, 'w')
    expect(runOperation).toHaveBeenLastCalledWith('updateResource', {
      id: 'c',
      input: {
        attributes: { status: 'disabled', metadata: { region_id: 'north' }, route: 'soil' },
      },
    })
  })
  it('does not create a duplicate device after key creation fails', async () => {
    let attempts = 0
    runOperation.mockImplementation(async (op) => {
      if (op === 'createEntity') return { id: 'created' }
      if (op === 'createSharedKey' && attempts++ === 0) throw new Error('temporary key failure')
      return {}
    })
    const wrapper = mount(ResourceCreate, { props: { section: 'devices', workspace: 'w' } })
    await flushPromises()
    await wrapper.find('input').setValue('Soil sensor')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('temporary key failure')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(runOperation.mock.calls.filter(([op]) => op === 'createEntity')).toHaveLength(1)
    expect(runOperation.mock.calls.filter(([op]) => op === 'createSharedKey')).toHaveLength(2)
    expect(runOperation.mock.calls[0][1].input).toMatchObject({
      tenantId: 'w',
      kind: 'device',
      name: 'Soil sensor',
    })
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })
  it('merges metadata edits with the latest unrelated resource attributes', async () => {
    const wrapper = mount(ResourceSettings, {
      props: {
        section: 'channels',
        workspace: 'w',
        record: { id: 'c', tenantId: 'w', name: 'Soil', attributes: { metadata: { old: 1 } } },
      },
    })
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '查看元数据')
      .trigger('click')
    await wrapper
      .findAll('button')
      .find((b) => b.text() === '编辑')
      .trigger('click')
    await wrapper.find('textarea').setValue('{"region_id":"south"}')
    runOperation.mockResolvedValueOnce({
      id: 'c',
      tenantId: 'w',
      attributes: { metadata: { old: 1 }, route: 'soil', custom: 'changed-elsewhere' },
    })
    await wrapper.find('form.modal-card').trigger('submit')
    await flushPromises()
    expect(runOperation).toHaveBeenLastCalledWith('updateResource', {
      id: 'c',
      input: {
        attributes: {
          metadata: { region_id: 'south' },
          route: 'soil',
          custom: 'changed-elsewhere',
        },
      },
    })
    wrapper.unmount()
  })
})
