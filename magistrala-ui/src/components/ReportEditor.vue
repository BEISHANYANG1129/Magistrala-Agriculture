<template>
  <form @submit.prevent="execute(record.id ? 'schedule' : mode)">
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <section class="step-panel">
      <h2>{{ tr('配置', 'Configuration') }}</h2>
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('名称', 'Name') }} *<input v-model="draft.name" required /></label
        ><label class="field"
          >{{ tr('描述', 'Description') }}<input v-model="draft.description" /></label
        ><label class="field"
          >{{ tr('报表标题', 'Report title') }} *<input
            v-model="draft.config.title"
            required /></label
        ><label class="field"
          >{{ tr('文件格式', 'Report format')
          }}<select v-model="draft.config.file_format">
            <option>pdf</option>
            <option>csv</option>
          </select></label
        ><label class="field"
          >{{ tr('时区', 'Timezone')
          }}<input v-model="draft.config.timezone" list="report-timezones" /><datalist
            id="report-timezones"
          >
            <option>Asia/Shanghai</option>
            <option>America/Sao_Paulo</option>
            <option>UTC</option>
          </datalist></label
        ><label class="field"
          >{{ tr('开始时间（相对时间或 ISO 时间）', 'From (relative or ISO time)') }} *<input
            v-model="draft.config.from"
            placeholder="-1h"
            required /></label
        ><label class="field"
          >{{ tr('结束时间', 'To')
          }}<input v-model="draft.config.to" placeholder="now" required /></label
        ><label class="field"
          >{{ tr('聚合方法', 'Aggregation method')
          }}<select v-model="draft.config.aggregation.agg_type">
            <option v-for="v in ['none', 'max', 'min', 'sum', 'count', 'avg']" :key="v">
              {{ v }}
            </option>
          </select></label
        ><label v-if="draft.config.aggregation.agg_type !== 'none'" class="field"
          >{{ tr('聚合间隔', 'Aggregation interval')
          }}<input v-model="draft.config.aggregation.interval" placeholder="5m" required
        /></label>
      </div>
    </section>
    <section class="step-panel">
      <h2>{{ tr('指标', 'Metrics') }}</h2>
      <div v-for="(m, i) in draft.metrics" :key="i" class="panel">
        <div class="native-form-grid">
          <label class="field"
            >{{ tr('指标名称', 'Metric name') }} *<input v-model="m.name" required /></label
          ><label class="field"
            >{{ tr('通道', 'Channel') }} *<select v-model="m.channel_id" required>
              <option value="">{{ tr('选择通道', 'Select channel') }}</option>
              <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select></label
          ><label class="field"
            >{{ tr('设备', 'Devices')
            }}<select multiple v-model="m.device_ids">
              <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option>
            </select></label
          ><label class="field">{{ tr('子主题', 'Subtopic') }}<input v-model="m.subtopic" /></label
          ><label class="field"
            >{{ tr('协议', 'Protocol')
            }}<select v-model="m.protocol">
              <option value="">{{ tr('全部', 'All') }}</option>
              <option v-for="p in ['http', 'mqtt', 'coap', 'ws']" :key="p">{{ p }}</option>
            </select></label
          >
        </div>
        <button
          type="button"
          class="quiet danger"
          :disabled="draft.metrics.length === 1"
          @click="draft.metrics.splice(i, 1)"
        >
          {{ tr('移除', 'Remove') }}
        </button>
      </div>
      <button
        type="button"
        class="quiet"
        @click="draft.metrics.push({ name: '', channel_id: '', device_ids: [] })"
      >
        + {{ tr('添加指标', 'Add metric') }}
      </button>
    </section>
    <section v-if="mode === 'email' || mode === 'schedule' || record.id" class="step-panel">
      <h2>{{ tr('邮件配置', 'Email configuration') }}</h2>
      <label class="field"
        >{{ tr('收件人（逗号分隔）', 'Recipients (comma separated)') }} *<input
          v-model="recipientText"
          required /></label
      ><label class="field"
        >{{ tr('邮件主题', 'Email subject') }}<input v-model="draft.email.subject" /></label
      ><label class="field"
        >{{ tr('正文', 'Content') }}<textarea v-model="draft.email.content" />
      </label>
    </section>
    <section v-if="mode === 'schedule' || record.id" class="step-panel">
      <h2>{{ tr('执行计划', 'Schedule') }}</h2>
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('开始执行时间', 'Active from')
          }}<input type="datetime-local" v-model="scheduleTime" :required="!record.id" /></label
        ><label class="field"
          >{{ tr('重复间隔', 'Recurring interval')
          }}<select v-model="draft.schedule.recurring">
            <option v-for="v in ['none', 'hourly', 'daily', 'weekly', 'monthly']" :key="v">
              {{ v }}
            </option>
          </select></label
        ><label class="field"
          >{{ tr('重复周期', 'Recurring period')
          }}<input type="number" min="1" v-model.number="draft.schedule.recurring_period"
        /></label>
      </div>
    </section>
    <details class="panel">
      <summary>{{ tr('报表模板', 'Report template') }}</summary>
      <textarea
        v-model="template"
        rows="10"
        class="wide"
        :aria-label="tr('报表模板', 'Report template')"
      /><button v-if="record.id" type="button" class="quiet" :disabled="busy" @click="saveTemplate">
        {{ tr('保存模板', 'Save template') }}</button
      ><button
        v-if="record.id"
        type="button"
        class="quiet"
        :disabled="busy"
        @click="resetTemplate = true"
      >
        {{ tr('恢复默认模板', 'Reset template') }}
      </button>
    </details>
    <div class="toolbar">
      <button v-if="record.id" :disabled="busy" type="submit">
        {{ tr('保存配置', 'Save configuration') }}</button
      ><button v-else-if="mode === 'schedule'" :disabled="busy">
        {{ tr('保存定时报表', 'Schedule report') }}</button
      ><button v-if="mode === 'email'" type="button" @click="execute('email')" :disabled="busy">
        {{ tr('发送报表邮件', 'Email report') }}</button
      ><button type="button" class="quiet" :disabled="busy" @click="execute('view')">
        {{ tr('生成即时报表', 'Generate instant report') }}</button
      ><button type="button" class="quiet" :disabled="busy" @click="execute('download')">
        {{ tr('下载报表', 'Download report') }}</button
      ><button type="button" class="quiet" @click="mode = 'email'">
        {{ tr('邮件报表', 'Email report') }}</button
      ><button type="button" class="quiet" @click="mode = 'schedule'">
        {{ tr('计划', 'Schedule') }}</button
      ><button
        v-if="record.id"
        type="button"
        class="quiet"
        :disabled="busy"
        @click="statusAction = record.status === 'enabled' ? 'disable' : 'enable'"
      >
        {{ record.status === 'enabled' ? tr('禁用', 'Disable') : tr('启用', 'Enable') }}
      </button>
    </div>
    <section v-if="preview" class="panel">
      <h2>{{ tr('报表预览', 'Report preview') }}</h2>
      <iframe
        v-if="typeof preview === 'string'"
        :srcdoc="preview"
        sandbox=""
        class="report-preview"
        :title="tr('报表预览', 'Report preview')"
      />
      <pre v-else>{{ JSON.stringify(preview, null, 2) }}</pre>
    </section>
    <ModalDialog v-if="statusAction || resetTemplate" :busy="busy" confirmation @close="() => { statusAction = ''; resetTemplate = false }">
      <section class="modal-card compact-modal">
        <h2>{{ tr('确认修改', 'Confirm change') }}</h2>
        <p>
          {{
            resetTemplate
              ? tr('恢复默认报表模板？', 'Reset report template?')
              : statusAction === 'disable'
                ? tr('禁用定时报表？', 'Disable scheduled report?')
                : tr('启用定时报表？', 'Enable scheduled report?')
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <button
          type="button"
          class="quiet"
          @click="
            () => {
              statusAction = ''
              resetTemplate = false
            }
          "
        >
          {{ tr('取消', 'Cancel') }}</button
        ><button type="button" :disabled="busy" @click="confirmChange">
          {{ tr('确认', 'Confirm') }}
        </button>
      </section>
    </ModalDialog>
  </form>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onMounted } from 'vue'
import { tr } from '../i18n'
import { request, listAll, errorText } from '../api/platform'
const clone = (v) => JSON.parse(JSON.stringify(v))
const props = defineProps({ record: Object, workspace: String }),
  emit = defineEmits(['saved'])
const draft = ref({
    name: '',
    description: '',
    ...clone(props.record),
    config: {
      from: '-1h',
      to: 'now',
      title: '',
      file_format: 'pdf',
      timezone: 'Asia/Shanghai',
      ...props.record.config,
      aggregation: { agg_type: 'none', interval: '5m', ...props.record.config?.aggregation },
    },
    metrics: clone(props.record.metrics || [{ name: '', channel_id: '', device_ids: [] }]),
    email: { to: [], subject: '', content: '', ...props.record.email },
    schedule: { recurring: 'none', recurring_period: 1, ...props.record.schedule },
  }),
  channels = ref([]),
  devices = ref([]),
  mode = ref(props.record.id ? 'schedule' : ''),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  preview = ref(null),
  recipientText = ref((props.record.email?.to || []).join(', ')),
  scheduleTime = ref(''),
  template = ref(props.record.report_template || ''),
  statusAction = ref(''),
  resetTemplate = ref(false)
const originalSchedule = JSON.stringify(draft.value.schedule)
const root = `/api/platform/reports/${props.workspace}/reports`,
  url = props.record.id ? `${root}/configs/${props.record.id}` : ''
onMounted(async () => {
  try {
    ;[channels.value, devices.value] = await Promise.all([
      listAll('resources', { tenantId: props.workspace, kind: 'channel' }),
      listAll('entities', { tenantId: props.workspace, kind: 'device' }),
    ])
  } catch (e) {
    error.value = errorText(e)
  }
})
function body(action) {
  const d = draft.value
  if (
    !d.config.title.trim() ||
    !d.config.from ||
    !d.config.to ||
    !d.metrics.length ||
    d.metrics.some((m) => !m.name.trim() || !m.channel_id)
  )
    throw new Error(
      tr(
        '请填写报表标题、时间范围及每个指标的名称和通道。',
        'Fill in the title, time range and every metric name and channel.',
      ),
    )
  if (d.config.aggregation.agg_type !== 'none' && !d.config.aggregation.interval)
    throw new Error(tr('请填写聚合间隔。', 'Enter the aggregation interval.'))
  const b = {
    config: d.config,
    metrics: d.metrics.map((m) =>
      Object.fromEntries(Object.entries(m).filter(([, v]) => v !== '')),
    ),
  }
  if (action === 'schedule' || action === 'email') {
    const to = recipientText.value
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean)
    if (!to.length || to.some((v) => !/^\S+@\S+\.\S+$/.test(v)))
      throw new Error(tr('请填写有效的收件人邮箱。', 'Enter valid recipient emails.'))
    b.email = { ...d.email, to }
  }
  if (template.value) b.report_template = template.value
  return b
}
async function execute(action) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const b = body(action)
    if (action === 'schedule') {
      if (!draft.value.name.trim()) throw new Error(tr('请填写名称。', 'Enter a name.'))
      const schedule = { ...draft.value.schedule }
      if (
        schedule.recurring !== 'none' &&
        (!Number.isInteger(schedule.recurring_period) || schedule.recurring_period < 1)
      )
        throw new Error(
          tr('重复周期必须是正整数。', 'Recurring period must be a positive integer.'),
        )
      if (scheduleTime.value) {
        const date = new Date(scheduleTime.value)
        if (!Number.isFinite(date.getTime()) || date <= new Date())
          throw new Error(tr('开始执行时间必须在未来。', 'Active from must be in the future.'))
        schedule.start_datetime = date.toISOString()
        schedule.time = date.toISOString()
      } else if (!props.record.id)
        throw new Error(tr('请设置开始执行时间。', 'Set the schedule start time.'))
      if (props.record.id) {
        const { report_template, ...patch } = b
        await request(url, {
          method: 'PATCH',
          body: { name: draft.value.name, description: draft.value.description, ...patch },
        })
        if (scheduleTime.value || JSON.stringify(schedule) !== originalSchedule)
          await request(url + '/schedule', { method: 'PATCH', body: { schedule } })
        emit('saved', { id: props.record.id })
      } else {
        const r = await request(root + '/configs', {
          method: 'POST',
          body: { name: draft.value.name, description: draft.value.description, ...b, schedule },
        })
        emit('saved', r)
      }
    } else {
      const r = await request(root + '?action=' + action, {
        method: 'POST',
        body: b,
        download: action === 'download',
      })
      if (action === 'view') preview.value = r
      notice.value =
        action === 'email'
          ? tr('报表邮件已提交发送。', 'Report email submitted.')
          : tr('报表已生成。', 'Report generated.')
    }
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function saveTemplate() {
  busy.value = true
  error.value = ''
  try {
    if (!template.value.trim()) throw new Error(tr('模板不能为空。', 'Template cannot be empty.'))
    await request(url + '/template', { method: 'PUT', body: { report_template: template.value } })
    notice.value = tr('模板已保存。', 'Template saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function confirmChange() {
  busy.value = true
  try {
    await request(url + (resetTemplate.value ? '/template' : '/' + statusAction.value), {
      method: resetTemplate.value ? 'DELETE' : 'POST',
    })
    resetTemplate.value = false
    statusAction.value = ''
    emit('saved', { id: props.record.id })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
