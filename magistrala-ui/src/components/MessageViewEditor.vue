<template>
  <ModalDialog :busy="busy" @close="emit('close')">
    <form class="modal-card" @submit.prevent="save">
      <h2>
        {{
          removing
            ? tr('删除消息视图', 'Delete message view')
            : record.id
              ? tr('编辑视图', 'Edit view')
              : tr('新建视图', 'New view')
        }}
      </h2>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="removing">{{ record.name }}</p>
      <template v-else>
        <label class="field"
          >{{ tr('视图名称', 'View name') }} *<input v-model="name" required
        /></label>
        <div class="native-form-grid">
          <label class="field"
            >{{ tr('格式', 'Format')
            }}<select v-model="format">
              <option value="json">JSON</option>
              <option value="senml">SenML</option>
            </select></label
          >
          <label class="field"
            >{{ tr('子主题', 'Subtopic') }}<input v-model="subtopic" placeholder="sensors/soil"
          /></label>
        </div>
        <fieldset>
          <legend>{{ tr('发送设备', 'Publishers') }}</legend>
          <label v-for="d in devices" :key="d.id" class="check"
            ><input v-model="publishers" type="checkbox" :value="d.id" />{{ d.name }}</label
          >
        </fieldset>
        <template v-if="format === 'json'">
          <h3>{{ tr('配置表格', 'Configure table') }}</h3>
          <div class="actions">
            <button type="button" class="quiet" @click="columns.forEach((c) => (c.enabled = true))">
              {{ tr('全选', 'Select all') }}</button
            ><button
              type="button"
              class="quiet"
              @click="columns.forEach((c) => (c.enabled = false))"
            >
              {{ tr('全不选', 'Select none') }}
            </button>
          </div>
          <div v-for="(c, i) in columns" :key="i" class="toolbar">
            <input
              v-model="c.enabled"
              type="checkbox"
              :aria-label="tr('显示此列', 'Include column')"
            />
            <label class="field"
              >{{ tr('JSON 字段', 'JSON key') }}<input v-model="c.key" required
            /></label>
            <label class="field"
              >{{ tr('列名称', 'Column label') }}<input v-model="c.label" required
            /></label>
            <button
              type="button"
              class="quiet"
              :disabled="i === 0"
              :aria-label="tr('上移', 'Move up')"
              @click="move(i, -1)"
            >
              ↑</button
            ><button
              type="button"
              class="quiet"
              :disabled="i === columns.length - 1"
              :aria-label="tr('下移', 'Move down')"
              @click="move(i, 1)"
            >
              ↓</button
            ><button type="button" class="quiet" @click="columns.splice(i, 1)">
              {{ tr('移除', 'Remove') }}
            </button>
          </div>
          <button
            type="button"
            class="quiet"
            @click="columns.push({ key: '', label: '', enabled: true })"
          >
            + {{ tr('添加列', 'Add column') }}
          </button>
        </template>
      </template>
      <div class="actions">
        <button type="button" class="quiet" :disabled="busy" @click="emit('close')">
          {{ tr('取消', 'Cancel') }}</button
        ><button :disabled="busy">
          {{ removing ? tr('确认删除', 'Confirm deletion') : tr('保存', 'Save') }}
        </button>
      </div>
    </form>
  </ModalDialog>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref } from 'vue'
import { tr } from '../i18n'
import { request, errorText } from '../api/platform'
import { viewSettings, viewBody } from '../api/messageViews'
const props = defineProps({
    record: Object,
    workspace: String,
    channel: String,
    devices: Array,
    removing: Boolean,
  }),
  emit = defineEmits(['close', 'saved'])
const settings = viewSettings(props.record),
  name = ref(props.record.name || ''),
  format = ref(settings.format),
  subtopic = ref(settings.subtopic),
  publishers = ref(settings.publishers),
  columns = ref(settings.columns),
  error = ref(''),
  busy = ref(false)
function move(i, d) {
  const [c] = columns.value.splice(i, 1)
  columns.value.splice(i + d, 0, c)
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    const url =
      `/api/platform/ui/${props.workspace}/channels/${props.channel}/message-views` +
      (props.record.id ? '/' + encodeURIComponent(props.record.id) : '')
    const body = props.removing
      ? undefined
      : viewBody(props.record, {
          name: name.value,
          format: format.value,
          subtopic: subtopic.value,
          publishers: publishers.value,
          columns: columns.value,
        })
    await request(url, {
      method: props.removing ? 'DELETE' : props.record.id ? 'PUT' : 'POST',
      body,
    })
    emit('saved')
    emit('close')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
