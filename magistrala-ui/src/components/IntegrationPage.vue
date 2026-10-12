<template>
  <section>
    <header class="page-heading">
      <div>
        <p class="eyebrow">AGRICULTURAL DATA</p>
        <h1>{{ tr('农业数据集成', 'Agricultural integration') }}</h1>
        <p>
          {{
            tr(
              '查看原集成服务的状态和传感器映射。',
              'Inspect the existing integration service and its sensor mappings.',
            )
          }}
        </p>
      </div>
      <div class="actions">
        <button class="quiet" :disabled="busy" @click="load">{{ tr('刷新', 'Refresh') }}</button
        ><button :disabled="busy" @click="scan">
          {{ tr('重新扫描传感器', 'Rescan sensors') }}
        </button>
      </div>
    </header>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div class="workspace-grid">
      <div v-for="s in cards" :key="s.key" class="workspace-card">
        <span class="metric">{{ stats[s.key] ?? '—' }}</span>
        <p>{{ tr(s.zh, s.en) }}</p>
      </div>
    </div>
    <p v-if="stats.last_error" class="warning">{{ stats.last_error }}</p>
    <div class="toolbar">
      <input
        v-model="search"
        :placeholder="tr('搜索传感器、设备或分区', 'Search sensor, device or partition')"
      />
    </div>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{{ tr('传感器', 'Sensor') }}</th>
            <th>{{ tr('设备', 'Device') }}</th>
            <th>{{ tr('分区', 'Partition') }}</th>
            <th>{{ tr('最近值', 'Latest value') }}</th>
            <th>{{ tr('状态', 'Status') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in filtered" :key="`${m.deviceAddr}_${m.nodeId}_${m.registerId}`">
            <td>
              {{ m.factorName }}<small>{{ m.unit }}</small>
            </td>
            <td>
              {{ m.deviceName }}<small>{{ m.clientId }}</small>
            </td>
            <td>{{ m.partition }}</td>
            <td>{{ m.lastValue ?? '—' }}</td>
            <td>{{ m.isActive ? tr('活跃', 'Active') : tr('未激活', 'Inactive') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
<script setup>
import { ref, computed, onMounted } from 'vue'
import { request, errorText } from '../api/platform'
import { tr } from '../i18n'
const stats = ref({}),
  mappings = ref([]),
  busy = ref(false),
  error = ref(''),
  search = ref('')
const cards = [
  { key: 'total_sensors', zh: '发现传感器', en: 'Sensors found' },
  { key: 'active_mappings', zh: '活跃映射', en: 'Active mappings' },
  { key: 'messages_sent', zh: '已发送消息', en: 'Messages sent' },
]
const filtered = computed(() =>
  mappings.value.filter((m) =>
    [m.factorName, m.deviceName, m.partition]
      .join(' ')
      .toLowerCase()
      .includes(search.value.toLowerCase()),
  ),
)
async function load() {
  busy.value = true
  error.value = ''
  try {
    const [s, m] = await Promise.all([request('/agri-api/stats'), request('/agri-api/mappings')])
    stats.value = s.data || {}
    mappings.value = m.data || []
  } catch (e) {
    error.value =
      tr('农业集成服务不可用。', 'Agricultural integration service unavailable. ') + errorText(e)
  } finally {
    busy.value = false
  }
}
async function scan() {
  busy.value = true
  error.value = ''
  try {
    await request('/agri-api/refresh', { method: 'POST' })
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
