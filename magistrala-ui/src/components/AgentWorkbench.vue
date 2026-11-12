<template>
  <section class="workbench">
    <header class="page-heading">
      <div>
        <p class="eyebrow">AGRICULTURE · AI</p>
        <h1>{{ tr('智能工作台', 'Intelligent workbench') }}</h1>
        <p>
          {{
            tr(
              '把问题交给助手，把结论追溯到数据，把行动留给确认。',
              'Ask questions, inspect evidence, and confirm proposed actions.',
            )
          }}
        </p>
      </div>
      <button class="quiet" :disabled="busy" @click="refresh">
        {{ tr('刷新状态', 'Refresh') }}
      </button>
    </header>
    <div class="service-status">
      <span class="status-dot" :class="{ online: connected }"></span
      ><span>{{
        connected ? tr('服务已连接', 'Service connected') : tr('服务未连接', 'Service disconnected')
      }}</span
      ><span class="tag">{{ status.model || 'qwen3.5:9b' }}</span
      ><span class="tag">{{ tr('本地文本检索 RAG', 'Local lexical RAG') }}</span
      ><span class="tag">{{
        status.camera_control_registered
          ? tr('摄像头：已接入 · 阀门：模拟', 'Camera: integrated · valve: simulated')
          : tr('阀门执行：模拟', 'Valve: simulated')
      }}</span>
    </div>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <div v-if="!connected" class="notice">
      <strong>{{ tr('无法连接智能工作台服务', 'Cannot connect to the workbench service') }}</strong>
      <p>
        {{
          tr(
            '请根据上方错误检查访问权限和服务连接。如果服务尚未启动，在项目目录运行下面的命令，再刷新状态。',
            'Check the error above for access or connection issues. If the service is not running, run this command from the project directory, then refresh.',
          )
        }}
      </p>
      <code>powershell -ExecutionPolicy Bypass -File .\run-local.ps1 agent-ui</code>
    </div>
    <div v-else-if="!status.inference_enabled" class="notice">
      {{
        tr(
          '当前为无模型预览：可查看数据和检索资料，不生成模型回答。',
          'Preview mode: evidence and retrieval are available; model answers are disabled.',
        )
      }}
    </div>
    <nav class="workbench-tabs" :aria-label="tr('工作台功能', 'Workbench tools')">
      <button
        v-for="item in tabs"
        :key="item.id"
        :disabled="busy"
        :class="{ selected: tab === item.id }"
        :aria-current="tab === item.id ? 'page' : undefined"
        @click="tab = item.id"
      >
        {{ tr(item.zh, item.en) }}
      </button>
    </nav>
    <div v-if="tab === 'qa'" class="workbench-layout">
      <main class="workbench-main">
        <section class="workbench-panel chat-panel">
          <div class="panel-heading">
            <div>
              <h2>{{ tr('农业问答助手', 'Agricultural assistant') }}</h2>
            </div>
            <button type="button" class="quiet" :disabled="busy" @click="newChat">
              {{ tr('＋ 新对话', '＋ New chat') }}
            </button>
          </div>
          <template v-if="tab === 'qa'">
            <div v-if="!chatTurns.length" class="chat-welcome">
              <h3>{{ tr('今天想了解什么？', 'What would you like to explore?') }}</h3>
              <p class="muted">
                {{
                  tr(
                    '直接询问观测数据、比较不同日期，或查询项目资料；我会说明回答依据。',
                    'Ask about observations, compare dates, or search project documents, with sources.',
                  )
                }}
              </p>
              <div class="question-examples">
                <button
                  v-for="q in examples"
                  :key="q"
                  type="button"
                  class="quiet"
                  @click="useExample(q)"
                >
                  {{ q }}
                </button>
              </div>
            </div>
            <div
              v-else
              ref="transcript"
              class="chat-transcript"
              :aria-label="tr('问答对话', 'Conversation')"
              tabindex="0"
              @scroll="trackChatScroll"
            >
              <article v-for="(turn, index) in chatTurns" :key="turn.key" class="chat-turn">
                <div class="chat-question">
                  <p>{{ turn.question }}</p>
                </div>
                <div class="chat-answer">
                  <p v-if="turn.stage" role="status" class="chat-stage">
                    {{ chatStage(turn.stage) }}
                  </p>
                  <details v-if="turn.thinking" class="chat-thinking">
                    <summary>{{ tr('思考内容', 'Model thinking') }}</summary>
                    <p>{{ turn.thinking }}</p>
                  </details>
                  <template v-if="turn.run">
                    <div
                      v-for="result in visibleQueryResults(turn.run)"
                      :key="result.id"
                      class="answer-claim query-answer"
                    >
                      <p>{{ result.answer || readableEvidence(result.text) }}</p>
                    </div>
                  </template>
                  <div
                    v-for="(claim, claimIndex) in turn.claims"
                    :key="claimIndex"
                    class="answer-claim"
                  >
                    <p>{{ claim.text }}</p>
                    <template v-if="turn.run">
                      <a
                        v-for="citation in claim.citations"
                        :key="citation"
                        :href="`#chat-source-${index}-${citation}`"
                        @click.prevent="openChatSource(index, citation)"
                      >
                        {{ chatCitation(turn.run, citation) }}
                      </a>
                    </template>
                  </div>
                  <p v-if="turn.error" :role="turn.stopped ? 'status' : 'alert'" class="muted">
                    {{
                      turn.stopped
                        ? tr('已停止生成。', 'Generation stopped.')
                        : chatFallback('model_failed')
                    }}
                  </p>
                  <details v-if="turn.error && !turn.stopped" class="chat-sources">
                    <summary>{{ tr('查看失败原因', 'View error details') }}</summary>
                    <p>{{ turn.error }}</p>
                  </details>
                  <template v-if="turn.run">
                    <p v-for="note in turn.run.answer_notes || []" :key="note" class="muted">
                      {{ note }}
                    </p>
                    <p v-if="turn.run.clarification" role="status">
                      {{ turn.run.clarification }}
                    </p>
                  </template>
                  <p
                    v-if="
                      turn.run &&
                      !turn.claims.length &&
                      !turn.run.clarification &&
                      !turn.run.query_results?.length
                    "
                    role="status"
                    class="muted"
                  >
                    {{ chatFallback(turn.run.status) }}
                  </p>
                  <details
                    v-if="
                      turn.run &&
                      (turn.run.knowledge?.length ||
                        turn.run.query_results?.length ||
                        turn.run.evidence?.metrics?.length ||
                        turn.run.limitations?.length)
                    "
                    class="chat-sources"
                  >
                    <summary>{{ tr('查看依据', 'View sources') }}</summary>
                    <p v-if="turn.run.evidence" class="muted">
                      {{
                        turn.run.evidence.mode === 'history'
                          ? tr('历史回放', 'Historical replay')
                          : tr('平台查询', 'Platform query')
                      }}
                      · {{ date(turn.run.evidence.start) }} — {{ date(turn.run.evidence.end) }} ·
                      {{ turn.run.evidence.rows }} {{ tr('条记录', 'records') }}
                    </p>
                    <article
                      v-for="result in turn.run.query_results || []"
                      :id="`chat-source-${index}-${result.id}`"
                      :key="result.id"
                      class="source-card"
                    >
                      <strong>{{ result.title }}</strong>
                      <p>{{ readableEvidence(result.text) }}</p>
                      <small>{{ readableEvidence(result.source) }}</small>
                    </article>
                    <article
                      v-for="source in turn.run.knowledge"
                      :id="`chat-source-${index}-${source.id}`"
                      :key="source.id"
                      class="source-card"
                    >
                      <strong>{{ source.title }}</strong>
                      <p>{{ source.text }}</p>
                      <small>{{ source.source }} · {{ source.version }}</small>
                    </article>
                    <div
                      v-for="metric in turn.run.evidence?.metrics"
                      :id="`chat-source-${index}-${metric.id}`"
                      :key="metric.id"
                      class="source-card"
                    >
                      <strong>{{ metric.label }}</strong> · {{ number(metric.min) }}–{{
                        number(metric.max)
                      }}
                      {{ readableEvidence(metric.unit) }} · {{ metric.count }}
                      {{ tr('条', 'records') }}
                      <span v-if="metric.unit_confirmation === 'pending'">
                        · {{ tr('单位暂定', 'Provisional unit') }}</span
                      >
                      <p v-for="warning in metric.warnings" :key="warning" class="muted">
                        {{ warning }}
                      </p>
                    </div>
                    <ul v-if="turn.run.limitations?.length">
                      <li v-for="(limit, i) in turn.run.limitations" :key="i">{{ limit }}</li>
                    </ul>
                  </details>
                </div>
              </article>
            </div>
          </template>
          <label class="check-line"
            ><input v-model="includeData" type="checkbox" />{{
              tr('附上观测数据', 'Include observations')
            }}</label
          >
          <fieldset v-if="includeData" class="data-window">
            <legend>{{ tr('数据范围', 'Data scope') }}</legend>
            <label
              >{{ tr('数据来源', 'Source')
              }}<select v-model="dataMode">
                <option v-if="status.history_available" value="history">
                  {{ tr('已有气象日志 · 历史回放', 'Weather logs · historical replay') }}
                </option>
                <option value="platform">
                  {{ tr('原平台消息 · 只读查询', 'Platform messages · read only') }}
                </option>
              </select></label
            >
            <template v-if="dataMode === 'platform'"
              ><label
                >{{ tr('平台设备', 'Platform device')
                }}<select v-model="publisher">
                  <option value="">{{ tr('请选择设备', 'Choose a device') }}</option>
                  <option v-for="d in devices" :key="d.id" :value="d.id">
                    {{ d.name || d.id }}
                  </option>
                </select></label
              ><label
                >{{ tr('平台通道', 'Platform channel')
                }}<select v-model="channel">
                  <option value="">{{ tr('请选择通道', 'Choose a channel') }}</option>
                  <option v-for="c in channels" :key="c.id" :value="c.id">
                    {{ c.name || c.id }}
                  </option>
                </select></label
              ></template
            >
            <label
              >{{ tr('开始时间（本地）', 'Start (local time)')
              }}<input v-model="start" type="datetime-local" step="1" /></label
            ><label
              >{{ tr('结束时间（不含该时刻）', 'End (exclusive)')
              }}<input v-model="end" type="datetime-local" step="1"
            /></label>
            <p class="muted full-width">
              {{
                dataMode === 'history'
                  ? tr(
                      '只回放当前工作区导入的日志；接收时间不等于采样时间。',
                      'Replay is scoped to this workspace. Reception time is not sampling time.',
                    )
                  : tr(
                      '直接使用当前账号读取原平台设备与通道，不创建另一套设备。',
                      'Read existing platform devices and channels with your current account.',
                    )
              }}
            </p>
          </fieldset>
          <form class="chat-composer" @submit.prevent="sendChat">
            <label for="workbench-question">{{ tr('你的问题', 'Your question') }}</label>
            <textarea
              id="workbench-question"
              ref="composer"
              v-model="question"
              maxlength="2000"
              rows="2"
              :placeholder="
                tr('现有数据能支持哪些农业判断？', 'What conclusions can existing data support?')
              "
              required
              @keydown="composerKeydown"
            ></textarea>
            <div class="form-footer">
              <span class="muted">{{
                tr(
                  'Enter 发送 · Shift + Enter 换行',
                  'Enter to send · Shift + Enter for a new line',
                )
              }}</span>
              <button
                v-if="chatController"
                type="button"
                class="quiet"
                @click="chatController.abort()"
              >
                {{ tr('停止生成', 'Stop generating') }}
              </button>
              <button v-else :disabled="busy || !connected || !question.trim()">
                {{ tr('发送', 'Send') }}
              </button>
            </div>
          </form>
        </section>
      </main>
      <aside class="workbench-aside">
        <section class="workbench-panel">
          <h2>{{ tr('对话记录', 'Conversations') }}</h2>
          <p v-if="!conversations.length" class="muted">
            {{ tr('本工作区还没有对话。', 'No conversations in this workspace yet.') }}
          </p>
          <div v-else class="conversation-list">
            <button
              v-for="entry in conversations"
              :key="entry.id"
              type="button"
              :disabled="busy"
              class="history-item quiet"
              :class="{ 'active-conversation': tab === 'qa' && activeConversation === entry.id }"
              :aria-current="tab === 'qa' && activeConversation === entry.id ? 'true' : undefined"
              @click="openConversation(entry.id)"
            >
              <strong>{{ entry.title }}</strong>
              <small>{{ date(entry.updated_at) }}</small>
            </button>
          </div>
        </section>
        <section class="workbench-panel">
          <h2>{{ tr('平台资源', 'Platform resources') }}</h2>
          <p class="muted">
            {{
              tr(
                '设备、分组、位置与围栏沿用原平台。',
                'Devices, groups, locations and perimeters use the original platform.',
              )
            }}
          </p>
          <router-link :to="`/domain/${workspace}/devices`"
            >{{ tr('查看设备', 'View devices') }} →</router-link
          ><br /><router-link :to="`/domain/${workspace}/groups`"
            >{{ tr('查看分组', 'View groups') }} →</router-link
          >
        </section>
      </aside>
    </div>
    <AutomationWorkbench
      v-else
      :key="workspace"
      :workspace="workspace"
      :connected="connected"
      :status="status"
      :devices="devices"
      :channels="channels"
    />
  </section>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { errorText, listAll, request } from '../api/platform'
