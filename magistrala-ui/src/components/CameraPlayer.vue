<template>
  <section class="camera-player" :class="{ compact }" :aria-busy="state === 'starting'">
    <header class="camera-toolbar">
      <div><h2>{{ title || tr('实时视频', 'Live video') }}</h2><p>4G · {{ tr('持续播放', 'Continuous preview') }}</p></div>
      <div class="camera-actions">
        <span class="camera-state" :class="state" role="status">{{ stateLabel }}</span>
        <button v-if="enabled === false" :disabled="preview || controlling" @click="control(true)">{{ tr('开启播放', 'Start playback') }}</button>
        <button v-else class="quiet" :disabled="preview || controlling || enabled === null" @click="control(false)">{{ tr('停止播放', 'Stop playback') }}</button>
      </div>
    </header>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div class="camera-grid" :class="{ single: visibleChannels.length === 1 }">
      <figure v-for="channel in visibleChannels" :key="channel.id" class="camera-view">
        <video :ref="(el) => videos[channel.id] = el" muted autoplay playsinline :aria-label="channel.label" :style="{ transform: `scale(${zooms[channel.id] || 1})` }" />
        <div v-if="state !== 'ready'" class="camera-placeholder">{{ stateLabel }}</div>
        <figcaption>{{ channel.label }}</figcaption>
        <div v-if="showControls" class="view-tools">
          <select :aria-label="`${channel.label}数字放大`" v-model.number="zooms[channel.id]">
            <option :value="1">1×</option><option :value="1.5">1.5×</option><option :value="2">2×</option>
          </select>
          <button :aria-label="`${channel.label}截图`" :disabled="state !== 'ready'" @click="snapshot(channel)">{{ tr('截图', 'Snapshot') }}</button>
        </div>
      </figure>
    </div>
    <section v-if="showControls" class="ptz-controls" :aria-label="tr('云台控制', 'PTZ control')">
      <div class="ptz-pad">
        <button class="ptz-up" :disabled="preview || state !== 'ready' || moving" @click="movePtz('up')" aria-label="云台向上">↑</button>
        <button class="ptz-left" :disabled="preview || state !== 'ready' || moving" @click="movePtz('left')" aria-label="云台向左">←</button>
        <button class="ptz-stop" :disabled="preview || enabled !== true" @click="stopPtz" aria-label="停止云台">{{ tr('停止', 'Stop') }}</button>
        <button class="ptz-right" :disabled="preview || state !== 'ready' || moving" @click="movePtz('right')" aria-label="云台向右">→</button>
        <button class="ptz-down" :disabled="preview || state !== 'ready' || moving" @click="movePtz('down')" aria-label="云台向下">↓</button>
      </div>
      <div class="ptz-description">
        <strong>{{ tr('云台方向', 'Camera direction') }}</strong>
        <label>{{ tr('单次移动', 'Move duration') }} <select v-model.number="moveDuration" aria-label="云台单次移动时长"><option :value="200">0.2 秒</option><option :value="400">0.4 秒</option><option :value="800">0.8 秒</option></select></label>
        <p>{{ tr('点击方向短时移动，到时自动发送停止指令。数字放大不改变镜头方向。', 'Click for a short movement followed by a stop command. Digital zoom does not move the lens.') }}</p>
        <output role="status">{{ motionNotice }}</output>
      </div>
    </section>
    <p class="camera-note">{{ preview
      ? tr('草稿预览不会连接摄像头。', 'Draft preview does not connect to the camera.')
      : tr('摄像头开启后持续取流，离开页面也会保持；手动停止后，需再次开启才恢复。此开关对该摄像头的所有观看页面生效。', 'Streaming continues after leaving this page. A manual stop is remembered until playback is enabled again. This switch applies to all viewers of this camera.') }}</p>
  </section>
</template>
<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { tr } from '../i18n'
import { errorText } from '../api/platform'
import { ensureSession } from '../api/session'
import { cameraBase, cameraMediaUrl, cameraRequest, cameraPtz } from '../api/camera'

const props = defineProps({ workspace: String, deviceId: String, title: String, compact: Boolean, preview: Boolean, showControls: Boolean, view: { type: String, default: 'all' } })
const state = ref('idle'), sessionId = ref(''), error = ref('')
const enabled = ref(null), controlling = ref(false)
const moving = ref(false), moveDuration = ref(400), motionNotice = ref('')
const zooms = ref({0:1, 1:1, 2:1})
const channels = ref([0, 1, 2].map(id => ({ id, label: tr(`画面 ${id + 1}`, `View ${id + 1}`) })))
const visibleChannels = computed(() => channels.value.filter((c) => props.view === 'all' || String(c.id) === props.view))
const stateLabel = computed(() => ({ idle: tr('正在读取播放状态…', 'Loading playback status…'), stopped: tr('已停止', 'Stopped'), starting: tr('正在连接摄像头…', 'Connecting…'), ready: tr('实时预览中', 'Live'), reconnecting: tr('正在重连…', 'Reconnecting…'), error: tr('暂时无法播放', 'Unavailable') })[state.value])
const videos = {}, players = []
let timer, generation = '', epoch = 0, polling = false, authToken = '', disposed = false

