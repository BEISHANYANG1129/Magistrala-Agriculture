// Modal keyboard/focus behavior is exercised with real Teleport in interactionParity.spec.js.
vi.mock('../components/ModalDialog.vue', () => ({ default: { template: '<div class="modal"><slot /></div>' } }))
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { runOperation, request, listAll } from '../api/platform'
import { parseCsv, csvJobs, importJob, batchResource } from '../api/batchWorkflow'
import { editedMetadata } from '../api/editedMetadata'
import { permissionScope, actionFilter } from '../api/permissionScopes'
import { viewBody, viewSettings, viewCell, readerViewParams } from '../api/messageViews'
import BatchActions from '../components/BatchActions.vue'
import CsvImport from '../components/CsvImport.vue'
import RuleCanvas from '../components/RuleCanvas.vue'
import ReportEditor from '../components/ReportEditor.vue'
import ProfileEditor from '../components/ProfileEditor.vue'
import BootstrapEditor from '../components/BootstrapEditor.vue'
import WorkspaceRoleEditor from '../components/WorkspaceRoleEditor.vue'

vi.mock('../api/platform', () => ({
  runOperation: vi.fn(),
  request: vi.fn(),
  listAll: vi.fn(),
  gql: vi.fn(),
  errorText: (e) => e.message,
}))
const button = (w, text) => w.findAll('button').find((b) => b.text() === text)
beforeEach(() => {
  vi.resetAllMocks()
  listAll.mockResolvedValue([])
  request.mockResolvedValue({ profiles: [] })
  runOperation.mockResolvedValue({})
})

