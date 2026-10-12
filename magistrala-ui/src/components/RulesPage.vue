<template>
  <section>
    <header class="page-heading">
      <h1>{{ editing ? draft.name || tr('创建规则', 'Create rule') : tr('规则引擎', 'Rules') }}</h1>
      <router-link v-if="editing" class="button quiet" :to="base"
        >← {{ tr('返回规则列表', 'Back to rules') }}</router-link
      ><router-link v-else class="button" :to="base + '/create'"
        >+ {{ tr('创建规则', 'Create rule') }}</router-link
      >
    </header>
    <nav v-if="editing && id !== 'create'" class="tabs">
      <router-link class="button quiet" :to="base + '/' + id">{{
        tr('规则画布', 'Rule canvas')
      }}</router-link>
      <router-link class="button quiet" :to="base + '/' + id + '/role-management'">{{
        tr('角色', 'Roles')
      }}</router-link>
      <router-link class="button quiet" :to="base + '/' + id + '/members'">{{
        tr('成员', 'Members')
      }}</router-link>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <ResourceAccess
      v-if="
        editing && ['role-management', 'members'].includes(String(route.params.tab)) && draft.id
      "
      :workspace="workspace"
      section="rules"
      :record="draft"
      :tab="route.params.tab === 'members' ? 'users' : 'role-management'"
    />
    <template v-else-if="!editing"
      ><form class="toolbar" @submit.prevent="() => { offset = 0; load() }">
        <input v-model="search" :placeholder="tr('按名称搜索', 'Search by name')" /><button
          class="quiet"
        >
          {{ tr('搜索', 'Search') }}
        </button>
      </form>
      <div v-if="selectedRules.length" class="toolbar">
        <strong>{{ tr('已选择', 'Selected') }} {{ selectedRules.length }}</strong
        ><button class="quiet" @click="openBatch('enable')">{{ tr('启用', 'Enable') }}</button
        ><button class="quiet" @click="openBatch('disable')">{{ tr('禁用', 'Disable') }}</button
        ><button class="quiet danger" @click="openBatch('delete')">
          {{ tr('删除', 'Delete') }}
        </button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  :aria-label="tr('选择当前页', 'Select current page')"
                  :checked="!!records.length && selectedRules.length === records.length" :indeterminate="selectedRules.length > 0 && selectedRules.length < records.length"
                  @change="selectedRules = $event.target.checked ? records.map((r) => r.id) : []"
                />
              </th>
              <th>{{ tr('名称', 'Name') }}</th>
              <th>{{ tr('标签', 'Tags') }}</th>
              <th>{{ tr('状态', 'Status') }}</th>
              <th>{{ tr('更新时间', 'Updated') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in records" :key="r.id">
              <td>
                <input
                  v-model="selectedRules"
                  type="checkbox"
                  :value="r.id"
                  :aria-label="tr('选择 ', 'Select ') + r.name"
                />
              </td>
              <td>
                <router-link :to="base + '/' + r.id">{{ r.name }}</router-link>
              </td>
              <td>{{ (r.tags || []).join(', ') }}</td>
              <td>{{ r.status }}</td>
              <td>{{ r.updated_at || r.created_at }}</td>
            </tr>
            <tr v-if="!records.length">
              <td colspan="5">
                {{ busy ? tr('读取中…', 'Loading…') : tr('暂无规则。', 'No rules.') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <TablePagination :page="Math.floor(offset / limit) + 1" :size="limit" :total="total" :busy="busy" @update:page="changePage" @update:size="changeSize" /></template
    >
    <template v-else-if="draft">
      <div class="toolbar">
        <template v-if="id !== 'create'">
          <button
            class="quiet"
            :disabled="busy"
            @click="pendingAction = draft.status === 'enabled' ? 'disable' : 'enable'"
          >
            {{
              draft.status === 'enabled'
                ? tr('禁用规则', 'Disable rule')
                : tr('启用规则', 'Enable rule')
            }}
          </button>
          <button class="quiet danger" :disabled="busy" @click="pendingAction = 'delete'">
            {{ tr('删除规则', 'Delete rule') }}
          </button>
        </template>
        <button class="quiet" @click="openNode('schedule')">
          {{ tr('添加 / 编辑计划', 'Add / edit schedule') }}</button
        ><button :disabled="busy || !ready" @click="nameDialog = true">
          {{ tr('保存规则', 'Save rule') }}</button
        ><span class="muted">{{
          draft.logic?.mode === 'unrestricted'
            ? tr('现有执行模式：不受限', 'Existing mode: unrestricted')
            : tr('执行模式：沙箱', 'Mode: sandboxed')
        }}</span>
      </div>
      <RuleCanvas
        :draft="draft"
        :channels="channels"
        :output-label="outputLabel"
        @add="openNode"
        @edit="openNode"
        @remove="removeNode"
        @layout="saveLayout"
        @edges="saveEdges"
      />
      <p v-if="!ready" class="muted">
        {{
          tr(
            '添加输入、逻辑和至少一个输出后即可保存。',
            'Add input, logic and at least one output to save.',
          )
        }}
      </p>
      <div v-if="hasSchedule" class="panel">
        <strong>{{ tr('执行计划', 'Schedule') }}</strong>
        <p>
          {{ draft.schedule.start_datetime || draft.schedule.time }} ·
          {{ draft.schedule.recurring }}
        </p>
      </div>
    </template>
    <ModalDialog v-if="nodeDialog" @close="nodeDialog = ''">
      <form class="modal-card" @submit.prevent="applyNode">
        <h2>{{ nodeTitle }}</h2>
        <template v-if="nodeDialog === 'input'"
          ><label class="field"
            >{{ tr('通道', 'Channel') }} *<select v-model="node.input_channel" required>
              <option value="">{{ tr('选择通道', 'Select a channel') }}</option>
              <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select></label
          ><label class="field"
            >{{ tr('子主题', 'Topic') }}<input v-model="node.input_topic" /></label
        ></template>
        <template v-if="nodeDialog === 'logic'">
          <label class="field"
            >{{ tr('逻辑类型', 'Logic type')
            }}<select v-model="logicEditor">
              <option value="comparison">{{ tr('比较条件', 'Comparison') }}</option>
              <option value="editor">{{ tr('代码编辑器', 'Code Editor') }}</option>
              <option value="custom">{{ tr('自定义节点', 'Custom Nodes') }}</option>
            </select></label
          >
          <template v-if="logicEditor === 'comparison'">
            <div v-for="(c, i) in comparisons" :key="i" class="comparison-row">
              <label v-if="i" class="field"
                >{{ tr('组合条件', 'Combine')
                }}<select v-model="c.join">
                  <option value="and">AND</option>
                  <option value="or">OR</option>
                </select></label
              >
              <label class="field"
                >{{ tr('输入字段', 'Input field')
                }}<input v-model="c.input" placeholder="message.payload[1].v" required
              /></label>
              <label class="field"
                >{{ tr('比较', 'Compare')
                }}<select v-model="c.operator">
                  <option v-for="op in comparisonOperators" :key="op">{{ op }}</option>
                </select></label
              >
              <label class="field"
                >{{ tr('类型', 'Type')
                }}<select v-model="c.type">
                  <option value="number">{{ tr('数值', 'Number') }}</option>
                  <option value="string">{{ tr('文本', 'String') }}</option>
                  <option value="boolean">{{ tr('布尔值', 'Boolean') }}</option>
                </select></label
              >
              <label class="field"
                >{{ tr('比较值', 'Value')
                }}<select v-if="c.type === 'boolean'" v-model="c.value">
                  <option value="true">true</option>
                  <option value="false">false</option></select
                ><input
                  v-else
                  v-model="c.value"
                  :type="c.type === 'number' ? 'number' : 'text'"
                  step="any"
                  required
              /></label>
              <button
                type="button"
                class="quiet"
                :disabled="comparisons.length === 1"
                @click="comparisons.splice(i, 1)"
              >
                {{ tr('移除', 'Remove') }}
              </button>
            </div>
            <button
              type="button"
              class="quiet"
              @click="
                comparisons.push({
                  input: '',
                  operator: '==',
                  type: 'number',
                  value: '20',
                  join: 'and',
                })
              "
            >
              + {{ tr('添加条件', 'Add condition') }}
            </button>
          </template>
          <label v-if="logicEditor === 'custom'" class="field"
            >{{ tr('从自定义节点复制脚本', 'Copy script from custom node')
            }}<select v-model="templateId" @change="useTemplate">
              <option value="">{{ tr('选择模板', 'Select template') }}</option>
              <option v-for="c in customNodes" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select></label
          ><label v-if="logicEditor !== 'comparison'" class="field"
            >{{ tr('脚本类型', 'Script type')
            }}<select v-model.number="node.type">
              <option :value="0">Lua</option>
              <option :value="1">Go</option>
            </select></label
          ><label v-if="logicEditor !== 'comparison'" class="field"
            >{{ tr('脚本', 'Script') }} *<textarea
              v-model="node.value"
              rows="16"
              required
              spellcheck="false"
            />
          </label>
          <label class="field"
            >{{ tr('执行模式', 'Execution mode')
            }}<select v-model="node.mode">
              <option value="unrestricted">{{ tr('不受限', 'Unrestricted') }}</option>
              <option value="sandboxed">{{ tr('沙箱', 'Sandboxed') }}</option>
            </select></label
          ></template
        >
        <template v-if="nodeDialog === 'output'"
          ><label class="field"
            >{{ tr('输出类型', 'Output type')
            }}<select v-model="node.type" @change="resetOutput">
              <option v-for="o in outputTypes" :key="o[0]" :value="o[0]">
                {{ tr(o[1], o[2]) }}
              </option>
            </select></label
          ><template v-if="node.type === 'channels'"
            ><label class="field"
              >{{ tr('通道', 'Channel')
              }}<select v-model="node.channel" required>
                <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select></label
            ><label class="field"
              >{{ tr('子主题', 'Topic') }}<input v-model="node.topic" /></label></template
          ><template v-if="node.type === 'email'"
            ><label class="field"
              >{{ tr('收件人（逗号分隔）', 'Recipients (comma separated)')
              }}<input v-model="recipients" required /></label
            ><label class="field"
              >{{ tr('主题', 'Subject') }}<input v-model="node.subject" required /></label
            ><label class="field"
              >{{ tr('正文模板', 'Content template')
              }}<textarea v-model="node.content" rows="6" required /></label></template
          ><template v-if="node.type === 'save_remote_pg'"
            ><label
              v-for="k in ['host', 'port', 'user', 'database', 'table']"
              :key="k"
              class="field"
              >{{ label(k)
              }}<input v-model="node[k]" :type="k === 'port' ? 'number' : 'text'" required /></label
            ><label class="field"
              >{{ tr('密码（编辑时留空保留原值）', 'Password (leave blank to keep on edit)')
              }}<input
                v-model="node.password"
                type="password"
                :required="!node.id"
                autocomplete="new-password" /></label
            ><label class="field"
              >{{ tr('映射模板', 'Mapping template')
              }}<textarea v-model="node.mapping" required /></label></template
          ><template v-if="node.type === 'slack'"
            ><label class="field"
              >{{ tr('令牌（编辑时留空保留原值）', 'Token (leave blank to keep on edit)')
              }}<input
                v-model="node.token"
                type="password"
                :required="!node.id"
                autocomplete="new-password" /></label
            ><label class="field"
              >{{ tr('Slack 通道编号', 'Slack channel ID')
              }}<input v-model="node.channel_id" required /></label
            ><label class="field"
              >{{ tr('消息模板', 'Message template')
              }}<textarea v-model="node.message" required /></label></template
        ></template>
        <template v-if="nodeDialog === 'schedule'"
          ><label class="field"
            >{{ tr('开始时间', 'Start time')
            }}<input type="datetime-local" v-model="scheduleTime" required /></label
          ><label class="field"
            >{{ tr('重复', 'Recurring')
            }}<select v-model="node.recurring">
              <option
                v-for="v in ['none', 'hourly', 'daily', 'weekly', 'monthly']"
                :key="v"
                :value="v"
              >
                {{ label(v) }}
              </option>
            </select></label
          ><label class="field"
            >{{ tr('间隔', 'Recurring period')
            }}<input type="number" min="1" v-model.number="node.recurring_period" /></label
        ></template>
        <p v-if="nodeError" class="error">{{ nodeError }}</p>
        <div class="actions">
          <button type="button" class="quiet" @click="nodeDialog = ''">
            {{ tr('关闭', 'Close') }}</button
          ><button>{{ tr('应用', 'Apply') }}</button>
        </div>
      </form>
    </ModalDialog>
    <BatchActions
      v-if="batch"
      :records="batch.records"
      :title="batch.title"
      :perform="batch.perform"
      @close="batch = null"
      @updated="load"
    />
    <ModalDialog v-if="pendingAction" :busy="busy" confirmation @close="pendingAction = ''">
      <div class="modal-card compact-modal">
        <h2>{{ tr('确认规则操作', 'Confirm rule action') }}</h2>
        <p>
          {{ draft.name }} ·
          {{
            pendingAction === 'delete'
              ? tr('删除规则', 'Delete rule')
              : pendingAction === 'disable'
                ? tr('禁用规则', 'Disable rule')
                : tr('启用规则', 'Enable rule')
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button class="quiet" :disabled="busy" @click="pendingAction = ''">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy" @click="changeStatus">{{ tr('确认', 'Confirm') }}</button>
        </div>
      </div>
    </ModalDialog>
    <ModalDialog v-if="nameDialog" :busy="busy" @close="nameDialog = false">
      <form class="modal-card compact-modal" @submit.prevent="save">
        <h2>{{ tr('保存规则', 'Save rule') }}</h2>
        <label class="field"
          >{{ tr('名称', 'Name') }} *<input v-model="draft.name" required /></label
        ><label class="field"
          >{{ tr('标签（逗号分隔）', 'Tags (comma separated)') }}<input v-model="tagText"
        /></label>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="nameDialog = false">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="busy">{{ tr('保存', 'Save') }}</button>
        </div>
      </form>
    </ModalDialog>
  </section>
</template>
<script setup>
import TablePagination from './TablePagination.vue'
import ModalDialog from './ModalDialog.vue'
import ResourceAccess from './ResourceAccess.vue'
import { ref, computed, watch } from 'vue'
import BatchActions from './BatchActions.vue'
import RuleCanvas from './RuleCanvas.vue'
import { comparisonOperators, comparisonScript, parseComparison } from '../api/comparisonLogic'
import {
  decodeRuleFlow,
  encodeRuleFlow,
  writeRuleFlow,
  removeFlowNode,
  changeOutputFlowType,
} from '../api/ruleFlow'
import { useRoute, useRouter } from 'vue-router'
import { tr, label } from '../i18n'
import { request, listAll, errorText } from '../api/platform'
const logicEditor = ref('editor'),
  comparisons = ref([])
const route = useRoute(),
  router = useRouter(),
  workspace = computed(() => String(route.params.id)),
  id = computed(() => String(route.params.recordId || '')),
  base = computed(() => `/domain/${workspace.value}/rules`),
  api = computed(() => `/api/platform/rules/${workspace.value}/rules`),
  editing = computed(() => !!id.value)
const selectedRules = ref([]),
  batch = ref(null),
  customNodes = ref([]),
  templateId = ref('')
const records = ref([]),
  total = ref(0),
  offset = ref(0),
  limit = ref(10),
  search = ref(''),
  channels = ref([]),
  draft = ref({}),
  error = ref(''),
  notice = ref(''),
  busy = ref(false),
  nodeDialog = ref(''),
  node = ref({}),
  nodeIndex = ref(-1),
  nodeError = ref(''),
  nameDialog = ref(false),
  tagText = ref(''),
  scheduleTime = ref(''),
  recipients = ref('')
let original = null,
  generation = 0
const pendingAction = ref('')
async function changeStatus() {
  const seq = generation,
    target = `${api.value}/${id.value}`,
    action = pendingAction.value,
    destination = base.value
  busy.value = true
  error.value = ''
  try {
    await request(action === 'delete' ? target : `${target}/${action}`, {
      method: action === 'delete' ? 'DELETE' : 'POST',
    })
    if (seq === generation) {
      pendingAction.value = ''
      if (action === 'delete') await router.push(destination)
      else await load()
    }
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
const outputTypes = [
  ['save_senml', '内部数据库（SenML）', 'Internal DB (SenML)'],
  ['save_json', '内部数据库（JSON）', 'Internal DB (JSON)'],
  ['channels', '通道发布', 'Channel publisher'],
  ['alarms', '告警', 'Alarms'],
  ['email', '电子邮件', 'Email'],
  ['save_remote_pg', '远程 PostgreSQL', 'Remote PostgreSQL'],
  ['slack', 'Slack', 'Slack'],
]
const outputLabel = (v) => {
  const o = outputTypes.find((o) => o[0] === v)
  return o ? tr(o[1], o[2]) : v
}
const ready = computed(
  () => draft.value.input_channel && draft.value.logic?.value && draft.value.outputs?.length,
)
const hasSchedule = computed(() => {
  const time = draft.value.schedule?.start_datetime || draft.value.schedule?.time
  return time && !String(time).startsWith('0001-')
})
const nodeTitle = computed(
  () =>
    ({
      input: tr('通道订阅', 'Channel subscriber'),
      logic: tr('脚本编辑', 'Script editor'),
      output: tr('规则输出', 'Rule output'),
      schedule: tr('执行计划', 'Schedule'),
    })[nodeDialog.value],
)
function openBatch(action) {
  const root = api.value
  batch.value = {
    records: records.value.filter((r) => selectedRules.value.includes(r.id)),
    title: {
      enable: tr('批量启用', 'Enable selected'),
      disable: tr('批量禁用', 'Disable selected'),
      delete: tr('批量删除', 'Delete selected'),
    }[action],
    perform: (r) =>
      request(root + '/' + encodeURIComponent(r.id) + (action === 'delete' ? '' : '/' + action), {
        method: action === 'delete' ? 'DELETE' : 'POST',
      }),
  }
}
function useTemplate() {
  const c = customNodes.value.find((c) => c.id === templateId.value)
  if (c?.logic) node.value = { ...node.value, type: c.logic.type, value: c.logic.value }
}
async function loadTemplates(seq) {
  try {
    const d = await request(`/api/platform/ui/${workspace.value}/custom-nodes?limit=100`)
    if (seq === generation) customNodes.value = d.custom_nodes || []
  } catch {
    /* Rule editing remains available without UI-service permissions. */
  }
}
async function load() {
  const seq = ++generation
  busy.value = true
  error.value = ''
  try {
    if (!editing.value) {
      const d = await request(
        `${api.value}?limit=${limit.value}&offset=${offset.value}&name=${encodeURIComponent(search.value)}`,
      )
      if (seq === generation) {
        selectedRules.value = []
        records.value = d.rules || []
        total.value = d.total || 0
      }
    } else {
      const [c, r] = await Promise.all([
        listAll('resources', { tenantId: workspace.value, kind: 'channel' }),
        id.value === 'create' ? null : request(`${api.value}/${id.value}`),
      ])
      if (seq !== generation) return
      channels.value = c
      loadTemplates(seq)
      original = r
      draft.value = r
        ? JSON.parse(JSON.stringify(r))
        : { name: '', input_channel: '', input_topic: '', outputs: [], metadata: {} }
      tagText.value = (r?.tags || []).join(', ')
    }
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
watch(
  () => [workspace.value, id.value],
  () => {
    nodeDialog.value = ''
    nameDialog.value = false
    pendingAction.value = ''
    batch.value = null
    selectedRules.value = []
    notice.value = ''
    load()
  },
  { immediate: true },
)
function openNode(kind, index = -1) {
  templateId.value = ''
  nodeDialog.value = kind
  nodeIndex.value = index
  nodeError.value = ''
  if (kind === 'input')
    node.value = {
      input_channel: draft.value.input_channel || '',
      input_topic: draft.value.input_topic || '',
    }
  if (kind === 'logic') {
    node.value = JSON.parse(
      JSON.stringify(
        draft.value.logic || { type: 0, value: 'return message.payload', mode: 'sandboxed' },
      ),
    )
    const flow = decodeRuleFlow(draft.value.metadata),
      saved = flow?.nodes.find((n) => ['comparison', 'editor', 'custom'].includes(n.type))
    const parsed = parseComparison(node.value.value || '')
    logicEditor.value =
      saved?.type === 'comparison' && parsed.length
        ? 'comparison'
        : saved?.type === 'custom'
          ? 'custom'
          : 'editor'
    comparisons.value = parsed.length
      ? parsed
      : [
          {
            input: 'message.payload[1].v',
            operator: '==',
            type: 'number',
            value: '20',
            join: 'and',
          },
        ]
    templateId.value = saved?.data?.data?.customNodeId || ''
  }
  if (kind === 'output') {
    node.value =
      index >= 0 ? JSON.parse(JSON.stringify(draft.value.outputs[index])) : { type: 'save_senml' }
    recipients.value = (node.value.to || []).join(', ')
  }
  if (kind === 'schedule') {
    node.value = { recurring: 'none', recurring_period: 1, ...draft.value.schedule }
    const raw = node.value.start_datetime || node.value.time
    scheduleTime.value = raw
      ? new Date(new Date(raw).getTime() - new Date(raw).getTimezoneOffset() * 60000)
          .toISOString()
          .slice(0, 16)
      : ''
  }
}
function saveLayout(positions) {
  try {
    draft.value.metadata = writeRuleFlow(draft.value, positions, undefined, channels.value)
  } catch (e) {
    error.value = errorText(e)
  }
}
function saveEdges(edges) {
  try {
    draft.value.metadata = writeRuleFlow(draft.value, undefined, edges, channels.value)
  } catch (e) {
    error.value = errorText(e)
  }
}
function removeNode(kind, index) {
  removeFlowNode(
    draft.value,
    kind === 'output' ? 'output-' + (draft.value.outputs[index].id || index) : kind,
  )
  if (kind === 'input') {
    draft.value.input_channel = ''
    draft.value.input_topic = ''
  } else if (kind === 'logic') delete draft.value.logic
  else draft.value.outputs.splice(index, 1)
}
function resetOutput() {
  node.value = { type: node.value.type }
  recipients.value = ''
}
function applyNode() {
  try {
    if (nodeDialog.value === 'input') Object.assign(draft.value, node.value)
    if (nodeDialog.value === 'logic') {
      draft.value.logic =
        logicEditor.value === 'comparison'
          ? { type: 0, value: comparisonScript(comparisons.value), mode: node.value.mode }
          : { ...node.value }
      const metadata = writeRuleFlow(draft.value, undefined, undefined, channels.value),
        flow = decodeRuleFlow(metadata),
        n = flow.nodes.find((n) => ['comparison', 'editor', 'custom'].includes(n.type))
      if (n) {
        n.type = logicEditor.value
        if (logicEditor.value === 'custom') {
          const c = customNodes.value.find((c) => c.id === templateId.value)
          if (c)
            n.data = {
              data: {
                customNodeId: c.id,
                customNodeName: c.name,
                customNodeVersion: c.version,
                icon: c.icon,
                description: c.description,
              },
            }
        } else delete n.data
      }
      draft.value.metadata = { ...metadata, flow: encodeRuleFlow(flow) }
    }
    if (nodeDialog.value === 'output') {
      const output = { ...node.value }
      if (output.type === 'channels') output.topic ||= ''
      if (output.type === 'email')
        output.to = recipients.value
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean)
      if (output.type === 'save_remote_pg') output.port = Number(output.port)
      if (output.id) {
        if (!output.password) delete output.password
        if (!output.token) delete output.token
      }
      if (nodeIndex.value < 0) draft.value.outputs.push(output)
      else {
        changeOutputFlowType(draft.value, nodeIndex.value, output.type)
        draft.value.outputs[nodeIndex.value] = output
      }
    }
    if (nodeDialog.value === 'schedule') {
      const time = new Date(scheduleTime.value)
      if (time <= new Date())
        throw new Error(tr('开始时间不能早于当前时间。', 'Start time must be in the future.'))
      draft.value.schedule = {
        ...node.value,
        start_datetime: time.toISOString(),
        time: time.toISOString(),
      }
    }
    nodeDialog.value = ''
  } catch (e) {
    nodeError.value = errorText(e)
  }
}
async function save() {
  const seq = generation,
    targetId = id.value,
    targetApi = api.value,
    targetBase = base.value
  const snapshot = JSON.parse(JSON.stringify(draft.value)),
    expected = original
  const tags = tagText.value
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean)
  const body = {
    name: snapshot.name,
    input_channel: snapshot.input_channel,
    input_topic: snapshot.input_topic || '',
    logic: snapshot.logic,
    outputs: snapshot.outputs,
    metadata: snapshot.metadata || {},
  }
  let partiallySaved = false
  busy.value = true
  error.value = ''
  try {
    body.metadata = writeRuleFlow(snapshot, undefined, undefined, channels.value)
    if (targetId === 'create') {
      const created = await request(targetApi, {
        method: 'POST',
        body: { ...body, tags, ...(snapshot.schedule ? { schedule: snapshot.schedule } : {}) },
      })
      if (seq === generation) {
        nameDialog.value = false
        await router.push(created?.id ? `${targetBase}/${created.id}` : targetBase)
      }
    } else {
      const url = `${targetApi}/${targetId}`,
        latest = await request(url)
      if (seq !== generation) return
      if (JSON.stringify(latest) !== JSON.stringify(expected))
        throw new Error(
          tr(
            '规则已被其他操作修改，请刷新后重试。',
            'The rule changed elsewhere. Reload before saving.',
          ),
        )
      await request(url, { method: 'PATCH', body })
      partiallySaved = true
      // Track our own successful partial updates so a failed later step can be retried.
      if (seq === generation) original = { ...latest, ...body }
      if (JSON.stringify(tags) !== JSON.stringify(latest.tags || [])) {
        await request(`${url}/tags`, { method: 'PATCH', body: { tags } })
        if (seq === generation) original = { ...original, tags }
      }
      if (
        snapshot.schedule &&
        JSON.stringify(snapshot.schedule) !== JSON.stringify(latest.schedule)
      ) {
        await request(`${url}/schedule`, { method: 'PATCH', body: { schedule: snapshot.schedule } })
        if (seq === generation) original = { ...original, schedule: snapshot.schedule }
      }
      if (seq === generation) {
        nameDialog.value = false
        await load()
        notice.value = tr('规则已保存。', 'Rule saved.')
      }
    }
  } catch (e) {
    if (seq === generation) {
      error.value = errorText(e)
      if (partiallySaved) {
        original = null
        error.value +=
          ' ' +
          tr(
            '规则主体已保存，但后续配置未全部完成。请刷新后核对标签和计划，再继续编辑。',
            'The rule was saved, but subsequent settings did not all complete. Reload and review tags and schedule before editing again.',
          )
      }
    }
  } finally {
    if (seq === generation) busy.value = false
  }
}
function changePage(page) { const next = (page - 1) * limit.value; if (next !== offset.value) { offset.value = next; load() } }
function changeSize(size) { limit.value = size; offset.value = 0; load() }
</script>

<style scoped>
.comparison-row {
  display: flex;
  gap: 10px;
  align-items: end;
  flex-wrap: wrap;
  border-bottom: 1px solid #dce3eb;
  margin-bottom: 12px;
}
.comparison-row .field {
  flex: 1;
  min-width: 90px;
}
.comparison-row .field:first-child {
  flex: 2;
}
</style>
