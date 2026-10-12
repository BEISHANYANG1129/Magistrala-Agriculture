<template>
  <div class="channel-messages-card">
    <div class="messages-toolbar">
      <input
        class="search-input"
        v-model="search"
        placeholder="Search Messages"
      />
      <div class="toolbar-actions">
        <button class="toolbar-btn">Download Messages</button>
        <button class="toolbar-btn" @click="openAnalyze">智能分析</button>
      </div>
    </div>
    <table class="messages-table">
      <thead>
        <tr>
          <th>选择</th>
          <th>Publisher</th>
          <th>Subtopic</th>
          <th>Protocol</th>
          <th>Name</th>
          <th>Unit</th>
          <th>内容</th>
          <th>分区</th>
          <th>时间</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(msg, idx) in enrichedMessages" :key="msg.id">
          <td>
            <input type="checkbox" v-model="selected[idx]" />
          </td>
          <td>{{ msg.publisher }}</td>
          <td>{{ msg.subtopic }}</td>
          <td>{{ msg.protocol }}</td>
          <td>{{ msg.name }}</td>
          <td>{{ msg.unit }}</td>
          <td>
            <span v-if="isImage(msg.string_value)">
              <img
                :src="msg.string_value"
                class="msg-img"
                @click="showImg(msg.string_value)"
                alt="图片"
              />
            </span>
            <span v-else-if="msg.value !== undefined">{{ msg.value }}</span>
            <span v-else>{{ msg.string_value }}</span>
          </td>
          <td>{{ msg.partition_name || msg.partition_id || '-' }}</td> <!-- 展示分区名或ID -->
          <td>{{ formatTime(msg.sent_at) }}</td>
        </tr>
      </tbody>
    </table>
    <div class="messages-pagination">
      Rows per page
      <select v-model="rowsPerPage">
        <option :value="10">10</option>
        <option :value="20">20</option>
      </select>
      Page {{ page }} of {{ totalPages }}
      <button @click="prevPage" :disabled="page === 1">&lt;</button>
      <button @click="nextPage" :disabled="page === totalPages">&gt;</button>
    </div>

    <!-- 智能分析弹窗 -->
    <div v-if="analyzeVisible" class="analyze-modal">
      <div class="analyze-content">
        <div v-if="analyzeLoading" class="analyze-loading">
          正在分析，请稍候...
        </div>
        <div v-else>
          <div v-if="analyzeResult">
            <h3>智能分析结果</h3>
            <b>一句话总结：</b>{{ analyzeResult.summary || '' }}<br>
            <b>诊断：</b>{{ analyzeResult.diagnosis || '' }}<br>
            <b>风险：</b>{{ (analyzeResult.risks || []).join('，') }}<br>
            <b>建议：</b>{{ (analyzeResult.suggestions || []).join('，') }}<br>
            <b>详细分析：</b>{{ analyzeResult.raw_analysis || '' }}
          </div>
          <div v-else>
            <span style="color: #e74c3c;">分析失败或无结果</span>
          </div>
        </div>
        <button class="analyze-close-btn" @click="analyzeVisible = false">关闭</button>
      </div>
    </div>

    <!-- 图片放大弹窗 -->
    <div v-if="imgModalVisible" class="img-modal" @click="imgModalVisible = false">
      <img :src="imgModalSrc" alt="放大图片" />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const domainId = route.params.id
const channelId = route.params.channelId
const userToken = localStorage.getItem('token')
const analyzeApi = 'http://localhost:8091/analyze'

const messages = ref([])
const total = ref(0)
const search = ref('')
const rowsPerPage = ref(10)
const page = ref(1)
const loading = ref(false)
const selected = ref([])

const analyzeVisible = ref(false)
const analyzeLoading = ref(false)
const analyzeResult = ref(null)

const imgModalVisible = ref(false)
const imgModalSrc = ref('')

// 新增：保存 channel 下所有 client 信息
const clients = ref([])

// 构建 clientId 到 client 的映射
const clientMap = computed(() => {
  const map = {}
  clients.value.forEach(c => { map[c.id] = c })
  return map
})

// 展示时将分区信息合并到 message
const enrichedMessages = computed(() =>
  messages.value.map(msg => {
    const client = clientMap.value[msg.publisher]
    
    return {
      ...msg,
      partition_id: client?.metadata?.partition_id || '',
      partition_name: client?.metadata?.partition_name || '',
      deviceType: client?.tags?.[0] || '',
      client_name: client?.name || ''
    }
  })
)

function isImage(url) {
  return url && /^https?:\/\/.+\.(png|jpg|jpeg|gif|bmp)$/i.test(url)
}

function showImg(src) {
  imgModalSrc.value = src
  imgModalVisible.value = true
}

/**
 * 获取当前 channel 下已 connect 的 client 列表
 */
