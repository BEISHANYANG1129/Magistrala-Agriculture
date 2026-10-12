<template>
  <div class="channels-container">
    <div class="channels-header">
      <h2>Channels</h2>
      <div class="channels-actions">
        <button class="btn-create" @click="showCreate = true">+ Create</button>
        <button class="btn-upload">Upload</button>
      </div>
    </div>
    <div class="channels-toolbar">
      <input class="search-input" v-model="search" placeholder="Search Channel" />
    </div>
    <p v-if="error" role="alert" class="empty-tip">{{ error }}</p>
    <table class="channels-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Route</th>
          <th>Tags</th>
          <th>Status</th>
          <th>Created At</th>
          <th>Updated At</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="filteredChannels.length === 0">
          <td colspan="6" class="empty-tip">No channels found.</td>
        </tr>
        <tr
          v-for="channel in filteredChannels"
          :key="channel.id"
          class="channel-row"
          @click="goChannel(channel.id)"
          style="cursor:pointer;"
        >
          <td>{{ channel.name }}</td>
          <td>{{ channel.route ? channel.route : '-' }}</td>
          <td>{{ channel.tags?.length ? channel.tags.join(', ') : '-' }}</td>
          <td>
            <span>{{ channel.status || '—' }}</span>
          </td>
          <td>{{ formatDate(channel.created_at) }}</td>
          <td>{{ formatDate(channel.updated_at) }}</td>
        </tr>
      </tbody>
    </table>
    <div class="channels-pagination">
      Rows per page
      <select v-model="rowsPerPage">
        <option :value="10">10</option>
        <option :value="20">20</option>
      </select>
      Page {{ page }} of {{ totalPages }}
      <button @click="prevPage" :disabled="page === 1">&lt;</button>
      <button @click="nextPage" :disabled="page === totalPages">&gt;</button>
    </div>

    <!-- 新建 Channel 弹窗 -->
    <div v-if="showCreate" class="modal-mask">
      <div class="modal-wrapper">
        <div class="modal-container">
          <h3>Create Channel</h3>
          <form @submit.prevent="createChannel">
            <div class="form-row">
              <label>Name *</label>
              <input v-model="form.name" required placeholder="Enter Name" />
            </div>
            <div class="form-row">
              <label>Route</label>
              <input v-model="form.route" placeholder="Enter route" />
              <small class="tip">A user-friendly alias for this channel's ID, useful for referencing or subscribing without the full UUID.<br>
                <b>It is set only during creation and cannot be changed later.</b> Choose something short and descriptive.</small>
            </div>
            <div class="form-row">
              <label>Tags</label>
              <input v-model="form.tags" placeholder="Enter tags" />
            </div>
            <div class="form-row">
              <label>Status（由核心管理）</label>
              <select v-model="form.status" disabled>
                <option value="enabled">Enabled</option>
                <option value="disabled">Disabled</option>
              </select>
            </div>
            <div class="form-row">
              <label>Background Image</label>
              <input type="file" @change="handleBgUpload" />
              <div v-if="form.background" style="margin-top:8px;">
                <img :src="form.background" alt="bg" style="max-width:100%;max-height:120px;border-radius:8px;" />
              </div>
            </div>
            <div class="form-row">
              <label>Metadata</label>
              <textarea v-model="form.metadata" rows="4" placeholder='{"partitions":[{"id":"A","name":"A区","shape":"polygon","points":[[100,200],[150,250],[120,300]],"color":"#ff0000"}]}'></textarea>
              <small class="tip">建议分区信息由系统自动生成，格式如 partitions 字段。</small>
            </div>
            <div class="modal-actions">
              <button type="button" @click="showCreate = false">Close</button>
              <button type="submit">Create</button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { graphql } from '../api/atom'

const route = useRoute()
const router = useRouter()
const domainId = route.params.id
const error = ref('')

const channels = ref([])
const search = ref('')
const rowsPerPage = ref(10)
const page = ref(1)

const showCreate = ref(false)
const form = ref({
  name: '',
  route: '',
  tags: '',
  status: 'enabled',
  background: '',
  metadata: '{}'
})

// 九宫格分区生成
function autoPartition(rows = 3, cols = 3) {
  const partitions = []
  let idx = 0
  const colors = [
    '#ffb3b3', '#b3ffb3', '#b3b3ff',
    '#ffe0b3', '#b3fff0', '#e0b3ff',
    '#ffd6b3', '#b3d6ff', '#d6ffb3'
  ]
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const x0 = c / cols
      const y0 = r / rows
      const x1 = (c + 1) / cols
      const y1 = (r + 1) / rows
      partitions.push({
        id: `G${r * cols + c + 1}`,
        name: `分区${r * cols + c + 1}`,
        shape: 'rect',
        // points用比例坐标
        points: [
          [x0, y0],
          [x1, y0],
          [x1, y1],
          [x0, y1]
        ],
        color: colors[idx++ % colors.length]
      })
    }
  }
  return partitions
}