import { tr } from '../i18n'
import AutomationWorkbench from './AutomationWorkbench.vue'
const route = useRoute(),
  workspace = computed(() => String(route.params.id || ''))
const tab = ref('qa'),
  question = ref(''),
  status = ref({}),
  error = ref(''),
  busy = ref(false),
  connected = ref(false)
const conversations = ref([]),
  activeConversation = ref(''),
  devices = ref([]),
  channels = ref([])
const chatTurns = ref([])
const chatController = ref(null)
const transcript = ref(null)
const composer = ref(null)
let chatSequence = 0
let chatPinned = true
const includeData = ref(false),
  dataMode = ref('platform'),
  start = ref(''),
  end = ref(''),
  publisher = ref(''),
  channel = ref('')
let generation = 0,
  controller = new AbortController()
const tabs = [
  { id: 'qa', zh: '问答助手', en: 'Assistant' },
  { id: 'automation', zh: '自动化任务', en: 'Automation' },
]
const examples = computed(() => [
  tr('当前有哪些可以查询的观测指标？', 'Which observation metrics can I query?'),
  tr('风速一直为零就说明设备坏了吗？', 'Does zero wind speed mean sensor failure?'),
  tr('现在能根据气温自动开阀灌溉吗？', 'Can temperature trigger automatic irrigation now?'),
])
const date = (value) => (value ? new Date(value).toLocaleString() : '—')
const localInput = (value) => {
  const d = new Date(value)
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 19)
}
const number = (value) =>
  value == null ? '—' : Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })
