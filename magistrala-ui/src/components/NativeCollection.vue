<template>
  <section>
    <header class="page-heading">
      <h1>
        {{
          record?.name ||
          record?.displayName ||
          (section === 'bootstrap-profiles'
            ? tr('引导模板', 'Bootstrap profiles')
            : sectionLabel(section))
        }}
      </h1>
      <router-link v-if="record" class="button quiet" :to="{ path: base, query: route.query }"
        >← {{ tr('返回列表', 'Back to list') }}</router-link
      ><router-link v-else-if="section !== 'alarms'" class="button" :to="base + '/create'"
        >+ {{ tr('创建', 'Create') }}</router-link
      >
    </header>
    <nav v-if="['bootstraps', 'bootstrap-profiles'].includes(section)" class="tabs">
      <router-link class="button quiet" :to="`/domain/${workspace}/bootstraps`">{{
        tr('设备注册', 'Enrollments')
      }}</router-link
      ><router-link class="button quiet" :to="`/domain/${workspace}/bootstrap-profiles`">{{
        tr('配置模板', 'Profiles')
      }}</router-link>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="loading">{{ tr('读取中…', 'Loading…') }}</p>
    <template v-if="!record"
      ><div class="toolbar">
        <input
          v-model="search"
          :aria-label="tr('按名称搜索', 'Search by name')"
          :placeholder="tr('按名称搜索', 'Search by name')"
        /><button class="quiet" @click="load">{{ tr('刷新', 'Refresh') }}</button
        ><button
          v-if="section === 'bootstrap-profiles'"
          class="quiet"
          @click="profileUpload = true"
        >
          {{ tr('上传模板', 'Upload profile') }}</button
        ><select
          v-if="section === 'bootstraps'"
          v-model="bootstrapStatus"
          :aria-label="tr('引导状态', 'Bootstrap status')"
        >
          <option value="enabled">{{ tr('启用', 'Enabled') }}</option>
          <option value="disabled">{{ tr('禁用', 'Disabled') }}</option>
          <option value="all">{{ tr('全部', 'All') }}</option></select
        ><button
          v-if="selected.length && section !== 'device-types'"
          class="quiet danger"
          @click="openDelete"
        >
          {{ tr('删除已选', 'Delete selected') }} {{ selected.length }}
        </button>
        <template v-if="section === 'dashboards'"
          ><button class="quiet" @click="dashboardCards = !dashboardCards">
            {{ dashboardCards ? tr('列表视图', 'List view') : tr('卡片视图', 'Card view') }}</button
          ><label class="button quiet"
            >{{ tr('上传仪表盘', 'Upload dashboard')
            }}<input
              type="file"
              accept=".json,application/json"
              @change="importDashboardFile" /></label
        ></template>
        <PopoverMenu class="column-picker" :label="tr('显示列', 'View')">
          <template #trigger>{{ tr('显示列', 'View') }}</template>
          <label class="check"
            ><input type="checkbox" v-model="showDescription" />{{
              tr('描述', 'Description')
            }}</label
          ><label class="check"
            ><input type="checkbox" v-model="showUpdated" />{{ tr('更新时间', 'Updated') }}</label
          >
        </PopoverMenu>
      </div>
      <details v-if="section === 'alarms'" class="panel">
        <summary>{{ tr('告警筛选', 'Alarm filters') }}</summary>
        <form @submit.prevent="applyAlarmFilters">
          <AlarmFilters :filters="alarmFilters" :workspace="workspace" sorting />
          <div class="actions">
            <button type="submit">{{ tr('应用筛选', 'Apply filters') }}</button
            ><button type="button" class="quiet" @click="resetAlarmFilters">
              {{ tr('重置', 'Reset') }}
            </button>
          </div>
        </form>
      </details>
      <div v-if="section === 'dashboards' && dashboardCards" class="dashboard-cards">
        <article v-for="r in visible" :key="r.id" class="panel">
          <img v-if="r.thumbnail" :src="thumbnailUrl(r.thumbnail)" :alt="r.name" />
          <div class="actions">
            <input
              type="checkbox"
              v-model="selected"
              :value="r.id"
              :aria-label="tr('选择', 'Select') + ' ' + r.name"
            /><router-link :to="{ path: base + '/' + r.id, query: route.query }"
              ><strong>{{ r.name }}</strong></router-link
            >
          </div>
          <p>{{ r.description }}</p>
          <small>{{ r.tags?.join(' · ') }}</small>
        </article>
        <p v-if="!visible.length">
          {{
            tr(
              '暂无仪表盘。创建一个或上传已有仪表盘文件。',
              'No dashboards. Create one or upload an existing dashboard file.',
            )
          }}
        </p>
      </div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  :aria-label="tr('选择当前页', 'Select current page')"
                  :checked="visible.length > 0 && visible.every((r) => selected.includes(r.id))" :indeterminate="visible.some(r => selected.includes(r.id)) && !visible.every(r => selected.includes(r.id))"
                  @change="selectPage($event.target.checked)"
                />
              </th>
              <th>{{ section === 'alarms' ? tr('规则', 'Rule') : tr('名称', 'Name') }}</th>
              <th v-if="showDescription">
                {{ section === 'alarms' ? tr('原因', 'Cause') : tr('描述', 'Description') }}
              </th>
              <th v-if="section === 'alarms'">{{ tr('严重程度', 'Severity') }}</th>
              <template v-if="section === 'bootstrap-profiles'"
                ><th>{{ tr('模板格式', 'Template format') }}</th>
                <th>{{ tr('内容类型', 'Content type') }}</th></template
              >
              <th v-if="section === 'bootstraps'">
                {{ tr('外部设备编号', 'External device ID') }}
              </th>
              <th v-if="section !== 'bootstrap-profiles'">{{ tr('状态', 'Status') }}</th>
              <template v-if="section === 'alarms'"
                ><th v-for="col in alarmFields" :key="col[0]">
                  {{ tr(col[1], col[2]) }}
                </th></template
              >
              <th v-if="showUpdated">{{ tr('更新时间', 'Updated') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in visible" :key="r.id" class="clickable-row" @click="openRecord($event, r)">
              <td>
                <input
                  type="checkbox"
                  v-model="selected"
                  :value="r.id"
                  :aria-label="tr('选择', 'Select') + ' ' + (r.name || r.displayName || r.id)"
                />
              </td>
              <td>
                <router-link :to="{ path: base + '/' + r.id, query: route.query }">{{
                  r.name || r.displayName || alarmRules[r.rule_id] || r.rule_id || r.id
                }}</router-link>
              </td>
              <td v-if="showDescription">
                {{ (section === 'alarms' ? alarmCause(r) : r.description) || '—' }}
              </td>
              <td v-if="section === 'alarms'">{{ r.severity ?? '—' }}</td>
              <template v-if="section === 'bootstrap-profiles'"
                ><td>{{ r.content_format }}</td>
                <td>{{ r.content_type }}</td></template
              >
              <td v-if="section === 'bootstraps'">{{ r.external_id }}</td>
              <td v-if="section !== 'bootstrap-profiles'">{{ label(r.status || '—') }}</td>
              <template v-if="section === 'alarms'"
                ><td v-for="col in alarmFields" :key="col[0]">
                  {{ alarmMembers[r[col[0]]] || r[col[0]] || '—' }}
                </td></template
              >
              <td v-if="showUpdated">
                {{ alarmTimestamp(r.updatedAt || r.updated_at || r.createdAt || r.created_at) }}
              </td>
            </tr>
            <tr v-if="!visible.length">
              <td colspan="7">{{ tr('暂无记录。', 'No records.') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <TablePagination v-model:page="page" v-model:size="size" :total="filtered.length" :selected="selected.length" /></template
    >
    <template v-else>
      <nav v-if="section === 'reports' && record.id" class="tabs">
        <router-link class="button quiet" :to="base + '/' + id">{{
          tr('配置', 'Configuration')
        }}</router-link
        ><router-link class="button quiet" :to="base + '/' + id + '/role-management'">{{
          tr('角色', 'Roles')
        }}</router-link
        ><router-link class="button quiet" :to="base + '/' + id + '/members'">{{
          tr('成员', 'Members')
        }}</router-link>
      </nav>
      <ResourceAccess
        v-if="
          section === 'reports' &&
          record.id &&
          ['role-management', 'members'].includes(String(route.params.tab))
        "
        :workspace="workspace"
        :section="section"
        :record="record"
        :tab="route.params.tab === 'members' ? 'users' : 'role-management'" />
      <ProfileEditor
        v-else-if="section === 'device-types'"
        :key="id + editorRevision"
        :record="record"
        :workspace="workspace"
        @saved="saved" /><ReportEditor
        v-else-if="section === 'reports'"
        :key="id + editorRevision"
        :record="record"
        :workspace="workspace"
        @saved="saved" /><BootstrapEditor
        :profile-mode="section === 'bootstrap-profiles'"
        @deleted="router.push(base)"
        v-else-if="['bootstraps', 'bootstrap-profiles'].includes(section)"
        :key="id + editorRevision"
        :record="record"
        :workspace="workspace"
        @saved="saved" /><SimpleNativeEditor
        v-else
        :key="section + id + editorRevision"
        :section="section"
        :record="record"
        :workspace="workspace"
        @saved="saved"
        @deleted="router.push(base)"
    /></template>
    <BootstrapProfileUpload
      v-if="profileUpload"
      :key="workspace"
      :workspace="workspace"
      @close="closeProfileUpload"
      @uploaded="closeProfileUpload"
    />
    <BatchActions
      v-if="batch"
      :records="batch.records"
      :title="tr('删除', 'Delete')"
      :perform="batch.perform"
      @close="batch = null"
      @updated="deleted"
    />
  </section>
</template>
<script setup>
import PopoverMenu from './PopoverMenu.vue'
import { useListLocation } from '../api/listLocation'
import TablePagination from './TablePagination.vue'

import ResourceAccess from './ResourceAccess.vue'
import AlarmFilters from './AlarmFilters.vue'
import { alarmQuery } from '../api/alarmFilters'
import { alarmCause, alarmTimestamp } from '../api/alarmPresentation'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tr, label, sectionLabel } from '../i18n'
import { request, listAll, runOperation, errorText } from '../api/platform'
import ProfileEditor from './ProfileEditor.vue'
import ReportEditor from './ReportEditor.vue'
import BootstrapEditor from './BootstrapEditor.vue'
import BootstrapProfileUpload from './BootstrapProfileUpload.vue'
import SimpleNativeEditor from './SimpleNativeEditor.vue'
import BatchActions from './BatchActions.vue'
import { dashboardImport } from '../api/dashboardLayout'
const alarmFilters = ref({ status: 'all', order: 'updated_at', dir: 'desc' }),
  appliedAlarmQuery = ref('')
function applyAlarmFilters() {
  try {
    appliedAlarmQuery.value = alarmQuery(alarmFilters.value).toString()
    page.value = 1
    selected.value = []
    load()
  } catch (e) {
    error.value = errorText(e)
  }
}
function resetAlarmFilters() {
  alarmFilters.value = { status: 'all', order: 'updated_at', dir: 'desc' }
  applyAlarmFilters()
}
const dashboardCards = ref(true)
const profileUpload = ref(false),
  bootstrapStatus = ref('enabled')
function closeProfileUpload() {
  profileUpload.value = false
  load()
}
function thumbnailUrl(value) {
  return /^(https?:|data:image\/(png|jpeg|webp);base64,|\/)/i.test(value) ? value : undefined
}
async function importDashboardFile(e) {
  const file = e.target.files?.[0]
  if (!file) return
  try {
    if (file.size > 5 * 1024 * 1024)
      throw new Error(tr('文件不能超过 5 MB', 'File must be under 5 MB'))
    record.value = dashboardImport(await file.text())
    editorRevision.value++
  } catch (e) {
    error.value = errorText(e)
  } finally {
    e.target.value = ''
  }
}
const route = useRoute(),
  router = useRouter(),
  section = computed(() => String(route.params.section)),
  workspace = computed(() => String(route.params.id)),
  id = computed(() => String(route.params.recordId || '')),
  base = computed(() => `/domain/${workspace.value}/${section.value}`)
const endpoints = {
  'bootstrap-profiles': ['bootstrap', 'devices/bootstrap/profiles', 'profiles'],
  reports: ['reports', 'reports/configs', 'report_configs'],
  bootstraps: ['bootstrap', 'devices/configs', 'configs'],
  alarms: ['alarms', 'alarms', 'alarms'],
  'custom-nodes': ['ui', 'custom-nodes', 'custom_nodes'],
  dashboards: ['ui', 'dashboards', 'dashboards'],
}
const records = ref([]),
  record = ref(null),
  editorRevision = ref(0),
  error = ref(''),
  loading = ref(false),
  search = ref(''),
  page = ref(1),
  size = ref(10),
  selected = ref([]),
  batch = ref(null),
  showDescription = ref(true),
  showUpdated = ref(true)
const filtered = computed(() =>
    records.value.filter((r) =>
      [r.name, r.displayName, r.description, r.id, r.cause, r.message, r.rule_id].some((v) =>
        v?.toLowerCase().includes(search.value.toLowerCase()),
      ),
    ),
  ),
  visible = computed(() =>
    filtered.value.slice((page.value - 1) * size.value, page.value * size.value),
  )
const alarmRules = ref({}),
  alarmMembers = ref({})
const alarmFields = [
  ['measurement', '测量项', 'Measurement'],
  ['value', '数值', 'Value'],
  ['threshold', '阈值', 'Threshold'],
  ['assignee_id', '指派给', 'Assignee'],
  ['resolved_by', '解除人', 'Resolved by'],
  ['acknowledged_by', '确认人', 'Acknowledged by'],
]
const restoringList = useListLocation({ page, size, filters: { q: [search, ''] }, active: () => !id.value })
let generation = 0
function api() {
  const [service, path] = endpoints[section.value]
  return `/api/platform/${service}/${workspace.value}/${path}`
}
async function load() {
  const seq = ++generation
  loading.value = true
  error.value = ''
  try {
    if (id.value === 'create') record.value = {}
    else if (id.value) {
      const d =
        section.value === 'device-types'
          ? await runOperation('profile', { id: id.value })
          : await request(api() + '/' + id.value)
      if (seq !== generation) return
      if (
        (d.tenantId && d.tenantId !== workspace.value) ||
        (d.workspace_id && d.workspace_id !== workspace.value)
      )
        throw new Error(tr('记录不属于当前工作区。', 'Record is outside this workspace.'))
      record.value = d
      editorRevision.value++
    } else {
      record.value = null
      let rows = []
      if (section.value === 'device-types')
        rows = await listAll('profiles', {
          tenantId: workspace.value,
          objectKind: 'entity',
          kind: 'device',
        })
      else
        for (let offset = 0; offset < 10000; offset += 100) {
          const d = await request(
              api() +
                `?limit=100&offset=${offset}` +
                (section.value === 'bootstraps' ? `&status=${bootstrapStatus.value}` : '') +
                (section.value === 'alarms' && appliedAlarmQuery.value
                  ? `&${appliedAlarmQuery.value}`
                  : ''),
            ),
            part = d[endpoints[section.value][2]] || d.reports || d.configurations || []
          rows.push(...part)
          if (!part.length || rows.length >= (d.total ?? rows.length)) break
        }
      if (section.value === 'alarms') {
        const [ruleResult, memberResult] = await Promise.allSettled([
          request(`/api/platform/rules/${workspace.value}/rules?limit=100&offset=0`),
          listAll('tenantMembers', { tenantId: workspace.value }),
        ])
        if (seq !== generation) return
        const rules = ruleResult.status === 'fulfilled' ? ruleResult.value.rules || [] : []
        const members = memberResult.status === 'fulfilled' ? memberResult.value : []
        alarmRules.value = Object.fromEntries(rules.map((r) => [r.id, r.name]))
        alarmMembers.value = Object.fromEntries(
          members.map((m) => [
            m.userId || m.id,
            m.user?.firstName
              ? `${m.user.firstName} ${m.user.lastName || ''}`
              : m.name || m.user?.name || m.userId || m.id,
          ]),
        )
      }
      if (seq === generation) records.value = rows
    }
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) loading.value = false
  }
}
watch(
  () => [section.value, workspace.value, id.value],
  () => {
    record.value = null
    records.value = []
    selected.value = []
    if (!restoringList.value) page.value = 1
    batch.value = null
    profileUpload.value = false
    alarmFilters.value = { status: 'all', order: 'updated_at', dir: 'desc' }
    appliedAlarmQuery.value = ''
    load()
  },
  { immediate: true },
)
watch(search, () => {
  if (!restoringList.value) page.value = 1
  selected.value = []
})
watch(bootstrapStatus, () => {
  page.value = 1
  selected.value = []
  load()
})
function openRecord(event, record) { if (!event.target.closest('a, button, input, select, label')) router.push({path: base.value + '/' + record.id, query: route.query}) }
function selectPage(v) {
  const ids = visible.value.map((r) => r.id)
  selected.value = v
    ? [...new Set([...selected.value, ...ids])]
    : selected.value.filter((id) => !ids.includes(id))
}
function openDelete() {
  const url = api()
  batch.value = {
    records: records.value.filter((r) => selected.value.includes(r.id)),
    perform: (r) => request(url + '/' + r.id, { method: 'DELETE' }),
  }
}
function deleted(ids) {
  selected.value = selected.value.filter((id) => !ids.includes(id))
  load()
}
function saved(r) {
  if (r?.id && id.value === 'create') router.replace(base.value + '/' + r.id)
  else if (id.value === 'create') router.push(base.value)
  else load()
}
</script>

<style scoped>
.dashboard-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(270px, 1fr));
  gap: 18px;
}
.dashboard-cards img {
  width: 100%;
  height: 150px;
  object-fit: cover;
  border-radius: 6px;
}
.dashboard-cards .panel {
  margin: 0;
}
</style>
