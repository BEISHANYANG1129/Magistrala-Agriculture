<template>
  <div class="panel" :class="{ 'dashboard-expanded': expanded }">
    <div class="actions">
      <h2>{{ dashboard.name }} · {{ tr('仪表盘', 'Dashboard') }}</h2>
      <label class="check"
        ><input
          type="checkbox"
          role="switch"
          :checked="editing"
          :disabled="busy"
          :aria-label="tr('编辑模式', 'Edit Mode')"
          @change="toggleEdit"
        />{{ tr('编辑模式', 'Edit Mode') }}</label
      >
      <template v-if="editing">
        <button class="quiet" :disabled="busy" @click="openSettings">{{ tr('编辑仪表盘', 'Edit Dashboard') }}</button>
        <button class="quiet" :disabled="busy" @click="chooser = true">
          + {{ tr('添加组件', 'Add Widget') }}
        </button>
        <button v-if="!record.id" :disabled="busy" @click="save">
          {{ tr('应用到草稿', 'Apply to draft') }}
        </button>
        <button class="quiet" :disabled="busy" @click="cancelEdit">
          {{ tr('取消编辑', 'Cancel editing') }}
        </button>
      </template>
      <button class="quiet" :disabled="busy" @click="refreshRevision++">
        {{ tr('刷新仪表盘', 'Refresh dashboard') }}
      </button>
      <label v-if="editing" class="field"
        >{{ tr('屏幕布局', 'Screen layout')
        }}<select v-model="preferredBreakpoint">
          <option value="auto">{{ tr('自动适应', 'Automatic') }}</option>
          <option v-for="(cols, bp) in gridColumns" :key="bp" :value="bp">
            {{ bp }} · {{ cols }} {{ tr('列', 'columns') }}
          </option>
        </select></label
      >
      <button v-if="!editing && record.id" class="quiet" @click="emit('share')">{{ tr('分享仪表盘', 'Share Dashboard') }}</button>
      <button v-if="!editing" class="quiet" @click="downloadDashboard(exportRecord())">
        {{ tr('下载 JSON', 'Download JSON') }}
      </button>
      <label v-if="editing" class="button quiet"
        >{{ tr('导入 JSON', 'Import JSON')
        }}<input type="file" accept=".json,application/json" @change="importFile"
      /></label>
      <button v-if="!editing" class="quiet" @click="expanded = !expanded">
        {{ expanded ? tr('退出全屏', 'Exit full screen') : tr('全屏', 'Full screen') }}</button
      ><button v-if="!editing" class="quiet" @click="print">
        {{ tr('打印 / 导出 PDF', 'Print / export PDF') }}
      </button>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <DashboardGrid
      :cells="layouts[breakpoint] || []"
      :columns="gridColumns[breakpoint]"
      :widgets="widgets"
      :editable="editing"
      @update:cells="layouts[breakpoint] = $event"
      @width="viewportWidth = $event"
    >
      <template #actions="{ id }"
        ><div class="actions">
          <button v-if="editing" class="quiet" @click="edit(widgets.find((w) => w.id === id))">
            {{ tr('编辑', 'Edit') }}</button
          ><button v-if="editing" class="quiet" @click="removingWidget = id">
            {{ tr('移除', 'Remove') }}
          </button>
        </div></template
      >
      <template #default="{ id }"
        ><DashboardWidget
          :key="id + ':' + refreshRevision"
          :widget="widgets.find((w) => w.id === id)"
          :workspace="workspace"
          :preview="!record.id"
      /></template>
    </DashboardGrid>
    <p v-if="!widgets.length">
      {{
        tr(
          '还没有组件，可以添加趋势图、数值卡或数据表。',
          'No widgets yet. Add a trend chart, value card or data table.',
        )
      }}
    </p>
    <ModalDialog v-if="settings" @close="settings = null">
      <form class="modal-card" @submit.prevent="applySettings">
        <h2>{{ tr('编辑仪表盘', 'Edit Dashboard') }}</h2>
        <label class="field">{{ tr('名称', 'Name') }} *<input v-model="settings.name" required /></label>
        <label class="field">{{ tr('描述', 'Description') }}<textarea v-model="settings.description" /></label>
        <label class="field">{{ tr('标签（逗号分隔）', 'Tags (comma separated)') }}<input v-model="settings.tags" /></label>
        <label class="field">{{ tr('缩略图', 'Thumbnail') }}<input v-model="settings.thumbnail" /><input type="file" accept="image/png,image/jpeg,image/webp" @change="settingsThumbnail" /></label>
        <label class="field">{{ tr('元数据', 'Metadata') }}<textarea v-model="settings.metadata" rows="6" /></label>
        <p v-if="settingsError" class="error" role="alert">{{ settingsError }}</p>
        <div class="actions"><button type="button" class="quiet" @click="settings = null">{{ tr('关闭', 'Close') }}</button><button>{{ tr('更新', 'Update') }}</button></div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="chooser" @close="chooser = false">
      <div class="modal-card">
        <h2>{{ tr('添加组件', 'Add Widget') }}</h2>
        <div class="widget-picker">
          <button v-for="[type, zh, en] in chartTypes" :key="type" class="quiet" @click="add(type)">
            {{ tr(zh, en) }}
          </button>
        </div>
        <div class="actions">
          <button class="quiet" @click="chooser = false">{{ tr('关闭', 'Close') }}</button>
        </div>
      </div>
    </ModalDialog>
    <ModalDialog v-if="removingWidget" confirmation @close="removingWidget = ''">
      <div class="modal-card compact-modal">
        <h2>{{ tr('确定删除此组件？', 'Are you sure?') }}</h2>
        <div class="actions">
          <button class="quiet" @click="removingWidget = ''">{{ tr('取消', 'Cancel') }}</button
          ><button
            @click="confirmWidgetRemoval"
          >
            {{ tr('删除', 'Delete') }}
          </button>
        </div>
      </div>
    </ModalDialog>
    <ModalDialog v-if="draft" @close="draft = null">
      <form class="modal-card" @submit.prevent="apply">
        <h2>{{ tr('组件设置', 'Widget settings') }}</h2>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <div class="form-grid">
          <label class="field"
            >{{ tr('标题', 'Title') }}<input v-model="draft.title" required /></label
          ><label class="field"
            >{{ tr('显示类型', 'Visualization')
            }}<select v-model="draftType">
              <option v-for="t in chartTypes" :key="t[0]" :value="t[0]">
                {{ tr(t[1], t[2]) }}
              </option>
            </select></label
          ><label v-if="telemetryDraft" class="field"
            >{{ tr('指标名称', 'Metric name') }}<input v-model="draft.chart.valueName" /></label
          ><label v-if="telemetryDraft" class="field"
            >{{ tr('最多读取条数', 'Maximum rows')
            }}<input v-model.number="draft.chart.limit" type="number" min="1" max="1000"
          /></label>
        </div>
        <WidgetSettings
          :workspace="workspace"
          :widget="draft"
          :channels="channels"
          :devices="devices"
          :groups="groups"
        />
        <template v-if="telemetryDraft">
          <h3>{{ tr('数据源', 'Data sources') }}</h3>
          <div v-for="(s, i) in draft.chart.dataSources" :key="i" class="array-row">
            <select v-model="s.channel.id" required>
              <option value="">{{ tr('通道', 'Channel') }}</option>
              <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option></select
            ><select v-model="s.client.id">
              <option value="">{{ tr('全部设备', 'All devices') }}</option>
              <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option></select
            ><button type="button" class="quiet" @click="draft.chart.dataSources.splice(i, 1)">
              {{ tr('移除', 'Remove') }}
            </button>
          </div>
          <button
            type="button"
            class="quiet"
            @click="draft.chart.dataSources.push({ channel: { id: '' }, client: { id: '' } })"
          >
            + {{ tr('添加数据源', 'Add source') }}
          </button>
        </template>
        <details>
          <summary>{{ tr('完整组件配置', 'Full widget configuration') }}</summary>
          <textarea
            :value="JSON.stringify(draft, null, 2)"
            rows="12"
            @change="advanced($event)"
          ></textarea>
        </details>
        <div class="actions">
          <button>
            {{
              widgets.some((w) => w.id === draft.id) ? tr('更新', 'Update') : tr('创建', 'Create')
            }}</button
          ><button type="button" class="quiet" @click="draft = null">
            {{ tr('取消', 'Cancel') }}
          </button>
        </div>
      </form>
    </ModalDialog>
  </div>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, watch, computed } from 'vue'