function api(path, body) {
  return request('/api/agent/v2/workbench' + path, {
    method: body === undefined ? 'GET' : 'POST',
    body,
    headers: { 'X-Workspace-ID': workspace.value },
    signal: controller.signal,
    timeout: 620000,
  })
}
async function refresh() {
  const current = generation
  busy.value = true
  error.value = ''
  try {
    const [s, rows] = await Promise.all([api('/status'), api('/conversations')])
    if (current !== generation) return
    status.value = s
    conversations.value = rows
    connected.value = true
    if (!start.value) {
      start.value = localInput(s.first_at || Date.now() - 3600000)
      end.value = localInput(s.last_at ? new Date(s.last_at).getTime() + 1000 : Date.now())
      dataMode.value = s.history_available ? 'history' : 'platform'
    }
    const objects = await Promise.allSettled([
      listAll('entities', { tenantId: workspace.value, kind: 'device' }),
      listAll('resources', { tenantId: workspace.value, kind: 'channel' }),
    ])
    if (current !== generation) return
    devices.value = objects[0].status === 'fulfilled' ? objects[0].value : []
    channels.value = objects[1].status === 'fulfilled' ? objects[1].value : []
    if (objects.some((x) => x.status === 'rejected'))
      error.value = tr(
        '平台资源列表暂时无法读取；可使用当前工作区的历史回放。',
        'Platform resources are unavailable. Workspace replay may still be used.',
      )
  } catch (e) {
    if (current === generation) {
      connected.value = false
      error.value = errorText(e)
    }
  } finally {
    if (current === generation) busy.value = false
  }
}
function query() {
  if (
    !start.value ||
    !end.value ||
    !Number.isFinite(new Date(start.value).getTime()) ||
    !Number.isFinite(new Date(end.value).getTime())
  )
    throw new Error(tr('请选择有效时间窗口。', 'Choose a valid time window.'))
  return {
    mode: dataMode.value,
    start: new Date(start.value).toISOString(),
    end: new Date(end.value).toISOString(),
    ...(dataMode.value === 'platform'
      ? { publisher_id: publisher.value, channel_id: channel.value }
      : {}),
  }
}
async function perform(fn) {
  const current = generation
  busy.value = true
  error.value = ''
  try {
    await fn(current)
  } catch (e) {
    if (current === generation) error.value = errorText(e)
  } finally {
    if (current === generation) busy.value = false
  }
}
function chatStage(stage) {
  return {
    accepted: tr('正在接收问题…', 'Receiving your question…'),
    observations: tr('正在读取观测数据…', 'Reading observations…'),
    planning: tr('正在识别查询条件…', 'Planning the query…'),
    platform_query: tr('正在查询平台数据…', 'Querying platform data…'),
    retrieval: tr('正在检索资料…', 'Retrieving sources…'),
    compaction: tr('正在整理较早的对话…', 'Summarizing earlier conversation…'),
    generation: tr('正在生成回答…', 'Generating an answer…'),
    review: tr('正在核对依据…', 'Checking the evidence…'),
  }[stage]
}
function chatFallback(status) {
  if (status === 'time_recognizer_unavailable') {
    return tr(
      '时间解析组件不可用，请稍后重试。',
      'Time parsing is unavailable. Please try again later.',
    )
  }
  if (status === 'model_failed' || status === 'model_unavailable') {
    return tr(
      '暂时无法生成回答，请稍后重试。',
      'An answer could not be generated. Please try again shortly.',
    )
  }
  return tr(
    '暂时无法可靠回答这个问题。可以补充相关信息，或换一种问法。',
    'I cannot answer this reliably yet. Please add relevant details or rephrase the question.',
  )
}
function readableEvidence(text = '') {
  return String(text)
    .replace(/\s*\bCel\b/g, '°C')
    .replace(/平台 Reader/g, tr('平台观测记录', 'Platform observations'))
}
function chatCitation(run, id) {
  return (
    run.query_results?.find((result) => result.id === id)?.title?.split(' · ')[0] ||
    run.knowledge?.find((source) => source.id === id)?.title ||
    run.evidence?.metrics?.find((metric) => metric.id === id)?.label ||
    id
  )
}
function visibleQueryResults(run) {
  if (!run.synthesized) return run.query_results || []
  const cited = new Set((run.claims || []).flatMap((claim) => claim.citations || []))
  return (run.query_results || []).filter(
    (result) => !cited.has(result.id) && !result.summary && !result.source?.startsWith('程序计算'),
  )
}
function openChatSource(index, citation) {
  const target = document.getElementById(`chat-source-${index}-${citation}`)
  if (!target) return
  const details = target.closest('details')
  if (details) details.open = true
  target.scrollIntoView({ block: 'nearest' })
}
function trackChatScroll() {
  const el = transcript.value
  if (el) chatPinned = el.scrollHeight - el.scrollTop - el.clientHeight < 48
}
function scrollChat(force = false) {
  if (force) chatPinned = true
  nextTick(() => {
    if (chatPinned && transcript.value) transcript.value.scrollTop = transcript.value.scrollHeight
  })
}
function composerKeydown(event) {
  if (
    tab.value !== 'qa' ||
    event.key !== 'Enter' ||
    event.shiftKey ||
    event.isComposing ||
    event.keyCode === 229
  )
    return
  event.preventDefault()
  void sendChat()
}
function newChat() {
  activeConversation.value = ''
  chatTurns.value = []
  question.value = ''
  includeData.value = false
  nextTick(() => composer.value?.focus())
}
function useExample(value) {
  question.value = value
  composer.value?.focus()
}
async function sendChat() {
  if (busy.value || !connected.value || !question.value.trim()) return
  await perform(async (current) => {
    const body = { task: 'qa', question: question.value.trim() }
    const parent = chatTurns.value.findLast((turn) => turn.run)?.run.id
    if (parent) body.parent_id = parent
    if (includeData.value) body.query = query()
    const abort = new AbortController()
    chatController.value = abort
    const index = chatTurns.value.length
    chatTurns.value.push({
      key: ++chatSequence,
      question: body.question,
      claims: [],
      thinking: '',
      stage: 'accepted',
    })
    const turn = chatTurns.value[index]
    question.value = ''
    scrollChat(true)
    try {
      await request('/api/agent/v2/workbench/chat', {
        method: 'POST',
        body,
        headers: { 'X-Workspace-ID': workspace.value },
        signal: abort.signal,
        timeout: 620000,
        onRecord(event) {
          if (current !== generation || abort.signal.aborted) return
          if (event.type === 'stage') turn.stage = event.stage
          if (event.type === 'thinking') turn.thinking += event.delta
          if (event.type === 'claim') turn.claims.push(event.claim)
          if (event.type === 'error') throw new Error(event.error)
          if (event.type === 'result') {
            turn.run = event.run
            turn.claims = event.run.claims || []
            turn.stage = ''
            if (!activeConversation.value) activeConversation.value = event.run.id
          }
          scrollChat()
        },
      })
      if (!turn.run)
        throw new Error(
          tr('连接已断开，回答未完成。', 'The connection closed before the answer completed.'),
        )
      if (current === generation) {
        const rows = await api('/conversations')
        if (current === generation) conversations.value = rows
      }
    } catch (e) {
      if (current !== generation || turn.run) return
      turn.stage = ''
      turn.claims = []
      turn.stopped = abort.signal.aborted
      turn.error = turn.stopped
        ? tr('已停止生成。', 'Generation stopped.')
        : errorText(e) || chatFallback('model_failed')
      if (!question.value) question.value = body.question
      scrollChat()
    } finally {
      if (current === generation) chatController.value = null
    }
  })
}
async function openConversation(id) {
  await perform(async (current) => {
    const rows = await api('/conversations/' + encodeURIComponent(id))
    if (current !== generation) return
    tab.value = 'qa'
    activeConversation.value = id
    chatTurns.value = rows.map((row) => ({
      key: ++chatSequence,
      question: row.question,
      claims: row.claims || [],
      run: row,
    }))
    question.value = ''
    scrollChat(true)
  })
}
watch(
  workspace,
  () => {
    generation++
    chatController.value?.abort()
    chatController.value = null
    chatTurns.value = []
    controller.abort()
    controller = new AbortController()
    status.value = {}
    conversations.value = []
    activeConversation.value = ''
    devices.value = []
    channels.value = []
    start.value = ''
    end.value = ''
    publisher.value = ''
    channel.value = ''
    question.value = ''
    includeData.value = false
    connected.value = false
    refresh()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  generation++
  chatController.value?.abort()
  controller.abort()
})
</script>

