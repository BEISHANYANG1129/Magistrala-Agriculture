<template>
  <section class="camera-task" aria-label="摄像头任务计划">
    <header>
      <div>
        <h3>{{ camera.source.name }}</h3>
        <small>{{
          tr(
            '真实摄像头 · 整条计划确认一次',
            'Physical camera · one confirmation for the whole plan',
          )
        }}</small>
      </div>
      <button type="button" class="stop-camera" :disabled="stopping" @click="stop">
        {{
          stopping
            ? tr('正在停止…', 'Stopping…')
            : tr('停止转动并中止任务', 'Stop motion and cancel task')
        }}
      </button>
    </header>
    <ol class="operations">
      <li v-for="(operation, index) in camera.request.operations" :key="index">
        <span>{{ describe(operation) }}</span>
        <small>{{ receiptText(index) }}</small>
      </li>
    </ol>
    <p v-if="camera.request.analyze">
      {{ tr('截图后自动分析：', 'Analyze captured images: ') }}{{ camera.request.instruction }}
    </p>
    <p class="muted">
      {{
        tr(
          '画面 1 可转动；画面 2、3 为固定视图。转动按持续时间微调，没有角度或位置反馈。截图等待本任务之后的新视频片段。',
          'Only view 1 moves; views 2 and 3 are fixed. Movement uses timed pulses without position feedback. Captures wait for fresh video segments.',
        )
      }}
    </p>
    <template v-if="task.status === 'waiting_camera_confirmation'">
      <p class="muted">
        {{ tr('确认有效期至：', 'Confirmation expires: ') }}{{ date(camera.expires_at) }}
      </p>
      <label class="confirm-camera"
        ><input v-model="confirmed" type="checkbox" :disabled="busy" />{{
          tr(
            '我已核对以上全部步骤，同意系统自动完成这条摄像头任务。',
            'I confirm every step and authorize this complete camera task.',
          )
        }}</label
      >
      <button type="button" :disabled="!confirmed || busy" @click="execute">
        {{
          busy
            ? tr('正在提交…', 'Submitting…')
            : tr('确认整条计划并执行', 'Confirm plan and execute')
        }}
      </button>
    </template>
    <p
      v-if="
        task.status === 'camera_unknown' && !camera.receipts?.length && !camera.captures?.length
      "
      class="muted"
    >
      {{
        tr(
          '尚未发送摄像头操作。可继续本任务重新准备计划，核对后再次确认。',
          'No camera operation was dispatched. Continue this task to prepare and confirm a fresh plan.',
        )
      }}
    </p>
    <p v-else-if="['camera_unknown', 'camera_stopped'].includes(task.status)" class="muted">
      {{
        tr(
          '后续步骤已中止。已发出的转动不会自动重试，可查看回执和已保存截图。',
          'Further steps have stopped. Dispatched movement is never retried automatically. Receipts and saved captures remain available.',
        )
      }}
    </p>
    <p v-if="error" role="alert" class="camera-error">{{ error }}</p>
    <div v-if="camera.captures?.length" class="captures">
      <article v-for="capture in camera.captures" :key="capture.id">
        <strong>{{ tr('画面', 'View') }} {{ capture.view }}</strong>
        <small>{{ tr('接收时间：', 'Received: ') }}{{ date(capture.received_at) }}</small>
        <a v-if="images[capture.id]" :href="images[capture.id]" target="_blank" rel="noopener"
          ><img
            :src="images[capture.id]"
            :alt="tr('本任务保存的摄像头截图', 'Camera capture saved in this task')"
        /></a>
        <button
          v-else
          type="button"
          :disabled="loading[capture.id]"
          @click="loadCapture(capture.id)"
        >
          {{ loading[capture.id] ? tr('正在读取…', 'Loading…') : tr('查看截图', 'View capture') }}
        </button>
        <a
          v-if="images[capture.id]"
          :href="images[capture.id]"
          :download="'camera-view-' + capture.view + '-' + capture.id + '.jpg'"
          >{{ tr('下载截图', 'Download capture') }}</a
        >
      </article>
    </div>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { request, errorText } from '../api/platform'
import { tr } from '../i18n'
const props = defineProps({
  task: { type: Object, required: true },
  workspace: { type: String, required: true },
})
const emit = defineEmits(['refresh'])
const camera = computed(() => props.task.automation.camera)
const confirmed = ref(false),
  busy = ref(false),
  stopping = ref(false),
  error = ref(''),
  images = ref({}),
  loading = ref({})
