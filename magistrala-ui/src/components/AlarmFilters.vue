<template>
  <div class="native-form-grid">
    <label class="field"
      >{{ tr('状态', 'Status')
      }}<select v-model="filters.status">
        <option value="all">{{ tr('全部', 'All') }}</option>
        <option value="active">{{ tr('活动', 'Active') }}</option>
        <option value="cleared">{{ tr('已清除', 'Cleared') }}</option>
      </select></label
    >
    <label class="field"
      >{{ tr('严重程度', 'Severity')
      }}<input v-model="filters.severity" type="number" min="0" max="100" step="1"
    /></label>
    <label class="field">{{ tr('子主题', 'Subtopic') }}<input v-model="filters.subtopic" /></label>
    <label v-for="[field, , zh, en] in alarmReferences" :key="field" class="field"
      >{{ tr(zh, en) }}
      <select :value="filters[field]?.id || ''" @change="select(field, $event.target.value)">
        <option value="">{{ tr('全部', 'All') }}</option>
        <option v-for="r in options(field)" :key="r.id" :value="r.id">{{ r.name || r.id }}</option>
      </select>
    </label>
    <label class="field"
      >{{ tr('创建起始时间', 'Created from')
      }}<input
        type="datetime-local"
        :value="localDate(filters.createdFrom)"
        @input="filters.createdFrom = $event.target.value"
    /></label>
    <label class="field"
      >{{ tr('创建结束时间', 'Created to')
      }}<input
        type="datetime-local"
        :value="localDate(filters.createdTo)"
        @input="filters.createdTo = $event.target.value"
    /></label>
    <template v-if="sorting">
      <label class="field"
        >{{ tr('排序字段', 'Order by')
        }}<select v-model="filters.order">
          <option value="updated_at">{{ tr('更新时间', 'Updated') }}</option>
          <option value="created_at">{{ tr('创建时间', 'Created') }}</option>
        </select></label
      >
      <label class="field"
        >{{ tr('排序方向', 'Direction')
        }}<select v-model="filters.dir">
          <option value="desc">{{ tr('降序', 'Descending') }}</option>
          <option value="asc">{{ tr('升序', 'Ascending') }}</option>
        </select></label
      >
    </template>
  </div>
  <p v-if="error" class="error" role="alert">
    {{ error }} <button type="button" class="quiet" @click="load">{{ tr('重试', 'Retry') }}</button>
  </p>
</template>
<script setup>
import { ref, watch } from 'vue'
import { tr } from '../i18n'
import { request, listAll, errorText } from '../api/platform'
import { alarmReferences } from '../api/alarmFilters'
const props = defineProps({
  filters: Object,
  workspace: String,
  channels: Array,
  devices: Array,
  sorting: Boolean,
})
const resources = ref({}),
  error = ref('')
let generation = 0
function options(field) {
  const rows =
    field === 'channel'
      ? props.channels || resources.value.channels || []
      : field === 'client'
        ? props.devices || resources.value.devices || []
        : field === 'rule'
          ? resources.value.rules || []
          : resources.value.members || []
  const selected = props.filters[field]
  return selected?.id && !rows.some((r) => r.id === selected.id) ? [selected, ...rows] : rows
}
function select(field, id) {
  const row = options(field).find((r) => r.id === id)
  props.filters[field] = { id, name: row?.name || '' }
}
function localDate(value) {
  if (!value) return ''
  const date = new Date(value)
  return Number.isFinite(date.getTime())
    ? new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
    : ''
}
async function rules(workspace) {
  const rows = []
  for (let offset = 0; ; offset += 100) {
    const data = await request(`/api/platform/rules/${workspace}/rules?limit=100&offset=${offset}`)
    const part = data.rules || []
    rows.push(...part)
    if (!part.length || rows.length >= (data.total ?? rows.length)) return rows
  }
}
async function load() {
  const seq = ++generation,
    workspace = props.workspace
  resources.value = {}
  error.value = ''
  if (!workspace) return
  const jobs = {
    rules: () => rules(workspace),
    members: () => listAll('tenantMembers', { tenantId: workspace }),
    channels: () =>
      props.channels || listAll('resources', { tenantId: workspace, kind: 'channel' }),
    devices: () => props.devices || listAll('entities', { tenantId: workspace, kind: 'device' }),
  }
  const results = await Promise.allSettled(Object.values(jobs).map((job) => job()))
  if (seq !== generation) return
  results.forEach((result, i) => {
    if (result.status === 'fulfilled') resources.value[Object.keys(jobs)[i]] = result.value
    else error.value ||= errorText(result.reason)
  })
}
watch(() => props.workspace, load, { immediate: true })
</script>
