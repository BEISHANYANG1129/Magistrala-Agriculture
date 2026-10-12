<template>
  <div class="connections-container">
    <div class="connections-header">
      <h2>Connections</h2>
      <button class="btn-create" @click="showCreate = true">+ Create</button>
    </div>

    <!-- 未定位提醒与筛选 -->
    <div class="connections-banner" v-if="unlocatedCount > 0">
      有 {{ unlocatedCount }} 个设备尚未定位
      <label style="margin-left:12px;">
        <input type="checkbox" v-model="showUnlocatedOnly" />
        仅显示未定位
      </label>
    </div>

    <div class="connections-toolbar">
      <input class="search-input" v-model="search" placeholder="Search Client" />
      <select v-model="sortKey">
        <option value="name">Name</option>
      </select>
      <button class="btn-status">Status</button>
      <button class="btn-view">View</button>
    </div>

    <table class="connections-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Tags</th>
          <th>Status</th>
          <th>Connection Types</th>
          <th>Created At</th>
          <th>Location</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="filteredClients.length === 0">
          <td colspan="7" class="empty-tip">No clients connected yet. Get started by connecting a client to the channel.</td>
        </tr>
        <tr
          v-for="client in filteredClients"
          :key="client.id"
          @click="goClientDetail(client.id)"
          style="cursor:pointer;"
        >
          <td>{{ client.name }}</td>
          <td>{{ client.tags ? client.tags.join(', ') : '-' }}</td>
          <td>{{ client.status || '-' }}</td>
          <td>{{ client.connection_types ? client.connection_types.join(', ') : '-' }}</td>
          <td>{{ formatDate(client.created_at) }}</td>
          <td>{{ hasPosition(client) ? formatPos(client.metadata.position) : '-' }}</td>
          <td>
            <button
              v-if="!hasPosition(client)"
              class="btn-locate"
              @click.stop="openLocate(client)"
            >
              定位
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="connections-pagination">
      Rows per page
      <select v-model="rowsPerPage">
        <option :value="10">10</option>
        <option :value="20">20</option>
      </select>
      Page {{ page }} of {{ totalPages }}
      <button @click="prevPage" :disabled="page === 1">&lt;</button>
      <button @click="nextPage" :disabled="page === totalPages">&gt;</button>
    </div>

    <!-- 新建 Client 弹窗 -->
    <div v-if="showCreate" class="modal-mask">
      <div class="modal-wrapper">
        <div class="modal-container">
          <h3>Create Client</h3>
          <form @submit.prevent="createClient">
            <div class="form-row">
              <label>Name *</label>
              <input v-model="form.name" required placeholder="Enter Name" />
            </div>
            <div class="form-row">
              <label>设备类型 *</label>
              <select v-model="form.deviceType" required>
                <option value="sensor">采集信息设备</option>
                <option value="actuator">执行操作设备</option>
              </select>
            </div>
            <div class="form-row">
              <label>选择位置</label>
              <div style="display:flex;align-items:center;gap:8px;">
                <input v-model="positionStr" readonly placeholder="未选择" style="flex:1;" />
                <button type="button" @click="showPick = true">在底图上选点</button>
              </div>
            </div>
            <div class="modal-actions">
              <button type="button" @click="showCreate = false">取消</button>
              <button type="submit">创建</button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- 选点弹窗（创建或编辑定位） -->
    <div v-if="showPick" class="modal-mask">
      <div class="modal-wrapper" style="max-width:900px;">
        <div class="modal-container">
          <h3>在底图上选择设备位置</h3>
          <div v-if="bgUrl" class="pick-bg-wrap">
            <img
              :src="bgUrl"
              class="pick-bg"
              ref="bgImg"
              @load="onBgLoad"
              @error="onImgError"
              @click="onImgClick"
              style="cursor: crosshair;"
            />
            <svg
              v-if="bgLoaded"
              :width="displayWidth"
              :height="displayHeight"
              class="pick-svg"
              style="position:absolute;left:0;top:0;pointer-events:none;"
            >
              <g v-for="part in partitions" :key="part.id">
                <polygon
                  v-if="part.shape === 'polygon'"
                  :points="polygonPoints(part.points)"
                  :fill="part.color"
                  fill-opacity="0.15"
                  stroke="black"
                  stroke-width="1"
                />
                <rect
                  v-else-if="part.shape === 'rect'"
                  v-bind="rectAttrs(part)"
                  :fill="part.color"
                  fill-opacity="0.15"
                  stroke="black"
                  stroke-width="1"
                />
              </g>
              <circle
                v-if="tempPosition"
                :cx="tempPosition.x * displayWidth"
                :cy="tempPosition.y * displayHeight"
                r="8"
                fill="#174e8a"
                fill-opacity="0.7"
              />
            </svg>
          </div>
          <div v-else class="pick-empty">未设置底图</div>
          <div class="modal-actions">
            <button type="button" @click="showPick = false">取消</button>
            <button type="button" :disabled="!tempPosition" @click="confirmPick">确认</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const domainId = route.params.id
