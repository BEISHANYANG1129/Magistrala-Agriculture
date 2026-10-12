<template>
  <section class="workspace-page">
    <header>
      <h1>{{ title }}</h1>
      <p>{{ description }}</p>
    </header>

    <p v-if="loading" class="message">正在加载...</p>
    <p v-else-if="error" class="message error" role="alert">{{ error }}</p>
    <div v-else-if="unsupported" class="message">
      这个项目的前端尚未实现此页面。
      <a :href="coreUrl" target="_blank" rel="noopener noreferrer">在核心平台打开对应页面</a>
    </div>

    <div v-else-if="section === 'home'" class="cards">
      <router-link v-for="card in cards" :key="card.path" :to="workspacePath(card.path)" class="card">
        <strong>{{ card.label }}</strong>
        <span>{{ counts[card.path] ?? 0 }}</span>
      </router-link>
    </div>

    <dl v-else-if="section === 'info' || detailId" class="details">
      <template v-for="field in detailFields" :key="field.label">
        <dt>{{ field.label }}</dt>
        <dd>{{ field.value || '—' }}</dd>
      </template>
    </dl>

    <template v-else>
      <p v-if="records.length === 0" class="message">当前工作区暂无{{ title }}数据。</p>
      <div v-else class="table-wrap">
        <table>
          <thead><tr><th>名称</th><th>别名</th><th>状态 / 类型</th><th>ID</th></tr></thead>
          <tbody>
            <tr v-for="record in records" :key="record.id">
              <td>
                <router-link v-if="section === 'channels' || section === 'devices'" :to="workspacePath(`${section}/${record.id}`)">
                  {{ record.name || record.alias || record.id }}
                </router-link>
                <span v-else>{{ record.name || record.alias || record.id }}</span>
              </td>
              <td>{{ record.alias || '—' }}</td>
              <td>{{ record.status || record.groupType || record.kind || '—' }}</td>
              <td class="identifier">{{ record.id }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const error = ref('')
const records = ref([])
const detail = ref(null)
const counts = ref({})
let requestVersion = 0

const labels = {
  home: '工作区概览',
  dashboards: '仪表盘',
  info: '工作区信息',
  users: '成员',
  'role-management': '角色',
  invitations: '邀请',
  groups: '分组',
  channels: '频道',
  devices: '设备',
  gateways: '网关',
  'device-types': '设备类型',
  messages: '消息',
  rules: '规则',
  'custom-nodes': '自定义节点',
  alarms: '告警',
  reports: '报表',
  bootstraps: '引导配置'
}
const cards = [
  { path: 'channels', label: '频道' },
  { path: 'devices', label: '设备' },
  { path: 'users', label: '成员' },
  { path: 'groups', label: '分组' }
]
const section = computed(() => route.path.split('/')[3] || 'home')
const detailId = computed(() => route.params.channelId || route.params.clientId)
const unsupported = computed(() => !['home', 'info', 'channels', 'devices', 'users', 'groups'].includes(section.value) || Boolean(route.params.tab))
const coreUrl = computed(() => `http://localhost:3000/en/workspace/${encodeURIComponent(String(route.params.id))}/${route.path.split('/').slice(3).map(encodeURIComponent).join('/')}`)
const title = computed(() => labels[section.value] || '页面')
const description = computed(() => unsupported.value ? '对应功能可在核心平台使用' : detailId.value ? `${title.value}详情` : '来自当前 Magistrala 核心的实时数据')
const detailFields = computed(() => {
  const item = detail.value || {}
  return [
    { label: '名称', value: item.name },
    { label: 'ID', value: item.id },
    { label: '别名', value: item.alias },
    { label: '状态', value: item.status },
    { label: '类型', value: item.kind || item.groupType },
    { label: '创建时间', value: item.createdAt }
  ]
})

function workspacePath(path) {
  return `/domain/${route.params.id}/${path}`
}

async function graphql(query, variables) {
  const token = localStorage.getItem('token')
  if (!token) {
    await router.replace('/login')
    throw new Error('请先登录')
  }
  const response = await fetch('/graphql', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ query, variables })
  })
  if (response.status === 401) {
    localStorage.removeItem('token')
    await router.replace('/login')
    throw new Error('登录已过期，请重新登录')
  }
  const body = await response.json()
  if (!response.ok || body.errors?.length) {
    throw new Error(body.errors?.[0]?.message || '核心服务请求失败')
  }
  return body.data
}

