<template>
  <section>
    <header class="page-heading">
      <div>
        <p class="eyebrow">FIELD MAP</p>
        <h1>{{ tr('农田分区与设备位置', 'Field areas & device locations') }}</h1>
        <p>
          {{
            tr(
              '每个分组一张底图，显示本组设备；分区和设备位置分别随这张图保存。',
              'Each group has its own map, areas and device positions. Only devices in this group are shown.',
            )
          }}
        </p>
      </div>
    </header>
    <div class="toolbar">
      <select
        :value="groupId"
        :disabled="busy"
        :aria-label="tr('选择分组', 'Choose group')"
        @change="selectGroup"
      >
        <option value="">{{ tr('选择分组', 'Choose group') }}</option>
        <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option></select
      ><button class="quiet" :disabled="!groupId || busy" @click="requestLoad(groupId)">
        {{ tr('刷新', 'Refresh') }}</button
      ><button :disabled="!original || busy || points.length > 0" @click="saveMap">
        {{ tr('保存底图与分区', 'Save background & areas') }}
      </button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <p v-if="busy" role="status">{{ tr('处理中…', 'Working…') }}</p>
    <p v-if="!groupId" class="notice">
      {{ tr('请选择分组，查看或设置它的地图。', 'Choose a group to view or configure its map.') }}
    </p>
    <p v-if="dirty" class="notice">
      {{
        tr(
          '底图或分区尚未保存；绘制中的区域请先完成或取消。',
          'Background or areas are unsaved. Finish or cancel the area being drawn.',
        )
      }}
    </p>
    <fieldset v-if="original" class="panel map-editor" :disabled="busy">
      <p>
        {{ tr('本组设备', 'Group devices') }}：{{ devices.length }} ·
        {{ tr('已定位', 'Positioned') }}：{{ positioned.length }}
      </p>
      <p v-if="!devices.length" class="notice">
        {{
          tr(
            '此分组尚未关联设备。可先设置底图和分区；设备关联请在分组的设备页管理。',
            'No devices are associated with this group. You can configure the map; manage associations on the group Devices page.',
          )
        }}
      </p>
      <label class="field"
        >{{ tr('底图地址', 'Background image URL')
        }}<input v-model="background" type="url" placeholder="http://..."
      /></label>
      <div class="toolbar">
        <select v-model="mode" :aria-label="tr('操作模式', 'Map mode')">
          <option value="view">{{ tr('浏览', 'View') }}</option>
          <option value="area">{{ tr('绘制多边形区域', 'Draw polygon area') }}</option>
          <option value="device">{{ tr('定位设备', 'Place device') }}</option></select
        ><input
          v-if="mode === 'area'"
          v-model="areaName"
          :placeholder="tr('区域名称', 'Area name')"
        /><select
          v-if="mode === 'device'"
          v-model="device"
          :aria-label="tr('选择设备', 'Choose device')"
        >
          <option value="">{{ tr('选择设备', 'Choose device') }}</option>
          <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option></select
        ><button v-if="mode === 'area'" class="quiet" :disabled="points.length < 3" @click="finish">
          {{ tr('完成区域', 'Finish area') }}</button
        ><button class="quiet" @click="points = []">{{ tr('取消绘制', 'Cancel drawing') }}</button>
      </div>
      <p v-if="mode === 'device'" class="notice">
        {{
          tr(
            '选中设备后点击图面，位置立即保存到当前分组地图。',
            'Choose a device and click the map to save its position on this group map immediately.',
          )
        }}
      </p>
      <svg
        class="map-canvas"
        viewBox="0 0 1000 650"
        @click="pick"
        role="img"
        :aria-label="tr('农田分区图', 'Field area map')"
      >
        <rect width="1000" height="650" fill="#edf3e9" />
        <image
          v-if="safeBackground"
          :href="safeBackground"
          width="1000"
          height="650"
          preserveAspectRatio="none"
        />
        <g v-for="a in areas" :key="a.id">
          <polygon
            v-if="a.shape === 'polygon'"
            :points="polygon(a.points)"
            :fill="a.color || '#19624f'"
            fill-opacity=".2"
            :stroke="a.color || '#19624f'"
            stroke-width="2"
          />
          <rect
            v-else
            :x="(a.x || 0) * 1000"
            :y="(a.y || 0) * 650"
            :width="(a.width || a.w || 0) * 1000"
            :height="(a.height || a.h || 0) * 650"
            :fill="a.color || '#19624f'"
            fill-opacity=".2"
            stroke="#19624f"
          />
          <text
            :x="(a.points?.[0]?.x ?? a.x ?? 0) * 1000 + 6"
            :y="(a.points?.[0]?.y ?? a.y ?? 0) * 650 + 18"
            font-size="16"
            fill="#163d2c"
          >
            {{ a.name || a.id }}
          </text>
        </g>
        <polyline :points="polygon(points)" fill="none" stroke="#e89432" stroke-width="3" />
        <g v-for="d in positioned" :key="d.id">
          <circle :cx="position(d).x * 1000" :cy="position(d).y * 650" r="7" fill="#1d5973" />
          <text :x="position(d).x * 1000 + 10" :y="position(d).y * 650 - 10" font-size="13">
            {{ d.name }}
          </text>
        </g>
      </svg>
      <div v-for="a in areas" :key="a.id" class="toolbar">
        <input v-model="a.name" :aria-label="tr('分区名称', 'Area name')" /><input
          type="color"
          v-model="a.color"
          style="width: 70px"
        /><button class="quiet" @click="areas = areas.filter((x) => x.id !== a.id)">
          {{ tr('移除分区', 'Remove area') }}
        </button>
      </div>
    </fieldset>
    <ModalDialog v-if="pendingGroup !== null" confirmation @close="pendingGroup = null">
      <section class="modal-card compact-modal">
        <h2>{{ tr('放弃未保存的地图修改？', 'Discard unsaved map changes?') }}</h2>
        <p>
          {{
            tr(
              '底图和分区草稿将被放弃，已保存的设备位置不受影响。',
              'Background and area drafts will be discarded. Saved device positions are kept.',
            )
          }}
        </p>
        <button class="quiet" @click="pendingGroup = null">
          {{ tr('继续编辑', 'Keep editing') }}
        </button>
        <button @click="confirmLoad">{{ tr('放弃并继续', 'Discard and continue') }}</button>
      </section>
    </ModalDialog>
  </section>
