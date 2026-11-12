<template>
  <div class="automation">
    <p v-if="error" role="alert" class="automation-error">{{ error }}</p>
    <div class="automation-grid">
      <section class="automation-panel">
        <div class="automation-heading">
          <h2>
            {{ resuming ? tr('补充任务资料', 'Complete task inputs') : tr('新建任务', 'New task') }}
          </h2>
          <button
            v-if="resuming"
            type="button"
            class="quiet"
            :disabled="submitting"
            @click="resetForm"
          >
            {{ tr('新建另一条任务', 'Start a different task') }}
          </button>
        </div>
        <div v-if="status.camera_control_registered" class="material camera-available">
          <strong>{{ tr('已接入摄像头任务', 'Camera tasks are integrated') }}</strong>
          <p class="muted">
            {{
              tr(
                '在任务目标中直接交代转动、截图，以及是否分析画面。系统会先列出完整计划，确认一次后自动继续；截图保存在同一条任务里。',
                'Describe movement, capture, and whether to analyze the image in your goal. Review and confirm the complete plan once; the system continues automatically and keeps captures in the same task.',
              )
            }}
          </p>
        </div>
        <form @submit.prevent="submit">
          <label>
            {{ tr('任务目标', 'Task goal') }}
            <textarea
              v-model="goal"
              :readonly="Boolean(resuming) && !resumeNeeds.includes('goal')"
              required
              maxlength="2000"
              rows="3"
              :placeholder="
                tr(
                  '例如：让乔安摄像头向左微调，截一张新照片，并说明画面里有什么。',
                  'For example: Summarize imported weather logs and inspect the latest uploaded leaf photo.',
                )
              "
            ></textarea>
          </label>
          <label v-if="resuming">
            {{ tr('补充说明', 'Additional instructions') }}
            <textarea
              v-model="clarification"
              maxlength="2000"
              rows="2"
              :placeholder="
                tr(
                  '直接回答待确认的问题，也可以用自然语言补充时间、对象或动作。',
                  'Answer the pending question, or describe the time, target or action in your own words.',
                )
              "
            ></textarea>
          </label>
          <h3>{{ tr('补充资料（可选）', 'Additional materials (optional)') }}</h3>
          <p class="muted">
            {{
              tr(
                '系统先按目标查找资料。你也可以指定时段或上传照片。',
                'The task searches for evidence first. You can also supply a window or photo.',
              )
            }}
          </p>
          <details :open="windowOpen" class="material">
            <summary>
              {{ tr('观测时段', 'Observation window') }}
              <small v-if="useWindow">{{ tr('已附', 'Attached') }}</small>
            </summary>
            <label class="check">
              <input v-model="useWindow" type="checkbox" :disabled="environmentSaved" />
              {{ tr('附上观测时段', 'Attach an observation window') }}
            </label>
            <fieldset v-if="useWindow" :disabled="environmentSaved">
              <label>
                {{ tr('数据来源', 'Data source') }}
                <select v-model="mode">
                  <option v-if="status.history_available" value="history">
                    {{ tr('历史气象日志', 'Historical weather logs') }}
                  </option>
                  <option value="platform">{{ tr('原平台消息', 'Platform messages') }}</option>
                </select>
              </label>
              <p v-if="mode === 'platform' && station" class="muted">
                {{ tr('使用已匹配的气象站', 'Using matched station') }} · {{ station }}
              </p>
              <div v-if="mode === 'platform' && !station" class="fields">
                <label>
                  {{ tr('设备', 'Device') }}
                  <select v-model="publisher" required>
                    <option value="">{{ tr('请选择设备', 'Choose a device') }}</option>
                    <option v-for="d in devices" :key="d.id" :value="d.id">
                      {{ d.name || d.id }}
                    </option>
                  </select>
                </label>
                <label>
                  {{ tr('通道', 'Channel') }}
                  <select v-model="channel" required>
                    <option value="">{{ tr('请选择通道', 'Choose a channel') }}</option>
                    <option v-for="c in channels" :key="c.id" :value="c.id">
                      {{ c.name || c.id }}
                    </option>
                  </select>
                </label>
              </div>
              <div class="fields">
                <label
                  >{{ tr('开始时间', 'Start')
                  }}<input v-model="start" type="datetime-local" step="1" required
                /></label>
                <label
                  >{{ tr('结束时间（不含）', 'End (exclusive)')
                  }}<input v-model="end" type="datetime-local" step="1" required
                /></label>
              </div>
              <label class="check">
                <input v-model="compareEnabled" type="checkbox" />
                {{ tr('附上对照时段', 'Attach a comparison window') }}
              </label>
              <div v-if="compareEnabled" class="fields">
                <label
                  >{{ tr('时段 B 开始', 'Window B start')
                  }}<input v-model="compareStart" type="datetime-local" step="1" required
                /></label>
                <label
                  >{{ tr('时段 B 结束', 'Window B end')
                  }}<input v-model="compareEnd" type="datetime-local" step="1" required
                /></label>
              </div>
            </fieldset>
          </details>
          <details :open="imageOpen" class="material">
            <summary>
              {{ tr('叶片照片', 'Leaf photo') }}
              <small v-if="image || savedImage">{{ tr('已附', 'Attached') }}</small>
            </summary>
            <fieldset :disabled="imageSaved">
              <label
                >{{ tr('上传照片（PNG/JPEG，最大 2 MiB）', 'Upload PNG/JPEG, up to 2 MiB') }}
                <input
                  :key="fileKey"
                  type="file"
                  accept="image/png,image/jpeg"
                  @change="attachImage"
                />
              </label>
              <div v-if="image" class="fields">
                <label
                  >{{ tr('图片来源', 'Image source') }}
                  <select v-model="imageSource">
                    <option value="user_upload">{{ tr('自己上传', 'User upload') }}</option>
                    <option value="public_dataset">{{ tr('公开数据集', 'Public dataset') }}</option>
                  </select>
                </label>
                <label
                  >{{ tr('作物', 'Crop') }}
                  <select v-model="crop">
                    <option value="">{{ tr('未知', 'Unknown') }}</option>
                    <option value="coffee">{{ tr('咖啡', 'Coffee') }}</option>
                    <option value="cotton">{{ tr('棉花', 'Cotton') }}</option>
                    <option value="other">{{ tr('其他', 'Other') }}</option>
                  </select>
                </label>
              </div>
            </fieldset>
          </details>
          <label v-if="resumeNeeds.includes('duration')" class="duration">
            {{ tr('模拟开启时长（秒）', 'Simulated open duration (seconds)') }}
            <input v-model.number="duration" type="number" min="1" max="300" />
          </label>
          <div class="automation-footer">
            <small>{{
              tr('系统按任务目标安排步骤', 'The system plans the steps from your goal')
            }}</small>
            <button
              class="run-task"
              :disabled="
                submitting ||
                attaching ||
                !connected ||
                !goal.trim() ||
                ['running', 'camera_running'].includes(selected?.task.status)
              "
            >
              {{ resuming ? tr('继续任务', 'Resume task') : tr('运行任务', 'Run task') }}
            </button>
          </div>
        </form>
      </section>

      <section class="automation-panel" aria-live="polite">
        <h2>{{ tr('任务进度', 'Task progress') }}</h2>
        <p v-if="!selected" class="muted">
          {{ tr('运行任务或选择一条历史记录。', 'Run a task or select a saved task.') }}
        </p>
        <template v-else>
          <div class="automation-heading">
            <strong>{{ selected.task.question }}</strong>
            <span class="state" :data-state="selected.task.status">{{
              statusText(selected.task.status)
            }}</span>
          </div>
          <div v-if="selected.task.automation.goal_plan" class="result-summary understood-task">
            <h3>{{ tr('已理解的任务', 'Understood task') }}</h3>
            <p>{{ selected.task.automation.goal_plan.summary }}</p>
            <template v-if="selected.task.automation.goal_plan.execution">
              <p>
                {{ tr('请求对象', 'Requested target') }} ·
                {{
                  selected.task.automation.goal_plan.execution.target || tr('待明确', 'Unspecified')
                }}
                <br />{{ tr('动作', 'Action') }} ·
                {{ actionText(selected.task.automation.goal_plan.execution.action) }}
                <template
                  v-if="selected.task.automation.goal_plan.execution.duration_seconds != null"
                >
                  · {{ selected.task.automation.goal_plan.execution.duration_seconds }} s
                </template>
                <br />{{ tr('请求模式', 'Requested mode') }} ·
                {{
                  selected.task.automation.goal_plan.execution.mode === 'simulation'
                    ? tr('模拟', 'Simulation')
                    : tr('真实操作', 'Physical operation')
                }}
              </p>
              <p v-if="selected.task.automation.execution_mode === 'simulation'" class="muted">
                {{
                  tr(
                    '本任务已选择改为模拟，执行对象为通用虚拟阀门。',
                    'This task was switched to simulation using the generic virtual valve.',
                  )
                }}
              </p>
            </template>
          </div>
          <CameraTaskPanel
            v-if="selected.task.automation.camera"
            :key="selected.task.id"
            :task="selected.task"
            :workspace="workspace"
            @refresh="selectTask(selected.task.id)"
          />
          <ol class="steps">
            <li v-for="(row, index) in stepRows" :key="index" :class="{ branches: row.length > 1 }">
              <div v-for="step in row" :key="step.role" class="step">
                <span class="step-dot" :data-state="step.status" aria-hidden="true">{{
                  step.status === 'completed' ? '✓' : '•'
                }}</span>
                <div>
                  <strong>{{ roleText(step.role) }}</strong>
                  <small>{{ step.detail || statusText(step.status) }}</small>
                </div>
              </div>
            </li>
          </ol>
          <div
            v-if="
              selected.task.automation.observation_source || selected.task.automation.photo_source
            "
            class="result-summary"
          >
            <h3>{{ tr('自动找到的资料', 'Automatically selected evidence') }}</h3>
            <p v-if="selected.task.automation.observation_source">
              <strong>{{ selected.task.automation.observation_source.name }}</strong>
              <br />
              {{ date(selected.task.automation.query?.start) }} –
              {{ date(selected.task.automation.query?.end) }}
              <small v-if="selected.task.automation.observation_source.publisher_id"
                ><br />{{ tr('设备', 'Device') }} ·
                {{ deviceName(selected.task.automation.observation_source.publisher_id) }}</small
              >
              <small v-if="selected.task.automation.observation_source.channel_id"
                ><br />{{ tr('通道', 'Channel') }} ·
                {{ channelName(selected.task.automation.observation_source.channel_id) }}</small
              >
            </p>
            <p v-if="selected.task.automation.photo_source">
              <strong>{{ selected.task.automation.photo_source.name }}</strong>
              <small v-if="selected.task.automation.photo_source.message_time"
                ><br />{{ tr('平台消息时间', 'Platform message time') }} ·
                {{ date(selected.task.automation.photo_source.message_time) }}</small
              >
              <small v-if="selected.task.automation.photo_source.channel_id"
                ><br />{{ tr('通道', 'Channel') }} ·
                {{ channelName(selected.task.automation.photo_source.channel_id) }}</small
              >
            </p>
          </div>
          <ul v-if="selected.task.automation.needs?.length" class="needs">
            <li v-for="need in selected.task.automation.needs" :key="need">{{ needText(need) }}</li>
          </ul>
          <button
            v-if="
              selected.task.status === 'needs_input' &&
              selected.task.automation.needs?.includes('execution_mode')
            "
            type="button"
            :disabled="submitting"
            @click="switchToSimulation"
          >
            {{ tr('改为模拟', 'Switch to simulation') }}
          </button>
          <button
            v-if="
              ([
                'needs_input',
                'interrupted',
                'planner_failed',
                'waiting_camera_confirmation',
              ].includes(selected.task.status) ||
                canReprepareCamera) &&
              resuming !== selected.task.id
            "
            type="button"
            class="quiet"
            @click="resumeForm"
          >
            {{
              selected.task.status === 'planner_failed'
                ? tr('重试任务规划', 'Retry task planning')
                : tr('补充资料或继续任务', 'Provide inputs or resume')
            }}
          </button>
          <div
            v-if="selected.reports.length || selected.task.limitations?.length"
            class="result-summary"
          >
            <h3>{{ tr('结果摘要', 'Results') }}</h3>
            <div v-for="report in selected.reports" :key="report.id">
              <strong>{{ roleText(report.task) }}</strong>
              <p v-for="(claim, i) in report.claims?.slice(0, 2)" :key="i">{{ claim.text }}</p>
              <p v-if="!report.claims?.length" class="muted">{{ statusText(report.status) }}</p>
            </div>
            <p v-for="limit in selected.task.limitations" :key="limit" class="muted">{{ limit }}</p>
          </div>
          <details v-if="selected.reports.length" class="material">
            <summary>{{ tr('查看完整报告', 'View complete report') }}</summary>
            <AutomationReport
              v-for="report in selected.reports"
              :key="report.id"
              :report="report"
            />
          </details>
          <div v-if="selected.task.plan" class="result-summary">
            <h3>{{ tr('阀门模拟草案', 'Valve simulation draft') }}</h3>
            <p>
              {{ tr('通用虚拟阀门', 'Generic virtual valve') }} ·
              {{
                selected.task.plan.action === 'open'
                  ? tr('模拟开启后关闭', 'Simulate open then close')
                  : tr('模拟关闭', 'Simulate close')
              }}
              · {{ selected.task.plan.duration_seconds }} s
            </p>
            <p class="muted">
              {{ tr('草案有效期至', 'Draft valid until') }}
              {{ date(selected.task.plan.expires_at) }}
            </p>
            <template v-if="selected.task.status === 'waiting_confirmation'">
              <label class="check">
                <input v-model="confirm" type="checkbox" />
                {{
                  tr(
                    '我已核对具体动作、时长和关闭步骤，同意模拟。',
                    'I confirm this exact simulated action, duration and closing step.',
                  )
                }}
              </label>
              <button type="button" :disabled="!confirm || submitting" @click="confirmSimulation">
                {{ tr('确认并模拟执行', 'Confirm simulation') }}
              </button>
            </template>
            <p v-if="selected.task.receipt">{{ selected.task.receipt }}</p>
            <p v-if="selected.task.execution_feedback" class="muted">
              <strong>{{ tr('模拟适配器回读', 'Simulator readback') }}</strong> ·
              {{
                selected.task.execution_feedback.transitions
                  .map(
                    (item) =>
                      item.offset_seconds +
                      ' s ' +
                      (item.state === 'open' ? tr('开启', 'open') : tr('关闭', 'closed')),
                  )
                  .join(' → ')
              }}
            </p>
          </div>
        </template>
      </section>
    </div>

    <section class="automation-panel">
      <h2>{{ tr('任务记录', 'Task history') }}</h2>
      <p v-if="!tasks.length" class="muted">
        {{ tr('暂无自动化任务。', 'No automation tasks yet.') }}
      </p>
      <div v-else class="task-list">
        <button
          v-for="task in tasks"
          :key="task.id"
          type="button"
          class="task-row quiet"
          :aria-current="selected?.task.id === task.id ? 'true' : undefined"
          @click="selectTask(task.id)"
        >
          <span>{{ task.goal }}</span>
          <small>{{ date(task.updated_at) }}</small>
          <span class="state" :data-state="task.status">{{ statusText(task.status) }}</span>
        </button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { errorText, request } from '../api/platform'
