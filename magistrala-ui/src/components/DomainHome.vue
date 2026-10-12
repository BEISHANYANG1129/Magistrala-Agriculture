<template>
  <div>
    <p v-if="error" role="alert">{{ error }}</p>
    <header class="main-header">
      <div class="main-title">Home Page</div>
      <div class="user-info">
        <span>{{ user.name }}</span>
        <span class="user-email">{{ user.email }}</span>
      </div>
    </header>
    <div class="summary-cards">
      <div class="summary-card">
        <div class="summary-icon">👤</div>
        <div class="summary-title">Domain Members</div>
        <div class="summary-value">{{ summary.members }}</div>
        <div class="summary-status">
          <span class="enabled">Enabled: {{ summary.enabledMembers }}</span>
          <span class="disabled">Disabled: {{ summary.disabledMembers }}</span>
        </div>
      </div>
      <div class="summary-card">
        <div class="summary-icon">📶</div>
        <div class="summary-title">Devices</div>
        <div class="summary-value">{{ summary.clients }}</div>
        <div class="summary-status">
          <span class="enabled">Enabled: {{ summary.enabledClients }}</span>
          <span class="disabled">Disabled: {{ summary.disabledClients }}</span>
        </div>
      </div>
      <div class="summary-card">
        <div class="summary-icon">📡</div>
        <div class="summary-title">Channels</div>
        <div class="summary-value">{{ summary.channels }}</div>
        <div class="summary-status">
          <span class="enabled">Enabled: {{ summary.enabledChannels }}</span>
          <span class="disabled">Disabled: {{ summary.disabledChannels }}</span>
        </div>
      </div>
      <div class="summary-card">
        <div class="summary-icon">🧩</div>
        <div class="summary-title">Groups</div>
        <div class="summary-value">{{ summary.groups }}</div>
        <div class="summary-status">
          <span class="enabled">Enabled: {{ summary.enabledGroups }}</span>
          <span class="disabled">Disabled: {{ summary.disabledGroups }}</span>
        </div>
      </div>
    </div>
    <div class="overview-dashboard-row">
      <div class="overview-card">
        <div class="overview-title">Overview</div>
        <div class="overview-chart">
          <div class="chart-bar" v-for="item in overview" :key="item.label">
            <div class="chart-label">{{ item.label }}</div>
            <div class="chart-bar-bg">
              <div class="chart-bar-enabled" :style="{height: item.enabled * 30 + 'px'}"></div>
              <div class="chart-bar-disabled" :style="{height: item.disabled * 30 + 'px'}"></div>
            </div>
            <div class="chart-value">
              <span class="enabled">{{ item.enabled }}</span>
              <span class="disabled">{{ item.disabled }}</span>
            </div>
          </div>
        </div>
      </div>
      <div class="dashboard-card">
        <div class="dashboard-title-row">
          <span class="dashboard-title">Dashboards</span>
          <button class="view-all-btn">View All</button>
        </div>
        <table class="dashboard-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Created At</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="dashboard in dashboards" :key="dashboard.name">
              <td>
                <span class="dashboard-icon">📊</span>
                {{ dashboard.name }}
              </td>
              <td>{{ dashboard.createdAt }}</td>
            </tr>
          </tbody>
        </table>
        <div class="dashboard-desc">{{ dashboards.length ? 'A list of your latest dashboards' : '暂无仪表盘数据' }}</div>
      </div>
      <div class="distribution-card">
        <div class="distribution-title">Distributions</div>
        <div class="distribution-item">
          <div class="distribution-label">Created At</div>
          <div class="distribution-value">{{ domain.created_at }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { graphql } from '../api/atom'

const route = useRoute()
const domainId = route.params.id || ''

const user = ref({ name: sessionStorage.getItem('workspaceName') || 'Workspace', email: '' })

const domain = ref({ id: domainId, created_at: '' })

const summary = ref({
  members: 0,
  enabledMembers: 0,
  disabledMembers: 0,
  clients: 0,
  enabledClients: 0,
  disabledClients: 0,
  channels: 0,
  enabledChannels: 0,
  disabledChannels: 0,
  groups: 0,
  enabledGroups: 0,
  disabledGroups: 0,
})

const overview = computed(() => [
  { label: 'Domain Members', enabled: summary.value.enabledMembers, disabled: summary.value.disabledMembers },
  { label: 'Devices', enabled: summary.value.enabledClients, disabled: summary.value.disabledClients },
  { label: 'Channels', enabled: summary.value.channels, disabled: 0 },
  { label: 'Groups', enabled: summary.value.enabledGroups, disabled: summary.value.disabledGroups },
])

const dashboards = ref([])
const error = ref('')

onMounted(async () => {
  try {
    const data = await graphql(`query($id: ID!) {
      tenant(id: $id) { id name createdAt }
      channels: resources(tenantId: $id, kind: "channel") { total }
      clients: entities(tenantId: $id, kind: "device", limit: 100) { total items { status } }
      members: entities(tenantId: $id, kind: "human", limit: 100) { total items { status } }
      groups(tenantId: $id, limit: 100) { total items { status } }
    }`, { id: String(domainId) })
    domain.value.created_at = data.tenant?.createdAt ? new Date(data.tenant.createdAt).toLocaleString() : '—'
    user.value.name = data.tenant?.name || user.value.name
    for (const key of ['clients', 'members', 'groups']) {
      const items = data[key]?.items || []
      const enabled = items.filter(item => item.status === 'enabled').length
      const title = key.charAt(0).toUpperCase() + key.slice(1)
      summary.value[key] = data[key]?.total || 0
      summary.value[`enabled${title}`] = enabled
      summary.value[`disabled${title}`] = items.length - enabled
    }
    summary.value.channels = data.channels?.total || 0
    summary.value.enabledChannels = summary.value.channels
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : '概览加载失败'
  }
})
</script>

<style scoped>
.main-content {
  flex: 1;
  padding: 32px 40px;
}
.main-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}
.main-title {
  font-size: 2em;
  font-weight: bold;
  color: #174e8a;
}
.user-info {
  text-align: right;
  font-size: 1em;
  color: #174e8a;
}
.user-email {
  display: block;
  font-size: 0.95em;
  color: #888;
}
.summary-cards {
  display: flex;
  gap: 24px;
  margin-bottom: 32px;
}
.summary-card {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 8px #ddd;
  padding: 18px 24px;
  min-width: 180px;
  text-align: center;
}
.summary-icon {
  font-size: 2em;
  margin-bottom: 6px;
}
.summary-title {
  font-size: 1.1em;
  font-weight: bold;
  margin-bottom: 8px;
}
.summary-value {
  font-size: 2em;
  font-weight: bold;
  margin-bottom: 8px;
}
.summary-status {
  font-size: 0.95em;
  display: flex;
  justify-content: center;
  gap: 12px;
}
.enabled {
  color: #0a3566;
  font-weight: bold;
}
.disabled {
  color: #e67c6b;
}
.overview-dashboard-row {
  display: flex;
  gap: 32px;
  margin-bottom: 32px;
}
.overview-card {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 8px #ddd;
  padding: 24px;
  min-width: 340px;
  flex: 2;
}
.overview-title {
  font-size: 1.1em;
  font-weight: bold;
  margin-bottom: 12px;
}
.overview-chart {
  display: flex;
  gap: 18px;
  align-items: flex-end;
  height: 120px;
  margin-top: 16px;
}
.chart-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 60px;
}
.chart-label {
  font-size: 0.95em;
  margin-bottom: 4px;
  text-align: center;
}
.chart-bar-bg {
  width: 28px;
  height: 100px;
  background: #eaf7f6;
  border-radius: 6px;
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  margin-bottom: 4px;
}
.chart-bar-enabled {
  width: 100%;
  background: #0a3566;
  border-radius: 6px 6px 0 0;
}
.chart-bar-disabled {
  width: 100%;
  background: #e67c6b;
  border-radius: 0 0 6px 6px;
}
.chart-value {
  font-size: 0.95em;
  display: flex;
  gap: 4px;
  justify-content: center;
}
.dashboard-card {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 8px #ddd;
  padding: 24px;
  min-width: 320px;
  flex: 1.2;
  margin-right: 16px;
  display: flex;
  flex-direction: column;
}
.dashboard-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.dashboard-title {
  font-size: 1.1em;
  font-weight: bold;
}
.dashboard-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 8px;
}
.dashboard-table th,
.dashboard-table td {
  padding: 6px 8px;
  text-align: left;
  font-size: 1em;
}
.dashboard-table th {
  color: #888;
  font-weight: normal;
  border-bottom: 1px solid #eee;
}
.dashboard-icon {
  margin-right: 6px;
}
.dashboard-desc {
  font-size: 0.95em;
  color: #666;
}
.view-all-btn {
  background: #0a3566;
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 6px 14px;
  font-size: 1em;
  cursor: pointer;
  font-weight: bold;
}
.distribution-card {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 8px #ddd;
  padding: 24px;
  min-width: 220px;
  flex: 1;
  display: flex;
  flex-direction: column;
}
.distribution-title {
  font-size: 1.1em;
  font-weight: bold;
  margin-bottom: 12px;
}
.distribution-item {
  margin-bottom: 8px;
}
.distribution-label {
  font-size: 0.95em;
  color: #888;
}
.distribution-value {
  font-size: 1em;
  color: #174e8a;
  font-weight: bold;
}
</style>