async function fetchClients() {
  const res = await fetch(
    `/Clients/${domainId}/clients?channel=${channelId}`,
    {
      headers: { 'Authorization': `Bearer ${userToken}` }
    }
  )
  if (res.ok) {
    const data = await res.json()
    clients.value = data.clients || []
  } else {
    clients.value = []
  }
}

async function fetchMessages() {
  loading.value = true
  const offset = (page.value - 1) * rowsPerPage.value
  const limit = rowsPerPage.value
  let url = `/Messages/${domainId}/channels/${channelId}/messages?offset=${offset}&limit=${limit}`
  if (search.value) {
    url += `&search=${encodeURIComponent(search.value)}`
  }
  const res = await fetch(url, {
    headers: {
      'Authorization': `Bearer ${userToken}`
    }
  })
  if (res.ok) {
    const data = await res.json()
    messages.value = data.messages || []
    total.value = data.total || messages.value.length
    selected.value = messages.value.map(() => true) // 默认全选
  }
  loading.value = false
}

onMounted(async () => {
  await fetchClients()
  await fetchMessages()
})
watch([page, rowsPerPage, search], fetchMessages)

/**
 * 智能分析相关
 */
function openAnalyze() {
  analyzeVisible.value = true
  analyzeLoading.value = true
  analyzeResult.value = null

  // 收集选中的消息，合并分区信息
  const selectedMessages = enrichedMessages.value
    .map((msg, idx) => selected.value[idx] ? msg : null)
    .filter(Boolean)
    .map(msg => ({
      name: msg.name,
      unit: msg.unit,
      partition_name: msg.partition_name || '',
      partition_id: msg.partition_id || '',
      value: msg.value,
      string_value: msg.string_value
    }))

  if (selectedMessages.length === 0) {
    analyzeLoading.value = false
    analyzeResult.value = null
    return
  }

  fetch(analyzeApi, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages: selectedMessages })
  })
    .then(res => res.json())
    .then(data => {
      analyzeResult.value = data
      analyzeLoading.value = false
    })
    .catch(() => {
      analyzeResult.value = null
      analyzeLoading.value = false
    })
}

const totalPages = computed(() =>
  Math.max(1, Math.ceil(total.value / rowsPerPage.value))
)

function prevPage() {
  if (page.value > 1) page.value--
}
function nextPage() {
  if (page.value < totalPages.value) page.value++
}

function formatTime(ts) {
  if (!ts) return "";
  // 支持毫秒或秒时间戳
  let date = new Date(
    String(ts).length > 10 ? Number(ts) : Number(ts) * 1000
  );
  return date.toLocaleString();
}
</script>

<style scoped>
.channel-messages-card {
  min-width: 0;
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 0 24px #eee;
  padding: 32px 32px 24px 32px;
  align-self: flex-start;
  position: relative;
}

.messages-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 18px;
}
.search-input {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid #ccc;
  font-size: 1em;
  width: 220px;
}
.toolbar-actions {
  display: flex;
  gap: 12px;
}
.toolbar-btn {
  background: #174e8a;
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 8px 18px;
  font-size: 1em;
  cursor: pointer;
  font-weight: bold;
}
.messages-table {
  width: 100%;
  table-layout: fixed;
  word-break: break-all;
}
.messages-table th,
.messages-table td {
  word-break: break-all;
  white-space: normal;
  overflow-wrap: break-word;
  max-width: 180px;
}
.messages-table th {
  background: #f8fcfc;
  font-weight: bold;
}
.messages-pagination {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.msg-img {
  max-width: 120px;
  max-height: 80px;
  border-radius: 4px;
  cursor: pointer;
  transition: box-shadow 0.2s;
}
.msg-img:hover {
  box-shadow: 0 0 8px #2980b9;
}

/* 智能分析弹窗样式 */
.analyze-modal {
  position: fixed;
  left: 0; top: 0;
  width: 100vw; height: 100vh;
  background: rgba(0,0,0,0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}
.analyze-content {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 16px #aaa;
  padding: 32px 28px 24px 28px;
  min-width: 340px;
  max-width: 90vw;
  max-height: 80vh;
  overflow-y: auto;
  position: relative;
}
.analyze-loading {
  text-align: center;
  font-size: 1.2em;
  color: #174e8a;
  padding: 32px 0;
}
.analyze-close-btn {
  margin-top: 24px;
  background: #174e8a;
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 10px 24px;
  font-size: 1em;
  cursor: pointer;
  font-weight: bold;
  display: block;
  width: 100%;
}

/* 图片弹窗 */
.img-modal {
  position: fixed;
  left: 0; top: 0;
  width: 100vw; height: 100vh;
  background: rgba(0,0,0,0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9998;
}
.img-modal img {
  max-width: 90vw;
  max-height: 90vh;
  border-radius: 8px;
}
</style>