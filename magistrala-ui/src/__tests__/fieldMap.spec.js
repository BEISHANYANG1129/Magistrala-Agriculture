import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { listAll, runOperation } from '../api/platform'
import { groupMap, loadGroupMap, saveGroupMap, sameMapValue } from '../api/fieldMap'
import FieldMap from '../components/FieldMap.vue'

vi.mock('../api/platform', () => ({
  listAll: vi.fn(),
  runOperation: vi.fn(),
  errorText: (e) => e.message,
}))
vi.mock('../components/ModalDialog.vue', () => ({
  default: { template: '<div role="alertdialog"><slot /></div>' },
}))
const clone = (v) => JSON.parse(JSON.stringify(v))
const group = (id) => ({
  id,
  name: id,
  tenantId: 'w',
  groupType: 'object',
  attributes: {
    tags: ['keep'],
    metadata: { location: { latitude: 1 }, perimeter: { type: 'polygon' } },
  },
})
const device = (id, ids) => ({
  id,
  name: id,
  kind: 'device',
  tenantId: 'w',
  objectGroupIds: ids,
  attributes: { position: { x: 0.9, y: 0.9 } },
})
let records, devices, wrapper
beforeEach(() => {
  vi.resetAllMocks()
  records = { a: group('a'), b: group('b'), empty: group('empty') }
  devices = [device('shared', ['a', 'b']), device('only-b', ['b']), device('orphan', [])]
  listAll.mockImplementation(async (name) =>
    clone(name === 'objectGroups' ? Object.values(records) : devices),
  )
  runOperation.mockImplementation(async (op, args) => {
    if (op === 'group') return clone(records[args.id])
    if (op === 'entity') return clone(devices.find((d) => d.id === args.id))
    if (op === 'updateGroup') {
      records[args.id] = { ...records[args.id], ...clone(args.input) }
      return clone(records[args.id])
    }
    throw new Error(`Unexpected operation: ${op}`)
  })
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

describe('group map persistence', () => {
  it('does not mark unchanged polygons dirty when JSONB reorders object keys', () => {
    expect(
      sameMapValue(
        [{ name: 'area', points: [{ x: 0.1, y: 0.2 }] }],
        [{ points: [{ y: 0.2, x: 0.1 }], name: 'area' }],
      ),
    ).toBe(true)
    expect(sameMapValue([{ x: 0.1, y: 0.2 }], [{ x: 0.2, y: 0.1 }])).toBe(false)
  })
  it('filters direct group devices even when the list contains other groups, deleted or foreign devices', async () => {
    devices.push(
      { ...device('foreign', ['a']), tenantId: 'other' },
      { ...device('deleted', ['a']), deletedAt: 'today' },
    )
    expect((await loadGroupMap('a', 'w')).devices.map((d) => d.id)).toEqual(['shared'])
    expect(listAll).toHaveBeenCalledWith(
      'entities',
      expect.objectContaining({ parentGroupId: 'a', tenantId: 'w' }),
    )
  })
  it('keeps the same device independent on two maps without modifying the device or associations', async () => {
    await saveGroupMap(clone(records.a), 'w', { position: { x: 0.1, y: 0.2 } }, 'shared')
    await saveGroupMap(clone(records.b), 'w', { position: { x: 0.6, y: 0.7 } }, 'shared')
    expect(groupMap(records.a).positions.shared).toEqual({ x: 0.1, y: 0.2 })
    expect(groupMap(records.b).positions.shared).toEqual({ x: 0.6, y: 0.7 })
    expect(devices[0].attributes.position).toEqual({ x: 0.9, y: 0.9 })
    expect(
      runOperation.mock.calls.filter(([op]) => !['group', 'entity'].includes(op)).map(([op]) => op),
    ).toEqual(['updateGroup', 'updateGroup'])
  })
  it('preserves geography and newer unrelated metadata while saving the map', async () => {
    const original = clone(records.a)
    records.a.attributes.metadata.note = 'new note'
    records.a.attributes.tags.push('new tag')
    await saveGroupMap(original, 'w', { background: 'https://example.com/map.png', partitions: [] })
    expect(records.a.attributes.metadata).toMatchObject({
      location: { latitude: 1 },
      perimeter: { type: 'polygon' },
      note: 'new note',
    })
    expect(records.a.attributes.tags).toEqual(['keep', 'new tag'])
  })
  it('refuses stale map overwrites', async () => {
    const original = clone(records.a)
    await saveGroupMap(original, 'w', { position: { x: 0.1, y: 0.2 } }, 'shared')
    await expect(saveGroupMap(original, 'w', { background: '', partitions: [] })).rejects.toThrow(
      '元数据已被其他操作修改',
    )
    expect(runOperation.mock.calls.filter(([op]) => op === 'updateGroup')).toHaveLength(1)
  })
  it('refuses removed memberships and foreign groups before writing', async () => {
    devices[0].objectGroupIds = ['b']
    await expect(
      saveGroupMap(records.a, 'w', { position: { x: 0.2, y: 0.2 } }, 'shared'),
    ).rejects.toThrow('设备已不属于此分组')
    await expect(
      saveGroupMap(records.b, 'other', { background: '', partitions: [] }),
    ).rejects.toThrow('分组不可用')
    expect(runOperation.mock.calls.some(([op]) => op === 'updateGroup')).toBe(false)
  })
})

async function render() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/domain/:id/field-map', component: FieldMap }],
  })
  await router.push('/domain/w/field-map')
  await router.isReady()
  wrapper = mount({ template: '<router-view />' }, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}
