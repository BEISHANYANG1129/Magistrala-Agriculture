<template>
  <section>
    <header class="page-heading">
      <div>
        <p class="eyebrow">REGIONAL INTELLIGENCE</p>
        <h1>{{ tr('区域与智能分析', 'Regions & AI analysis') }}</h1>
        <p>
          {{
            tr(
              '每次分析只使用所选区域的数据，控制建议只对应这个区域的设备。',
              'Each analysis uses only the selected region and proposes actions for its own devices.',
            )
          }}
        </p>
      </div>
      <div class="actions">
        <button class="quiet" :disabled="busy" @click="refresh">{{ tr('刷新', 'Refresh') }}</button
        ><button :disabled="busy || !registry.editable" @click="editRegion()">
          + {{ tr('新建区域', 'New region') }}
        </button>
      </div>
    </header>
    <div class="panel">
      <span class="badge">{{ modeText }}</span>
      <span class="badge">{{ tr('设备执行：仅模拟', 'Device execution: simulation only') }}</span>
      <p class="muted">
        {{
          tr(
            '真实模型模式会在每次推理前检查 GPU；占用时不会抢占其他任务。',
            'Real-model mode checks GPU availability before each inference and does not interrupt other tasks.',
          )
        }}
      </p>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success" role="status">{{ notice }}</p>
    <p v-if="busy && !connected" role="status">{{tr('正在连接区域服务…','Connecting to the regional service…')}}</p>
    <div v-if="!connected && !busy" class="panel">
      <h2>{{ tr('连接区域分析服务', 'Connect the regional analysis service') }}</h2>
      <p>
        {{
          tr(
            '请在项目目录启动 agent-ui 服务，然后点击刷新。',
            'Start the agent-ui service from the project directory, then refresh.',
          )
        }}
      </p>
      <code>powershell -ExecutionPolicy Bypass -File .\run-local.ps1 agent-ui</code>
    </div>
    <div class="split" v-if="connected">
      <div class="panel">
        <h2>{{ tr('农田区域', 'Field regions') }}</h2>
        <p v-if="!regions.length">
          {{
            tr(
              '当前工作区还没有区域，请先创建。',
              'This workspace has no regions. Create one to begin.',
            )
          }}
        </p>
        <button
          v-for="r in regions"
          :key="r.id"
          class="region-button quiet"
          :class="{ selected: r.id === selectedId }"
          :disabled="busy"
          @click="select(r.id)"
        >
          <strong>{{ r.name }}</strong
          ><small
            >{{ r.sensors.length }} {{ tr('个传感器', 'sensors') }} · {{ r.executors.length }}
            {{ tr('个执行器', 'actuators') }}</small
          >
        </button>
      </div>
      <div v-if="current" class="panel">
        <div class="actions">
          <h2>{{ current.name }}</h2>
          <button class="quiet" :disabled="busy || !registry.editable" @click="editRegion(current)">
            {{ tr('编辑区域', 'Edit region') }}
          </button>
        </div>
        <dl class="record">
          <dt>{{ tr('区域编号', 'Region ID') }}</dt>
          <dd>{{ current.id }}</dd>
          <dt>{{ tr('低湿度阈值', 'Low-moisture threshold') }}</dt>
          <dd>{{ current.soil_moisture_min ?? tr('未配置', 'Not configured') }} %</dd>
          <dt>{{ tr('模拟灌溉时长', 'Simulated irrigation') }}</dt>
          <dd>
            {{ current.irrigation_duration_min ?? tr('未配置', 'Not configured') }}
            {{ tr('分钟', 'minutes') }}
          </dd>
          <dt>{{ tr('传感器', 'Sensors') }}</dt>
          <dd>
            <div v-for="s in current.sensors" :key="s.publisher_id + s.channel_id">
              {{ deviceName(s.publisher_id) }}<small>{{ channelName(s.channel_id) }}</small>
            </div>
          </dd>
          <dt>{{ tr('执行设备', 'Actuators') }}</dt>
          <dd>{{ current.executors.map(deviceName).join(', ') || tr('未绑定', 'None') }}</dd>
        </dl>
        <button :disabled="busy" @click="analyze">
          {{ busy ? tr('处理中…', 'Working…') : tr('分析这个区域', 'Analyze this region') }}
        </button>
      </div>
    </div>
    <div v-if="result" class="panel">
      <div class="actions">
        <h2>{{ tr('分析结果', 'Analysis result') }}</h2>
        <span class="badge">{{ statusText(result.status) }}</span>
      </div>
      <small>{{ result.analysis_id }} · {{ new Date(result.created_at).toLocaleString() }}</small>
      <h3>{{ tr('本次使用的数据', 'Evidence used') }}</h3>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{{ tr('指标', 'Metric') }}</th>
              <th>{{ tr('数值', 'Value') }}</th>
              <th>{{ tr('设备', 'Device') }}</th>
              <th>{{ tr('观测时间', 'Observed at') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in result.evidence" :key="e.id">
              <td>{{ e.metric }}</td>
              <td>{{ e.value }} {{ e.unit }}</td>
              <td>{{ deviceName(e.publisher_id) }}</td>
              <td>{{ new Date(e.observed_at).toLocaleString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <h3>{{ tr('分析结论', 'Findings') }}</h3>
      <template v-for="(a, i) in result.assessments" :key="i"
        ><p v-for="(f, j) in a.findings" :key="j">
          <strong>{{ statusText(f.kind) }}</strong> — {{ f.explanation
          }}<small>{{ tr('数据依据', 'Evidence') }}: {{ f.evidence_ids.join(', ') }}</small>
        </p></template
      >
      <details>
        <summary>{{ tr('知识来源与处理记录', 'Knowledge sources & processing trace') }}</summary>
        <p v-for="k in result.knowledge" :key="k.id">{{ k.source }} · {{ k.text }}</p>
        <ol>
          <li v-for="(t, i) in result.trace" :key="i">{{ t.step }} — {{ t.detail }}</li>
        </ol>
      </details>
      <h3>{{ tr('控制建议', 'Control proposals') }}</h3>
      <p v-if="!result.tasks?.length">
        {{ tr('没有符合执行条件的建议。', 'No eligible action proposed.') }}
      </p>
      <div v-for="t in result.tasks" :key="t.task_id" class="panel">
        <strong>{{ tr('灌溉建议', 'Irrigation proposal') }}</strong>
        <p>{{ tr('执行设备', 'Actuators') }}: {{ t.executor_ids.map(deviceName).join(', ') }}</p>
        <pre>{{ JSON.stringify(t.params, null, 2) }}</pre>
      </div>
      <button v-if="result.tasks?.length" :disabled="busy || !!simulation" @click="simulate">
        {{ tr('模拟执行建议', 'Simulate proposed actions') }}
      </button>
      <p v-if="simulation" class="success">{{ statusText(simulation.status) }}</p>
      <div v-for="receipt in simulation?.receipts || []" :key="receipt.task_id">
        <strong>{{ deviceName(receipt.device_id) }}</strong> · {{ statusText(receipt.status) }}
        <p>{{ receipt.detail }}</p>
      </div>
    </div>
    <div class="panel" v-if="connected">
      <h2>{{ tr('区域分析历史', 'Region analysis history') }}</h2>
      <p v-if="!history.length">{{ tr('暂无分析记录。', 'No analyses yet.') }}</p>
      <div v-for="h in history" :key="h.result.analysis_id" class="toolbar">
        <button
          class="quiet"
          :disabled="busy"
          @click="() => { result = h.result; simulation = h.simulation }"
        >
          {{ new Date(h.result.created_at).toLocaleString() }} ·
          {{ statusText(h.result.status) }}</button
        ><span>{{
          h.simulation ? statusText(h.simulation.status) : tr('未模拟执行', 'Not simulated')
        }}</span>
      </div>
    </div>
    <ModalDialog v-if="editing" :busy="busy" @close="editing = false">
      <form class="modal-card" @submit.prevent="save">
        <h2>{{ tr('区域配置', 'Region configuration') }}</h2>
        <div class="form-grid">
          <label class="field"
            >{{ tr('名称', 'Name') }}<input v-model="draft.name" required /></label
          ><label class="field"
            >{{ tr('区域编号', 'Region ID')
            }}<input v-model="draft.id" :readonly="!isNew" required /></label
          ><label class="field"
            >{{ tr('低湿度阈值（%）', 'Low-moisture threshold (%)')
            }}<input
              type="number"
              min="0"
              max="100"
              step="any"
              v-model="draft.soil_moisture_min" /></label
          ><label class="field"
            >{{ tr('模拟时长（分钟）', 'Simulation duration (minutes)')
            }}<input
              type="number"
              min="0.01"
              max="60"
              step="any"
              v-model="draft.irrigation_duration_min"
          /></label>
        </div>
        <h3>{{ tr('传感器绑定', 'Sensor bindings') }}</h3>
        <div v-for="(s, i) in draft.sensors" :key="i" class="array-row">
          <select v-model="s.channel_id" required>
            <option value="">{{ tr('选择通道', 'Select channel') }}</option>
            <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option></select
          ><select v-model="s.publisher_id" required>
            <option value="">{{ tr('选择设备', 'Select device') }}</option>
            <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option></select
          ><button type="button" class="quiet" @click="draft.sensors.splice(i, 1)">
            {{ tr('移除', 'Remove') }}
          </button>
        </div>
        <button
          type="button"
          class="quiet"
          @click="draft.sensors.push({ channel_id: '', publisher_id: '' })"
        >
          + {{ tr('添加传感器', 'Add sensor') }}
        </button>
        <h3>{{ tr('执行设备', 'Actuators') }}</h3>
        <div v-for="(id, i) in draft.executors" :key="i" class="array-row">
          <input v-model="draft.executors[i]" list="agent-devices" required /><button
            type="button"
            class="quiet"
            @click="draft.executors.splice(i, 1)"
          >
            {{ tr('移除', 'Remove') }}
          </button>
        </div>
        <datalist id="agent-devices">
          <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option></datalist
        ><button type="button" class="quiet" @click="draft.executors.push('')">
          + {{ tr('添加执行设备', 'Add actuator') }}
        </button>
        <p class="muted">
          {{
            tr(
              '执行仍为模拟。模拟设备可使用 simulation-only- 开头的编号。',
              'Execution remains simulated. Virtual device IDs may start with simulation-only-.',
            )
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button :disabled="busy">{{ tr('保存', 'Save') }}</button
          ><button type="button" class="quiet" @click="editing = false">
            {{ tr('取消', 'Cancel') }}
          </button>
        </div>
      </form>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { request, runOperation, errorText } from '../api/platform'
import { tr } from '../i18n'
const route = useRoute(),
  workspace = computed(() => String(route.params.id)),
  registry = ref({ regions: [] }),
  health = ref({}),
  selectedId = ref(''),
  connected = ref(false),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  result = ref(null),
  simulation = ref(null),
  history = ref([]),
  devices = ref([]),
  channels = ref([]),
  editing = ref(false),
  draft = ref({}),
  isNew = ref(false)
let sequence = 0
const regions = computed(() =>
    registry.value.regions.filter((r) => r.workspace_id === workspace.value),
  ),
  current = computed(() => regions.value.find((r) => r.id === selectedId.value))
const modeText = computed(() =>
  health.value.mode === 'ollama'
    ? tr('平台数据 + 真实模型', 'Platform data + real model')
    : health.value.mode === 'reader-demo'
      ? tr('平台数据 + 模拟分析', 'Platform data + simulated analysis')
      : tr('演示数据 + 模拟分析', 'Demo data + simulated analysis'),
)
function statusText(s) {
  const labels = {
    proposal_ready: ['建议已生成', 'Proposal ready'],
    analyzed: ['分析完成', 'Analyzed'],
    insufficient_data: ['数据不足或已过期', 'Missing or stale data'],
    soil_dry: ['低于湿度阈值', 'Below moisture threshold'],
    normal: ['未低于配置阈值', 'Not below configured threshold'],
    observation: ['观察记录', 'Observation'],
    uncertain: ['无法确定', 'Uncertain'],
    simulated_completed: ['模拟执行完成', 'Simulation completed'],
    completed: ['完成', 'Completed'],
    failed: ['失败', 'Failed'],
    unknown: ['状态不确定', 'Unknown state'],
  }
  return labels[s] ? tr(...labels[s]) : s
}
function deviceName(id) {
  return devices.value.find((d) => d.id === id)?.name || id
}
function channelName(id) {
  return channels.value.find((c) => c.id === id)?.name || id
}
function api(path, body) {
  return request('/api/agent' + path, {
    method: body ? 'POST' : 'GET',
    body,
    headers: { 'X-Workspace-ID': workspace.value },
    timeout: 150000,
  })
}
async function refresh() {
  const seq = ++sequence
  busy.value = true
  error.value = ''
  try {
    const [r, h, d, c] = await Promise.all([
      api('/v1/regions'),
      api('/health'),
      runOperation('entities', { tenantId: workspace.value, kind: 'device', limit: 100 }),
      runOperation('resources', { tenantId: workspace.value, kind: 'channel', limit: 100 }),
    ])
    if (seq !== sequence) return
    registry.value = r
    health.value = h
    devices.value = d.items
    channels.value = c.items
    connected.value = true
    if (!current.value) selectedId.value = regions.value[0]?.id || ''
    await loadHistory()
  } catch (e) {
    if (seq === sequence) {
      error.value = errorText(e)
      connected.value = false
    }
  } finally {
    if (seq === sequence) busy.value = false
  }
}
async function loadHistory() {
  const scope = `${workspace.value}/${selectedId.value}`
  const rows = await api(
    `/v1/history?workspace_id=${encodeURIComponent(workspace.value)}${selectedId.value ? '&region_id=' + encodeURIComponent(selectedId.value) : ''}`,
  )
  if (scope === `${workspace.value}/${selectedId.value}`) history.value = rows
}
async function select(id) {
  selectedId.value = id
  result.value = null
  simulation.value = null
  try {
    await loadHistory()
  } catch (e) {
    error.value = errorText(e)
  }
}
async function analyze() {
  const generation = sequence
  busy.value = true
  error.value = ''
  notice.value = ''
  result.value = null
  simulation.value = null
  try {
    const response = await api('/v1/analyze', {
      workspace_id: workspace.value,
      region_id: current.value.id,
      mapping_version: registry.value.version,
    })
    if (generation !== sequence) return
    result.value = response
    notice.value = tr(
      '分析已完成，未发送真实设备命令。',
      'Analysis completed. No physical device command was sent.',
    )
    await loadHistory()
  } catch (e) {
    if (generation === sequence) error.value = errorText(e)
  } finally {
    if (generation === sequence) busy.value = false
  }
}
async function simulate() {
  const generation = sequence
  busy.value = true
  error.value = ''
  try {
    const response = await api('/v1/simulate', { analysis_id: result.value.analysis_id })
    if (generation !== sequence) return
    simulation.value = response
    await loadHistory()
  } catch (e) {
    if (generation === sequence) error.value = errorText(e)
  } finally {
    if (generation === sequence) busy.value = false
  }
}
function editRegion(region) {
  isNew.value = !region
  draft.value = region
    ? JSON.parse(JSON.stringify(region))
    : {
        id: '',
        name: '',
        workspace_id: workspace.value,
        sensors: [{ channel_id: '', publisher_id: '' }],
        executors: [],
      }
  editing.value = true
  error.value = ''
}
async function save() {
  const generation = sequence
  busy.value = true
  error.value = ''
  try {
    const r = JSON.parse(JSON.stringify(draft.value))
    r.workspace_id = workspace.value
    for (const key of ['soil_moisture_min', 'irrigation_duration_min']) {
      if (r[key] === '' || r[key] === null || r[key] === undefined) delete r[key]
      else r[key] = Number(r[key])
    }
    const next = JSON.parse(JSON.stringify(regions.value))
    const index = next.findIndex((x) => x.id === r.id)
    if (isNew.value && index >= 0)
      throw new Error(tr('区域编号已存在。', 'Region ID already exists.'))
    if (index >= 0) next[index] = r
    else next.push(r)
    const response = await api('/v1/regions', {
      expected_revision: registry.value.revision,
      registry: { version: registry.value.version, regions: next },
    })
    if (generation !== sequence) return
    registry.value = response
    selectedId.value = r.id
    editing.value = false
    result.value = null
    simulation.value = null
    notice.value = tr(
      '区域配置已保存，请使用新配置重新分析。',
      'Region saved. Analyze again using the new configuration.',
    )
    await loadHistory()
  } catch (e) {
    if (generation === sequence) error.value = errorText(e)
  } finally {
    if (generation === sequence) busy.value = false
  }
}
watch(
  workspace,
  () => {
    result.value = null
    simulation.value = null
    history.value = []
    editing.value = false
    selectedId.value = ''
    refresh()
  },
  { immediate: true },
)
</script>
<style scoped>
.region-button {
  display: block;
  width: 100%;
  text-align: left;
  margin: 9px 0;
}
.region-button.selected {
  background: #e3efe6;
  border-color: #75a48b;
}
</style>
