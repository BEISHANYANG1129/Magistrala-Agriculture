<template>
  <ModalDialog :busy="busy" @close="emit('close')">
    <form class="modal-card compact-modal" @submit.prevent="save">
      <h2>{{ tr('编辑设备密钥', 'Edit device key') }}</h2>
      <p>{{ record.name }}</p>
      <p>
        {{
          tr(
            '保存后所选旧密钥失效，使用它的设备需要同步更新密钥。其他凭据不受影响。',
            'The selected old key becomes invalid after saving. Update the device using that key. Other credentials are unaffected.',
          )
        }}
      </p>
      <label class="field"
        >{{ tr('替换的密钥', 'Key to replace')
        }}<select v-model="oldId" :disabled="busy || !!state.created" :required="keys.length > 0">
          <option value="" :disabled="keys.length > 0">
            {{
              keys.length
                ? tr('选择凭据', 'Select credential')
                : tr('创建首个密钥', 'Create first key')
            }}
          </option>
          <option v-for="c in keys" :key="c.id" :value="c.id" :disabled="c.managedBy === 'config'">
            {{ c.identifier || c.id }} · {{ c.createdAt }}
          </option>
        </select></label
      >
      <label class="field"
        >{{ tr('新设备密钥', 'New device key') }} *<input
          v-model="key"
          type="password"
          required
          autocomplete="new-password"
          :disabled="busy || !!state.created"
      /></label>
      <p v-if="state.created && !done" class="notice">
        {{
          tr(
            '新密钥已创建，旧密钥尚未撤销。重试只会继续撤销旧密钥。',
            'New key created; old key has not been revoked. Retry only continues revocation.',
          )
        }}
      </p>
      <p v-if="done" class="success">{{ tr('设备密钥已更新。', 'Device key updated.') }}</p>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <div class="actions">
        <button type="button" class="quiet" :disabled="busy" @click="emit('close')">
          {{ tr('关闭', 'Close') }}</button
        ><button v-if="!done" :disabled="busy || state.unknown || !loaded">
          {{ state.created ? tr('重试撤销旧密钥', 'Retry revocation') : tr('保存', 'Save') }}
        </button>
      </div>
    </form>
  </ModalDialog>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { tr } from '../i18n'
import { runOperation, errorText } from '../api/platform'
import { activeSharedKeys, replaceDeviceKey } from '../api/deviceCredentials'
const props = defineProps({ record: Object, workspace: String }),
  emit = defineEmits(['close', 'updated'])
const keys = ref([]),
  oldId = ref(''),
  key = ref(''),
  busy = ref(false),
  error = ref(''),
  done = ref(false),
  loaded = ref(false),
  state = reactive({})
onMounted(async () => {
  try {
    keys.value = activeSharedKeys(await runOperation('credentials', { entityId: props.record.id }))
    if (keys.value.length === 1 && keys.value[0].managedBy !== 'config')
      oldId.value = keys.value[0].id
    loaded.value = true
  } catch (e) {
    error.value = errorText(e)
  }
})
async function save() {
  busy.value = true
  error.value = ''
  try {
    await replaceDeviceKey(state, props.record, props.workspace, key.value, oldId.value)
    key.value = ''
    if (state.created) delete state.created.key
    done.value = true
    emit('updated')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
onBeforeUnmount(() => {
  key.value = ''
  if (state.created) delete state.created.key
})
</script>