const channelId = route.params.channelId
const userToken = localStorage.getItem('token')

// 客户端列表及分页、搜索相关
const clients = ref([])
const search = ref('')
const sortKey = ref('name')
const rowsPerPage = ref(10)
const page = ref(1)

// 控制新建 client 弹窗和选点弹窗的显示
const showCreate = ref(false)
const showPick = ref(false)

// 新建 client 表单数据
const form = ref({
  name: '',
  deviceType: 'sensor',
  position: null,
  partition_id: null,
  partition_name: null
})

// 选点后显示在输入框的字符串
const positionStr = computed(() =>
  form.value.position ? `${form.value.position.x.toFixed(2)}, ${form.value.position.y.toFixed(2)}` : ''
)

// 物理拓扑底图和分区相关
const bgUrl = ref('')
const partitions = ref([])
const bgImg = ref(null)
const bgLoaded = ref(false)
const displayWidth = ref(800)
const displayHeight = ref(600)

// 未定位筛选与编辑定位状态
const showUnlocatedOnly = ref(false)
const editingClient = ref(null) // 当前在定位的客户端对象

// 页面加载时获取当前 channel 下的 client 列表和底图
onMounted(() => {
  fetchClients()
  fetchChannelMeta()
})

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

/**
 * 获取 channel 的底图和分区信息
 */
async function fetchChannelMeta() {
  const res = await fetch(`/Channels/${domainId}/channels/${channelId}`, {
    headers: { 'Authorization': `Bearer ${userToken}` }
  })
  if (res.ok) {
    const data = await res.json()
    const meta = data.metadata || {}
    bgUrl.value = meta.background || ''
    partitions.value = meta.partitions || []
  }
}

/**
 * 图片加载后获取实际显示宽高
 */
function onBgLoad() {
  nextTick(() => {
    if (bgImg.value) {
      displayWidth.value = bgImg.value.clientWidth
      displayHeight.value = bgImg.value.clientHeight
      bgLoaded.value = true
    }
  })
}

/**
 * 图片加载失败
 */
function onImgError(e) {
  console.error('图片加载失败', bgUrl.value, e)
}

/**
 * 多边形分区点格式化
 */
function polygonPoints(points) {
  return points.map(pt => [
    pt[0] * displayWidth.value,
    pt[1] * displayHeight.value
  ].join(',')).join(' ')
}

/**
 * 矩形分区属性
 */
function rectAttrs(part) {
  const x = part.points[0][0] * displayWidth.value
  const y = part.points[0][1] * displayHeight.value
  const width = (part.points[2][0] - part.points[0][0]) * displayWidth.value
  const height = (part.points[2][1] - part.points[0][1]) * displayHeight.value
  return { x, y, width, height }
}

/**
 * 判断点是否在矩形分区内
 */
function isPointInRect(x, y, rectPoints) {
  const [x0, y0] = rectPoints[0]
  const [x2, y2] = rectPoints[2]
  return x >= x0 && x <= x2 && y >= y0 && y <= y2
}

