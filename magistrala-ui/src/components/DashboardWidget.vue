<template>
  <div class="widget-content" :class="{ 'weather-widget': chart.presentation === 'weather' }" :style="{ '--metric-accent': chart.accent || '#208b77' }" :aria-busy="loading">
    <p v-if="error" class="error widget-error" role="status">{{ error }}</p>
    <p v-if="loading && !loaded" class="widget-loading">{{ tr('正在加载…', 'Loading…') }}</p>
    <CameraPlayer
      v-if="type === 'camera_video'"
      :workspace="workspace"
      :device-id="chart.cameraDeviceId"
      :view="chart.cameraView || 'all'"
      :title="widget.title"
      :preview="preview"
      compact
    />
    <BuildingOverview
      v-else-if="type === 'building_overview' && !chart.map?.image"
      :buildings="chart.buildings || []"
    />
    <div v-else-if="type.includes('control')">
      <p v-if="preview" class="notice">
        {{ tr('草稿预览不会发送控制消息。', 'Draft preview cannot send control messages.') }}
      </p>
      <p>
        {{
          tr(
            '点击发送会发布设备消息，请核对数据源和设备。',
            'Sending publishes a device message. Check the source and device.',
          )
        }}
      </p>
      <input
        v-if="type === 'slider_control'"
        type="range"
        v-model.number="controlValue"
        :min="chart.dataSource?.minValue ?? chart.min ?? 0"
        :max="chart.dataSource?.maxValue ?? chart.max ?? 100"
        :step="chart.steps || 1"
      /><button
        v-else-if="chart.switchStyle === 'button'"
        type="button"
        :aria-pressed="switchValue"
        @click="switchValue = !switchValue"
      >
        {{ switchValue ? tr('开启', 'On') : tr('关闭', 'Off') }}</button
      ><label v-else class="check"
        ><input type="checkbox" v-model="switchValue" />{{ tr('开关', 'Switch') }}</label
      ><input
        v-model="key"
        type="password"
        :placeholder="tr('设备密钥（不保存）', 'Device key (not saved)')"
      /><button :disabled="loading || !key || preview" @click="send">
        {{ tr('发送控制消息', 'Send control message') }}
      </button>
      <output v-if="type === 'slider_control'"
        >{{ controlValue }} {{ unitText(chart.unit) }}</output
      >
    </div>
    <section v-else-if="type === 'count_card'" class="count-card">
      <div class="count-main">
        <NavIcon :name="entitySection(chart.dataSource?.entity)" />
        <div>
          <h3>{{ chart.dataSource?.label || resourceLabel }}</h3>
          <strong>{{ value }}</strong>
        </div>
      </div>
      <p>
        {{ tr('状态', 'Status') }}:
        {{
          chart.dataSource?.status === 'all'
            ? tr('全部', 'All')
            : label(chart.dataSource?.status || 'enabled')
        }}
      </p>
    </section>
    <section
      v-else-if="type === 'alarm_count'"
      class="alarm-count"
      :style="{ color: chart.color || 'blue' }"
    >
      <NavIcon :name="chart.icon || 'bell'" />
      <div>
        <p>{{ chart.label || tr('告警', 'Alarms') }}</p>
        <strong>{{ value }}</strong>
      </div>
    </section>
    <section v-else-if="type === 'value_card'" class="value-card">
      <p v-if="chart.presentation !== 'weather'">{{ chart.label || chart.dataSource?.label || chart.dataSources?.[0]?.label }}</p>
      <p class="metric" :style="type === 'alarm_count' ? { color: chart.color } : {}">
        {{ chart.presentation === 'weather' && typeof value === 'number' ? value.toFixed(chart.decimals ?? 1) : value }}
        <small v-if="type !== 'alarm_count'">{{
          unitText(chart.dataSources?.[0]?.unit || rows[0]?.unit)
        }}</small>
      </p>
      <div v-if="chart.presentation === 'weather'" class="metric-context">
        <span>{{ chart.caption }}</span><span>{{ latestTime }}</span>
      </div>
    </section>
    <div v-else-if="type.includes('table')" class="table-wrap">
      <table>
        <thead>
          <tr>
            <th v-for="column in tableColumns" :key="column.key">{{ column.label }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, index) in rows" :key="r.id || index">
            <td v-for="column in tableColumns" :key="column.key">
              <router-link
                v-if="type === 'table_card' && column.key === 'name' && r.id"
                :to="`/domain/${workspace}/${entitySection(chart.dataSource?.entity)}/${r.id}`"
                >{{ r.name || r.id }}</router-link
              >
              <template v-else>{{
                type === 'alarm_table' && ['cause', 'message'].includes(column.key)
                  ? alarmCause(r)
                  : (r[column.key] ?? r.attributes?.[column.key] ?? '—')
              }}</template>
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td :colspan="tableColumns.length">{{ tr('暂无数据', 'No data available') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <TelemetryMap
      v-else-if="type.includes('map') || type === 'building_overview'"
      :rows="rows"
      :type="type"
      :config="{
        ...chart,
        ...chart.map,
        ...(chart.cardType && (type === 'map' || type === 'polygon_map')
          ? { latitudeKey: 'latitude', longitudeKey: 'longitude' }
          : {}),
      }"
      :shapes="shapes"
    />
    <div v-else ref="canvas" class="chart-canvas"></div>
    <div v-if="chart.presentation === 'weather' && type !== 'value_card'" class="weather-footnote">
      <span>{{ chart.caption }}</span><span>{{ rows.length ? '最近 ' + latestTime : '暂无近期数据' }}</span>
    </div>
    <button v-if="type !== 'camera_video' && (chart.presentation !== 'weather' || preview)" class="quiet" :disabled="loading" @click="load">
      {{ tr('刷新组件', 'Refresh widget') }}
    </button>
  </div>
</template>
<script setup>
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { request, listAll, errorText } from '../api/platform'
import { tr, label } from '../i18n'
import NavIcon from './NavIcon.vue'
import TelemetryMap from './TelemetryMap.vue'
import BuildingOverview from './BuildingOverview.vue'
import CameraPlayer from './CameraPlayer.vue'
import { widgetType, entitySection } from '../api/widgetConfig'
import { chartOptions, unitText, messageTime } from '../api/chartPresentation'
import { alarmCause, completeAlarmColumns } from '../api/alarmPresentation'
import { loadWidgetData } from '../api/widgetData'
import { readResource } from '../api/resourceWorkflow'
const props = defineProps({ widget: Object, workspace: String, preview: Boolean, refreshToken: Number }),
  chart = computed(() => props.widget.chart || {}),
  type = computed(() => widgetType(props.widget)),
  shapes = ref([]),
  canvas = ref(null),
  rows = ref([]),
  value = ref('—'),
  loading = ref(false),
  loaded = ref(false),
  error = ref(''),
  key = ref(''),
  controlValue = ref(0),
  switchValue = ref(false)
const latestTime = computed(() => rows.value.length
  ? new Date(messageTime(rows.value[0].time)).toLocaleTimeString('zh-CN', { hour12: false }) : '等待上报')
const resourceLabel = computed(() =>
  entitySection(chart.value.dataSource?.entity) === 'channels'
    ? tr('通道', 'Channels')
    : entitySection(chart.value.dataSource?.entity) === 'groups'
      ? tr('分组', 'Groups')
      : tr('设备', 'Devices'),
)
const tableColumns = computed(() =>
  type.value === 'alarm_table'
    ? completeAlarmColumns(chart.value.columns)
        .filter((c) => c.visible)
        .sort((a, b) => (a.order || 0) - (b.order || 0))
    : [
        { key: 'name', label: tr('名称', 'Name') },
        { key: 'status', label: tr('状态', 'Status') },
        { key: 'id', label: 'ID' },
      ],
)
let refreshTimer,
  instance,
  observer,
  chartElement,
  painted = false,
  seq = 0
async function load() {
  if (type.value === 'camera_video') { loaded.value = true; loading.value = false; error.value = ''; return }
  const current = ++seq
  loading.value = true
  try {
    const result = await loadWidgetData(props.widget, props.workspace, {
      request,
      listAll,
      readResource,
    })
    if (current !== seq) return
    rows.value = result.rows
    shapes.value = result.shapes
    value.value = result.value ?? '—'
    await nextTick()
    await draw(current)
    if (current === seq) {
      loaded.value = true
      error.value = ''
    }
  } catch (e) {
    if (current === seq) error.value = errorText(e)
  } finally {
    if (current === seq) loading.value = false
  }
}
function releaseChart() {
  observer?.disconnect()
  instance?.dispose()
  observer = undefined
  instance = undefined
  chartElement = undefined
  painted = false
}
function updateChart() {
  if (!instance || !chartElement) return
  const options = chartOptions(props.widget, rows.value, {
    width: chartElement.clientWidth,
    height: chartElement.clientHeight,
  })
  // Keep the canvas alive; replacing options also clears series when data is empty.
  instance.setOption({ ...options, animation: !painted, animationDurationUpdate: 0 }, { notMerge: true })
  painted = true
}
async function draw(current) {
  if (current !== seq) return
  if (!canvas.value) {
    releaseChart()
    return
  }
  const echarts = await import('echarts')
  if (current !== seq || !canvas.value) return
  if (chartElement !== canvas.value) {
    releaseChart()
    chartElement = canvas.value
    instance = echarts.init(chartElement)
    observer = new ResizeObserver(() => {
      instance?.resize()
      if (type.value === 'gauge_chart') updateChart()
    })
    observer.observe(chartElement)
  }
  updateChart()
}
async function send() {
  if (props.preview) return
  loading.value = true
  error.value = ''
  try {
    const source = chart.value.dataSource?.channel?.id
      ? chart.value.dataSource
      : chart.value.dataSources?.[0]
    if (!source?.channel?.id || !source?.client?.id)
      throw new Error(
        tr('控制组件必须指定通道和设备。', 'A control widget requires a channel and device.'),
      )
    if (
      type.value === 'slider_control' &&
      (!Number.isFinite(controlValue.value) ||
        controlValue.value < Number(source.minValue ?? chart.value.min ?? 0) ||
        controlValue.value > Number(source.maxValue ?? chart.value.max ?? 100))
    )
      throw new Error(tr('控制值超出配置范围。', 'Control value is outside the configured range.'))
    await request(
      `/api/platform/http/http/m/${props.workspace}/c/${source.channel.id}${chart.value.subtopic ? '/' + chart.value.subtopic.split('.').map(encodeURIComponent).join('/') : ''}`,
      {
        method: 'POST',
        auth: false,
        headers: {
          Authorization: `Basic ${btoa(source.client.id + ':' + key.value)}`,
          'Content-Type': 'application/senml+json',
        },
        body: [
          {
            n: source.label || chart.value.valueName || 'command',
            ...(type.value === 'slider_control'
              ? { v: controlValue.value }
              : { vb: switchValue.value }),
            t: Date.now() / 1000,
          },
        ],
      },
    )
    key.value = ''
  } catch (e) {
    error.value = errorText(e)
  } finally {
    loading.value = false
  }
}
watch(
  () => [props.widget, props.workspace],
  () => {
    clearInterval(refreshTimer)
    controlValue.value = Math.max(
      Number(chart.value.dataSource?.minValue ?? chart.value.min ?? 0),
      Math.min(
        Number(chart.value.dataSource?.maxValue ?? chart.value.max ?? 100),
        controlValue.value,
      ),
    )
    load()
    const seconds = Number(chart.value.dataRefreshInterval)
    if (seconds > 0 && !type.value.includes('control') && type.value !== 'building_overview')
      refreshTimer = setInterval(
        () => {
          if (!loading.value && !document.hidden) load()
        },
        Math.max(seconds, 1) * 1000,
      )
  },
  { deep: true, immediate: true },
)
watch(() => props.refreshToken, () => load())
onBeforeUnmount(() => {
  clearInterval(refreshTimer)
  seq++
  releaseChart()
})
</script>
<style scoped>
.widget-content {
  position: relative;
  min-height: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.widget-loading, .widget-error {
  position: absolute;
  z-index: 1;
  top: 0;
  left: 0;
  right: 0;
  margin: 0;
  padding: 6px 8px;
  background: rgba(255, 255, 255, .95);
  pointer-events: none;
}
.widget-content > button {
  align-self: flex-start;
  flex-shrink: 0;
}
.chart-canvas {
  flex: 1;
  min-height: 160px;
  width: 100%;
}
.count-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  flex: 1;
  min-height: 100px;
}
.count-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 12px;
  gap: 16px;
  flex: 1;
}
.count-main :deep(svg) {
  width: 54px;
  height: 54px;
  color: #27887d;
}
.count-main > div {
  text-align: center;
}
.count-main h3 {
  margin: 0 0 8px;
}
.count-main strong {
  font-size: 42px;
}
.count-card > p {
  color: #637180;
}
.alarm-count {
  display: flex;
  align-items: center;
  justify-content: space-around;
  gap: 16px;
  flex: 1;
}
.alarm-count :deep(svg) {
  width: 54px;
  height: 54px;
}
.alarm-count strong {
  font-size: 42px;
}
.value-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.weather-widget { gap: 4px; min-height: 0; }
.weather-widget .value-card { justify-content: space-between; gap: 10px; }
.weather-widget .metric { font-size: clamp(30px, 3vw, 42px); font-weight: 600; line-height: 1.2; margin: 3px 0 8px; color: var(--metric-accent); letter-spacing: -1.5px; font-variant-numeric: tabular-nums; }
.weather-widget .metric small { font-size: 14px; margin-left: 7px; color: #8a999f; font-weight: 400; letter-spacing: 0; }
.metric-context, .weather-footnote { display: flex; justify-content: space-between; gap: 10px; color: #8b999f; font-size: 10px; line-height: 1.6; }
.weather-footnote { flex-shrink: 0; }
.weather-widget .chart-canvas { min-height: 120px; }
</style>