const controller = new AbortController()
onBeforeUnmount(() => controller.abort())
watch(
  () => camera.value.revision,
  () => {
    confirmed.value = false
    error.value = ''
    images.value = {}
  },
)
const date = (value) => new Date(value).toLocaleString()
const directions = {
  left: () => tr('左', 'left'),
  right: () => tr('右', 'right'),
  up: () => tr('上', 'up'),
  down: () => tr('下', 'down'),
}
function describe(op) {
  if (op.action === 'move')
    return (
      tr('画面 1 向', 'Move view 1 ') +
      (directions[op.direction]?.() || op.direction) +
      ' · ' +
      op.duration_ms +
      ' ms · ' +
      tr('到时自动停止', 'automatic stop')
    )
  if (op.action === 'snapshot')
    return tr('等待新画面并截图 · 画面 ', 'Wait for fresh video and capture · view ') + op.view
  if (op.action === 'wait') return tr('等待 ', 'Wait ') + op.wait_seconds + ' s'
  return tr('发送停止指令', 'Send stop command')
}
function receiptText(index) {
  const row = camera.value.receipts?.find((r) => r.index === index)
  return !row
    ? tr('待执行', 'Pending')
    : {
        dispatching: tr('已记录，等待回执', 'Recorded; awaiting receipt'),
        completed: tr('已返回回执', 'Receipt received'),
        unknown: tr('未确认结果', 'Outcome unconfirmed'),
      }[row.status] || row.status
}
function api(tail, body) {
  return request(
    '/api/agent/v2/workbench/automation/' + encodeURIComponent(props.task.id) + '/camera/' + tail,
    {
      method: body === undefined ? 'GET' : 'POST',
      body,
      headers: { 'X-Workspace-ID': props.workspace },
      signal: controller.signal,
      timeout: 20000,
    },
  )
}
async function execute() {
  busy.value = true
  error.value = ''
  try {
    await api('confirm', { revision: camera.value.revision, confirmed: confirmed.value })
    confirmed.value = false
    emit('refresh')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function stop() {
  stopping.value = true
  error.value = ''
  try {
    await api('stop', {})
    emit('refresh')
  } catch (e) {
    error.value = errorText(e)
    emit('refresh')
  } finally {
    stopping.value = false
  }
}
async function loadCapture(id) {
  loading.value[id] = true
  error.value = ''
  try {
    const result = await api('captures/' + encodeURIComponent(id))
    if (!['image/jpeg', 'image/png'].includes(result.mime) || typeof result.image !== 'string')
      throw new Error('Invalid capture')
    images.value[id] = 'data:' + result.mime + ';base64,' + result.image
  } catch (e) {
    error.value = errorText(e)
  } finally {
    loading.value[id] = false
  }
}
</script>

<style scoped>
.camera-task {
  border: 1px solid var(--border, #dce6ef);
  border-radius: 12px;
  padding: 18px;
  margin: 16px 0;
  background: var(--surface, #fff);
}
header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}
h3 {
  margin: 0 0 6px;
}
small {
  display: block;
  color: var(--muted, #65758a);
}
.operations {
  padding-left: 24px;
}
.operations li {
  padding: 8px 0;
}
.operations li small {
  margin-top: 4px;
}
.muted {
  font-size: 13px;
  color: var(--muted, #65758a);
  line-height: 1.7;
}
.confirm-camera {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 14px 0;
  line-height: 1.6;
}
.confirm-camera input {
  margin-top: 6px;
}
button {
  border: 1px solid #ccd8e5;
  border-radius: 8px;
  padding: 9px 13px;
  background: var(--surface, #fff);
  cursor: pointer;
  color: var(--text, #24364b);
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.stop-camera {
  color: #a93131;
  border-color: #e6b6b6;
  white-space: nowrap;
}
.camera-error {
  color: #a93131;
}
.captures {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 14px;
  margin-top: 18px;
}
.captures article {
  display: flex;
  flex-direction: column;
  gap: 9px;
  min-width: 0;
}
.captures img {
  width: 100%;
  border-radius: 8px;
  display: block;
}
.captures a {
  color: #14768a;
  font-size: 13px;
}
@media (max-width: 650px) {
  header {
    flex-direction: column;
  }
  .captures {
    grid-template-columns: 1fr;
  }
}
</style>