<style scoped>
.panel-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}
.chat-panel {
  --chat-divider: #e5ebe5;
}
.chat-welcome {
  display: grid;
  align-content: center;
  text-align: center;
  min-height: 230px;
}
.chat-welcome h3 {
  margin-bottom: 4px;
}
.chat-welcome .question-examples {
  justify-content: center;
}
.chat-transcript {
  max-height: min(56vh, 660px);
  min-height: 160px;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 12px 8px 16px 0;
  border-bottom: 1px solid var(--chat-divider);
}
.chat-turn + .chat-turn {
  margin-top: 24px;
}
.chat-question {
  width: fit-content;
  max-width: 88%;
  margin-left: auto;
  padding: 10px 14px;
  border-radius: 14px 14px 3px 14px;
  background: #edf4ee;
}
.chat-question p,
.chat-answer p {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin: 4px 0;
}
.chat-answer {
  padding: 12px 4px 4px;
}
.chat-answer a {
  margin-left: 0.65em;
  font-size: 0.85rem;
}
.chat-stage {
  color: #52705e;
  font-size: 0.9rem;
}
.chat-thinking {
  margin: 6px 0 12px;
  padding: 6px 0 6px 12px;
  border-left: 2px solid var(--chat-divider);
  color: #52705e;
  font-size: 0.88rem;
}
.chat-thinking summary,
.chat-sources summary {
  cursor: pointer;
}
.chat-sources {
  margin-top: 10px;
  font-size: 0.88rem;
}
.chat-composer textarea {
  min-height: 70px;
}
.eyebrow {
  font-size: 11px;
  letter-spacing: 0.16em;
  color: #698275;
  font-weight: 700;
  margin: 0 0 7px;
}
.service-status {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  margin-bottom: 24px;
  font-size: 13px;
  color: #626d78;
}
.status-dot {
  height: 7px;
  width: 7px;
  border-radius: 50%;
  background: #a4aab2;
}
.status-dot.online {
  background: #3e8c6d;
}
.tag {
  padding: 5px 9px;
  background: #f2f5f4;
  border: 1px solid #e5ebe7;
  border-radius: 5px;
  font-size: 12px;
  color: #456553;
}
.workbench-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 280px;
  gap: 22px;
  align-items: start;
}
.workbench-main,
.workbench-aside {
  min-width: 0;
}
.workbench-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
  overflow-x: auto;
}
.workbench-tabs button {
  white-space: nowrap;
  background: none;
  border: 0;
  border-radius: 0;
  color: #68737f;
  padding: 12px 17px;
  border-bottom: 3px solid transparent;
}
.workbench-tabs button.selected {
  border-color: #3b775c;
  color: #28563f;
  font-weight: 600;
}
.workbench-panel {
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 24px;
  margin-bottom: 20px;
  background: #fff;
}
.workbench-panel h2 {
  font-size: 17px;
  margin: 0 0 13px;
}
.muted {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.7;
}
.notice {
  background: #f8f6ef;
  border: 1px solid #ece6d6;
  padding: 14px 16px;
  border-radius: 7px;
  font-size: 13px;
  line-height: 1.8;
  margin: 14px 0;
}
.notice code {
  white-space: normal;
  overflow-wrap: anywhere;
}
.question-examples {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  margin: 20px 0;
}
.question-examples button {
  font-size: 12px;
  text-align: left;
  background: #f8faf9;
}
.workbench label {
  display: block;
  font-size: 13px;
  color: #505b65;
  margin-bottom: 9px;
}
.workbench textarea {
  margin-top: 7px;
  resize: vertical;
  line-height: 1.7;
}
.check-line {
  display: flex !important;
  gap: 9px;
  align-items: flex-start;
  margin: 16px 0 !important;
  line-height: 1.6;
}
.check-line input {
  margin-top: 4px;
}
.form-footer {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  margin-top: 12px;
}
.form-footer button {
  flex-shrink: 0;
  background: #315f49;
  border-color: #315f49;
}
.data-window {
  border: 1px solid var(--border);
  border-radius: 7px;
  padding: 16px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 18px 0;
}
.data-window legend {
  font-size: 13px;
  color: #53635a;
  padding: 0 6px;
}
.data-window select {
  width: 100%;
  margin-top: 7px;
}
.full-width {
  grid-column: 1/-1;
}
.data-window .full-width {
  margin: 0;
}
.history-item {
  display: block;
  width: 100%;
  text-align: left;
  border: 0;
  border-bottom: 1px solid #edf0ed;
  border-radius: 0;
  padding: 11px 0;
}
.history-item strong {
  font-size: 12px;
  font-weight: 500;
  display: block;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.history-item small {
  font-size: 10px;
  color: #8b9199;
  display: block;
  margin-top: 6px;
}
.conversation-list {
  max-height: min(56vh, 34rem);
  overflow-y: auto;
}
.history-item.active-conversation {
  background: #edf4ee;
}
.workbench-aside .workbench-panel {
  padding: 20px;
}
.workbench-aside a {
  font-size: 12px;
  line-height: 2;
  color: #3d6b51;
}
.answer-claim {
  padding: 16px 0;
  border-bottom: 1px solid #eef0f2;
}
.answer-claim p {
  line-height: 1.85;
  font-size: 14px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin: 9px 0;
}
.workbench details {
  margin-top: 18px;
  font-size: 12px;
}
.workbench summary {
  cursor: pointer;
  color: #56735f;
  font-weight: 500;
}
.workbench code {
  overflow-wrap: anywhere;
  font-size: 10px;
  display: block;
  max-width: 100%;
}
.source-card {
  margin: 12px 0;
  padding: 14px;
  background: #f8faf8;
  border-radius: 6px;
  line-height: 1.8;
}
.source-card p {
  margin: 7px 0;
}
.source-card small,
.source-card code {
  display: block;
  color: #89918b;
}
@media (max-width: 1200px) {
  .workbench-layout {
    grid-template-columns: minmax(0, 1fr) 240px;
  }
}
@media (max-width: 900px) {
  .workbench-layout {
    grid-template-columns: 1fr;
  }
  .workbench-aside {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
  }
  .workbench-aside .workbench-panel {
    margin-bottom: 0;
  }
  .workbench-panel {
    padding: 18px;
  }
}
@media (max-width: 600px) {
  .workbench-aside,
  .data-window {
    grid-template-columns: minmax(0, 1fr);
  }
  .form-footer {
    align-items: flex-start;
    flex-direction: column;
  }
  .workbench-tabs button {
    padding: 12px 10px;
    font-size: 12px;
  }
  .service-status {
    gap: 8px;
  }
}
</style>
