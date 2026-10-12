<template>
  <div class="clients-container">
    <div class="clients-header">
      <h2>Devices</h2>
      <div class="clients-actions">
        <button class="btn-create" @click="showCreate = true">+ Create</button>
        <button class="btn-upload">Upload</button>
      </div>
    </div>
    <div class="clients-toolbar">
      <input class="search-input" v-model="search" placeholder="Search Device" />
    </div>
    <p v-if="error" role="alert" class="empty-tip">{{ error }}</p>
    <table class="clients-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Tags</th> <!-- 修改为展示 tags -->
          <th>Area</th>
          <th>Status</th>
          <th>Created At</th>
          <th>Updated At</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="filteredClients.length === 0">
          <td colspan="6" class="empty-tip">No clients found.</td>
        </tr>
        <tr
          v-for="client in filteredClients"
          :key="client.id"
          class="client-row"
          @click="goClient(client.id)"
          style="cursor:pointer;"
        >
          <td>{{ client.name }}</td>
          <td>{{ client.tags ? client.tags.join(', ') : '-' }}</td> <!-- 展示 tags -->
          <td>{{ client.metadata?.area || '-' }}</td>
          <td>
            <span :class="client.status === 'enabled' ? 'status-enabled' : 'status-disabled'">
              {{ client.status === 'enabled' ? 'Enabled' : 'Disabled' }}
            </span>
          </td>
          <td>{{ formatDate(client.created_at) }}</td>
          <td>{{ formatDate(client.updated_at) }}</td>
        </tr>
      </tbody>
    </table>
    <div class="clients-pagination">
      Rows per page
      <select v-model="rowsPerPage">
        <option :value="10">10</option>
        <option :value="20">20</option>
      </select>
      Page {{ page }} of {{ totalPages }}
      <button @click="prevPage" :disabled="page === 1">&lt;</button>
      <button @click="nextPage" :disabled="page === totalPages">&gt;</button>
    </div>

    <!-- 新增 Client 弹窗 -->
    <div v-if="showCreate" class="modal-mask">
      <div class="modal-wrapper">
        <div class="modal-container">
          <h3>Create Device</h3>
          <form @submit.prevent="createClient">
            <div class="form-row">
              <label>Name *</label>
              <input v-model="form.name" required placeholder="Enter Name" />
            </div>
            <div class="form-row">
              <label>Key</label>
              <input v-model="form.key" placeholder="Enter client key" />
            </div>
            <div class="form-row">
              <label>Tags</label>
              <input v-model="form.tags" placeholder="Enter tags" />
            </div>
            <div class="form-row">
              <label>Metadata</label>
              <textarea v-model="form.metadata" rows="4" placeholder='{"area":"A区"}'></textarea>
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

const clients = ref([])
const search = ref('')
const rowsPerPage = ref(10)
const page = ref(1)

const showCreate = ref(false)
const form = ref({
  name: '',
  key: '',
  tags: '',
  metadata: '{}'
})

function formatDate(dateStr) {
  if (!dateStr || dateStr.startsWith('0001-01-01')) return 'Not updated yet'
  const d = new Date(dateStr)
  return d.toLocaleString()
}

onMounted(async () => {
  await fetchClients()
})

async function fetchClients() {
  try {
    const data = await graphql(`query($id: ID!) { entities(tenantId: $id, kind: "device", limit: 100) { items { id name alias status attributes createdAt updatedAt } } }`, { id: String(domainId) })
    clients.value = (data.entities?.items || []).map(item => ({
      ...item, tags: item.attributes?.tags || [], metadata: item.attributes || {}, created_at: item.createdAt, updated_at: item.updatedAt
    }))
    error.value = ''
  } catch (caught) {
    clients.value = []
    error.value = caught instanceof Error ? caught.message : '设备加载失败'
  }
}

async function createClient() {
  alert('当前核心创建设备需要指定 Profile。请在核心管理界面 http://localhost:3000 创建。')
}

const filteredClients = computed(() => {
  let result = clients.value
  if (search.value) {
    result = result.filter(c => c.name && c.name.toLowerCase().includes(search.value.toLowerCase()))
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

function goClient(clientId) {
  router.push(`/domain/${domainId}/devices/${clientId}`)
}
</script>

<style scoped>
.clients-container {
  background: #f8fcfc;
  border-radius: 16px;
  box-shadow: 0 0 24px #eee;
  padding-bottom: 40px;
}
.clients-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px 32px 0 32px;
}
.clients-actions button {
  margin-left: 8px;
  padding: 6px 16px;
  border-radius: 4px;
  border: none;
  background: #174e8a;
  color: #fff;
  font-weight: bold;
  cursor: pointer;
}
.clients-toolbar {
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
.clients-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 16px;
  background: #fff;
  box-shadow: 0 2px 8px #eee;
}
.clients-table th, .clients-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #eee;
  text-align: left;
}
.client-row:hover {
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
.clients-pagination {
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
.form-row input, .form-row textarea {
  padding: 6px 8px;
  border-radius: 4px;
  border: 1px solid #ccc;
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