/**
 * 判断点是否在多边形分区内
 */
function isPointInPolygon(x, y, polygonPoints) {
  let inside = false
  for (let i = 0, j = polygonPoints.length - 1; i < polygonPoints.length; j = i++) {
    const xi = polygonPoints[i][0], yi = polygonPoints[i][1]
    const xj = polygonPoints[j][0], yj = polygonPoints[j][1]
    const intersect = ((yi > y) !== (yj > y)) &&
      (x < (xj - xi) * (y - yi) / ((yj - yi) || 1) + xi)
    if (intersect) inside = !inside
  }
  return inside
}

/**
 * 根据坐标判断所属分区
 */
function getPartitionForPoint(x, y) {
  for (const part of partitions.value) {
    if (part.shape === 'rect' && isPointInRect(x, y, part.points)) {
      return part
    }
    if (part.shape === 'polygon' && isPointInPolygon(x, y, part.points)) {
      return part
    }
  }
  return null
}

/**
 * 选点事件，记录比例坐标
 */
const tempPosition = ref(null) // 临时选点

function onImgClick(e) {
  const rect = bgImg.value.getBoundingClientRect()
  const x = (e.clientX - rect.left) / rect.width
  const y = (e.clientY - rect.top) / rect.height
  tempPosition.value = { x, y }
}

/**
 * 打开编辑定位
 */
function openLocate(client) {
  editingClient.value = client
  tempPosition.value = null
  showPick.value = true
}

/**
 * 确认按钮事件（创建或编辑）
 */
function confirmPick() {
  if (!tempPosition.value) return

  // 编辑已有设备定位：直接更新该 client 的 metadata
  if (editingClient.value) {
    const part = getPartitionForPoint(tempPosition.value.x, tempPosition.value.y)
    const metadata = {
      ...(editingClient.value.metadata || {}),
      position: tempPosition.value,
      partition_id: part ? part.id : null,
      partition_name: part ? part.name : null
    }
    updateClientPosition(editingClient.value.id, metadata)
    showPick.value = false
    return
  }

  // 创建流程：把位置写入表单，后续创建接口会带上 metadata
  form.value.position = tempPosition.value
  const part = getPartitionForPoint(tempPosition.value.x, tempPosition.value.y)
  if (part) {
    form.value.partition_id = part.id
    form.value.partition_name = part.name
  } else {
    form.value.partition_id = null
    form.value.partition_name = null
  }
  showPick.value = false
}

/**
 * 更新 client 的 metadata（仅位置与分区）
 * 若你的后端只支持 PUT，请将 method 改为 'PUT' 并发送完整对象
 */
