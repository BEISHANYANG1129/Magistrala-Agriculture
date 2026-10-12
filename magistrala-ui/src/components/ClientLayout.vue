<!-- filepath: /home/liar/Magistrala/magistrala-ui/src/components/ClientLayout.vue -->
<template>
  <div class="client-layout">
    <div class="client-menu-area">
      <ClientMenu
        :name="client.name"
        :id="client.id"
        :menu="menuList"
        :active="activeMenu"
        @menu-click="handleMenuClick"
      />
    </div>
    <div class="client-content-area">
      <router-view />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ClientMenu from './ClientMenu.vue'
import { graphql } from '../api/atom'

const route = useRoute()
const router = useRouter()
const domainId = route.params.id
const clientId = route.params.clientId

const client = ref({
  name: '',
  id: '',
})

const menuList = [
  { key: 'configurations', label: 'Configurations' },
  { key: 'connections', label: 'Connections' },
  { key: 'roles', label: 'Roles' },
  { key: 'members', label: 'Members' },
  { key: 'audit', label: 'Audit Logs' }
]

const activeMenu = ref('configurations')

watch(
  () => route.path,
  (newPath) => {
    const matched = menuList.find(item =>
      newPath.endsWith(item.key) || (item.key === 'configurations' && newPath === `/domain/${domainId}/devices/${clientId}`)
    )
    activeMenu.value = matched ? matched.key : 'configurations'
  },
  { immediate: true }
)

function handleMenuClick(key) {
  activeMenu.value = key
  if (key === 'configurations') {
    router.push(`/domain/${domainId}/devices/${clientId}`)
  } else {
    router.push(`/domain/${domainId}/devices/${clientId}/${key}`)
  }
}

onMounted(async () => {
  try {
    const data = await graphql('query($id: ID!) { entity(id: $id) { id name } }', { id: String(clientId) })
    client.value = data.entity || { name: '', id: String(clientId) }
  } catch { client.value.id = String(clientId) }
})
</script>

<style scoped>
.client-layout {
  display: flex;
  width: 100%;
  min-height: 100%;
}
.client-menu-area {
  flex: 0 0 20%;
  max-width: 20%;
  min-width: 200px;
}
.client-content-area {
  flex: 1;
  background: #fff;
  padding: 32px 32px 24px 32px;
  border-radius: 18px;
  box-shadow: 0 0 24px #eee;
  min-width: 0;
}
</style>