const button = (w, text) => w.findAll('button').find((b) => b.text() === text)
describe('group map interactions', () => {
  it('requires a decision before discarding drafts, refreshes devices, and shows an empty group honestly', async () => {
    const w = await render()
    const select = w.get('[aria-label="选择分组"]')
    await select.setValue('a')
    await flushPromises()
    expect(w.text()).toContain('本组设备：1')
    await w.get('input[type="url"]').setValue('https://example.com/draft.png')
    await select.setValue('b')
    await flushPromises()
    expect(w.find('[role="alertdialog"]').exists()).toBe(true)
    await button(w, '继续编辑').trigger('click')
    expect(w.get('input[type="url"]').element.value).toContain('draft.png')
    expect(select.element.value).toBe('a')
    await select.setValue('b')
    await button(w, '放弃并继续').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('本组设备：2')
    expect(w.get('input[type="url"]').element.value).toBe('')
    devices = devices.filter((d) => d.id !== 'only-b')
    await button(w, '刷新').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('本组设备：1')
    await select.setValue('empty')
    await flushPromises()
    expect(w.text()).toContain('此分组尚未关联设备')
    expect(w.findAll('svg circle')).toHaveLength(0)
  })
  it('saves a point immediately without accidentally saving the background draft, then saves the draft separately', async () => {
    const w = await render()
    await w.get('[aria-label="选择分组"]').setValue('a')
    await flushPromises()
    await w.get('input[type="url"]').setValue('https://example.com/draft.png')
    await w.get('[aria-label="操作模式"]').setValue('device')
    await w.get('[aria-label="选择设备"]').setValue('shared')
    const svg = w.get('svg')
    svg.element.createSVGPoint = () => ({ matrixTransform: () => ({ x: 200, y: 325 }) })
    svg.element.getScreenCTM = () => ({ inverse: () => ({}) })
    await svg.trigger('click')
    await flushPromises()
    expect(groupMap(records.a).positions.shared).toEqual({ x: 0.2, y: 0.5 })
    expect(groupMap(records.a).background).toBe('')
    expect(w.text()).toContain('底图或分区尚未保存')
    await button(w, '保存底图与分区').trigger('click')
    await flushPromises()
    expect(groupMap(records.a).background).toBe('https://example.com/draft.png')
    expect(groupMap(records.a).positions.shared).toEqual({ x: 0.2, y: 0.5 })
  })
})