async function loadPage() {
  const version = ++requestVersion
  records.value = []
  detail.value = null
  counts.value = {}
  error.value = ''
  if (unsupported.value) return
  loading.value = true
  try {
    const id = String(route.params.id)
    const variables = { id }
    let data
    if (section.value === 'home') {
      data = await graphql(`query($id: ID!) {
        tenant(id: $id) { id name }
        channels: resources(tenantId: $id, kind: "channel") { total }
        clients: entities(tenantId: $id, kind: "device") { total }
        members: entities(tenantId: $id, kind: "human") { total }
        groups(tenantId: $id) { total }
      }`, variables)
      if (version !== requestVersion) return
      detail.value = data.tenant
      counts.value = {
        channels: data.channels?.total,
        clients: data.clients?.total,
        members: data.members?.total,
        groups: data.groups?.total
      }
    } else if (section.value === 'info') {
      data = await graphql('query($id: ID!) { tenant(id: $id) { id name alias status createdAt } }', variables)
      if (version !== requestVersion) return
      detail.value = data.tenant
    } else if (detailId.value) {
      const field = section.value === 'channels' ? 'resource' : 'entity'
      const fields = field === 'resource' ? 'id name alias kind createdAt' : 'id name alias kind status createdAt'
      data = await graphql(`query($id: ID!) { ${field}(id: $id) { ${fields} } }`, { id: String(detailId.value) })
      if (version !== requestVersion) return
      detail.value = data[field]
      if (!detail.value) error.value = '未找到该记录'
    } else {
      const queries = {
        channels: 'resources(tenantId: $id, kind: "channel", limit: 100) { items { id name alias kind } }',
        devices: 'entities(tenantId: $id, kind: "device", limit: 100) { items { id name alias status kind } }',
        users: 'entities(tenantId: $id, kind: "human", limit: 100) { items { id name alias status kind } }',
        groups: 'groups(tenantId: $id, limit: 100) { items { id name groupType status } }'
      }
      data = await graphql(`query($id: ID!) { ${queries[section.value]} }`, variables)
      if (version !== requestVersion) return
      records.value = data[section.value === 'channels' ? 'resources' : section.value === 'groups' ? 'groups' : 'entities']?.items || []
    }
  } catch (caught) {
    if (version === requestVersion) error.value = caught instanceof Error ? caught.message : '页面加载失败'
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

watch(() => [route.params.id, route.path], loadPage, { immediate: true })
</script>

<style scoped>
.workspace-page { max-width: 1100px; margin: 0 auto; color: #17324d; }
h1 { margin: 0 0 8px; }
header p { margin: 0 0 24px; color: #60758a; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 16px; }
.card { display: flex; flex-direction: column; gap: 14px; padding: 22px; background: #eef6fb; border-radius: 12px; color: #174e8a; text-decoration: none; }
.card span { font-size: 2rem; font-weight: 700; }
.message { padding: 24px; background: #eef6fb; border-radius: 12px; }
.error { color: #9a2f2f; background: #fff0f0; }
.details { display: grid; grid-template-columns: 130px 1fr; gap: 12px; padding: 24px; background: #eef6fb; border-radius: 12px; }
dt { font-weight: 700; }
dd { margin: 0; overflow-wrap: anywhere; }
.table-wrap { overflow-x: auto; background: white; border: 1px solid #dce8ef; border-radius: 12px; }
table { width: 100%; border-collapse: collapse; }
th, td { text-align: left; padding: 14px; border-bottom: 1px solid #e7eef3; }
th { background: #eef6fb; }
.identifier { font-family: monospace; font-size: 0.85rem; }
</style>
