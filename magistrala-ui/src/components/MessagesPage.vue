<template>
  <section>
    <header v-if="!embedded" class="page-heading">
      <div>
        <p class="eyebrow">TELEMETRY</p>
        <h1>{{ tr('消息与视图', 'Messages & views') }}</h1>
        <p>
          {{
            tr(
              '按通道和设备查看数据，也可以发送一条传感器消息。',
              'Inspect data by channel and device, or publish a sensor reading.',
            )
          }}
        </p>
      </div>
    </header>
    <div class="toolbar">
      <button :disabled="!channel" @click="sendOpen = true">
        {{ tr('发送消息', 'Send message') }}</button
      ><button class="quiet" @click="filtersOpen = !filtersOpen">{{ tr('筛选', 'Filter') }}</button
      ><button class="quiet" :disabled="!channel || busy" @click="load">
        {{ tr('刷新', 'Refresh') }}</button
      ><button class="quiet" :disabled="!rows.length" @click="downloadPage">
        {{ tr('下载当前页', 'Download current page') }}
      </button>
    </div>
    <label v-if="!embedded" class="field"
      >{{ tr('选择通道', 'Select channel')
      }}<select v-model="channel" @change="load">
        <option value="">{{ tr('请选择', 'Select') }}</option>
        <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option>
      </select></label
    >
    <form
      v-if="filtersOpen"
      class="panel"
      @submit.prevent="
        () => {
          offset = 0
          load()
        }
      "
    >
      <div class="form-grid">
        <label class="field"
          >{{ tr('通道', 'Channel')
          }}<select v-model="channel" required :disabled="embedded">
            <option value="">{{ tr('选择通道', 'Choose a channel') }}</option>
            <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name || c.id }}</option>
          </select></label
        ><label class="field"
          >{{ tr('设备筛选', 'Device filter')
          }}<select v-model="publisher">
            <option value="">{{ tr('全部设备', 'All devices') }}</option>
            <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option>
          </select></label
        ><label class="field"
          >{{ tr('指标名称', 'Metric name')
          }}<input v-model="metric" placeholder="soil_moisture" /></label
        ><label class="field"
          >{{ tr('每页数量', 'Page size')
          }}<select v-model.number="limit">
            <option :value="20">20</option>
            <option :value="50">50</option>
            <option :value="100">100</option>
          </select></label
        ><label class="field"
          >{{ tr('开始时间', 'From') }}<input type="datetime-local" v-model="from" /></label
        ><label class="field"
          >{{ tr('结束时间', 'To') }}<input type="datetime-local" v-model="to"
        /></label>
      </div>
      <button :disabled="busy">{{ tr('查询消息', 'Load messages') }}</button>
    </form>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <nav
      v-if="channel"
      class="tabs message-view-tabs"
      :aria-label="tr('消息视图', 'Message views')"
    >
      <button class="quiet" :class="{ active: !selectedView }" @click="selectView(null)">
        {{ tr('全部消息', 'All messages') }}
      </button>
      <span v-for="v in views" :key="v.id" class="actions">
        <button class="quiet" :class="{ active: selectedView?.id === v.id }" @click="selectView(v)">
          {{ v.name }}
        </button>
        <button
          class="quiet"
          :aria-label="tr('编辑视图', 'Edit view') + ' ' + v.name"
          @click="openView('PUT', v)"
        >
          ⚙
        </button>
      </span>
      <button class="quiet" @click="openView('POST')">+ {{ tr('新建视图', 'New view') }}</button>
    </nav>
    <div class="tabs">
      <button class="quiet" :class="{ active: tab === 'table' }" @click="tab = 'table'">
        {{ tr('表格', 'Table') }}</button
      ><button class="quiet" :class="{ active: tab === 'chart' }" @click="tab = 'chart'">
        {{ tr('趋势', 'Trend') }}</button
      ><button class="quiet" :class="{ active: tab === 'raw' }" @click="tab = 'raw'">
        {{ tr('原始记录', 'Raw records') }}</button
      ><button
        class="quiet"
        :disabled="!channel"
        @click="
          () => {
            viewsOpen = !viewsOpen
            loadViews()
          }
        "
      >
        {{ tr('保存的消息视图', 'Saved message views') }}
      </button>
    </div>
    <div v-if="tab === 'chart'" class="panel">
      <svg
        viewBox="0 0 800 220"
        role="img"
        :aria-label="tr('当前页数值趋势', 'Numeric trends for this page')"
      >
        <g v-for="(series, index) in chartSeries" :key="series.name">
          <polyline
            :points="series.points"
            fill="none"
            :stroke="['#19624f', '#e9a23b', '#5779c9'][index % 3]"
            stroke-width="2"
          />
          <text x="10" :y="16 + index * 16" font-size="12">{{ series.name }}</text>
        </g></svg
      ><small>{{
        tr(
          '当前页数据；不同单位请分别筛选查看。',
          'Current page only. Filter metrics separately when units differ.',
        )
      }}</small>
    </div>
    <pre v-else-if="tab === 'raw'" class="panel">{{ JSON.stringify(rows, null, 2) }}</pre>
    <div v-else class="table-wrap">
      <table>
        <thead>
          <tr>
            <th v-for="c in visibleColumns" :key="c.key">{{ c.label }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, index) in displayRows" :key="index">
            <td v-for="c in visibleColumns" :key="c.key">{{ cell(row, c.key) }}</td>
          </tr>
          <tr v-if="!displayRows.length">
            <td :colspan="visibleColumns.length">
              {{
                tr(
                  '暂无消息。上传成功后仍需匹配存储规则，才能在此查询。',
                  'No messages. Publishing also needs a matching storage rule before records appear here.',
                )
              }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <TablePagination :page="Math.floor(offset / limit) + 1" :size="limit" :total="total" :busy="busy" @update:page="changePage" @update:size="changeSize" />
    <div v-if="viewsOpen" class="panel">
      <h2>{{ tr('消息视图', 'Message views') }}</h2>
      <p>
        {{
          tr(
            '应用视图会按格式、设备和子主题重新查询消息。',
            'Applying a view queries messages by format, publishers and subtopic.',
          )
        }}
      </p>
      <button v-if="selectedView" class="quiet" @click="selectView(null)">
        {{ tr('恢复默认视图', 'Reset view') }}
      </button>
      <button @click="openView('POST')">+ {{ tr('新建视图', 'Create view') }}</button>
      <div v-for="v in views" :key="v.id" class="toolbar">
        <button class="quiet" @click="selectView(v)">{{ tr('使用视图', 'Apply view') }}</button
        ><strong>{{ v.name || v.id }}</strong
        ><button class="quiet" @click="openView('PUT', v)">{{ tr('编辑', 'Edit') }}</button
        ><button class="quiet" @click="openView('DELETE', v)">{{ tr('删除', 'Delete') }}</button>
      </div>
    </div>
    <ModalDialog v-if="sendOpen" :busy="busy" @close="sendOpen = false">
      <form class="modal-card compact-modal" @submit.prevent="publish">
        <h2>{{ tr('发送消息', 'Send message') }}</h2>
        <label class="field"
          >{{ tr('名称', 'Name') }} *<input v-model="sendMetric" required
        /></label>
        <label class="field"
          >{{ tr('单位', 'Unit') }}<input v-model="sendUnit" list="message-units" /><datalist
            id="message-units"
          >
            <option value="%" />
            <option value="Cel" />
            <option value="V" />
            <option value="A" />
            <option value="lx" /></datalist
        ></label>
        <label class="field"
          >{{ tr('数值类型', 'Value type') }} *<select v-model="sendType">
            <option value="number">{{ tr('数值', 'Number') }}</option>
            <option value="string">{{ tr('文本', 'String') }}</option>
            <option value="boolean">{{ tr('布尔值', 'Boolean') }}</option>
            <option value="data">{{ tr('二进制数据（Base64）', 'Data (Base64)') }}</option>
          </select></label
        >
        <label class="field"
          >{{ tr('值', 'Value') }} *<select v-if="sendType === 'boolean'" v-model="sendValue">
            <option :value="true">true</option>
            <option :value="false">false</option></select
          ><input
            v-else
            v-model="sendValue"
            :type="sendType === 'number' ? 'number' : 'text'"
            step="any"
            required
        /></label>
        <label class="field"
          >{{ tr('发送设备', 'Publisher') }} *<select
            v-model="sendDevice"
            required
            @change="sendKey = ''"
          >
            <option value="">{{ tr('选择设备', 'Select a device') }}</option>
            <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option>
          </select></label
        >
        <label class="field"
          >{{ tr('子主题', 'Subtopic') }}<input v-model="subtopic" placeholder="sensors/soil"
        /></label>
        <details>
          <summary>{{ tr('手动填写设备密钥', 'Enter device key manually') }}</summary>
          <label class="field"
            >{{
              tr(
                '留空时使用当前账户权限获取设备密钥',
                'Leave empty to retrieve the device key with your account',
              )
            }}<input v-model="sendKey" type="password" autocomplete="new-password"
          /></label>
        </details>
        <details>
          <summary>{{ tr('查看发送内容', 'Preview payload') }}</summary>
          <pre>{{ JSON.stringify(payload, null, 2) }}</pre>
        </details>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="sendOpen = false">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="busy || !channel">{{ tr('发送', 'Send') }}</button>
        </div>
      </form>
    </ModalDialog>
    <MessageViewEditor
      v-if="activeView"
      :key="channel"
      :record="activeView.record"
      :removing="activeView.removing"
      :workspace="workspace"
      :channel="channel"
      :devices="devices"
      @close="activeView = null"
      @saved="loadViews"
    />
  </section>
</template>
<script setup>
import TablePagination from './TablePagination.vue'
import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tr, language } from '../i18n'
import { runOperation, request, errorText } from '../api/platform'
import MessageViewEditor from './MessageViewEditor.vue'
import { viewSettings, viewCell, readerViewParams } from '../api/messageViews'
import { deviceKey } from '../api/resourceWorkflow'
const props = defineProps({ channelId: String, embedded: Boolean })
const sendOpen = ref(false),
  filtersOpen = ref(false),
  sendType = ref('number'),
  subtopic = ref('')
