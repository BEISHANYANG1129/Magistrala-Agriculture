<!-- filepath: /home/liar/Magistrala/magistrala-ui/src/components/ClientDetail.vue -->
<template>
  <div class="client-detail-card">
    <p v-if="error" role="alert">{{ error }}</p>
    <h2>Device Configuration</h2>
    <table class="detail-table">
      <tbody>
        <tr>
          <td>Name</td>
          <td>
            <input v-model="client.name" disabled />
          </td>
        </tr>
        <tr>
          <td>ID</td>
          <td>
            {{ client.id }}
            <button @click="copyId">复制</button>
          </td>
        </tr>
        <tr>
          <td>Client Key</td>
          <td>
            <input v-model="client.credentials.identity" disabled />
            <button @click="copyKey" :disabled="!client.credentials.identity">复制</button>
          </td>
        </tr>
        <tr>
          <td>Tags</td>
          <td>
            <span v-if="client.tags && client.tags.length">{{ client.tags.join(', ') }}</span>
            <span v-else>-</span>
          </td>
        </tr>
        <tr>
          <td>Metadata</td>
          <td>
            <button @click="showMeta = true">View Metadata</button>
          </td>
        </tr>
        <tr>
          <td>Status</td>
          <td>
            <span :class="client.status === 'enabled' ? 'status-enabled' : 'status-disabled'">
              <span v-if="client.status === 'enabled'">🛡️ Enabled</span>
              <span v-else>Disabled</span>
            </span>
          </td>
        </tr>
      </tbody>
    </table>
    <button class="delete-btn" disabled title="请在核心管理界面删除">Delete Device</button>

    <!-- Metadata 弹窗 -->
    <div v-if="showMeta" class="modal-mask">
      <div class="modal-wrapper">
        <div class="modal-container">
          <h3>Metadata</h3>
          <pre>{{ JSON.stringify(client.metadata, null, 2) }}</pre>
          <button @click="showMeta = false">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { graphql } from '../api/atom'

const route = useRoute()
const clientId = route.params.clientId
const error = ref('')

const client = ref({
  name: '',
  id: '',
  credentials: { identity: '', secret: '' },
  tags: [],
  metadata: {},
  status: '',
})

const showMeta = ref(false)

function copyId() {
  navigator.clipboard.writeText(client.value.id)
  alert('Client ID copied!')
}

function copyKey() {
  navigator.clipboard.writeText(client.value.credentials.identity)
  alert('Client Key copied!')
}

async function fetchClient() {
  try {
    const result = await graphql('query($id: ID!) { entity(id: $id) { id name alias status attributes } }', { id: String(clientId) })
    const data = result.entity || {}
    client.value = {
      name: data.name || '',
      id: data.id || '',
      credentials: { identity: '', secret: '' },
      tags: data.attributes?.tags || [],
      metadata: data.attributes || {},
      status: data.status || '',
    }
  } catch (caught) { error.value = caught instanceof Error ? caught.message : '设备加载失败' }
}

onMounted(fetchClient)
</script>

<style scoped>
.client-detail-card {
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 0 24px #eee;
  padding: 32px 32px 24px 32px;
  align-self: flex-start;
  max-width: 700px;
}
.detail-table {
  width: 100%;
  margin-bottom: 24px;
  border-collapse: collapse;
}
.detail-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #eee;
}
.detail-table input {
  width: 90%;
  padding: 6px 8px;
  border-radius: 4px;
  border: 1px solid #ccc;
  background: #f8f8f8;
}
.delete-btn {
  background: #e74c3c;
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 10px 24px;
  font-size: 1em;
  cursor: pointer;
  font-weight: bold;
}
.status-enabled {
  color: #0a3566;
  font-weight: bold;
}
.status-disabled {
  color: #e67c6b;
  font-weight: bold;
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
  box-shadow: 0 2px 16px rgba(0,0,0,0.2);
}
</style>
