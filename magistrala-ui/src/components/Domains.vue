<template>
  <main class="workspace-page">
    <header class="page-heading">
      <div>
        <p class="eyebrow">WORKSPACES</p>
        <h1>{{ tr('选择工作区', 'Choose a workspace') }}</h1>
        <p>
          {{
            tr(
              '每个工作区独立管理设备、数据与农田区域。',
              'Each workspace manages its own devices, data and field regions.',
            )
          }}
        </p>
      </div>
      <button @click="creating = true">+ {{ tr('创建工作区', 'Create workspace') }}</button>
    </header>
    <form
      class="toolbar"
      @submit.prevent="() => { offset = 0; load() }"
    >
      <input v-model="search" :placeholder="tr('搜索工作区', 'Search workspaces')" /><button
        class="quiet"
      >
        {{ tr('搜索', 'Search') }}
      </button>
    </form>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="loading">{{ tr('正在加载…', 'Loading…') }}</p>
    <div class="workspace-grid">
      <router-link
        v-for="w in records"
        :key="w.id"
        :to="`/domain/${w.id}/home`"
        class="workspace-card"
        ><span class="workspace-icon">{{ w.name?.[0] || 'W' }}</span>
        <h2>{{ w.name }}</h2>
        <span class="badge">{{ label(w.status) }}</span>
        <p>{{ w.alias || w.id }}</p>
        <strong>{{ tr('进入工作区 →', 'Open workspace →') }}</strong></router-link
      >
    </div>
    <p v-if="!loading && !records.length">
      {{ tr('没有匹配的工作区。', 'No matching workspaces.') }}
    </p>
    <TablePagination :page="Math.floor(offset / limit) + 1" :size="limit" :total="total" :busy="loading" @update:page="changePage" @update:size="changeSize" />
    <ModalDialog v-if="creating" @close="creating = false">
      <div class="modal-card">
        <OperationForm
          :op="createOp"
          :initial="{ input: {} }"
          @close="creating = false"
          @done="load"
        />
      </div>
    </ModalDialog>
  </main>
</template>
<script setup>
import TablePagination from './TablePagination.vue'
import ModalDialog from './ModalDialog.vue'
import { ref, onMounted } from 'vue'
import { runOperation, operation, errorText } from '../api/platform'
import OperationForm from './OperationForm.vue'
import { tr, label } from '../i18n'
const records = ref([]),
  total = ref(0),
  offset = ref(0),
  limit = ref(10),
  search = ref(''),
  loading = ref(false),
  error = ref(''),
  creating = ref(false),
  createOp = operation('createTenant')
async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await runOperation('tenants', {
      q: search.value || undefined,
      limit: limit.value,
      offset: offset.value,
    })
    records.value = data.items
    total.value = data.total
  } catch (e) {
    error.value = errorText(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
function changePage(page) { const next = (page - 1) * limit.value; if (next !== offset.value) { offset.value = next; load() } }
function changeSize(size) { limit.value = size; offset.value = 0; load() }
</script>
