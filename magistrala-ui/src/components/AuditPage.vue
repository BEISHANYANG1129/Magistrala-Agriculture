<template>
  <section>
    <h1>{{ tr('平台日志', 'Platform journals') }}</h1>
    <form
      class="toolbar"
      @submit.prevent="
        () => {
          offset = 0
          load()
        }
      "
    >
      <input v-model="event" :placeholder="tr('事件名称', 'Event name')" /><select
        v-model="outcome"
        :aria-label="tr('结果', 'Outcome')"
      >
        <option value="">{{ tr('全部结果', 'All outcomes') }}</option>
        <option value="allow">{{ tr('允许', 'Allow') }}</option>
        <option value="deny">{{ tr('拒绝', 'Deny') }}</option>
        <option value="error">{{ tr('错误', 'Error') }}</option></select
      ><button :disabled="busy">{{ tr('筛选', 'Filter') }}</button>
    </form>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{{ tr('时间', 'Time') }}</th>
            <th>{{ tr('事件', 'Event') }}</th>
            <th>{{ tr('结果', 'Outcome') }}</th>
            <th>{{ tr('操作者', 'Actor') }}</th>
            <th>{{ tr('目标', 'Target') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in records" :key="r.id">
            <td>{{ r.createdAt || r.timestamp }}</td>
            <td>{{ r.event }}</td>
            <td>{{ r.outcome }}</td>
            <td>{{ r.actorEntityId }}</td>
            <td>{{ r.targetKind }} · {{ r.targetId }}</td>
          </tr>
          <tr v-if="!records.length">
            <td colspan="5">{{ tr('暂无日志。', 'No journal entries.') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <TablePagination :page="Math.floor(offset / limit) + 1" :size="limit" :total="total" :busy="busy" @update:page="changePage" @update:size="changeSize" />
  </section>
</template>
<script setup>
import TablePagination from './TablePagination.vue'
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { tr } from '../i18n'
import { runOperation, errorText } from '../api/platform'
const route = useRoute(),
  records = ref([]),
  offset = ref(0),
  limit = ref(10),
  total = ref(0),
  event = ref(''),
  outcome = ref(''),
  error = ref(''),
  busy = ref(false)
let generation = 0
async function load() {
  const seq = ++generation
  busy.value = true
  error.value = ''
  try {
    const d = await runOperation('auditLogs', {
      tenantId: String(route.params.id),
      limit: limit.value,
      offset: offset.value,
      ...(event.value ? { event: event.value } : {}),
      ...(outcome.value ? { outcome: outcome.value } : {}),
    })
    if (seq === generation) {
      records.value = d.items || []
      total.value = d.total || 0
    }
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
watch(
  () => route.params.id,
  () => {
    offset.value = 0
    load()
  },
  { immediate: true },
)
function changePage(page) { const next = (page - 1) * limit.value; if (next !== offset.value) { offset.value = next; load() } }
function changeSize(size) { limit.value = size; offset.value = 0; load() }
</script>