describe('CSV and batch operations', () => {
  it('parses BOM, escaped quotes, CRLF and embedded newlines', () => {
    expect(parseCsv('\uFEFFName,Tags\r\n"sensor, \"\"A\"\"","north;soil\nwet"')).toEqual([
      ['Name', 'Tags'],
      ['sensor, "A"', 'north;soil\nwet'],
    ])
    expect(() => parseCsv('Name\n"broken')).toThrow('Unclosed')
  })
  it('rejects invalid rows and duplicate headers before writes', () => {
    expect(csvJobs('Name,Metadata\nSensor,[]', 'devices')[0].status).toBe('invalid')
    expect(() => csvJobs('Name,Name\nx,y', 'devices')).toThrow('headers')
    expect(csvJobs('Name,Route\nSoil,bad/path', 'channels')[0].status).toBe('invalid')
    expect(runOperation).not.toHaveBeenCalled()
  })
  it('retries a failed key step without duplicating the device; retains workspace and group', async () => {
    const job = csvJobs('Name,Key\nSoil,demo-key', 'devices')[0]
    let attempts = 0
    runOperation.mockImplementation(async (op) => {
      if (op === 'createEntity') return { id: 'new-device' }
      if (op === 'createSharedKey' && attempts++ === 0)
        throw Object.assign(new Error('rejected'), { status: 403 })
      return {}
    })
    await expect(importJob(job, 'devices', 'workspace-a', 'group-a')).rejects.toThrow('rejected')
    await importJob(job, 'devices', 'workspace-a', 'group-a')
    expect(runOperation.mock.calls.filter(([n]) => n === 'createEntity')).toHaveLength(1)
    expect(runOperation.mock.calls[0][1].input.tenantId).toBe('workspace-a')
    expect(runOperation).toHaveBeenLastCalledWith('addEntityToObjectGroup', {
      entityId: 'new-device',
      objectGroupId: 'group-a',
    })
    expect(job.values.key).toBeUndefined()
  })
  it('blocks batch operations across workspace boundaries and on config-managed resources', async () => {
    runOperation.mockResolvedValue({ id: 'device', tenantId: 'other', managedBy: 'api' })
    await expect(batchResource('delete', 'devices', 'w', { id: 'device' })).rejects.toThrow()
    runOperation.mockResolvedValue({ id: 'device', tenantId: 'w', managedBy: 'config' })
    await expect(batchResource('delete', 'devices', 'w', { id: 'device' })).rejects.toThrow(
      'configuration',
    )
    expect(runOperation.mock.calls.some(([n]) => n === 'deleteEntity')).toBe(false)
  })
  it('skips successes when retrying a rejected batch item', async () => {
    let fail = true
    const perform = vi.fn(async (r) => {
      if (r.id === 'b' && fail) {
        fail = false
        throw Object.assign(new Error('denied'), { status: 403 })
      }
    })
    const w = mount(BatchActions, {
      props: { records: [{ id: 'a' }, { id: 'b' }], title: 'Delete', perform },
    })
    await button(w, '确认 Delete').trigger('click')
    await flushPromises()
    await button(w, '确认 Delete').trigger('click')
    await flushPromises()
    expect(perform.mock.calls.map(([r]) => r.id)).toEqual(['a', 'b', 'b'])
    w.unmount()
  })
  it('stops and blocks retry after an ambiguous timeout', async () => {
    const perform = vi.fn().mockRejectedValue(new Error('timeout'))
    const w = mount(BatchActions, {
      props: { records: [{ id: 'a' }, { id: 'b' }], title: 'Delete', perform },
    })
    await button(w, '确认 Delete').trigger('click')
    await flushPromises()
    expect(perform).toHaveBeenCalledTimes(1)
    expect(button(w, '确认 Delete').attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('结果待核对')
    w.unmount()
  })
  it('previews CSV locally and does not resubmit an unknown creation result', async () => {
    const w = mount(CsvImport, { props: { section: 'devices', workspace: 'w' } })
    const file = w.find('input[type=file]')
    Object.defineProperty(file.element, 'files', {
      value: [{ size: 30, text: async () => 'Name\nSensor' }],
    })
    await file.trigger('change')
    await flushPromises()
    expect(w.text()).toContain('Sensor')
    expect(runOperation).not.toHaveBeenCalled()
    runOperation.mockRejectedValue(new Error('timeout'))
    await button(w, '提交导入').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('结果待核对')
    expect(button(w, '继续未完成项').attributes('disabled')).toBeDefined()
    w.unmount()
  })
})

describe('native editor contracts and preserving existing data', () => {
  it('keeps newer dashboard widgets when only settings metadata was edited', () => {
    const original = { widget: { old: true }, note: 'old' }
    expect(
      editedMetadata(
        original,
        { ...original, note: 'new' },
        { widget: { new: true }, note: 'old', other: 123 },
      ),
    ).toEqual({ widget: { new: true }, note: 'new', other: 123 })
    expect(() => editedMetadata({ note: 'old' }, { note: 'mine' }, { note: 'theirs' })).toThrow(
      'changed elsewhere',
    )
  })
  it('moves a canvas node and its edge without changing executable logic', async () => {
    const draft = {
      input_channel: 'c',
      logic: { type: 0, value: 'return message', mode: 'unrestricted' },
      outputs: [{ id: 'o', type: 'save_senml' }],
      metadata: { _agri_ui: { rule_canvas: { logic: { x: 410, y: 50 } } } },
    }
    const initial = JSON.stringify(draft)
    const w = mount(RuleCanvas, { props: { draft, channels: [], outputLabel: (x) => x } })
    await flushPromises()
    const edge = w.findAll('path[marker-end]')[0].attributes('d')
    await w.findAll('.flow-node-handle')[1].trigger('keydown', { key: 'ArrowRight' })
    expect(w.emitted('layout')[0][0].logic).toEqual({ x: 425, y: 50 })
    expect(w.findAll('path[marker-end]')[0].attributes('d')).not.toBe(edge)
    expect(JSON.stringify(draft)).toBe(initial)
    w.unmount()
  })
  it('uses tenant for the workspace itself and object_type for its devices', () => {
    expect(permissionScope('workspace')).toEqual({ scopeMode: 'tenant' })
    expect(permissionScope('device')).toEqual({
      scopeMode: 'object_type',
      objectKind: 'entity',
      objectType: 'entity:device',
    })
    expect(actionFilter('workspace')).toEqual({ objectKind: 'tenant' })
  })
  it('writes the local message-view contract and supports nested JSON columns', () => {
    const original = { type: 'table', config: { official: { keep: true }, columns: ['legacy'] } }
    const settings = viewSettings(original)
    settings.subtopic = 'sensors/soil'
    settings.columns = [{ key: 'payload.soil', label: 'Moisture', enabled: true }]
    const saved = viewBody(original, { name: 'Soil', ...settings })
    expect(saved).toEqual({
      name: 'Soil',
      format: 'json',
      subtopic: 'sensors/soil',
      publishers: [],
      columns: [{ key: 'payload.soil', label: 'Moisture' }],
      position: 0,
    })
    expect(original.config.official).toEqual({ keep: true })
    expect(viewSettings(saved).columns[0].key).toBe('payload.soil')
    expect(viewCell({ payload: { soil: 12 } }, 'payload.soil')).toBe(12)
    expect([...readerViewParams({ ...settings, publishers: ['a', 'b'] })]).toEqual([
      ['format', 'soil'],
      ['subtopic', 'sensors/soil'],
      ['publishers', 'a'],
      ['publishers', 'b'],
    ])
    expect(() => viewBody({}, { name: 'x', ...settings, columns: [] })).toThrow('least one')
  })
  it('creates a device type with displayName and the scoped device kind', async () => {
    const w = mount(ProfileEditor, { props: { record: {}, workspace: 'w' } })
    await flushPromises()
    await w.findAll('input')[0].setValue('Soil sensor')
    await w.findAll('input')[1].setValue('soil')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(runOperation).toHaveBeenCalledWith('createProfile', {
      input: {
        displayName: 'Soil sensor',
        description: '',
        key: 'soil',
        tenantId: 'w',
        objectKind: 'entity',
        kind: 'device',
      },
    })
    w.unmount()
  })
  const report = {
    id: 'r',
    name: 'Daily',
    config: { title: 'Soil', from: '-1h', to: 'now', extra: 'keep' },
    metrics: [{ name: 'soil', channel_id: 'c', device_ids: ['d'] }],
    email: { to: ['a@example.test'], subject: 'Soil' },
    schedule: { recurring: 'none', recurring_period: 0, start_datetime: '0001-01-01T00:00:00Z' },
  }
  it('downloads without sending email and preserves extended config', async () => {
    const w = mount(ReportEditor, { props: { record: report, workspace: 'w' } })
    await flushPromises()
    await button(w, '下载报表').trigger('click')
    await flushPromises()
    expect(request).toHaveBeenCalledTimes(1)
    const [url, options] = request.mock.calls[0]
    expect(url).toBe('/api/platform/reports/w/reports?action=download')
    expect(options.download).toBe(true)
    expect(options.body.config.extra).toBe('keep')
    expect(options.body.email).toBeUndefined()
    w.unmount()
  })
  it('does not submit an unchanged zero-date report schedule', async () => {
    const w = mount(ReportEditor, { props: { record: report, workspace: 'w' } })
    await flushPromises()
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).toHaveBeenCalledTimes(1)
    expect(request.mock.calls[0][0]).toBe('/api/platform/reports/w/reports/configs/r')
    expect(request.mock.calls[0][1].method).toBe('PATCH')
    w.unmount()
  })
  it('validates a future schedule before any report write', async () => {
    const w = mount(ReportEditor, { props: { record: report, workspace: 'w' } })
    await flushPromises()
    await w.find('input[type=datetime-local]').setValue('2000-01-01T00:00')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).not.toHaveBeenCalled()
    expect(w.text()).toContain('必须在未来')
    w.unmount()
  })
  it('rejects detaching a bootstrap template before modifying its configuration', async () => {
    const w = mount(BootstrapEditor, {
      props: {
        record: { id: 'e', name: 'Enrollment', external_id: 'serial', profile_id: 'p' },
        workspace: 'w',
        profileMode: false,
      },
    })
    await flushPromises()
    request.mockClear()
    await w.find('select').setValue('')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).not.toHaveBeenCalled()
    expect(w.text()).toContain('必须选择一个模板')
    w.unmount()
  })
  it('previews configuration and bindings without saving, and rejects a malformed snapshot', async () => {
    const w = mount(BootstrapEditor, {
      props: { record: { id: 'p' }, workspace: 'w', profileMode: true },
    })
    await button(w, '渲染预览').trigger('click')
    await w.find('input').setValue('existing-config')
    const fields = w.findAll('textarea')
    await fields[0].setValue('{"external_id":"sensor"}')
    await fields[1].setValue('{"zone":"north"}')
    await fields[2].setValue(
      '[{"slot":"sensor","type":"device","resource_id":"d","snapshot":{"name":"North"}}]',
    )
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).toHaveBeenCalledWith(
      '/api/platform/bootstrap/w/devices/bootstrap/profiles/p/render-preview',
      {
        method: 'POST',
        body: {
          config_id: 'existing-config',
          config: { external_id: 'sensor' },
          render_context: { zone: 'north' },
          bindings: [
            { slot: 'sensor', type: 'device', resource_id: 'd', snapshot: { name: 'North' } },
          ],
        },
      },
    )
    request.mockClear()
    await fields[2].setValue('{}')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(request).not.toHaveBeenCalled()
    expect(w.text()).toContain('绑定快照必须')
    w.unmount()
  })
  it('does not overwrite a role edited by another operation', async () => {
    const w = mount(WorkspaceRoleEditor, {
      props: {
        record: { id: 'role', name: 'Old', permissionBlocks: [{ id: 'b', actions: [] }] },
        workspace: 'w',
      },
    })
    await flushPromises()
    runOperation.mockResolvedValue({ id: 'role', name: 'Changed', permissionBlocks: [{ id: 'b' }] })
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(runOperation.mock.calls.map(([name]) => name)).toEqual(['role'])
    expect(w.text()).toContain('角色已被其他操作修改')
    w.unmount()
  })
})
