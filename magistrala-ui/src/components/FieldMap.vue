<template>
  <section>
    <header class="page-heading">
      <div>
        <p class="eyebrow">FIELD MAP</p>
        <h1>{{ tr('农田分区与设备位置', 'Field areas & device locations') }}</h1>
        <p>
          {{
            tr(
              '保留原项目的通道底图、分区和设备位置。绘图区分用于展示；分析归属在区域工作台中配置。',
              'Uses the original channel background, partitions and device positions. Configure analysis ownership separately in the regional workbench.',
            )
          }}
        </p>
      </div>
    </header>
    <div class="toolbar">
      <select v-model="channel" @change="load">
        <option value="">{{ tr('选择通道', 'Choose channel') }}</option>
        <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option></select
      ><button class="quiet" :disabled="!channel || busy" @click="load">
        {{ tr('刷新', 'Refresh') }}</button
      ><button :disabled="!channel || busy" @click="saveMap">
        {{ tr('保存底图与分区', 'Save background & areas') }}
      </button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <div v-if="channel" class="panel">
      <label class="field"
        >{{ tr('底图地址', 'Background image URL')
        }}<input v-model="background" type="url" placeholder="http://..."
      /></label>
      <div class="toolbar">
        <select v-model="mode">
          <option value="view">{{ tr('浏览', 'View') }}</option>
          <option value="area">{{ tr('绘制多边形区域', 'Draw polygon area') }}</option>
          <option value="device">{{ tr('定位设备', 'Place device') }}</option></select
        ><input
          v-if="mode === 'area'"
          v-model="areaName"
          :placeholder="tr('区域名称', 'Area name')"
        /><select v-if="mode === 'device'" v-model="device">
          <option value="">{{ tr('选择设备', 'Choose device') }}</option>
          <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option></select
        ><button v-if="mode === 'area'" class="quiet" :disabled="points.length < 3" @click="finish">
          {{ tr('完成区域', 'Finish area') }}</button
        ><button class="quiet" @click="points = []">{{ tr('取消绘制', 'Cancel drawing') }}</button>
      </div>
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
        <input v-model="a.name" /><input
          type="color"
          v-model="a.color"
          style="width: 70px"
        /><button class="quiet" @click="areas = areas.filter((x) => x.id !== a.id)">
          {{ tr('移除分区', 'Remove area') }}
        </button>
      </div>
    </div>
  </section>
</template>
<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { runOperation, listAll, errorText } from '../api/platform'
import { tr } from '../i18n'
const route = useRoute(),
  channel = ref(''),
  channels = ref([]),
  devices = ref([]),
  background = ref(''),
  areas = ref([]),
  points = ref([]),
  mode = ref('view'),
  device = ref(''),
  areaName = ref(''),
  busy = ref(false),
  error = ref(''),
  notice = ref('')
let original
const safeBackground = computed(() =>
  /^https?:\/\//i.test(background.value) ? background.value : '',
)
function position(d) {
  return (d.attributes?.metadata || d.attributes || {}).position || {}
}
const positioned = computed(() =>
  devices.value.filter((d) => Number.isFinite(position(d).x) && Number.isFinite(position(d).y)),
)
function polygon(p) {
  return (p || []).map((v) => `${v.x * 1000},${v.y * 650}`).join(' ')
}
async function load() {
  if (!channel.value) return
  busy.value = true
  error.value = ''
  try {
    original = await runOperation('resource', { id: channel.value })
    const m = original.attributes?.metadata || original.attributes || {}
    background.value = m.background || ''
    areas.value = JSON.parse(JSON.stringify(m.partitions || []))
    points.value = []
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
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
  try {
    const d = await runOperation('entity', { id: device.value })
    if (d.tenantId !== String(route.params.id))
      throw new Error(tr('设备不属于当前工作区。', 'Device is outside this workspace.'))
    const attrs = { ...d.attributes }
    if (attrs.metadata) attrs.metadata = { ...attrs.metadata, position: point }
    else attrs.position = point
    const saved = await runOperation('updateEntity', { id: d.id, input: { attributes: attrs } })
    devices.value = devices.value.map((x) => (x.id === d.id ? saved : x))
    notice.value = tr('设备位置已保存。', 'Device location saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function saveMap() {
  busy.value = true
  error.value = ''
  try {
    const latest = await runOperation('resource', { id: channel.value })
    if (JSON.stringify(latest.attributes) !== JSON.stringify(original.attributes))
      throw new Error(
        tr(
          '通道配置已被其他操作修改，请刷新后重试。',
          'Channel configuration changed. Refresh before saving.',
        ),
      )
    const attrs = { ...latest.attributes }
    if (attrs.metadata)
      attrs.metadata = { ...attrs.metadata, background: background.value, partitions: areas.value }
    else Object.assign(attrs, { background: background.value, partitions: areas.value })
    original = await runOperation('updateResource', {
      id: channel.value,
      input: { attributes: attrs },
    })
    notice.value = tr('底图与分区已保存。', 'Background and areas saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
watch(
  () => route.params.id,
  async (id) => {
    channel.value = ''
    try {
      ;[channels.value, devices.value] = await Promise.all([
        listAll('resources', { tenantId: id, kind: 'channel' }),
        listAll('entities', { tenantId: id, kind: 'device' }),
      ])
    } catch (e) {
      error.value = errorText(e)
    }
  },
  { immediate: true },
)
</script>
<style scoped>
.map-canvas {
  display: block;
  width: 100%;
  max-height: 680px;
  border: 1px solid #d7e5da;
  border-radius: 10px;
  cursor: crosshair;
}
</style>