</template>
<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useRoute, onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { listAll, errorText } from '../api/platform'
import { groupMap, loadGroupMap, saveGroupMap, validPosition, sameMapValue } from '../api/fieldMap'
import ModalDialog from './ModalDialog.vue'
import { tr } from '../i18n'
const route = useRoute(),
  groupId = ref(''),
  groups = ref([]),
  devices = ref([]),
  background = ref(''),
  areas = ref([]),
  points = ref([]),
  mode = ref('view'),
  device = ref(''),
  areaName = ref(''),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  original = ref(null),
  positions = ref({}),
  pendingGroup = ref(null)
let generation = 0,
  workspaceGeneration = 0
const dirty = computed(
  () =>
    original.value &&
    (points.value.length > 0 ||
      background.value !== groupMap(original.value).background ||
      !sameMapValue(areas.value, groupMap(original.value).partitions)),
)
function leave() {
  if (busy.value) return false
  return (
    !dirty.value ||
    window.confirm(
      tr('底图和分区尚未保存，放弃修改并离开？', 'Discard unsaved map changes and leave?'),
    )
  )
}
onBeforeRouteLeave(leave)
onBeforeRouteUpdate((to, from) => to.params.id === from.params.id || leave())
function beforeUnload(event) {
  if (dirty.value || busy.value) {
    event.preventDefault()
    event.returnValue = ''
  }
}
window.addEventListener('beforeunload', beforeUnload)
onBeforeUnmount(() => {
  generation++
  workspaceGeneration++
  window.removeEventListener('beforeunload', beforeUnload)
})
const safeBackground = computed(() =>
  /^https?:\/\//i.test(background.value) ? background.value : '',
)
function position(d) {
  return positions.value[d.id] || {}
}
const positioned = computed(() => devices.value.filter((d) => validPosition(position(d))))
function polygon(p) {
  return (p || []).map((v) => `${v.x * 1000},${v.y * 650}`).join(' ')
}
function selectGroup(event) {
  const next = event.target.value
  event.target.value = groupId.value
  requestLoad(next)
}
function requestLoad(next) {
  if (busy.value) return
  if (dirty.value) pendingGroup.value = next
  else load(next)
}
function confirmLoad() {
  const next = pendingGroup.value
  pendingGroup.value = null
  load(next)
}
async function load(next) {
  const seq = ++generation
  const workspace = String(route.params.id)
  groupId.value = next
  original.value = null
  devices.value = []
  positions.value = {}
  background.value = ''
  areas.value = []
  points.value = []
  device.value = ''
  mode.value = 'view'
  areaName.value = ''
  notice.value = ''
  error.value = ''
  if (!next) return
  busy.value = true
  try {
    const result = await loadGroupMap(next, workspace)
    if (seq !== generation) return
    original.value = result.record
    devices.value = result.devices
    const map = groupMap(result.record)
    background.value = map.background
    areas.value = map.partitions
    positions.value = map.positions
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
function finish() {
  areas.value.push({
    id: crypto.randomUUID(),
    name: areaName.value || tr('新区域', 'New area'),
    shape: 'polygon',
    points: [...points.value],
    color: '#399063',
  })
  points.value = []
  areaName.value = ''
}
async function pick(event) {
  if (busy.value || !original.value || mode.value === 'view') return
  const svg = event.currentTarget
  const pt = svg.createSVGPoint()
  pt.x = event.clientX
  pt.y = event.clientY
  const p = pt.matrixTransform(svg.getScreenCTM().inverse())
  const point = { x: Math.max(0, Math.min(1, p.x / 1000)), y: Math.max(0, Math.min(1, p.y / 650)) }
  if (mode.value === 'area') {
    points.value.push(point)
    return
  }
  if (mode.value !== 'device' || !device.value || busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  const seq = generation
  try {
    const saved = await saveGroupMap(
      original.value,
      String(route.params.id),
      { position: point },
      device.value,
    )
    if (seq !== generation) return
    original.value = saved
    positions.value = groupMap(saved).positions
    notice.value = tr('设备位置已保存到当前分组地图。', 'Device position saved to this group map.')
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
async function saveMap() {
  if (!original.value || busy.value || points.value.length) return
  busy.value = true
  error.value = ''
  notice.value = ''
  const seq = generation
  try {
    const saved = await saveGroupMap(original.value, String(route.params.id), {
      background: background.value.trim(),
      partitions: areas.value,
    })
    if (seq !== generation) return
    original.value = saved
    background.value = groupMap(saved).background
    notice.value = tr('底图与分区已保存到当前分组。', 'Background and areas saved to this group.')
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
watch(
  () => route.params.id,
  async (id) => {
    const seq = ++workspaceGeneration
    await load('')
    pendingGroup.value = null
    groups.value = []
    busy.value = true
    try {
      const rows = await listAll('objectGroups', { tenantId: String(id), deleted: 'live' })
      if (seq === workspaceGeneration)
        groups.value = rows.filter(
          (g) => g.tenantId === String(id) && g.groupType === 'object' && !g.deletedAt,
        )
    } catch (e) {
      if (seq === workspaceGeneration) error.value = errorText(e)
    } finally {
      if (seq === workspaceGeneration) busy.value = false
    }
  },
  { immediate: true },
)
</script>
<style scoped>
.map-editor {
  min-width: 0;
}
.map-canvas {
  display: block;
  width: 100%;
  max-height: 680px;
  border: 1px solid #d7e5da;
  border-radius: 10px;
  cursor: crosshair;
}
</style>
