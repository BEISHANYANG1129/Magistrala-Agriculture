<template>
  <section>
    <div class="toolbar">
      <button class="quiet" @click="importing = true">{{ tr('上传 CSV', 'Upload CSV') }}</button
      ><button
        @click="() => { 
          copySource = null
          creating = true
         }"
      >
        + {{ tr('创建', 'Create') }}</button
      ><button v-if="parentGroup" class="quiet" @click="openAssign">
        {{ tr('添加已有资源', 'Add existing') }}</button
      ><button class="quiet" :disabled="busy" @click="load">{{ tr('刷新', 'Refresh') }}</button>
    </div>
    <div v-if="selected.length" class="toolbar">
      <strong>{{ tr('已选择', 'Selected') }} {{ selected.length }}</strong
      ><button
        v-for="a in status === 'deleted' ? ['restore'] : ['enable', 'disable', 'delete']"
        :key="a"
        class="quiet"
        @click="openBatch(a)"
      >
        {{ actionLabel(a) }}</button
      ><button class="quiet" @click="selected = []">{{ tr('取消选择', 'Clear selection') }}</button>
    </div>
    <PopoverMenu class="column-picker" :label="tr('显示列', 'View')">
      <template #trigger>{{ tr('显示列', 'View') }}</template>
      <label v-for="c in columns" :key="c.key" class="check"
        ><input type="checkbox" v-model="shown" :value="c.key" />{{ tr(c.zh, c.en) }}</label
      >
    </PopoverMenu>
    <form class="toolbar" @submit.prevent="applyFilters">
      <input
        v-model="search"
        :placeholder="tr('搜索资源', 'Search resource')"
        :aria-label="tr('搜索资源', 'Search resource')"
      /><PopoverMenu :label="tr('搜索字段', 'Search by')"><template #trigger>{{ searchFieldLabel }}</template>
        <div><button v-for="[key, zh, en] in searchFields" :key="key" @click="searchField = key">{{ tr(zh, en) }}</button></div>
      </PopoverMenu><select v-model="sort" :aria-label="tr('排序', 'Sort')" @change="applyFilters">
        <option value="updated_at:desc">{{ tr('更新时间（最新）', 'Updated at (newest)') }}</option>
        <option value="created_at:desc">{{ tr('创建时间（最新）', 'Created at (newest)') }}</option>
        <option value="updated_at:asc">{{ tr('更新时间（最早）', 'Updated at (oldest)') }}</option>
        <option value="created_at:asc">{{ tr('创建时间（最早）', 'Created at (oldest)') }}</option>
        <option value="name:desc">{{ tr('名称（降序）', 'Name (Z–A)') }}</option>
        <option value="name:asc">{{ tr('名称（升序）', 'Name (A–Z)') }}</option></select
      ><select v-model="status" :aria-label="tr('状态', 'Status')" @change="applyFilters">
        <option value="active">{{ tr('状态：已启用', 'Status: Enabled') }}</option>
        <option value="disabled">{{ tr('状态：已禁用', 'Status: Disabled') }}</option>
        <option value="all">{{ tr('全部状态', 'All statuses') }}</option>
        <option value="deleted">{{ tr('已删除', 'Deleted') }}</option></select
      ><button class="quiet">{{ tr('搜索', 'Search') }}</button>
    </form>
    <p v-if="notice" class="success" role="status">{{ notice }}</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>
              <input
                type="checkbox"
                :aria-label="tr('选择当前页', 'Select current page')"
                :checked="selectable.length > 0 && selectable.every((r) => selected.includes(r.id))"
                :indeterminate="
                  selectable.some((r) => selected.includes(r.id)) &&
                  !selectable.every((r) => selected.includes(r.id))
                "
                :disabled="!selectable.length"
                @change="selectPage($event.target.checked)"
              />
            </th>
            <th>{{ tr('名称', 'Name') }}</th>
            <th v-if="isDevice && shown.includes('serial')">{{ tr('序列号', 'Serial') }}</th>
            <th v-if="shown.includes('tags')">{{ tr('标签', 'Tags') }}</th>
            <th v-if="shown.includes('status')">{{ tr('状态', 'Status') }}</th>
            <th v-if="shown.includes('created')">{{ tr('创建时间', 'Created at') }}</th>
            <th v-if="shown.includes('updated')">{{ tr('更新时间', 'Updated at') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in visible" :key="r.id" class="clickable-row" @click="openRecord($event, r)">
            <td>
              <input
                type="checkbox"
                v-model="selected"
                :value="r.id"
                :disabled="r.managedBy === 'config'"
                :aria-label="tr('选择', 'Select') + ' ' + r.name"
              />
            </td>
            <td>
              <router-link
                :to="{ path: `/domain/${workspace}/${section}/${r.id}`, query: route.query }"
                >{{ r.name || r.id }}</router-link
              >
            </td>
            <td v-if="isDevice && shown.includes('serial')">{{ r.externalId || '—' }}</td>
            <td v-if="shown.includes('tags')">
              <span v-for="t in tagsOf(r)" :key="t" class="badge">{{ t }}</span>
            </td>
            <td v-if="shown.includes('status')">{{ statusLabel(statusOf(r)) }}</td>
            <td v-if="shown.includes('created')">{{ date(r.createdAt) }}</td>
            <td v-if="shown.includes('updated')">
              {{ r.updatedAt ? date(r.updatedAt) : tr('尚未更新', 'Not updated yet') }}
            </td>
            <td>
              <PopoverMenu class="row-menu" :label="tr('操作菜单', 'Open menu')">
                <template #trigger>⋯</template>
                <div>
                  <router-link
                    :to="{ path: `/domain/${workspace}/${section}/${r.id}`, query: route.query }"
                    >{{ tr('查看详情', 'View') }}</router-link
                  >
                  <button @click="copyId(r)">{{ tr('复制 ID', 'Copy ID') }}</button>
                  <button
                    :disabled="r.managedBy === 'config'"
                    @click="() => { 
                      quickEdit = r
                      quickName = r.name
                      quickError = ''
                     }"
                  >
                    {{ tr('编辑名称', 'Edit Name') }}
                  </button>
                  <button
                    @click="() => { 
                      copySource = r
                      creating = true
                     }"
                  >
                    {{ tr('创建副本', 'Make a copy') }}
                  </button>
                  <button @click="sharing = r">{{ tr('分享', 'Share') }}</button>
                  <button v-if="section !== 'groups'" @click="connecting = r">
                    {{
                      isDevice
                        ? tr('连接通道', 'Connect to Channel')
                        : tr('连接设备', 'Connect to Device')
                    }}
                  </button>
                  <button
                    v-if="isDevice"
                    :disabled="r.managedBy === 'config'"
                    @click="gatewayTarget = r"
                  >
                    {{
                      r.attributes?.is_gateway
                        ? tr('取消网关', 'Unmake gateway')
                        : tr('设为网关', 'Make gateway')
                    }}
                  </button>
                  <button
                    v-for="action in statusOf(r) === 'deleted'
                      ? ['restore']
                      : [statusOf(r) === 'active' ? 'disable' : 'enable', 'delete']"
                    :key="action"
                    :disabled="r.managedBy === 'config'"
                    @click="openBatch(action, [r])"
                  >
                    {{ actionLabel(action) }}
                  </button>
                  <button v-if="parentGroup" class="quiet" @click="removing = r">
                    {{ tr('移出分组', 'Remove from group') }}
                  </button>
                </div>
              </PopoverMenu>
            </td>
          </tr>
          <tr v-if="!visible.length">
            <td :colspan="isDevice ? 7 : 6">
              {{
                busy
                  ? tr('读取中…', 'Loading…')
                  : tr('没有符合条件的记录。', 'No matching records.')
              }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <TablePagination
      v-model:page="page"
      v-model:size="size"
      :total="filtered.length"
      :selected="selected.length"
    />
    <ResourceConnections
      v-if="connecting"
      dialog-only
      :workspace="workspace"
      :section="section"
      :record="connecting"
      @close="connecting = null"
    />
    <ResourceAccess
      v-if="sharing"
      dialog-only
      tab="users"
      :workspace="workspace"
      :section="section"
      :record="sharing"
      @close="sharing = null"
    />
    <ModalDialog v-if="gatewayTarget" confirmation :busy="quickBusy" @close="gatewayTarget = null">
      <div class="modal-card compact-modal">
        <h2>
          {{
            gatewayTarget.attributes?.is_gateway
              ? tr('取消网关', 'Unmake gateway')
              : tr('设为网关', 'Make gateway')
          }}
        </h2>
        <p>{{ gatewayTarget.name }}</p>
        <p v-if="quickError" class="error" role="alert">{{ quickError }}</p>
        <div class="actions">
          <button class="quiet" :disabled="quickBusy" @click="gatewayTarget = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="quickBusy" @click="changeGateway">{{ tr('确认', 'Confirm') }}</button>
        </div>
      </div>
    </ModalDialog>
    <ModalDialog v-if="quickEdit" :busy="quickBusy" @close="quickEdit = null">
      <form class="modal-card compact-modal" @submit.prevent="saveName">
        <h2>{{ tr('编辑名称', 'Edit Name') }}</h2>
        <label class="field"
          >{{ tr('名称', 'Name') }} *<input v-model="quickName" required
        /></label>
        <p v-if="quickError" class="error" role="alert">{{ quickError }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="quickBusy" @click="quickEdit = null">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="quickBusy">{{ tr('更新', 'Update') }}</button>
        </div>
      </form>
    </ModalDialog>
    <CsvImport
      v-if="importing"
      :section="section"
      :workspace="workspace"
      :parent-group="parentGroup"
      @close="importing = false"
      @updated="load"
    />
    <BatchActions
      v-if="batch"
      :records="batch.records"
      :title="actionLabel(batch.action)"
      :perform="batch.perform"
      @close="batch = null"
      @updated="batchUpdated"
    />
    <ModalDialog v-if="creating" @close="creating = false">
      <div class="modal-card compact-modal">
        <ResourceCreate
          :section="section"
          :workspace="workspace"
          :parent-group="parentGroup"
          :initial="copySource"
          @close="creating = false"
          @saved="created"
        />
      </div>
    </ModalDialog>
    <ModalDialog v-if="assigning" :busy="busy" @close="assigning = false">
      <form class="modal-card compact-modal" @submit.prevent="assign">
        <h2>{{ tr('添加已有资源', 'Add existing resources') }}</h2>
        <label class="field"
          >{{ tr('选择资源', 'Select resources')
          }}<select multiple size="7" v-model="selectedIds" required>
            <option v-for="r in available" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select></label
        >
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="assigning = false">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy">{{ tr('添加', 'Add') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="removing" :busy="busy" confirmation @close="removing = null">
      <div class="modal-card compact-modal">
        <h2>{{ tr('移出分组', 'Remove from group') }}</h2>
        <p>{{ removing.name }}</p>
        <p>
          {{
            tr(
              '只移除分组关联，保留资源本身。',
              'The resource will be kept; only its group membership is removed.',
            )
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button class="quiet" :disabled="busy" @click="removing = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy" @click="remove">{{ tr('确认', 'Confirm') }}</button>
        </div>
      </div>
    </ModalDialog>
  </section>
</template>
<script setup>
import PopoverMenu from './PopoverMenu.vue'
import TablePagination from './TablePagination.vue'

import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tr, language } from '../i18n'
import { runOperation, errorText } from '../api/platform'
import {
  readResources,
  readResource,
  resourceConfig,
  statusOf,
  tagsOf,
} from '../api/resourceWorkflow'
import ResourceCreate from './ResourceCreate.vue'
import ResourceConnections from './ResourceConnections.vue'
import ResourceAccess from './ResourceAccess.vue'
import CsvImport from './CsvImport.vue'
import BatchActions from './BatchActions.vue'
import { useListLocation } from '../api/listLocation'
import { batchResource } from '../api/batchWorkflow'
const props = defineProps({ section: String, workspace: String, parentGroup: String }),
  route = useRoute(),
  router = useRouter()
const searchField = ref(String(route.query.field || 'name'))
const searchFields = computed(() => [['name', '名称', 'Name'], ['tag', '标签', 'Tag'], ['id', 'ID', 'ID'], ...(isDevice.value ? [['serial', '序列号', 'Serial']] : [])])
const searchFieldLabel = computed(() => { const field = searchFields.value.find(f => f[0] === searchField.value) || searchFields.value[0]; return tr(field[1], field[2]) })
const records = ref([]),
  search = ref(String(route.query.q || '')),
  sort = ref(String(route.query.sort || 'updated_at:desc')),
  status = ref(String(route.query.status || 'active')),
  size = ref(10),
  page = ref(1),
  busy = ref(false),
  error = ref(''),
  creating = ref(false),
  assigning = ref(false),
  available = ref([]),
  selectedIds = ref([]),
  removing = ref(null)
const sharing = ref(null),
  connecting = ref(null),
  gatewayTarget = ref(null)
const notice = ref(''),
  quickEdit = ref(null),
  quickName = ref(''),
  quickBusy = ref(false),
  quickError = ref(''),
  copySource = ref(null)
const selected = ref([]),
  importing = ref(false),
  batch = ref(null),
  shown = ref(['serial', 'tags', 'status', 'created', 'updated'])
const columns = [
  { key: 'serial', zh: '序列号', en: 'Serial' },
  { key: 'tags', zh: '标签', en: 'Tags' },
  { key: 'status', zh: '状态', en: 'Status' },
  { key: 'created', zh: '创建时间', en: 'Created at' },
  { key: 'updated', zh: '更新时间', en: 'Updated at' },
]
const actionLabel = (a) =>
  ({
    enable: tr('启用', 'Enable'),
    disable: tr('禁用', 'Disable'),
    delete: tr('删除', 'Delete'),
    restore: tr('恢复', 'Restore'),
  })[a]
function selectPage(checked) {
  const ids = visible.value.filter((r) => r.managedBy !== 'config').map((r) => r.id)
  selected.value = checked
    ? [...new Set([...selected.value, ...ids])]
    : selected.value.filter((id) => !ids.includes(id))
}
function openBatch(action, rows) {
  const section = props.section,
    workspace = props.workspace
  batch.value = {
    action,
    records: rows || records.value.filter((r) => selected.value.includes(r.id)),
    perform: (r) => batchResource(action, section, workspace, r),
  }
}
function batchUpdated(ids) {
  selected.value = selected.value.filter((id) => !ids.includes(id))
  load()
}
const isDevice = computed(() => ['devices', 'gateways'].includes(props.section))
const filtered = computed(() => {
  let data = records.value.filter(
    (r) =>
      (status.value === 'all' || statusOf(r) === status.value) &&
      (!search.value || String(searchField.value === 'id' ? r.id : searchField.value === 'serial' ? r.externalId || '' : searchField.value === 'tag' ? tagsOf(r).join(' ') : r.name || '').toLowerCase().includes(search.value.toLowerCase())),
  )
  const [field, dir] = sort.value.split(':')
  const key = field === 'name' ? 'name' : field === 'created_at' ? 'createdAt' : 'updatedAt'
  return data.sort(
    (a, b) =>
      String(a[key] || a.createdAt || '').localeCompare(
        String(b[key] || b.createdAt || ''),
        language.value,
      ) * (dir === 'asc' ? 1 : -1),
  )
})
const visible = computed(() =>
  filtered.value.slice((page.value - 1) * size.value, page.value * size.value),
)
const selectable = computed(() => visible.value.filter((r) => r.managedBy !== 'config'))
const restoringList = useListLocation({ page, size, filters: { q: [search, ''], field: [searchField, 'name'], sort: [sort, 'updated_at:desc'], status: [status, 'active'] } })
watch(search, () => { selected.value = [] })
watch(status, (value, old) => { if ((value === 'deleted') !== (old === 'deleted')) load() })
function openRecord(event, record) { if (!event.target.closest('a, button, input, select, label')) router.push({ path: `/domain/${props.workspace}/${props.section}/${record.id}`, query: route.query }) }
async function copyId(record) {
  try {
    await navigator.clipboard.writeText(record.id)
    notice.value = tr('已复制 ID。', 'ID copied.')
  } catch (e) {
    error.value = errorText(e)
  }
}
async function changeGateway() {
  if (quickBusy.value) return
  quickBusy.value = true
  quickError.value = ''
  try {
    const record = await readResource(props.section, gatewayTarget.value.id, props.workspace)
    await runOperation('updateEntity', {
      id: record.id,
      input: { attributes: { ...record.attributes, is_gateway: !record.attributes?.is_gateway } },
    })
    gatewayTarget.value = null
    await load()
  } catch (e) {
    quickError.value = errorText(e)
  } finally {
    quickBusy.value = false
  }
}
async function saveName() {
  if (quickBusy.value) return
  quickBusy.value = true
  quickError.value = ''
  try {
    const latest = await readResource(props.section, quickEdit.value.id, props.workspace)
    const name = quickName.value.trim()
    if (!name) throw new Error(tr('请输入名称。', 'Enter a name.'))
    await runOperation('update' + resourceConfig[props.section].type, {
      id: latest.id,
      input: { name },
    })
    quickEdit.value = null
    notice.value = tr('名称已更新。', 'Name updated.')
    await load()
  } catch (e) {
    quickError.value = errorText(e)
  } finally {
    quickBusy.value = false
  }
}
const date = (v) => new Date(v).toLocaleString(language.value === 'zh' ? 'zh-CN' : 'en-GB')
const statusLabel = (s) =>
  s === 'active'
    ? tr('已启用', 'Enabled')
    : s === 'deleted'
      ? tr('已删除', 'Deleted')
      : tr('已禁用', 'Disabled')
let generation = 0
async function load() {
  const seq = ++generation
  busy.value = true
  error.value = ''
  try {
    const rows = await readResources(props.section, props.workspace, {
      deleted: status.value === 'deleted' ? 'deleted' : 'live',
      ...(props.parentGroup ? { parentGroupId: props.parentGroup } : {}),
    })
    if (seq === generation)
      records.value = props.parentGroup
        ? rows.filter((r) => r.objectGroupIds?.includes(props.parentGroup))
        : rows
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
function applyFilters() { selected.value = []; page.value = 1 }

watch(
  () => [props.section, props.workspace, props.parentGroup],
  () => {
    if (!restoringList.value) page.value = 1
    selected.value = []
    batch.value = null
    importing.value = false
    creating.value = false
    assigning.value = false
    load()
  },
  { immediate: true },
)
async function created() {
  creating.value = false
  await load()
}
async function openAssign() {
  error.value = ''
  try {
    const all = await readResources(props.section, props.workspace)
    available.value = all.filter((r) => !r.objectGroupIds?.includes(props.parentGroup))
    selectedIds.value = []
    assigning.value = true
  } catch (e) {
    error.value = errorText(e)
  }
}
async function assign() {
  busy.value = true
  error.value = ''
  const done = []
  try {
    const c = resourceConfig[props.section]
    for (const id of selectedIds.value) {
      await runOperation(`add${c.type}ToObjectGroup`, {
        [c.type === 'Entity' ? 'entityId' : 'resourceId']: id,
        objectGroupId: props.parentGroup,
      })
      done.push(id)
    }
    assigning.value = false
    await load()
  } catch (e) {
    selectedIds.value = selectedIds.value.filter((id) => !done.includes(id))
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function remove() {
  busy.value = true
  error.value = ''
  try {
    const c = resourceConfig[props.section]
    await runOperation(`remove${c.type}FromObjectGroup`, {
      [c.type === 'Entity' ? 'entityId' : 'resourceId']: removing.value.id,
      objectGroupId: props.parentGroup,
    })
    removing.value = null
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