async function updateClientPosition(clientId, metadata) {
  const res = await fetch(`/Clients/${domainId}/clients/${clientId}`, {
    method: 'PATCH',
    headers: {
      'Authorization': `Bearer ${userToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ metadata })
  })
  if (!res.ok) {
    const err = await res.text()
    alert('定位保存失败: ' + err)
    return
  }
  editingClient.value = null
  await fetchClients()
}

/**
 * 辅助：位置与未定位统计
 */
function hasPosition(client) {
  return client?.metadata?.position &&
    typeof client.metadata.position.x === 'number' &&
    typeof client.metadata.position.y === 'number'
}
function formatPos(pos) {
  return `${pos.x.toFixed(2)}, ${pos.y.toFixed(2)}`
}
const unlocatedCount = computed(() => clients.value.filter(c => !hasPosition(c)).length)

/**
 * 时间格式化工具
 */
function formatDate(dateStr) {
  if (!dateStr || dateStr.startsWith('0001-01-01')) return '-'
  const d = new Date(dateStr)
  return d.toLocaleString()
}

/**
 * 客户端列表的搜索和分页（支持仅显示未定位）
 */
const filteredClients = computed(() => {
  let result = clients.value
  if (search.value) {
    result = result.filter(c => c.name && c.name.toLowerCase().includes(search.value.toLowerCase()))
  }
  if (showUnlocatedOnly.value) {
    result = result.filter(c => !hasPosition(c))
  }
  const start = (page.value - 1) * rowsPerPage.value
  return result.slice(start, start + rowsPerPage.value)
})

const totalPages = computed(() => Math.max(1, Math.ceil(clients.value.length / rowsPerPage.value)))

function prevPage() {
  if (page.value > 1) page.value--
}
function nextPage() {
  if (page.value < totalPages.value) page.value++
}

/**
 * 创建 client 并 connect 到当前 channel
 */
async function createClient() {
  if (!form.value.name || !form.value.position || !form.value.deviceType) {
    alert('请填写名称、类型并选择位置')
    return
  }
  const tagsArr = [form.value.deviceType]
  const body = {
    name: form.value.name,
    tags: tagsArr,
    metadata: {
      position: form.value.position,
      partition_id: form.value.partition_id,
      partition_name: form.value.partition_name
    }
  }
  const res = await fetch(`/Clients/${domainId}/clients`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${userToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  })
  if (!res.ok) {
    alert('创建失败')
    return
  }
  const client = await res.json()

  const connectRes = await fetch(`/Channels/${domainId}/channels/connect`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${userToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      channel_ids: [channelId],
      client_ids: [client.id],
      types: ["publish", "subscribe"]
    })
  })
  if (!connectRes.ok) {
    const errMsg = await connectRes.text()
    console.error('connect error:', errMsg)
    alert('连接失败: ' + errMsg)
    return
  }

  showCreate.value = false
  form.value = { name: '', deviceType: 'sensor', position: null, partition_id: null, partition_name: null }
  await fetchClients()
}

/**
 * 跳转到 client 详情页
 */
function goClientDetail(clientId) {
  router.push(`/domain/${domainId}/devices/${clientId}`)
}
</script>

<style scoped>
.connections-container {
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 0 24px #eee;
  padding: 24px 32px 40px 32px;
  min-width: 0;
}
.connections-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.btn-create {
  padding: 6px 16px;
  border-radius: 4px;
  border: none;
  background: #174e8a;
  color: #fff;
  font-weight: bold;
  cursor: pointer;
}
.connections-banner {
  margin-top: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  background: #fff8e1;
  color: #8a6d3b;
  box-shadow: 0 1px 6px #eee;
}
.connections-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 24px 0 0 0;
}
.search-input {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid #ccc;
  font-size: 1em;
}
.connections-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 16px;
  background: #fff;
  box-shadow: 0 2px 8px #eee;
}
.connections-table th, .connections-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #eee;
  text-align: left;
}
.empty-tip {
  color: #888;
  font-size: 1.2em;
  text-align: center;
}
.connections-pagination {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.modal-mask {
  position: fixed;
  z-index: 9999;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.2);
  display: flex;
  align-items: center;
  justify-content: center;
}
.modal-wrapper {
  width: 100%;
  max-width: 400px;
}
.modal-container {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  box-shadow: 0 2px 16px #aaa;
}
.form-row {
  margin-bottom: 16px;
  display: flex;
  flex-direction: column;
}
.form-row label {
  font-weight: bold;
  margin-bottom: 4px;
}
.form-row input, .form-row select {
  padding: 6px 8px;
  border-radius: 4px;
  border: 1px solid #ccc;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
}
.modal-actions button {
  padding: 6px 16px;
  border-radius: 4px;
  border: none;
  background: #174e8a;
  color: #fff;
  font-weight: bold;
  cursor: pointer;
}
.pick-bg-wrap {
  position: relative;
  width: 100%;
  max-width: 800px;
  margin: 0 auto;
}
.pick-bg {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 12px;
}
.pick-svg {
  position: absolute;
  left: 0; top: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.pick-empty {
  color: #888;
  font-size: 1.2em;
  text-align: center;
  padding: 32px 0;
}
.btn-locate {
  padding: 4px 10px;
  border-radius: 4px;
  border: none;
  background: #ffa000;
  color: #fff;
  cursor: pointer;
}
</style>