function releasePlayers() {
  for (const player of players.splice(0)) player.destroy()
  for (const video of Object.values(videos)) {
    if (video) { video.pause(); video.removeAttribute('src'); video.load() }
  }
}
const motionId = () => crypto.randomUUID().replaceAll('-', '')
async function movePtz(direction) {
  if (!props.showControls || props.preview || moving.value || state.value !== 'ready') return
  moving.value = true; motionNotice.value = tr('正在发送云台指令…', 'Sending movement…')
  try {
    await cameraPtz(props.workspace, props.deviceId, 'move', { requestId: motionId(), direction, durationMs: moveDuration.value })
    motionNotice.value = tr('移动及停止指令已发送，请以画面变化为准。', 'Movement and stop commands sent. Check the live image.')
  } catch(e) { motionNotice.value = e.status === 403 ? tr('没有这台设备的云台控制权限。', 'No permission to control this camera.') : errorText(e) }
  finally { moving.value = false }
}
async function stopPtz() {
  if (!props.showControls || props.preview) return
  try { await cameraPtz(props.workspace, props.deviceId, 'stop', {requestId:motionId()}); motionNotice.value = tr('停止指令已发送。', 'Stop command sent.') }
  catch(e) { motionNotice.value = errorText(e) }
}
function snapshot(channel) {
  const video = videos[channel.id]
  if (!video || video.readyState < 2) return
  const canvas = document.createElement('canvas')
  canvas.width = video.videoWidth; canvas.height = video.videoHeight
  const zoom = zooms.value[channel.id] || 1, width = canvas.width / zoom, height = canvas.height / zoom
  canvas.getContext('2d').drawImage(video, (canvas.width-width)/2, (canvas.height-height)/2, width, height, 0, 0, canvas.width, canvas.height)
  const link = document.createElement('a')
  link.href = canvas.toDataURL('image/png'); link.download = `camera-view-${channel.id+1}-${new Date().toISOString().replaceAll(':','-')}.png`; link.click()
}
async function apply(info, current) {
  if (current !== epoch) return
  enabled.value = info.enabled
  state.value = info.enabled === false ? 'stopped' : info.state
  if (info.channels?.length) channels.value = info.channels
  if (info.enabled === false) { generation = ''; releasePlayers(); return }
  if (info.state !== 'ready' || generation === info.generation) return
  generation = info.generation
  releasePlayers()
  await nextTick()
  const { default: Hls } = await import('hls.js')
  if (current !== epoch) return
  if (!Hls.isSupported()) throw new Error(tr('此浏览器暂不支持实时预览，请使用新版 Edge 或 Chrome。', 'Use a recent Edge or Chrome browser for live preview.'))
  for (const channel of visibleChannels.value) {
    const video = videos[channel.id]
    if (!video) continue
    const url = cameraMediaUrl(props.workspace, props.deviceId, channel.playlist)
    const prefix = cameraBase(props.workspace, props.deviceId)
    const player = new Hls({ enableWorker: false, liveSyncDurationCount: 2, liveMaxLatencyDurationCount: 4,
      maxBufferLength: 8, backBufferLength: 0,
      xhrSetup(xhr, resource) {
        const target = new URL(resource, location.href)
        if (target.origin !== location.origin || !target.pathname.startsWith(prefix)) throw new Error('Invalid camera media origin')
        if (xhr.readyState === 0) xhr.open('GET', target.href, true)
        xhr.setRequestHeader('Authorization', `Bearer ${authToken}`)
      },
    })
    player.on(Hls.Events.ERROR, (_event, data) => {
      if (data.fatal && current === epoch) { state.value = 'reconnecting'; generation = '' }
    })
    player.on(Hls.Events.MANIFEST_PARSED, () => video.play().catch(() => {}))
    player.attachMedia(video)
    player.loadSource(url)
    players.push(player)
  }
}
async function sync() {
  if (polling || controlling.value || disposed || props.preview || !props.workspace || !props.deviceId) return
  const current = epoch
  polling = true
  try {
    authToken = (await ensureSession())?.token || ''
    if (current !== epoch || disposed) return
    const sid = sessionId.value
    const info = await cameraRequest(props.workspace, props.deviceId, sid ? `sessions/${sid}/heartbeat` : 'sessions', 'POST')
    if (current !== epoch || disposed) {
      if (!sid && info.sessionId) await cameraRequest(props.workspace, props.deviceId, `sessions/${info.sessionId}`, 'DELETE').catch(() => {})
      return
    }
    sessionId.value = info.sessionId
    await apply(info, current)
    if (current === epoch) error.value = ''
  } catch (e) {
    if (current === epoch) {
      if (e.status === 404 || e.status === 410) sessionId.value = ''
      error.value = errorText(e); state.value = 'reconnecting'; generation = ''
    }
  } finally { polling = false }
}
async function control(shouldEnable) {
  if (props.preview || controlling.value || disposed) return
  controlling.value = true
  const current = ++epoch
  try {
    const info = await cameraRequest(props.workspace, props.deviceId, `playback/${shouldEnable ? 'start' : 'stop'}`, 'POST')
    await apply(info, current)
    if (current === epoch) error.value = ''
  } catch (e) {
    if (current === epoch) error.value = errorText(e)
  } finally { controlling.value = false }
  if (current === epoch) await sync()
}
async function detach() {
  epoch++; releasePlayers(); generation = ''
  const sid = sessionId.value
  sessionId.value = ''; state.value = 'idle'
  if (sid) await cameraRequest(props.workspace, props.deviceId, `sessions/${sid}`, 'DELETE').catch(() => {})
}
function unload() {
  if (!sessionId.value || !authToken) return
  fetch(cameraBase(props.workspace, props.deviceId) + `sessions/${sessionId.value}`, {
    method: 'DELETE', keepalive: true, headers: { Authorization: `Bearer ${authToken}` },
  }).catch(() => {})
}
window.addEventListener('pagehide', unload)
watch(() => [props.workspace, props.deviceId, props.preview], async () => { await detach(); enabled.value = null; await sync() })
watch(() => props.view, () => { generation = '' })
onMounted(() => { timer = setInterval(sync, 5000); sync() })
onBeforeUnmount(() => { disposed = true; clearInterval(timer); window.removeEventListener('pagehide', unload); detach() })
</script>
<style scoped>
.camera-player { padding: 20px; border: 1px solid var(--border, #dfe7e4); border-radius: 14px; background: var(--surface, #fff); }
.camera-toolbar, .camera-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.camera-toolbar h2 { margin: 0; font-size: 18px; }
.camera-toolbar p, .camera-note { color: #647570; font-size: 13px; line-height: 1.6; }
.camera-toolbar p { margin: 5px 0 0; }
.camera-state { font-size: 13px; color: #647570; }
.camera-state.ready { color: #168168; }
.camera-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-top: 18px; }
.camera-grid.single { grid-template-columns: minmax(0, 1fr); }
.camera-view { position: relative; margin: 0; overflow: hidden; border-radius: 10px; background: #14211f; aspect-ratio: 16/9; }
.camera-view video { display: block; width: 100%; height: 100%; object-fit: contain; }
.camera-view figcaption { position: absolute; top: 10px; left: 12px; color: white; background: #14211fa8; border-radius: 5px; padding: 3px 8px; font-size: 12px; pointer-events: none; }
.camera-placeholder { position: absolute; inset: 0; display: grid; place-items: center; color: #d3dfda; background: #14211f; }
.camera-note { margin-bottom: 0; }
.view-tools { position: absolute; right: 8px; bottom: 8px; display: flex; gap: 6px; }
.view-tools button, .view-tools select { padding: 4px 8px; font-size: 12px; color: #243c33; background: #fffffff0; border-radius: 6px; }
.ptz-controls { display: flex; flex-wrap: wrap; gap: 18px; align-items: center; margin-top: 16px; padding-top: 14px; border-top: 1px solid #e2ebe7; }
.ptz-pad { display: grid; grid-template: repeat(3, 36px) / repeat(3, 48px); gap: 4px; }
.ptz-pad button { padding: 0; min-height: 0; }
.ptz-up { grid-area: 1/2; }.ptz-left { grid-area: 2/1; }.ptz-stop { grid-area: 2/2; }.ptz-right { grid-area: 2/3; }.ptz-down { grid-area: 3/2; }
.ptz-description { flex: 1; min-width: 220px; font-size: 13px; color: #526961; }
.ptz-description label { margin-left: 14px; }.ptz-description select { padding: 4px; }.ptz-description p { margin: 8px 0; }
.compact { padding: 8px; border: 0; }
.compact .camera-toolbar h2 { font-size: 15px; }
.compact .camera-grid { gap: 8px; margin-top: 12px; }
@media (max-width: 760px) { .camera-grid { grid-template-columns: minmax(0, 1fr); } }
</style>