/**
 * 处理底图上传
 * 1. 用户选择图片后，先上传到图片服务
 * 2. 上传成功后，获取图片URL，自动生成九宫格分区并填充到 metadata
 */
async function handleBgUpload(e) {
  const file = e.target.files[0]
  if (!file) return

  // 构造 FormData 用于图片上传
  const formData = new FormData()
  formData.append('file', file, file.name)

  // 上传图片到图片服务
  let imageUrl = ''
  try {
    const res = await fetch('/image-upload/upload', {
      method: 'POST',
      body: formData
    })
    if (!res.ok) {
      alert('图片上传失败')
      return
    }
    // 假设服务直接返回图片URL字符串
    imageUrl = await res.text()
  } catch (err) {
    alert('图片上传异常: ' + err)
    return
  }

  // 用图片URL作为底图路径
  form.value.background = imageUrl

  // 自动九宫格分区（默认800x600，可根据实际图片尺寸调整）
  const partitions = autoPartition(3, 3)
  const metadataObj = {
    background: imageUrl,
    partitions
  }
  // 自动填充 metadata 字段
  form.value.metadata = JSON.stringify(metadataObj, null, 2)
}

async function createChannel() {
  if (!form.value.name) return
  let metadataObj = {}
  try {
    metadataObj = JSON.parse(form.value.metadata)
  } catch {
    alert('Metadata 格式错误，应为 JSON')
    return
  }
  try {
    await graphql(`mutation($input: CreateResourceInput!) { createResource(input: $input) { id } }`, {
      input: { kind: 'channel', tenantId: String(domainId), name: form.value.name, alias: form.value.route || undefined,
        attributes: { ...metadataObj, tags: form.value.tags ? form.value.tags.split(',').map(t => t.trim()) : [] } }
    })
    showCreate.value = false
    form.value = { name: '', route: '', tags: '', status: 'enabled', background: '', metadata: '{}' }
    await fetchChannels()
  } catch (caught) {
    alert(caught instanceof Error ? caught.message : '创建失败')
  }
}

function formatDate(dateStr) {
  if (!dateStr || dateStr.startsWith('0001-01-01')) return 'Not updated yet'
  const d = new Date(dateStr)
  return d.toLocaleString()
}

onMounted(async () => {
  await fetchChannels()
})

async function fetchChannels() {
  try {
    const data = await graphql(`query($id: ID!) { resources(tenantId: $id, kind: "channel", limit: 100) { items { id name alias attributes createdAt updatedAt } } }`, { id: String(domainId) })
    channels.value = (data.resources?.items || []).map(item => ({
      ...item, route: item.alias, tags: item.attributes?.tags || [], created_at: item.createdAt, updated_at: item.updatedAt
    }))
    error.value = ''
  } catch (caught) {
    channels.value = []
    error.value = caught instanceof Error ? caught.message : '频道加载失败'
  }
}

const filteredChannels = computed(() => {
  let result = channels.value
  if (search.value) {
    result = result.filter(c => c.name && c.name.toLowerCase().includes(search.value.toLowerCase()))
  }
  const start = (page.value - 1) * rowsPerPage.value
  return result.slice(start, start + rowsPerPage.value)
})

const totalPages = computed(() => Math.max(1, Math.ceil(channels.value.length / rowsPerPage.value)))

function prevPage() {
  if (page.value > 1) page.value--
}
function nextPage() {
  if (page.value < totalPages.value) page.value++
}

function goChannel(channelId) {
  router.push(`/domain/${domainId}/channels/${channelId}`)
}
</script>

<style scoped>
.channels-container {
  background: #f8fcfc;
  border-radius: 16px;
  box-shadow: 0 0 24px #eee;
  padding-bottom: 40px;
}
.channels-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px 32px 0 32px;
}
.channels-actions button {
  margin-left: 8px;
  padding: 6px 16px;
  border-radius: 4px;
  border: none;
  background: #174e8a;
  color: #fff;
  font-weight: bold;
  cursor: pointer;
}
.channels-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 32px 0 32px;
}
.search-input {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid #ccc;
  font-size: 1em;
}
.channels-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 16px;
  background: #fff;
  box-shadow: 0 2px 8px #eee;
}
.channels-table th, .channels-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #eee;
  text-align: left;
}
.channel-row:hover {
  background: #eaf7f6;
}
.status-enabled {
  color: #0a3566;
  font-weight: bold;
}
.status-disabled {
  color: #e67c6b;
  font-weight: bold;
}
.channels-pagination {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.empty-tip {
  color: #888;
  font-size: 1.2em;
  text-align: center;
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
.form-row input, .form-row textarea, .form-row select {
  padding: 6px 8px;
  border-radius: 4px;
  border: 1px solid #ccc;
}
.tip {
  font-size: 0.9em;
  color: #888;
  margin-top: 2px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
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
</style>
