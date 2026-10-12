<template>
  <section class="panel">
    <h2>{{ tr('设备类型', 'Device Type') }}</h2>
    <div class="setting-row">
      <strong>{{ tr('设备类型', 'Device Type') }}</strong
      ><span>{{
        profiles.find((p) => p.id === record.profileId)?.displayName ||
        record.profileId ||
        tr('未绑定', 'Not bound')
      }}</span
      ><button class="quiet" @click="open">{{ tr('绑定设备类型', 'Bind device type') }}</button>
    </div>
    <div class="setting-row">
      <strong>{{ tr('版本', 'Version') }}</strong
      ><span>{{ boundVersion?.version || record.profileVersionId || '—' }}</span>
    </div>
    <p>
      {{
        tr(
          '设备类型用于校验设备属性。绑定变更从下一次设备属性写入开始生效，不会改变传感器消息格式。',
          'Device types validate device attributes. Binding takes effect on the next attribute write; it does not change sensor message formats.',
        )
      }}
    </p>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <ModalDialog v-if="opened" :busy="busy" @close="opened = false">
      <form class="modal-card compact-modal" @submit.prevent="save">
        <h2>{{ tr('绑定设备类型', 'Bind device type') }}</h2>
        <label class="field"
          >{{ tr('设备类型', 'Device Type')
          }}<select v-model="profileId" @change="loadVersions">
            <option value="">{{ tr('解除绑定', 'Unbind') }}</option>
            <option v-for="p in profiles" :value="p.id" :key="p.id">
              {{ p.displayName || p.name }}
            </option>
          </select></label
        >
        <label v-if="profileId" class="field"
          >{{ tr('版本', 'Version')
          }}<select v-model="versionId" required :disabled="loading">
            <option value="">{{ tr('请选择版本', 'Select a version') }}</option>
            <option v-for="v in versions" :value="v.id" :key="v.id">
              {{ v.version }} · {{ v.status }}
            </option>
          </select></label
        >
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="opened = false">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="busy || loading || (!!profileId && !versionId)">
            {{ tr('保存', 'Save') }}
          </button>
        </div>
      </form>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, watch, computed } from 'vue'
import { tr } from '../i18n'
import { runOperation, errorText } from '../api/platform'
import { readResource } from '../api/resourceWorkflow'
const props = defineProps({ record: Object, workspace: String, profiles: Array }),
  emit = defineEmits(['updated'])
const opened = ref(false),
  profileId = ref(''),
  versionId = ref(''),
  versions = ref([]),
  error = ref(''),
  busy = ref(false),
  loading = ref(false)
const boundVersion = computed(() =>
  versions.value.find((v) => v.id === props.record.profileVersionId),
)
let generation = 0
async function loadVersions() {
  const seq = ++generation
  versions.value = []
  versionId.value = ''
  error.value = ''
  if (!profileId.value) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const r = await runOperation('profileVersions', { profileId: profileId.value })
    if (seq !== generation) return
    versions.value = r || []
    versionId.value =
      versions.value.find((v) => v.id === props.record.profileVersionId)?.id ||
      versions.value.filter((v) => v.status === 'active').sort((a, b) => b.version - a.version)[0]
        ?.id ||
      ''
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) loading.value = false
  }
}
function open() {
  profileId.value = props.record.profileId || ''
  opened.value = true
  loadVersions()
}
watch(
  () => props.record.profileId,
  () => {
    profileId.value = props.record.profileId || ''
    loadVersions()
  },
  { immediate: true },
)
async function save() {
  busy.value = true
  error.value = ''
  try {
    await readResource('devices', props.record.id, props.workspace)
    await runOperation('updateEntity', {
      id: props.record.id,
      input: {
        profileId: profileId.value || null,
        profileVersionId: profileId.value ? versionId.value : null,
      },
    })
    opened.value = false
    emit('updated')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