import { editedMetadata } from '../api/editedMetadata'
import { tr, language } from '../i18n'
import { request, listAll, errorText } from '../api/platform'
import DashboardWidget from './DashboardWidget.vue'
import DashboardGrid from './DashboardGrid.vue'
import WidgetSettings from './WidgetSettings.vue'
import { widgetType, setWidgetType, validateWidget, cardTypes } from '../api/widgetConfig'
import {
  gridColumns,
  normalizeLayout,
  dashboardImport,
  downloadDashboard,
} from '../api/dashboardLayout'
const props = defineProps({ record: Object, workspace: String }),
  emit = defineEmits(['draft', 'saved', 'share'])
const editing = ref(!props.record.id),
  expanded = ref(false),
  preferredBreakpoint = ref('auto'),
  viewportWidth = ref(1200),
  layouts = ref({})
const settings = ref(null), settingsError = ref(''), settingsPatch = ref({})
const chooser = ref(false),
  removingWidget = ref(''),
  refreshRevision = ref(0)
const dashboard = ref({}),
  widgets = ref([]),
  draft = ref(null),
  channels = ref([]),
  devices = ref([]),
  groups = ref([]),
  busy = ref(false),
  error = ref(''),
  notice = ref('')
const breakpoint = computed(() =>
  preferredBreakpoint.value === 'auto'
    ? viewportWidth.value >= 1200
      ? 'lg'
      : viewportWidth.value >= 996
        ? 'md'
        : viewportWidth.value >= 768
          ? 'sm'
          : viewportWidth.value >= 480
            ? 'xs'
            : 'xxs'
    : preferredBreakpoint.value,
)
watch(language, () => {
  notice.value = ''
})
const chartTypes = [
  ['line_chart', '折线图', 'Line'],
  ['area_chart', '面积图', 'Area'],
  ['bar_chart', '柱状图', 'Bar'],
  ['pie_chart', '饼图', 'Pie'],
  ['gauge_chart', '仪表', 'Gauge'],
  ['value_card', '数值卡', 'Value'],
  ['table_card', '资源表', 'Resource table'],
  ['count_card', '资源计数', 'Resource count'],
  ['alarm_count', '告警数量', 'Alarm count'],
  ['alarm_table', '告警列表', 'Alarm table'],
  ['map', '位置图', 'Locations'],
  ['route_map', '轨迹图', 'Routes'],
  ['polygon_map', '区域图', 'Polygons'],
  ['building_overview', '建筑概览', 'Building overview'],
  ['switch_control', '开关控制', 'Switch control'],
  ['slider_control', '滑块控制', 'Slider control'],
]
watch(
  () => props.record,
  async (r) => {
    dashboard.value = JSON.parse(JSON.stringify(r))
    widgets.value = Object.entries(dashboard.value.metadata?.widget || {}).map(([id, w]) => ({
      ...w,
      id: w.id || id,
    }))
    layouts.value = normalizeLayout(r.layout, widgets.value)
    draft.value = null
    try {
      ;[channels.value, devices.value, groups.value] = await Promise.all([
        listAll('resources', { tenantId: props.workspace, kind: 'channel' }),
        listAll('entities', { tenantId: props.workspace, kind: 'device' }),
        listAll('groups', { tenantId: props.workspace }),
      ])
    } catch (e) {
      error.value = errorText(e)
    }
  },
  { immediate: true },
)
const draftType = computed({
  get: () => widgetType(draft.value),
  set: (type) => setWidgetType(draft.value, type),
})
const telemetryDraft = computed(
  () => !cardTypes.includes(draftType.value) && !draftType.value.endsWith('_control'),
)
function add(type = 'line_chart') {
  chooser.value = false
  draft.value = {
    id: crypto.randomUUID(),
    title: '',
    name: '',
    type: 'chart',
    chart: {
      chartType: 'line_chart',
      limit: 50,
      timeWindowFormat: 'realtime',
      last: 3600000,
      dataRefreshInterval: 60,
      valueName: 'soil_moisture',
      dataSources: [{ channel: { id: '' }, client: { id: '' } }],
    },
  }
  setWidgetType(draft.value, type)
}
async function toggleEdit(event) {
  const control = event.currentTarget
  if (!busy.value) {
    if (editing.value) await save()
    else {
      error.value = ''
      notice.value = ''
      editing.value = true
    }
  }
  control.checked = editing.value
}
function edit(w) {
  draft.value = JSON.parse(JSON.stringify(w))
  draft.value.chart ||= {}
  draft.value.chart.dataSources ||= []
}
function apply() {
  error.value = ''
  try {
    const w = draft.value,
      c = w.chart
    setWidgetType(w, widgetType(w))
    c.isLive = c.timeWindowFormat === 'realtime'
    c.chartData ||= 'single'
    c.lastOption ||= 'custom'
    const label = (ref, items) => {
      if (ref?.id) {
        const item = items.find((r) => r.id === ref.id)
        if (item) ref.name = item.name
      }
    }
    for (const source of c.dataSources || []) {
      source.id ||= crypto.randomUUID()
      label(source.channel, channels.value)
      label(source.client, devices.value)
      label(source.entityDetails || source.entity, [
        ...channels.value,
        ...devices.value,
        ...groups.value,
      ])
      if (telemetryDraft.value) source.valueName ||= c.valueName
    }
    label(c.dataSource?.channel, channels.value)
    label(c.dataSource?.client, devices.value)
    validateWidget(w)
  } catch (e) {
    error.value = e.message
    return
  }
  const i = widgets.value.findIndex((w) => w.id === draft.value.id)
  if (i < 0) widgets.value.push(draft.value)
  else widgets.value[i] = draft.value
  draft.value = null
  layouts.value = normalizeLayout(layouts.value, widgets.value)
  notice.value = tr(
    '组件已更新，关闭编辑模式时保存仪表盘。',
    'Widget updated. Turn off Edit Mode to save the dashboard.',
  )
}
function advanced(e) {
  try {
    draft.value = JSON.parse(e.target.value)
    e.target.setCustomValidity('')
  } catch {
    e.target.setCustomValidity(tr('JSON 格式错误', 'Invalid JSON'))
  }
}
function confirmWidgetRemoval() { remove(removingWidget.value); removingWidget.value = '' }
function remove(id) {
  widgets.value = widgets.value.filter((w) => w.id !== id)
  layouts.value = normalizeLayout(layouts.value, widgets.value)
}
function move(i, d) {
  const a = widgets.value
  ;[a[i], a[i + d]] = [a[i + d], a[i]]
}
function print() {
  window.print()
}
function exportRecord() {
  return {
    ...dashboard.value,
    metadata: {
      ...dashboard.value.metadata,
      widget: Object.fromEntries(widgets.value.map((w) => [w.id, w])),
    },
    layout: layouts.value,
  }
}
function openSettings() {
  settingsError.value = ''
  settings.value = { name: dashboard.value.name || '', description: dashboard.value.description || '', tags: (dashboard.value.tags || []).join(', '), thumbnail: dashboard.value.thumbnail || '', metadata: JSON.stringify(dashboard.value.metadata || {}, null, 2) }
}
function settingsThumbnail(event) {
  const file = event.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024 || !['image/png','image/jpeg','image/webp'].includes(file.type)) { settingsError.value = tr('请选择不超过 5 MB 的 PNG、JPEG 或 WebP 图片。', 'Choose PNG, JPEG or WebP up to 5 MB.'); return }
  const current = settings.value, reader = new FileReader()
  reader.onload = () => { if (settings.value === current) settings.value.thumbnail = reader.result }
  reader.readAsDataURL(file)
}
function applySettings() {
  try {
    const metadata = JSON.parse(settings.value.metadata)
    if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) throw new Error(tr('元数据必须是 JSON 对象。', 'Metadata must be a JSON object.'))
    const next = { name: settings.value.name.trim(), description: settings.value.description, tags: settings.value.tags.split(/[,，]/).map(t => t.trim()).filter(Boolean), thumbnail: settings.value.thumbnail }
    if (!next.name) throw new Error(tr('请输入名称。', 'Enter a name.'))
    for (const [key, value] of Object.entries(next)) if (JSON.stringify(value) !== JSON.stringify(dashboard.value[key] ?? (key === 'tags' ? [] : ''))) settingsPatch.value[key] = value
    dashboard.value = { ...dashboard.value, ...next, metadata }
    settings.value = null
  } catch(e) { settingsError.value = e.message }
}
function cancelEdit() {
  dashboard.value = JSON.parse(JSON.stringify(props.record))
  settingsPatch.value = {}
  settings.value = null
  widgets.value = Object.entries(dashboard.value.metadata?.widget || {}).map(([id, w]) => ({
    ...JSON.parse(JSON.stringify(w)),
    id: w.id || id,
  }))
  layouts.value = normalizeLayout(dashboard.value.layout, widgets.value)
  draft.value = null
  editing.value = false
  chooser.value = false
  removingWidget.value = ''
  error.value = ''
  notice.value = ''
}
async function importFile(e) {
  const file = e.target.files?.[0]
  if (!file) return
  try {
    if (file.size > 5 * 1024 * 1024)
      throw new Error(tr('文件不能超过 5 MB', 'File must be under 5 MB'))
    const data = dashboardImport(await file.text())
    widgets.value = Object.values(data.metadata.widget)
    layouts.value = data.layout
    notice.value = tr(
      '已导入画布草稿，请核对数据源并保存。',
      'Imported into the canvas draft. Review sources and save.',
    )
  } catch (e) {
    error.value = e.message
  } finally {
    e.target.value = ''
  }
}
async function save() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    if (!props.record.id) {
      emit('draft', exportRecord())
      notice.value = tr(
        '已应用到本地草稿；点击设置中的创建才会提交。',
        'Applied to local draft. Use Create in Settings to submit.',
      )
      return
    }
    const latest = await request(
      `/api/platform/ui/${props.workspace}/dashboards/${dashboard.value.id}`,
    )
    const metadata = editedMetadata(
      props.record.metadata,
      {
        ...dashboard.value.metadata,
        widget: Object.fromEntries(widgets.value.map((w) => [w.id, w])),
      },
      latest.metadata,
    )
    const layout = editedMetadata(props.record.layout, layouts.value, latest.layout)
    const saved = await request(
      `/api/platform/ui/${props.workspace}/dashboards/${dashboard.value.id}`,
      { method: 'PATCH', body: { ...settingsPatch.value, metadata, layout } },
    )
    dashboard.value = { ...latest, ...settingsPatch.value, ...saved, metadata, layout }
    settingsPatch.value = {}
    editing.value = false
    emit('saved', dashboard.value)
    notice.value = tr('仪表盘已保存。', 'Dashboard saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
<style scoped>
.widget-picker {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin: 20px 0;
}
.widget-picker button {
  min-height: 64px;
}
.dashboard-expanded {
  position: fixed;
  inset: 10px;
  z-index: 90;
  overflow: auto;
  margin: 0;
}

.widget-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
  gap: 18px;
}
.widget-grid > .panel {
  margin: 10px 0;
  min-width: 0;
}
@media print {
  button,
  .modal {
    display: none;
  }
  .widget-grid {
    display: block;
  }
}
</style>
