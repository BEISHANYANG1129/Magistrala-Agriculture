<template>
  <ModalDialog :busy="busy" @close="$emit('close')">
    <form class="modal-card" @submit.prevent="upload">
      <h2>{{ tr('上传引导模板', 'Upload bootstrap profile') }}</h2>
      <p>
        {{
          tr(
            '选择 JSON、YAML 或 TOML 模板文件，最大 1 MB。预览后点击上传才会创建模板。',
            'Choose a JSON, YAML or TOML profile up to 1 MB. Upload creates the profile after preview.',
          )
        }}
      </p>
      <label class="field"
        >{{ tr('模板文件', 'Profile file') }}
        <input
          type="file"
          accept=".json,.yaml,.yml,.toml"
          :disabled="busy || uncertain"
          @change="selectFile"
        />
      </label>
      <p v-if="filename">{{ filename }}</p>
      <pre v-if="content" class="upload-preview">{{ content }}</pre>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="uncertain">
        {{
          tr(
            '提交结果不明确，请关闭弹窗并刷新列表核对，避免重复创建。',
            'The outcome is unknown. Close and refresh the list before another upload to avoid duplicates.',
          )
        }}
      </p>
      <div class="actions">
        <button type="button" class="quiet" :disabled="busy" @click="$emit('close')">
          {{ tr('关闭', 'Close') }}
        </button>
        <button :disabled="busy || !content || uncertain">{{ tr('上传', 'Upload') }}</button>
      </div>
    </form>
  </ModalDialog>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref } from 'vue'
import { tr } from '../i18n'
import { request, errorText } from '../api/platform'
const props = defineProps({ workspace: String })
const emit = defineEmits(['close', 'uploaded'])
const content = ref(''),
  filename = ref(''),
  contentType = ref(''),
  error = ref(''),
  busy = ref(false),
  uncertain = ref(false)
let selection = 0
async function selectFile(event) {
  const seq = ++selection
  content.value = ''
  filename.value = ''
  error.value = ''
  const file = event.target.files?.[0]
  if (!file) return
  try {
    if (file.size > 1024 * 1024)
      throw new Error(tr('文件不能超过 1 MB。', 'File must be under 1 MB.'))
    const ext = file.name.split('.').pop().toLowerCase()
    const mime = {
      json: 'application/json',
      yaml: 'application/yaml',
      yml: 'application/yaml',
      toml: 'application/toml',
    }[ext]
    if (!mime) throw new Error(tr('请选择 JSON、YAML 或 TOML 文件。', 'Choose JSON, YAML or TOML.'))
    const text = (await file.text()).replace(/^\uFEFF/, '')
    if (seq !== selection) return
    if (!text.trim()) throw new Error(tr('模板文件不能为空。', 'Profile file is empty.'))
    if (ext === 'json') {
      const value = JSON.parse(text)
      if (!value || typeof value !== 'object' || Array.isArray(value) || !value.name?.trim())
        throw new Error(
          tr('模板必须是包含名称的 JSON 对象。', 'Profile must be a JSON object with a name.'),
        )
    }
    filename.value = file.name
    contentType.value = mime
    content.value = text
  } catch (e) {
    if (seq === selection) error.value = errorText(e)
  }
}
async function upload() {
  if (busy.value || !content.value || uncertain.value) return
  busy.value = true
  error.value = ''
  try {
    const result = await request(
      `/api/platform/bootstrap/${props.workspace}/devices/bootstrap/profiles/upload`,
      {
        method: 'POST',
        headers: { 'Content-Type': contentType.value },
        body: content.value,
      },
    )
    emit('uploaded', result)
  } catch (e) {
    error.value = errorText(e)
    uncertain.value = !e.status || e.status >= 500
  } finally {
    busy.value = false
  }
}
</script>
<style scoped>
.upload-preview {
  max-height: 320px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
