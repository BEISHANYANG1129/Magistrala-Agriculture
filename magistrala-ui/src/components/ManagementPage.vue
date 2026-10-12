<template>
  <section>
    <header class="page-heading">
      <div>
        <p class="eyebrow">MAGISTRALA</p>
        <h1>
          {{
            selected
              ? selected.name || selected.title || sectionLabel(section)
              : sectionLabel(section)
          }}
        </h1>
      </div>
      <div class="actions">
        <button v-if="selected" class="quiet" @click="closeDetail">
          ← {{ tr('返回列表', 'Back to list') }}</button
        ><button class="quiet" @click="refresh" :disabled="loading">
          {{ tr('刷新', 'Refresh') }}</button
        ><button v-if="createOp && !selected" @click="open(createOp)">
          + {{ tr('创建', 'Create') }}
        </button>
      </div>
    </header>
    <template v-if="!selected">
      <div class="toolbar">
        <input
          v-model="search"
          :placeholder="tr('搜索名称或编号', 'Search name or ID')"
          @keyup.enter="
            () => {
              offset = 0
              load()
            }
          "
        /><button
          class="quiet"
          @click="
            () => {
              offset = 0
              load()
            }
          "
        >
          {{ tr('搜索', 'Search') }}</button
        ><select
          v-model.number="limit"
          @change="
            () => {
              offset = 0
              load()
            }
          "
        >
          <option :value="10">10</option>
          <option :value="20">20</option>
          <option :value="50">50</option>
        </select>
      </div>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="loading" role="status">{{ tr('正在读取平台数据…', 'Loading platform data…') }}</p>
      <div class="table-wrap" v-else>
        <table>
          <thead>
            <tr>
              <th>{{ tr('名称 / 编号', 'Name / ID') }}</th>
              <th>{{ tr('状态 / 类型', 'Status / type') }}</th>
              <th>{{ tr('更新时间', 'Updated') }}</th>
              <th>{{ tr('操作', 'Actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in records" :key="row.id || row.credentialId">
              <td>
                <button class="table-name" @click="show(row)">
                  {{
                    row.name ||
                    row.displayName ||
                    row.title ||
                    row.invitee_email ||
                    row.inviteeEmail ||
                    row.id ||
                    row.credentialId
                  }}</button
                ><small>{{ row.id || row.credentialId }}</small>
              </td>
              <td>
                <span class="badge">{{
                  label(String(row.status ?? row.kind ?? row.state ?? '—'))
                }}</span>
              </td>
              <td>
                {{ formatDate(row.updatedAt || row.updated_at || row.createdAt || row.created_at) }}
              </td>
              <td class="actions">
                <button class="quiet" @click="show(row)">{{ tr('详情', 'Details') }}</button
                ><button
                  v-if="editOp"
                  class="quiet"
                  :disabled="row.managedBy === 'config'"
                  @click="edit(row)"
                >
                  {{ tr('编辑', 'Edit') }}</button
                ><router-link
                  v-if="section === 'channels'"
                  :to="`/domain/${workspace}/messages?channel=${row.id}`"
                  >{{ tr('消息', 'Messages') }}</router-link
                >
              </td>
            </tr>
            <tr v-if="!records.length">
              <td colspan="4">{{ tr('没有符合条件的记录。', 'No matching records.') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <TablePagination :page="Math.floor(offset / limit) + 1" :size="limit" :total="total" :busy="loading" @update:page="changePage" @update:size="changeSize" />
    </template>
    <p v-if="selected && error" class="error" role="alert">{{ error }}</p>
    <div
      v-if="selected"
      class="detail-tabs"
      role="tablist"
      :aria-label="tr('资源详情', 'Resource details')"
    >
      <button
        role="tab"
        :aria-selected="detailTab === 'details'"
        :class="['quiet', { active: detailTab === 'details' }]"
        @click="detailTab = 'details'"
      >
        <NavIcon name="info" />{{ tr('基本信息', 'Details') }}
      </button>
      <button
        v-if="['devices', 'channels', 'gateways'].includes(section)"
        role="tab"
        :aria-selected="detailTab === 'connections'"
        :class="['quiet', { active: detailTab === 'connections' }]"
        @click="detailTab = 'connections'"
      >
        <NavIcon name="channels" />{{ tr('连接管理', 'Connections') }}
      </button>
      <router-link
        v-if="section === 'channels'"
        class="button quiet"
        :to="`/domain/${workspace}/messages?channel=${selected.id}`"
        ><NavIcon name="messages" /> {{ tr('消息', 'Messages') }}</router-link
      >
    </div>
    <div v-if="selected && detailTab === 'details'" class="panel">
      <div class="actions">
        <h2>{{ tr('详细信息', 'Details') }}</h2>
        <button
          v-if="editOp"
          class="quiet"
          :disabled="selected.managedBy === 'config'"
          @click="edit(selected)"
        >
          {{ tr('编辑', 'Edit') }}
        </button>
      </div>
      <dl class="record">
        <template v-for="[k, v] in detailFields" :key="k"
          ><dt>{{ label(k) }}</dt>
          <dd>
            <details v-if="v && typeof v === 'object'">
              <summary>{{ tr('查看内容', 'View content') }}</summary>
              <pre>{{ JSON.stringify(v, null, 2) }}</pre>
            </details>
            <span v-else>{{ ['status', 'kind', 'state'].includes(k) ? label(String(v)) : v }}</span>
          </dd></template
        >
      </dl>
    </div>
    <ConnectionsPanel
      v-if="
        selected &&
        detailTab === 'connections' &&
        ['devices', 'channels', 'gateways'].includes(section)
      "
      :workspace="workspace"
      :record="selected"
      :section="section"
    />
    <DashboardEditor
      v-if="section === 'dashboards' && selected"
      :record="selected"
      :workspace="workspace"
    />
    <details class="panel advanced-operations" :open="section === 'platform'">
      <summary>{{ tr('全部操作', 'All operations') }} · {{ available.length }}</summary>
      <input v-model="opSearch" :placeholder="tr('查找操作', 'Find an operation')" />
      <div class="operation-grid">
        <button v-for="op in filteredOps" :key="op.id" class="quiet" @click="open(op)">
          {{ label(op.name) }}
        </button>
      </div>
    </details>
    <ModalDialog v-if="activeOp" @close="activeOp = null">
      <div class="modal-card">
        <OperationForm
          :op="activeOp"
          :initial="initial"
          :choices="choices"
          @done="load"
          @close="activeOp = null"
        />
      </div>
    </ModalDialog>
  </section>
</template>
<script setup>
import TablePagination from './TablePagination.vue'
import ModalDialog from './ModalDialog.vue'
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import NavIcon from './NavIcon.vue'
import { tr, label, sectionLabel, language } from '../i18n'
import { runOperation, operation, errorText, request, items, listAll } from '../api/platform'
import { pages, opsFor, defaultsFor, mainRest } from '../api/catalog'
import OperationForm from './OperationForm.vue'
import ConnectionsPanel from './ConnectionsPanel.vue'
import DashboardEditor from './DashboardEditor.vue'
const router = useRouter(),
  detailTab = ref('details')
const props = defineProps({ sectionOverride: String })
const route = useRoute()
const workspace = computed(() => String(route.params.id || '')),
  section = computed(() => props.sectionOverride || String(route.params.section || 'info'))
const config = computed(() => pages[section.value] || pages.platform)
const records = ref([]),
  total = ref(0),
  limit = ref(20),
  offset = ref(0),
  search = ref(''),
  opSearch = ref(''),
  loading = ref(false),
  error = ref(''),
  selected = ref(null),
  activeOp = ref(null),
  initial = ref({}),
  choices = ref({})
let sequence = 0,
  controller
const detailFields = computed(() => {
  const priority = [
    'name',
    'title',
    'id',
    'status',
    'kind',
    'description',
    'alias',
    'tenantId',
    'attributes',
    'metadata',
    'createdAt',
    'updatedAt',
  ]
  return Object.entries(selected.value || {})
    .filter(
      ([, v]) => v !== null && v !== undefined && v !== '' && !(Array.isArray(v) && !v.length),
    )
    .sort(
      ([a], [b]) =>
        (priority.includes(a) ? priority.indexOf(a) : 99) -
        (priority.includes(b) ? priority.indexOf(b) : 99),
    )
})
const available = computed(() => opsFor(section.value))
const filteredOps = computed(() =>
  available.value.filter((o) =>
    (label(o.name) + ' ' + o.name).toLowerCase().includes(opSearch.value.toLowerCase()),
  ),
)
const createOp = computed(() =>
  config.value.rest
    ? asRest(mainRest(section.value, 'POST'))
    : config.value.create
      ? { ...operation(config.value.create), id: config.value.create }
      : null,
)
const editOp = computed(() =>
  config.value.rest
    ? asRest(mainRest(section.value, 'PATCH', true))
    : config.value.edit
      ? { ...operation(config.value.edit), id: config.value.edit }
      : null,
)
function asRest(op) {
  return op ? { ...op, kind: 'rest' } : null
}
async function load() {
  const seq = ++sequence
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  try {
    const p = config.value
    let data
    if (p.rest) {
      data = await request(
        `/api/platform/${p.service}/${encodeURIComponent(workspace.value)}/${p.collection}?limit=${limit.value}&offset=${offset.value}${search.value ? '&name=' + encodeURIComponent(search.value) : ''}`,
        { signal: controller.signal },
      )
      if (seq !== sequence) return
      records.value = data?.[p.key] || data?.reports || data?.configurations || []
      total.value = data?.total ?? records.value.length
    } else {
      const op = operation(p.list)
      const supported = new Set(op.args.map((a) => a.name))
      const candidate = {
        ...(p.defaults || {}),
        tenantId: workspace.value,
        id: p.single ? workspace.value : undefined,
        limit: limit.value,
        offset: offset.value,
        q: search.value || undefined,
      }
      if (p.unscoped) delete candidate.tenantId
      const vars = Object.fromEntries(
        Object.entries(candidate).filter(([k, v]) => supported.has(k) && v !== undefined),
      )
      data = await runOperation(p.list, vars, { signal: controller.signal })
      if (seq !== sequence) return
      records.value = p.single ? [data] : items(data)
      total.value = p.single ? 1 : (data?.total ?? records.value.length)
    }
  } catch (e) {
    if (seq === sequence) {
      error.value = errorText(e)
      records.value = []
      total.value = 0
    }
  } finally {
    if (seq === sequence) loading.value = false
  }
}
function formatDate(value) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(language.value === 'zh' ? 'zh-CN' : 'en-GB', {year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'})
}
function closeDetail() {
  selected.value = null
  detailTab.value = 'details'
  if (route.params.recordId) router.push(`/domain/${workspace.value}/${section.value}`)
}
async function refresh() {
  const record = selected.value
  await load()
  if (record) await show(record)
}
async function show(row) {
  const generation = sequence
  detailTab.value = 'details'
  selected.value = row
  try {
    const p = config.value
    let result = row
    if (p.detail) result = await runOperation(p.detail, { id: row.id })
    else if (p.rest && row.id)
      result = await request(
        `/api/platform/${p.service}/${encodeURIComponent(workspace.value)}/${p.collection}/${encodeURIComponent(row.id)}`,
      )
    if (generation !== sequence) return false
    if (result.tenantId && result.tenantId !== workspace.value)
      throw new Error(tr('记录不属于当前工作区。', 'Record is outside the current workspace.'))
    selected.value = result
    return true
  } catch (e) {
    if (generation === sequence) error.value = errorText(e)
    return false
  }
}
async function edit(row) {
  if (!(await show(row))) return
  open(editOp.value, selected.value || row)
}
async function open(op, row = {}) {
  initial.value = defaultsFor(op, workspace.value, row, section.value)
  activeOp.value = op
  try {
    const [entities, channels, groups, roles, profiles] = await Promise.all([
      listAll('entities', { tenantId: workspace.value }),
      listAll('resources', { tenantId: workspace.value, kind: 'channel' }),
      listAll('groups', { tenantId: workspace.value }),
      listAll('roles', { tenantId: workspace.value }),
      listAll('profiles', { tenantId: workspace.value }),
    ])
    choices.value = {
      entityId: entities,
      subjectId: entities,
      ownerId: entities,
      ownedId: entities,
      resourceId: channels,
      channelID: channels,
      input_channel: channels,
      groupId: groups,
      objectGroupId: groups,
      parentId: groups,
      roleId: roles,
      profileId: profiles,
    }
  } catch {
    /* The operation still accepts explicit IDs when a selector is not permitted. */
  }
}
watch(
  () => [section.value, workspace.value, route.params.recordId],
  async () => {
    offset.value = 0
    search.value = ''
    selected.value = null
    activeOp.value = null
    await load()
    if (route.params.recordId) await show({ id: String(route.params.recordId) })
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  sequence++
  controller?.abort()
})
function changePage(page) { const next = (page - 1) * limit.value; if (next !== offset.value) { offset.value = next; load() } }
function changeSize(size) { limit.value = size; offset.value = 0; load() }
</script>
