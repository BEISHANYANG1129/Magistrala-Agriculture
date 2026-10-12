<template>
  <div :class="{ 'group-workspace': section === 'groups' }">
    <aside v-if="section === 'groups'" class="group-browser">
      <button @click="creatingGroup = true">+ {{ tr('创建分组', 'Create group') }}</button
      ><input v-model="groupSearch" :placeholder="tr('搜索分组', 'Search groups')" />
      <div v-for="g in groupTree" :key="g.id" :style="{ paddingLeft: g.depth * 12 + 'px' }">
        <router-link
          :class="{ active: g.id === record?.id }"
          :to="`/domain/${workspace}/groups/${g.id}`"
          >{{ g.depth ? '↳ ' : '' }}{{ g.name }}</router-link
        >
      </div>
    </aside>
    <section>
      <header class="page-heading">
        <h1>{{ record?.name || sectionLabel(section) }}</h1>
        <router-link
          v-if="record && section !== 'groups'"
          class="button quiet"
          :to="{ path: `/domain/${workspace}/${section}`, query: listQuery }"
          >← {{ tr('返回列表', 'Back to list') }}</router-link
        >
      </header>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="loading">{{ tr('读取中…', 'Loading…') }}</p>
      <ResourceTable
        v-if="!recordId && section !== 'groups'"
        :section="section"
        :workspace="workspace"
      />
      <p v-if="!recordId && section === 'groups' && !loading">
        {{ tr('从左侧选择分组，或创建第一个分组。', 'Select a group or create your first group.') }}
      </p>
      <template v-if="record">
        <nav class="detail-tabs" :aria-label="tr('资源页面', 'Resource pages')">
          <router-link
            v-for="t in tabs"
            :key="t.key"
            :to="{path: `/domain/${workspace}/${section}/${record.id}${t.key ? '/' + t.key : ''}`, query: listQuery}"
            :class="['button', 'quiet', { active: tab === t.key }]"
            >{{ tr(t.zh, t.en) }}</router-link
          >
        </nav>
        <ResourceSettings
          :key="record.id"
          v-if="!tab"
          :section="section"
          :workspace="workspace"
          :record="record"
          @updated="loadRecord"
          @deleted="deleted"
        />
        <ResourceConnections
          v-else-if="tab === 'connections'"
          :key="record.id"
          :section="section"
          :workspace="workspace"
          :record="record"
        />
        <MessagesPage v-else-if="tab === 'messages'" :channel-id="record.id" embedded />
        <ResourceTable
          v-else-if="section === 'groups' && ['devices', 'channels'].includes(tab)"
          :section="tab"
          :workspace="workspace"
          :parent-group="record.id"
          :key="record.id + tab"
        />
        <ResourceAccess
          :key="record.id"
          v-else-if="['access', 'role-management', 'users'].includes(tab)"
          :section="section"
          :workspace="workspace"
          :record="record"
          :tab="tab"
        />
        <div v-else-if="tab === 'groups'">
          <div class="toolbar">
            <button @click="groupDialog = true">+ {{ tr('加入分组', 'Add to group') }}</button>
          </div>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{{ tr('分组', 'Group') }}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-for="g in memberships" :key="g.id">
                  <td>
                    <router-link :to="`/domain/${workspace}/groups/${g.id}`">{{
                      g.name
                    }}</router-link>
                  </td>
                  <td>
                    <button class="quiet" @click="removeGroup = g">
                      {{ tr('移出分组', 'Remove from group') }}
                    </button>
                  </td>
                </tr>
                <tr v-if="!memberships.length">
                  <td colspan="2">{{ tr('尚未加入分组。', 'No group memberships.') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-else-if="tab === 'journals'">
          <div class="toolbar">
            <button class="quiet" @click="loadJournal">{{ tr('刷新', 'Refresh') }}</button>
          </div>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{{ tr('时间', 'Time') }}</th>
                  <th>{{ tr('事件', 'Event') }}</th>
                  <th>{{ tr('结果', 'Outcome') }}</th>
                  <th>{{ tr('操作者', 'Actor') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in journals" :key="r.id || i">
                  <td>{{ r.createdAt || r.timestamp }}</td>
                  <td>{{ r.event }}</td>
                  <td>{{ r.outcome }}</td>
                  <td>{{ r.actorEntityId }}</td>
                </tr>
                <tr v-if="!journals.length">
                  <td colspan="4">{{ tr('暂无日志。', 'No journal entries.') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-else-if="tab === 'alarms'">
          <p>{{ tr('当前设备的告警', 'Alarms for this device') }}</p>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{{ tr('名称', 'Name') }}</th>
                  <th>{{ tr('状态', 'Status') }}</th>
                  <th>{{ tr('严重程度', 'Severity') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="a in alarms" :key="a.id">
                  <td>
                    <router-link :to="`/domain/${workspace}/alarms/${a.id}`">{{
                      a.name || a.rule_id || a.id
                    }}</router-link>
                  </td>
                  <td>{{ a.status }}</td>
                  <td>{{ a.severity }}</td>
                </tr>
                <tr v-if="!alarms.length">
                  <td colspan="3">{{ tr('暂无告警。', 'No alarms.') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </section>
    <ModalDialog v-if="creatingGroup" @close="creatingGroup = false">
      <div class="modal-card compact-modal">
        <ResourceCreate
          section="groups"
          :workspace="workspace"
          @close="creatingGroup = false"
          @saved="groupCreated"
        />
      </div>
    </ModalDialog>
    <ModalDialog v-if="groupDialog" :busy="saving" @close="groupDialog = false">
      <form class="modal-card compact-modal" @submit.prevent="addGroup">
        <h2>{{ tr('加入分组', 'Add to group') }}</h2>
        <label class="field"
          >{{ tr('分组', 'Group')
          }}<select v-model="groupId" required>
            <option value="">{{ tr('请选择', 'Select') }}</option>
            <option
              v-for="g in groups.filter((g) => !record.objectGroupIds?.includes(g.id))"
              :key="g.id"
              :value="g.id"
            >
              {{ g.name }}
            </option>
          </select></label
        >
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="saving" @click="groupDialog = false">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="saving">{{ tr('添加', 'Add') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="removeGroup" :busy="saving" confirmation @close="removeGroup = null">
      <div class="modal-card compact-modal">
        <h2>{{ tr('移出分组', 'Remove from group') }}</h2>
        <p>{{ removeGroup.name }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button class="quiet" :disabled="saving" @click="removeGroup = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="saving" @click="detachGroup">{{ tr('确认', 'Confirm') }}</button>
        </div>
      </div>
    </ModalDialog>
  </div>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tr, sectionLabel } from '../i18n'
import { runOperation, listAll, request, errorText } from '../api/platform'
import { resourceConfig, readResource } from '../api/resourceWorkflow'
import ResourceTable from './ResourceTable.vue'
import ResourceSettings from './ResourceSettings.vue'
import ResourceCreate from './ResourceCreate.vue'
import ResourceConnections from './ResourceConnections.vue'
import ResourceAccess from './ResourceAccess.vue'
import MessagesPage from './MessagesPage.vue'
const route = useRoute(),
  router = useRouter(),
  workspace = computed(() => String(route.params.id)),
  section = computed(() => String(route.params.section)),
  recordId = computed(() => String(route.params.recordId || '')),
  tab = computed(() => String(route.params.tab || ''))
const record = ref(null),
  groups = ref([]),
  error = ref(''),
  loading = ref(false),
  saving = ref(false),
  creatingGroup = ref(false),
  groupSearch = ref(''),
  groupDialog = ref(false),
  removeGroup = ref(null),
  groupId = ref(''),
  journals = ref([]),
  alarms = ref([]),
  listQuery = ref({})
const tabs = computed(() => [
  {
    key: '',
    zh: section.value === 'devices' || section.value === 'gateways' ? '配置' : '设置',
    en: section.value === 'devices' || section.value === 'gateways' ? 'Configuration' : 'Settings',
  },
  ...(section.value !== 'groups' ? [{ key: 'connections', zh: '连接', en: 'Connections' }] : []),
  ...(['devices', 'gateways'].includes(section.value)
    ? [
        { key: 'groups', zh: '分组', en: 'Groups' },
        { key: 'access', zh: '访问权限', en: 'Access' },
      ]
    : []),
  { key: 'role-management', zh: '角色', en: 'Roles' },
  { key: 'users', zh: '成员', en: 'Members' },
  ...(section.value === 'groups'
    ? [
        { key: 'devices', zh: '设备', en: 'Devices' },
        { key: 'channels', zh: '通道', en: 'Channels' },
      ]
    : []),
  ...(section.value === 'channels' ? [{ key: 'messages', zh: '消息', en: 'Messages' }] : []),
  ...(['devices', 'gateways'].includes(section.value)
    ? [{ key: 'alarms', zh: '告警', en: 'Alarms' }]
    : []),
  { key: 'journals', zh: '日志', en: 'Journals' },
])
const memberships = computed(() =>
  groups.value.filter((g) => record.value?.objectGroupIds?.includes(g.id)),
)
const groupTree = computed(() => {
  const result = [],
    seen = new Set()
  function visit(parent, depth) {
    for (const g of groups.value.filter((g) => (g.parentId || null) === parent)) {
      if (seen.has(g.id)) continue
      seen.add(g.id)
      if (!groupSearch.value || g.name.toLowerCase().includes(groupSearch.value.toLowerCase()))
        result.push({ ...g, depth })
      visit(g.id, depth + 1)
    }
  }
  visit(null, 0)
  for (const g of groups.value)
    if (!seen.has(g.id) && (!groupSearch.value || g.name.includes(groupSearch.value)))
      result.push({ ...g, depth: 0 })
  return result
})
let generation = 0
async function loadRecord() {
  const id = recordId.value,
    seq = ++generation
  error.value = ''
  loading.value = true
  try {
    const [allGroups, current] = await Promise.all([
      listAll('groups', { tenantId: workspace.value }),
      id ? readResource(section.value, id, workspace.value) : null,
    ])
    if (seq !== generation) return
    groups.value = allGroups
    record.value = current
    if (!id && section.value === 'groups' && allGroups.length)
      router.replace(`/domain/${workspace.value}/groups/${allGroups[0].id}`)
    if (tab.value === 'journals') await loadJournal()
    if (tab.value === 'alarms') await loadAlarms()
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) loading.value = false
  }
}
watch(
  () => [workspace.value, section.value, recordId.value],
  () => {
    record.value = null
    groupDialog.value = false
    removeGroup.value = null
    if (!recordId.value || Object.keys(route.query).length) listQuery.value = { ...route.query }
    loadRecord()
  },
  { immediate: true },
)
watch(tab, () => {
  error.value = ''
  if (tab.value === 'journals') loadJournal()
  if (tab.value === 'alarms') loadAlarms()
})
async function loadJournal() {
  const target = `${workspace.value}/${section.value}/${recordId.value}`
  journals.value = []
  try {
    const entries = await listAll('auditLogs', {
      tenantId: workspace.value,
      targetKind:
        section.value === 'groups' ? 'group' : section.value === 'channels' ? 'resource' : 'entity',
      targetId: recordId.value,
    })
    if (target === `${workspace.value}/${section.value}/${recordId.value}`) journals.value = entries
  } catch (e) {
    if (target === `${workspace.value}/${section.value}/${recordId.value}`)
      error.value = errorText(e)
  }
}
async function loadAlarms() {
  const target = `${workspace.value}/${recordId.value}`
  alarms.value = []
  try {
    const d = await request(
      `/api/platform/alarms/${workspace.value}/alarms?device_id=${encodeURIComponent(recordId.value)}&limit=100`,
    )
    if (target === `${workspace.value}/${recordId.value}`) alarms.value = d.alarms || []
  } catch (e) {
    if (target === `${workspace.value}/${recordId.value}`) error.value = errorText(e)
  }
}
function deleted() {
  router.push(`/domain/${workspace.value}/${section.value}`)
}
function groupCreated(r) {
  creatingGroup.value = false
  router.push(`/domain/${workspace.value}/groups/${r.id}`)
}
async function addGroup() {
  saving.value = true
  error.value = ''
  try {
    await runOperation('addEntityToObjectGroup', {
      entityId: recordId.value,
      objectGroupId: groupId.value,
    })
    groupDialog.value = false
    await loadRecord()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    saving.value = false
  }
}
async function detachGroup() {
  saving.value = true
  error.value = ''
  try {
    await runOperation('removeEntityFromObjectGroup', {
      entityId: recordId.value,
      objectGroupId: removeGroup.value.id,
    })
    removeGroup.value = null
    await loadRecord()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    saving.value = false
  }
}
</script>