import { tr } from '../i18n'
import AutomationReport from './AutomationReport.vue'
import CameraTaskPanel from './CameraTaskPanel.vue'

const props = defineProps({
  workspace: { type: String, required: true },
  connected: Boolean,
  status: { type: Object, required: true },
  devices: { type: Array, required: true },
  channels: { type: Array, required: true },
})
const tasks = ref([]),
  selected = ref(null),
  error = ref(''),
  submitting = ref(false)
const goal = ref(''),
  clarification = ref(''),
  resuming = ref(''),
  confirm = ref(false),
  duration = ref('')
const useWindow = ref(false),
  windowOpen = ref(false),
  mode = ref('platform')
const start = ref(''),
  end = ref(''),
  publisher = ref(''),
  channel = ref('')
const station = ref('')
const compareEnabled = ref(false),
  compareStart = ref(''),
  compareEnd = ref('')
const imageOpen = ref(false),
  image = ref(''),
  imageSource = ref('user_upload'),
  crop = ref('')
const fileKey = ref(0),
  attaching = ref(false),
  savedImage = ref(false)
const resumeNeeds = ref([]),
  environmentSaved = ref(false),
  imageSaved = ref(false)
const controller = new AbortController()
let timer,
  selection = 0,
  fileRead = 0
const date = (value) => (value ? new Date(value).toLocaleString() : '—')
const deviceName = (id) => props.devices.find((item) => item.id === id)?.name || id
const channelName = (id) => props.channels.find((item) => item.id === id)?.name || id
const canReprepareCamera = computed(() => {
  const task = selected.value?.task
  const camera = task?.automation.camera
  return (
    task?.status === 'camera_unknown' &&
    Boolean(camera) &&
    !camera.receipts?.length &&
    !camera.captures?.length
  )
})
const stepRows = computed(() => {
  const steps = selected.value?.task.automation.steps || []
  const plan = selected.value?.task.automation.goal_plan
  const analysisRows = plan?.environment?.depends_on?.includes('image')
    ? [['image'], ['environment']]
    : plan?.image?.depends_on?.includes('environment')
      ? [['environment'], ['image']]
      : [['environment', 'image']]
  return [
    ['planner'],
    ['camera'],
    ['materials'],
    ...analysisRows,
    ['review'],
    ['report'],
    ['simulation'],
  ]
    .map((roles) => steps.filter((step) => roles.includes(step.role)))
    .filter((row) => row.length)
})

