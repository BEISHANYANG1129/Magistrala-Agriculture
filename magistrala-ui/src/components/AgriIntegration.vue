<template>
  <div class="agri-integration">
    <div class="header">
      <h2>农业数据集成管理</h2>
      <div class="header-actions">
        <button 
          class="btn btn-refresh" 
          @click="refreshData"
          :disabled="loading"
        >
          <span v-if="loading" class="loading-spinner">⟳</span>
          刷新数据
        </button>
        <button 
          class="btn btn-refresh-sensors" 
          @click="refreshSensors"
          :disabled="loading"
        >
          重新扫描传感器
        </button>
      </div>
    </div>

    <!-- 统计信息卡片 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">📡</div>
        <div class="stat-content">
          <div class="stat-value">{{ stats.total_sensors || 0 }}</div>
          <div class="stat-label">发现传感器</div>
        </div>
      </div>
      
      <div class="stat-card">
        <div class="stat-icon">🔗</div>
        <div class="stat-content">
          <div class="stat-value">{{ stats.active_mappings || 0 }}</div>
          <div class="stat-label">活跃映射</div>
        </div>
      </div>
      
      <div class="stat-card">
        <div class="stat-icon">📨</div>
        <div class="stat-content">
          <div class="stat-value">{{ stats.messages_sent || 0 }}</div>
          <div class="stat-label">消息发送</div>
        </div>
      </div>
      
      <div class="stat-card" :class="{ 'error': stats.last_error }">
        <div class="stat-icon">{{ stats.is_running ? '🟢' : '🔴' }}</div>
        <div class="stat-content">
          <div class="stat-value">{{ stats.is_running ? '运行中' : '已停止' }}</div>
          <div class="stat-label">服务状态</div>
        </div>
      </div>
    </div>

    <!-- 错误信息 -->
    <div v-if="stats.last_error" class="error-banner">
      <div class="error-icon">⚠️</div>
      <div class="error-content">
        <div class="error-title">集成服务错误</div>
        <div class="error-message">{{ stats.last_error }}</div>
        <div class="error-count">错误次数: {{ stats.sync_errors }}</div>
      </div>
    </div>

    <!-- 最后同步信息 -->
    <div v-if="stats.last_sync" class="sync-info">
      <span class="sync-label">最后同步:</span>
      <span class="sync-time">{{ formatTimestamp(stats.last_sync) }}</span>
    </div>

    <!-- 传感器映射列表 -->
    <div class="mappings-section">
      <h3>传感器映射 ({{ mappings.length }})</h3>
      
      <!-- 过滤器 -->
      <div class="filters">
        <input 
          v-model="searchFilter" 
          type="text" 
          placeholder="搜索传感器名称或设备..." 
          class="search-input"
        >
        <select v-model="partitionFilter" class="partition-filter">
          <option value="">所有分区</option>
          <option v-for="partition in partitions" :key="partition" :value="partition">
            {{ partition }}
          </option>
        </select>
        <select v-model="statusFilter" class="status-filter">
          <option value="">所有状态</option>
          <option value="active">已激活</option>
          <option value="inactive">未激活</option>
        </select>
      </div>

      <!-- 映射表格 -->
      <div class="mappings-table-container">
        <table class="mappings-table">
          <thead>
            <tr>
              <th>传感器名称</th>
              <th>设备信息</th>
              <th>Magistrala客户端</th>
              <th>分区</th>
              <th>位置</th>
              <th>最后更新</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="mapping in filteredMappings" :key="mappingKey(mapping)">
              <td>
                <div class="sensor-info">
                  <div class="sensor-name">{{ mapping.factorName }}</div>
                  <div class="sensor-unit">{{ mapping.unit }}</div>
                </div>
              </td>
              <td>
                <div class="device-info">
                  <div class="device-name">{{ mapping.deviceName }}</div>
                  <div class="device-details">
                    设备:{{ mapping.deviceAddr }} | 节点:{{ mapping.nodeId }} | 寄存器:{{ mapping.registerId }}
                  </div>
                </div>
              </td>
              <td>
                <div class="client-info">
                  <div class="client-name">{{ mapping.clientName }}</div>
                  <div class="client-id">{{ mapping.clientId?.substring(0, 8) }}...</div>
                </div>
              </td>
              <td>
                <span class="partition-badge" :class="`partition-${mapping.partition}`">
                  {{ mapping.partition }}
                </span>
              </td>
              <td>
                <div class="position-info">
                  {{ Math.round(mapping.position?.x) }}%, {{ Math.round(mapping.position?.y) }}%
                </div>
              </td>
              <td>
                <div class="update-info">
                  <div v-if="mapping.lastUpdate" class="last-update">
                    {{ formatTimestamp(mapping.lastUpdate) }}
                  </div>
                  <div v-if="mapping.lastValue" class="last-value">
                    {{ mapping.lastValue }}
                  </div>
                </div>
              </td>
              <td>
                <span class="status-badge" :class="{ 'active': mapping.isActive, 'inactive': !mapping.isActive }">
                  {{ mapping.isActive ? '激活' : '未激活' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
        
        <div v-if="filteredMappings.length === 0" class="no-data">
          {{ mappings.length === 0 ? '暂无传感器映射数据' : '没有符合条件的传感器' }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

// 响应式数据
const stats = ref({})
const mappings = ref([])
const loading = ref(false)
const searchFilter = ref('')
const partitionFilter = ref('')
const statusFilter = ref('')

// 自动刷新定时器
let refreshTimer = null

// 计算属性
const partitions = computed(() => {
  const partitionSet = new Set()
  mappings.value.forEach(mapping => {
    if (mapping.partition) {
      partitionSet.add(mapping.partition)
    }
  })
  return Array.from(partitionSet).sort()
})

const filteredMappings = computed(() => {
  return mappings.value.filter(mapping => {
    // 搜索过滤
    if (searchFilter.value) {
      const search = searchFilter.value.toLowerCase()
      if (!mapping.factorName?.toLowerCase().includes(search) &&
          !mapping.deviceName?.toLowerCase().includes(search)) {
        return false
      }
    }
    
    // 分区过滤
    if (partitionFilter.value && mapping.partition !== partitionFilter.value) {
      return false
    }
    
    // 状态过滤
    if (statusFilter.value) {
      if (statusFilter.value === 'active' && !mapping.isActive) return false
      if (statusFilter.value === 'inactive' && mapping.isActive) return false
    }
    
    return true
  })
})

// 方法
const mappingKey = (mapping) => {
  return `${mapping.deviceAddr}_${mapping.nodeId}_${mapping.registerId}`
}

const formatTimestamp = (timestamp) => {
  if (!timestamp) return '从未'
  const date = new Date(timestamp * 1000)
  return date.toLocaleString('zh-CN')
}

const loadStats = async () => {
  try {
    const response = await fetch('/agri-api/stats')
    if (response.ok) {
      const result = await response.json()
      stats.value = result.data || {}
    }
  } catch (error) {
    console.error('Failed to load stats:', error)
  }
}

const loadMappings = async () => {
  try {
    const response = await fetch('/agri-api/mappings')
    if (response.ok) {
      const result = await response.json()
      mappings.value = result.data || []
    }
  } catch (error) {
    console.error('Failed to load mappings:', error)
  }
}

const refreshData = async () => {
  loading.value = true
  try {
    await Promise.all([loadStats(), loadMappings()])
  } finally {
    loading.value = false
  }
}

const refreshSensors = async () => {
  loading.value = true
  try {
    const response = await fetch('/agri-api/refresh', {
      method: 'POST'
    })
    
    if (response.ok) {
      // 刷新成功后重新加载数据
      setTimeout(() => {
        refreshData()
      }, 1000)
    } else {
      console.error('Failed to refresh sensors')
    }
  } catch (error) {
    console.error('Failed to refresh sensors:', error)
  } finally {
    loading.value = false
  }
}

const startAutoRefresh = () => {
  refreshTimer = setInterval(refreshData, 10000) // 每10秒刷新一次
}

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

// 生命周期
onMounted(() => {
  refreshData()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.agri-integration {
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header h2 {
  margin: 0;
  color: #333;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  transition: background-color 0.3s;
}

.btn-refresh {
  background-color: #4CAF50;
  color: white;
}

.btn-refresh:hover:not(:disabled) {
  background-color: #45a049;
}

.btn-refresh-sensors {
  background-color: #2196F3;
  color: white;
}

.btn-refresh-sensors:hover:not(:disabled) {
  background-color: #1976D2;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.loading-spinner {
  display: inline-block;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  background: white;
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  display: flex;
  align-items: center;
  gap: 12px;
}

.stat-card.error {
  border-left: 4px solid #f44336;
}

.stat-icon {
  font-size: 24px;
}

.stat-content {
  flex: 1;
}

.stat-value {
  font-size: 20px;
  font-weight: bold;
  color: #333;
}

.stat-label {
  font-size: 12px;
  color: #666;
  margin-top: 2px;
}

.error-banner {
  background-color: #fff3cd;
  border: 1px solid #ffeaa7;
  border-radius: 4px;
  padding: 12px;
  margin-bottom: 20px;
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.error-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.error-content {
  flex: 1;
}

.error-title {
  font-weight: bold;
  color: #856404;
  margin-bottom: 4px;
}

.error-message {
  color: #856404;
  margin-bottom: 4px;
}

.error-count {
  font-size: 12px;
  color: #6c6c6c;
}

.sync-info {
  background-color: #e8f5e8;
  border-radius: 4px;
  padding: 8px 12px;
  margin-bottom: 20px;
  font-size: 14px;
}

.sync-label {
  color: #2e7d2e;
  font-weight: 500;
}

.sync-time {
  color: #1b5e1b;
  margin-left: 8px;
}

.mappings-section h3 {
  margin-bottom: 16px;
  color: #333;
}

.filters {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.search-input, .partition-filter, .status-filter {
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
}

.search-input {
  flex: 1;
  min-width: 200px;
}

.mappings-table-container {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  overflow: hidden;
}

.mappings-table {
  width: 100%;
  border-collapse: collapse;
}

.mappings-table th {
  background-color: #f5f5f5;
  padding: 12px 8px;
  text-align: left;
  font-weight: 500;
  color: #333;
  border-bottom: 1px solid #ddd;
  font-size: 14px;
}

.mappings-table td {
  padding: 12px 8px;
  border-bottom: 1px solid #eee;
  vertical-align: top;
  font-size: 13px;
}

.mappings-table tbody tr:hover {
  background-color: #f9f9f9;
}

.sensor-info .sensor-name {
  font-weight: 500;
  color: #333;
  margin-bottom: 2px;
}

.sensor-info .sensor-unit {
  font-size: 11px;
  color: #666;
}

.device-info .device-name {
  font-weight: 500;
  color: #333;
  margin-bottom: 2px;
}

.device-info .device-details {
  font-size: 11px;
  color: #666;
}

.client-info .client-name {
  font-weight: 500;
  color: #333;
  margin-bottom: 2px;
}

.client-info .client-id {
  font-size: 11px;
  color: #666;
  font-family: monospace;
}

.partition-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
  background-color: #e3f2fd;
  color: #1976d2;
}

.partition-field_1 { background-color: #e8f5e8; color: #2e7d2e; }
.partition-field_2 { background-color: #fff3e0; color: #f57c00; }
.partition-field_3 { background-color: #fce4ec; color: #c2185b; }
.partition-greenhouse { background-color: #f3e5f5; color: #7b1fa2; }

.position-info {
  font-size: 11px;
  color: #666;
  font-family: monospace;
}

.update-info .last-update {
  font-size: 11px;
  color: #333;
  margin-bottom: 2px;
}

.update-info .last-value {
  font-size: 11px;
  color: #666;
  font-family: monospace;
}

.status-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
}

.status-badge.active {
  background-color: #e8f5e8;
  color: #2e7d2e;
}

.status-badge.inactive {
  background-color: #ffeaa7;
  color: #856404;
}

.no-data {
  text-align: center;
  padding: 40px 20px;
  color: #666;
  font-style: italic;
}

@media (max-width: 768px) {
  .agri-integration {
    padding: 16px;
  }
  
  .header {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }
  
  .header-actions {
    justify-content: center;
  }
  
  .stats-grid {
    grid-template-columns: 1fr;
  }
  
  .filters {
    flex-direction: column;
  }
  
  .search-input {
    min-width: unset;
  }
  
  .mappings-table-container {
    overflow-x: auto;
  }
  
  .mappings-table {
    min-width: 800px;
  }
}
</style>