const route = useRoute(),
  router = useRouter(),
  workspace = computed(() => String(route.params.id)),
  channels = ref([]),
  devices = ref([]),
  channel = ref(''),
  publisher = ref(''),
  metric = ref(''),
  from = ref(''),
  to = ref(''),
  limit = ref(20),
  offset = ref(0),
  rows = ref([]),
  total = ref(0),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  tab = ref('table'),
  views = ref([]),
  viewsOpen = ref(false),
  activeView = ref(null),
  selectedView = ref(null)
const sendDevice = ref(''),
  sendKey = ref(''),
  sendMetric = ref('soil_moisture'),
  sendValue = ref(12),
  sendUnit = ref('%')
watch(sendType, (type) => {
  sendValue.value = type === 'boolean' ? false : type === 'number' ? 0 : ''
})
let sequence = 0
const payload = computed(() => [
  {
    n: sendMetric.value,
    ...(sendUnit.value ? { u: sendUnit.value } : {}),
    ...{
      number: { v: Number(sendValue.value) },
      string: { vs: String(sendValue.value) },
      boolean: { vb: sendValue.value === true || sendValue.value === 'true' },
      data: { vd: String(sendValue.value) },
    }[sendType.value],
  },
])
function downloadPage() {
  const keys = [
    'time',
    'publisher',
    'name',
    'value',
    'unit',
    'string_value',
    'bool_value',
    'data_value',
  ]
  const quote = (v) => '"' + String(v ?? '').replace(/"/g, '""') + '"'
  const content = [
    keys.join(','),
    ...rows.value.map((r) => keys.map((k) => quote(r[k])).join(',')),
  ].join('\r\n')
  const url = URL.createObjectURL(
    new Blob(['\uFEFF' + content], { type: 'text/csv;charset=utf-8' }),
  )
  const a = document.createElement('a')
  a.href = url
  a.download = 'messages-current-page.csv'
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
function formatTime(time) {
  if (!time) return '—'
  if (typeof time === 'string' && !Number.isFinite(Number(time)))
    return new Date(time).toLocaleString(language.value === 'zh' ? 'zh-CN' : 'en-US')
  const n = Number(time)
  return new Date(n > 1e16 ? n / 1e6 : n > 1e11 ? n : n * 1000).toLocaleString(
    language.value === 'zh' ? 'zh-CN' : 'en-US',
  )
}
const chartSeries = computed(() => {
  const groups = new Map()
  for (const r of [...rows.value].reverse()) {
    if (typeof r.value !== 'number') continue
    const key = `${r.name} (${r.unit || ''})`
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key).push(r.value)
  }
  return [...groups].map(([name, values]) => {
    const min = Math.min(...values),
      max = Math.max(...values)
    return {
      name,
      points: values
        .map(
          (v, i) =>
            `${20 + (i * 760) / Math.max(1, values.length - 1)},${200 - ((v - min) * 130) / Math.max(0.001, max - min)}`,
        )
        .join(' '),
    }
  })
})
async function load() {
  if (!channel.value) return
  const seq = ++sequence
  busy.value = true
  error.value = ''
  try {
    const q = new URLSearchParams({
      limit: String(limit.value),
      offset: String(offset.value),
      order: 'time',
      dir: 'desc',
    })
    if (currentView.value) for (const [k, v] of readerViewParams(currentView.value)) q.append(k, v)
    if (publisher.value) q.set('publisher', publisher.value)
    if (metric.value) q.set('name', metric.value)
    if (from.value) q.set('from', String(new Date(from.value).getTime() * 1e6))
    if (to.value) q.set('to', String(new Date(to.value).getTime() * 1e6))
    const d = await request(
      `/api/platform/readers/${workspace.value}/channels/${channel.value}/messages?${q}`,
    )
    if (seq === sequence) {
      rows.value = d.messages || []
      total.value = d.total || 0
    }
  } catch (e) {
    if (seq === sequence) error.value = errorText(e)
  } finally {
    if (seq === sequence) busy.value = false
  }
}
async function loadViews() {
  if (!channel.value) return
  try {
    const context = workspace.value + '/channels/' + channel.value
    const d = await request(`/api/platform/ui/${context}/message-views`)
    if (context !== workspace.value + '/channels/' + channel.value) return
    views.value = (d.message_views || []).sort((a, b) => (a.position || 0) - (b.position || 0))
    selectedView.value =
      views.value.find((v) => v.id === (route.query.view || selectedView.value?.id)) || null
  } catch (e) {
    error.value = errorText(e)
  }
}
function selectView(v) {
  selectedView.value = v
  const query = { ...route.query }
  if (v) query.view = v.id
  else delete query.view
  router.replace({ query })
}
function openView(method, row = { position: views.value.length }) {
  activeView.value = { record: row, removing: method === 'DELETE' }
}
const currentView = computed(() => (selectedView.value ? viewSettings(selectedView.value) : null))
const displayRows = computed(() => rows.value)
const visibleColumns = computed(
  () =>
    currentView.value?.columns.filter((c) => c.enabled) || [
      { key: 'time', label: tr('时间', 'Time') },
      { key: 'name', label: tr('指标', 'Metric') },
      { key: 'value', label: tr('数值', 'Value') },
      { key: 'unit', label: tr('单位', 'Unit') },
      { key: 'publisher', label: tr('发送设备', 'Publisher') },
    ],
)
function cell(row, key) {
  if (key === 'time' || key === 'created') return formatTime(row[key])
  if (key === 'publisher')
    return devices.value.find((d) => d.id === row.publisher)?.name || row.publisher
  const v =
    key === 'value'
      ? (row.value ?? row.string_value ?? row.bool_value ?? row.data_value)
      : viewCell(row, key)
  return v === undefined ? '—' : typeof v === 'object' ? JSON.stringify(v) : v
}
async function publish() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    if (sendType.value === 'number' && !Number.isFinite(Number(sendValue.value)))
      throw new Error(tr('请输入有效数值。', 'Enter a valid number.'))
    if (
      sendType.value === 'data' &&
      !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(
        String(sendValue.value),
      )
    )
      throw new Error(tr('请输入有效的 Base64 数据。', 'Enter valid Base64 data.'))
    const targetWorkspace = workspace.value,
      targetChannel = channel.value,
      targetDevice = sendDevice.value
    const body = payload.value.map((p) => ({ ...p, t: Date.now() / 1000 }))
    const suffix = subtopic.value.trim()
      ? '/' + subtopic.value.trim().split('/').map(encodeURIComponent).join('/')
      : ''
    const key = sendKey.value || (await deviceKey(targetDevice))
    if (targetWorkspace !== workspace.value || targetChannel !== channel.value) return
    await request(`/api/platform/http/http/m/${targetWorkspace}/c/${targetChannel}${suffix}`, {
      method: 'POST',
      auth: false,
      headers: {
        Authorization: `Basic ${btoa(targetDevice + ':' + key)}`,
        'Content-Type': 'application/senml+json',
      },
      body,
    })
    notice.value = tr(
      '平台已接收消息。请点击查询，确认规则是否已将它保存。',
      'Platform accepted the message. Reload to check whether a rule stored it.',
    )
    sendKey.value = ''
    sendOpen.value = false
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
watch(
  () => [route.params.id, props.channelId],
  async () => {
    const seq = ++sequence
    rows.value = []
    error.value = ''
    sendKey.value = ''
    try {
      const [c, d] = await Promise.all([
        runOperation('resources', { tenantId: workspace.value, kind: 'channel', limit: 100 }),
        runOperation('entities', { tenantId: workspace.value, kind: 'device', limit: 100 }),
      ])
      if (seq !== sequence) return
      channels.value = c.items
      devices.value = d.items
      channel.value = String(props.channelId || route.query.channel || '')
      if (channel.value) {
        await loadViews()
        await load()
      }
    } catch (e) {
      if (seq === sequence) error.value = errorText(e)
    }
  },
  { immediate: true },
)
watch(selectedView, () => {
  offset.value = 0
  if (channel.value) load()
})
watch(channel, () => {
  rows.value = []
  views.value = []
  activeView.value = null
  selectedView.value = null
  offset.value = 0
  loadViews()
})
watch(
  () => route.query.view,
  (id) => {
    const next = views.value.find((v) => v.id === id) || null
    if (selectedView.value?.id !== next?.id) selectedView.value = next
  },
)
function changePage(page) { const next = (page - 1) * limit.value; if (next !== offset.value) { offset.value = next; load() } }
function changeSize(size) { limit.value = size; offset.value = 0; load() }
</script>