function statusText(value) {
  return (
    {
      running: tr('运行中', 'Running'),
      camera_running: tr('摄像头任务执行中', 'Camera task running'),
      waiting_camera_confirmation: tr('待确认摄像头计划', 'Confirm camera plan'),
      camera_stopped: tr('摄像头任务已停止', 'Camera task stopped'),
      camera_unknown: tr('摄像头结果未确认', 'Camera outcome unconfirmed'),
      unknown: tr('结果未确认', 'Outcome unconfirmed'),
      stopped: tr('已停止', 'Stopped'),
      pending: tr('待运行', 'Pending'),
      completed: tr('已完成', 'Completed'),
      partial: tr('部分完成', 'Partial'),
      failed: tr('失败', 'Failed'),
      skipped: tr('已跳过', 'Skipped'),
      needs_input: tr('待补充或确认', 'Input or decision needed'),
      planner_failed: tr('任务规划失败', 'Planning failed'),
      blocked: tr('等待前置步骤', 'Waiting for prerequisite'),
      interrupted: tr('已中断', 'Interrupted'),
      evidence_summary: tr('观测简报', 'Evidence summary'),
      data_insufficient: tr('观测不足', 'Insufficient observations'),
      model_failed: tr('模型调用失败', 'Model failed'),
      review_rejected: tr('复核未通过', 'Review rejected'),
      waiting_confirmation: tr('等待人工确认', 'Awaiting confirmation'),
      simulated_completed: tr('模拟完成', 'Simulation completed'),
      simulation_unknown: tr('模拟结果未知', 'Unknown simulation outcome'),
    }[value] || value
  )
}
function roleText(value) {
  return (
    {
      planner: tr('识别任务目标', 'Plan the task'),
      camera: tr('摄像头采集与控制', 'Camera capture and control'),
      materials: tr('查找资料', 'Locate evidence'),
      environment: tr('环境观测', 'Environment'),
      image: tr('图片分析', 'Image analysis'),
      simulation: tr('设备操作', 'Device operation'),
      review: tr('核对依据', 'Verify evidence'),
      report: tr('汇总报告', 'Consolidate reports'),
    }[value] || value
  )
}
function needText(value) {
  const detail = selected.value?.task.automation.input_needs?.find((item) => item.field === value)
  if (detail) return detail.message
  return (
    {
      goal: tr('需要更明确的任务目标。', 'Clarify the task goal.'),
      window: tr('需要补充可用的观测时段。', 'Provide an observation window.'),
      comparison: tr('需要补充另一个观测时段。', 'Provide a second observation window.'),
      image: tr('需要上传或补拍清晰的叶片照片。', 'Provide a clear leaf photo.'),
      duration: tr('需要给出模拟开启时长。', 'Provide the simulated open duration.'),
      execution_mode: tr(
        '真实设备控制尚未接通，可选择改为模拟。',
        'Physical control is unavailable. You can switch to simulation.',
      ),
      execution: tr(
        '当前执行能力无法完成所请求的动作，请补充执行方案。',
        'The requested action is not supported by an available executor.',
      ),
      clarification: tr('请补充任务中待确认的信息。', 'Clarify the pending task details.'),
    }[value] || value
  )
}
function actionText(value) {
  return { open: tr('开启后关闭', 'Open then close'), close: tr('关闭', 'Close') }[value] || value
}
function api(path, body) {
  return request('/api/agent/v2/workbench' + path, {
    method: body === undefined ? 'GET' : 'POST',
    body,
    headers: { 'X-Workspace-ID': props.workspace },
    signal: controller.signal,
    timeout: 20000,
  })
}
function syncEntry(task) {
  tasks.value = [
    {
      id: task.id,
      goal: task.question,
      status: task.status,
      updated_at: task.automation.updated_at,
    },
    ...tasks.value.filter((entry) => entry.id !== task.id),
  ].sort((a, b) => String(b.updated_at).localeCompare(String(a.updated_at)))
}
function show(detail, current) {
  if (controller.signal.aborted || current !== selection) return
  selected.value = detail
  syncEntry(detail.task)
  if (['running', 'camera_running'].includes(detail.task.status)) {
    timer = setTimeout(() => readTask(detail.task.id, current), 1000)
  }
}
async function readTask(id, current) {
  try {
    show(await api('/automation/' + encodeURIComponent(id)), current)
  } catch (e) {
    if (current === selection && !controller.signal.aborted) {
      error.value = errorText(e)
      if (
        selected.value?.task.id === id &&
        ['running', 'camera_running'].includes(selected.value.task.status)
      )
        timer = setTimeout(() => readTask(id, current), 3000)
    }
  }
}
function selectTask(id) {
  clearTimeout(timer)
  confirm.value = false
  error.value = ''
  void readTask(id, ++selection)
}
function localInput(value) {
  if (!value) return ''
  const parsed = new Date(value)
  return new Date(parsed.getTime() - parsed.getTimezoneOffset() * 60000).toISOString().slice(0, 19)
}
function resetForm() {
  goal.value = ''
  clarification.value = ''
  resuming.value = ''
  resumeNeeds.value = []
  environmentSaved.value = imageSaved.value = savedImage.value = false
  image.value = ''
  imageSource.value = 'user_upload'
  crop.value = ''
  fileRead++
  fileKey.value++
  attaching.value =
    imageOpen.value =
    useWindow.value =
    windowOpen.value =
    compareEnabled.value =
      false
  publisher.value = channel.value = compareStart.value = compareEnd.value = duration.value = ''
  station.value = ''
  mode.value = props.status.history_available ? 'history' : 'platform'
  start.value = localInput(props.status.first_at)
  end.value = props.status.last_at
    ? localInput(new Date(props.status.last_at).getTime() + 1000)
    : ''
}
function resumeForm() {
  const task = selected.value.task
  resetForm()
  resuming.value = task.id
  goal.value = task.question
  const a = task.automation
  resumeNeeds.value = a.needs || []
  environmentSaved.value = a.steps.some(
    (step) => step.role === 'environment' && step.status === 'completed',
  )
  imageSaved.value = a.steps.some((step) => step.role === 'image' && step.status === 'completed')
  savedImage.value = Boolean(a.image)
  useWindow.value = Boolean(a.query)
  windowOpen.value =
    useWindow.value ||
    resumeNeeds.value.includes('window') ||
    resumeNeeds.value.includes('comparison')
  compareEnabled.value = resumeNeeds.value.includes('comparison')
  imageOpen.value = resumeNeeds.value.includes('image')
  if (a.query) {
    mode.value = a.query.mode
    start.value = localInput(a.query.start)
    end.value = localInput(a.query.end)
    channel.value = a.query.channel_id || ''
    publisher.value = a.query.publisher_id || ''
    station.value = resumeNeeds.value.includes('window') ? '' : a.query.station_id || ''
  }
  if (a.compare) {
    compareEnabled.value = true
    compareStart.value = localInput(a.compare.start)
    compareEnd.value = localInput(a.compare.end)
  }
}
async function attachImage(event) {
  const current = ++fileRead
  const file = event.target.files?.[0]
  image.value = ''
  if (!file) {
    attaching.value = false
    return
  }
  attaching.value = true
  try {
    if (!['image/png', 'image/jpeg'].includes(file.type) || file.size > 2 * 1024 * 1024)
      throw new Error(
        tr('请选择不超过 2 MiB 的 PNG 或 JPEG 图片。', 'Choose PNG or JPEG up to 2 MiB.'),
      )
    const data = await new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(reader.result)
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(file)
    })
    if (current === fileRead) image.value = data.split(',')[1]
  } catch (e) {
    if (current === fileRead) error.value = errorText(e)
  } finally {
    if (current === fileRead) attaching.value = false
  }
}
function windowInput(from, until) {
  const a = new Date(from),
    b = new Date(until)
  if (!Number.isFinite(a.getTime()) || !Number.isFinite(b.getTime()) || a >= b)
    throw new Error(tr('请选择有效的起止时间。', 'Choose valid start and end times.'))
  return { start: a.toISOString(), end: b.toISOString() }
}
async function submit() {
  if (submitting.value || attaching.value || !goal.value.trim()) return
  error.value = ''
  submitting.value = true
  try {
    const body = {
      goal: goal.value.trim(),
      time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    }
    if (useWindow.value && !environmentSaved.value) {
      body.query = { mode: mode.value, ...windowInput(start.value, end.value) }
      if (mode.value === 'platform')
        Object.assign(body.query, {
          channel_id: channel.value,
          ...(station.value ? { station_id: station.value } : { publisher_id: publisher.value }),
        })
      if (compareEnabled.value) body.compare = windowInput(compareStart.value, compareEnd.value)
    }
    if (image.value)
      Object.assign(body, { image: image.value, image_source: imageSource.value, crop: crop.value })
    if (resumeNeeds.value.includes('duration') && duration.value !== '')
      body.duration_seconds = duration.value
    if (clarification.value.trim()) body.clarification = clarification.value.trim()
    const path = resuming.value
      ? '/automation/' + encodeURIComponent(resuming.value) + '/resume'
      : '/automation'
    const detail = await api(path, body)
    clearTimeout(timer)
    confirm.value = false
    resetForm()
    show(detail, ++selection)
  } catch (e) {
    if (!controller.signal.aborted) error.value = errorText(e)
  } finally {
    submitting.value = false
  }
}
async function switchToSimulation() {
  if (submitting.value || !selected.value) return
  error.value = ''
  submitting.value = true
  try {
    const detail = await api(
      '/automation/' + encodeURIComponent(selected.value.task.id) + '/resume',
      {
        execution_mode: 'simulation',
      },
    )
    clearTimeout(timer)
    confirm.value = false
    resetForm()
    show(detail, ++selection)
  } catch (e) {
    if (!controller.signal.aborted) error.value = errorText(e)
  } finally {
    submitting.value = false
  }
}
async function confirmSimulation() {
  if (!confirm.value || submitting.value) return
  submitting.value = true
  error.value = ''
  const task = selected.value.task
  const current = selection
  try {
    await api('/simulate', { id: task.id, revision: task.plan.revision, confirm: true })
    await readTask(task.id, current)
    confirm.value = false
  } catch (e) {
    if (!controller.signal.aborted) error.value = errorText(e)
  } finally {
    submitting.value = false
  }
}
const statusReady = computed(() => props.connected && !!props.workspace)
watch(
  statusReady,
  async (ready) => {
    if (!ready) return
    resetForm()
    try {
      tasks.value = await api('/automation')
      if (tasks.value.length) selectTask(tasks.value[0].id)
    } catch (e) {
      if (!controller.signal.aborted) error.value = errorText(e)
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  clearTimeout(timer)
  selection++
  fileRead++
  controller.abort()
})
</script>

<style scoped>
.automation-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
  gap: 18px;
  align-items: start;
}
.automation-panel {
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 22px;
  margin-bottom: 18px;
  background: #fff;
  overflow-wrap: anywhere;
}
.automation-panel h2 {
  font-size: 17px;
  margin: 0 0 14px;
}
.automation-panel h3 {
  font-size: 14px;
  margin: 16px 0 8px;
}
.automation-heading,
.automation-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.automation-heading .state {
  flex-shrink: 0;
}
.automation-heading strong {
  font-size: 14px;
}
.automation label {
  display: grid;
  gap: 6px;
  color: #505b65;
  font-size: 13px;
  margin: 8px 0;
  min-width: 0;
}
.automation textarea,
.automation select,
.automation input:not([type='checkbox']) {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.automation textarea {
  resize: vertical;
  line-height: 1.65;
}
.automation fieldset {
  border: 0;
  margin: 0;
  padding: 0;
  min-width: 0;
}
.fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.automation .check {
  display: flex;
  align-items: center;
  gap: 7px;
}
.duration {
  max-width: 240px;
}
.material {
  border-top: 1px solid var(--border);
  padding: 9px 0;
  font-size: 13px;
}
.material summary {
  cursor: pointer;
  color: #3d6b51;
}
.material summary small {
  color: var(--muted);
  margin-left: 6px;
}
.automation-footer {
  border-top: 1px solid var(--border);
  margin-top: 14px;
  padding-top: 14px;
}
.muted,
small {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
.run-task {
  background: #315f49;
  border-color: #315f49;
  flex-shrink: 0;
}
.steps {
  list-style: none;
  margin: 12px 0;
  padding: 0;
}
.steps li {
  position: relative;
  display: grid;
  gap: 8px;
  padding: 0 0 12px;
}
.steps li:not(:last-child)::after {
  content: '';
  position: absolute;
  inset: auto auto 0 50%;
  width: 1px;
  height: 12px;
  background: #c7d6cd;
}
.steps li.branches {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.step {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px;
  font-size: 13px;
}
.steps small {
  display: block;
  margin-top: 2px;
}
.step-dot {
  text-align: center;
  border-radius: 50%;
  background: #f0f2f4;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}
.step-dot[data-state='completed'] {
  color: white;
  background: #3e8c6d;
}
.step-dot[data-state='running'] {
  color: #315f49;
  background: #dcebe2;
}
.state {
  padding: 3px 7px;
  background: #f0f2f4;
  border-radius: 5px;
  color: #505b65;
  font-size: 12px;
}
.state[data-state='completed'],
.state[data-state='simulated_completed'] {
  background: #e6f0e9;
  color: #315f49;
}
.state[data-state='needs_input'],
.state[data-state='waiting_confirmation'] {
  background: #faf1db;
  color: #745b28;
}
.automation-error,
.needs {
  color: #8a3d36;
  font-size: 13px;
}
.result-summary {
  border-top: 1px solid var(--border);
  margin-top: 12px;
  font-size: 13px;
  line-height: 1.6;
}
.result-summary p {
  margin: 5px 0 10px;
}
.task-list {
  max-height: 20rem;
  overflow: auto;
}
.task-row {
  width: 100%;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 12px;
  align-items: center;
  padding: 9px 5px;
  text-align: left;
  border: 0;
  border-top: 1px solid var(--border);
  border-radius: 0;
}
.task-row[aria-current='true'] {
  background: #edf4ee;
}
@media (max-width: 940px) {
  .automation-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 0;
  }
}
@media (max-width: 520px) {
  .fields {
    grid-template-columns: minmax(0, 1fr);
  }
  .automation-panel {
    padding: 15px;
  }
  .task-row {
    grid-template-columns: minmax(0, 1fr) auto;
  }
  .task-row small {
    display: none;
  }
}
</style>
