<template>
  <div class="channel-detail-card">
    <p v-if="error" role="alert">{{ error }}</p>
    <table class="detail-table">
      <tbody>
        <tr>
          <td>Name</td>
          <td>{{ channel.name }}</td>
        </tr>
        <tr>
          <td>ID</td>
          <td>
            {{ channel.id }}
            <button @click="copyId">复制</button>
          </td>
        </tr>
        <tr>
          <td>Route</td>
          <td>{{ channel.route }}</td>
        </tr>
        <tr>
          <td>Tags</td>
          <td>{{ channel.tags.join(', ') }}</td>
        </tr>
        <tr>
          <td>Status</td>
          <td>{{ channel.status }}</td>
        </tr>
      </tbody>
    </table>
    <button class="delete-btn" disabled title="请在核心管理界面删除">Delete Channel</button>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { graphql } from '../api/atom'

const route = useRoute()
const channelId = route.params.channelId
const error = ref('')

const channel = ref({
  name: '',
  id: '',
  route: '',
  tags: [],
  status: '',
})

function copyId() {
  navigator.clipboard.writeText(channel.value.id)
  alert('Channel ID copied!')
}

onMounted(async () => {
  try {
    const result = await graphql('query($id: ID!) { resource(id: $id) { id name alias attributes } }', { id: String(channelId) })
    const data = result.resource || {}
    channel.value = {
      name: data.name || '',
      id: data.id || '',
      route: data.alias || '',
      tags: data.attributes?.tags || [],
      status: '—',
    }
  } catch (caught) { error.value = caught instanceof Error ? caught.message : '频道加载失败' }
})
</script>

<style scoped>
.channel-detail-card {
  /* width: 100%; */
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 0 24px #eee;
  padding: 32px 32px 24px 32px;
  align-self: flex-start;
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
</style>
