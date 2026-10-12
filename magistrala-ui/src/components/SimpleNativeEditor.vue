<template>
  <section>
    <nav v-if="!existingDashboard" class="tabs">
      <button class="quiet" @click="tab = 'settings'">{{ tr('设置', 'Settings') }}</button
      ><button v-if="section === 'dashboards'" class="quiet" @click="tab = 'widgets'">
        {{ tr('组件', 'Widgets') }}</button
      ><button v-if="section === 'dashboards' && record.id" class="quiet" @click="openShare">
        {{ tr('分享', 'Share') }}</button
      ><button class="quiet" @click="tab = 'metadata'">{{ tr('元数据', 'Metadata') }}</button>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <DashboardEditor
      v-if="existingDashboard || tab === 'widgets'"
      :record="existingDashboard ? record : dashboardDraft"
      :workspace="workspace"
      @share="openShare"
      @draft="applyDashboardDraft"
      @saved="emit('saved', $event)"
    />
    <form v-if="!existingDashboard && tab !== 'share' && tab !== 'widgets'" class="panel" @submit.prevent="save">
      <template v-if="tab === 'settings'"
        ><div v-if="section === 'alarms'" class="native-form-grid">
          <label class="field"
            >{{ tr('状态', 'Status')
            }}<select v-model="draft.status">
              <option value="active">{{ tr('活动', 'Active') }}</option>
              <option value="cleared">{{ tr('已解除', 'Cleared') }}</option>
            </select></label
          ><label class="field"
            >{{ tr('严重程度（0–100）', 'Severity (0–100)')
            }}<input
              v-model.number="draft.severity"
              type="number"
              min="0"
              max="100"
              required /></label
          ><label class="field"
            >{{ tr('指派给', 'Assignee')
            }}<select v-model="draft.assignee_id">
              <option value="">{{ tr('未指派', 'Unassigned') }}</option>
              <option v-for="m in members" :key="m.id" :value="m.id">{{ m.name }}</option>
            </select></label
          ><label class="check"
            ><input type="checkbox" v-model="draft.acknowledged" />{{
              tr('已确认告警', 'Acknowledged')
            }}</label
          >
          <dl class="record full-width">
            <dt>{{ tr('设备', 'Device') }}</dt>
            <dd>{{ record.device_id || record.client_id || '—' }}</dd>
            <dt>{{ tr('规则', 'Rule') }}</dt>
            <dd>{{ record.rule_id }}</dd>
            <dt>{{ tr('消息', 'Message') }}</dt>
            <dd>{{ alarmCause(record) }}</dd>
          </dl>
        </div>
        <div v-else class="native-form-grid">
          <label class="field"
            >{{ tr('名称', 'Name') }} *<input v-model="draft.name" required /></label
          ><label class="field"
            >{{ tr('描述', 'Description') }}<textarea v-model="draft.description" /></label
          ><label class="field"
            >{{ tr('标签（逗号分隔）', 'Tags (comma separated)') }}<input v-model="tags" /></label
          ><label v-if="section === 'dashboards'" class="field"
            >{{ tr('缩略图', 'Thumbnail')
            }}<input type="file" accept="image/png,image/jpeg,image/webp" @change="thumbnail" /><img
              v-if="draft.thumbnail"
              :src="draft.thumbnail"
              :alt="tr('缩略图预览', 'Thumbnail preview')"
              style="max-width: 240px; max-height: 140px; object-fit: contain" /></label
          ><template v-if="section === 'custom-nodes'"
            ><label class="field"
              >{{ tr('图标', 'Icon')
              }}<select v-model="draft.icon">
                <option
                  v-for="v in [
                    'code',
                    'function',
                    'filter',
                    'calculator',
                    'database',
                    'mail',
                    'bell',
                    'send',
                  ]"
                  :key="v"
                >
                  {{ v }}
                </option>
              </select></label
            ><label class="field"
              >{{ tr('语言', 'Language')
              }}<select v-model.number="draft.logic.type">
                <option :value="0">Lua</option>
                <option :value="1">Go</option>
              </select></label
            ><label class="field full-width"
              >{{ tr('代码', 'Code') }} *<textarea
                v-model="draft.logic.value"
                rows="16"
                required
                spellcheck="false"
              /></label
          ></template></div
      ></template>
      <label v-else class="field"
        >{{ tr('元数据（JSON 对象）', 'Metadata (JSON object)')
        }}<textarea v-model="metadata" rows="14" required />
      </label>
      <div class="actions">
        <button :disabled="busy">{{ record.id ? tr('保存', 'Save') : tr('创建', 'Create') }}</button
        ><button
          v-if="record.id"
          class="quiet danger"
          type="button"
          :disabled="busy"
          @click="deleting = true"
        >
          {{ tr('删除', 'Delete') }}
        </button>
      </div>
    </form>
    <component :is="existingDashboard ? ModalDialog : 'div'" v-if="tab === 'share'" :busy="busy" @close="tab = 'widgets'">
    <form :class="existingDashboard ? 'modal-card' : 'panel'" @submit.prevent="saveShare">
      <h2>{{ tr('分享仪表盘', 'Share dashboard') }}</h2>
      <label class="field"
        >{{ tr('分享方式', 'Share Option')
        }}<select v-model="shared.share_state">
          <option value="none">{{ tr('不分享', 'None') }}</option>
          <option value="to_all_workspace_users">
            {{ tr('工作区成员', 'Workspace Members') }}
          </option>
          <option value="selected_users">{{ tr('指定用户', 'Selected Users') }}</option>
          <option value="public">{{ tr('公开', 'Public') }}</option>
        </select></label
      >
      <label v-if="shared.share_state === 'selected_users'" class="field"
        >{{ tr('允许访问的成员', 'Members with access')
        }}<select multiple v-model="shared.user_ids">
          <option v-for="m in members" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select></label
      >
      <div class="actions">
        <button :disabled="busy">{{ tr('保存分享设置', 'Save sharing') }}</button
        ><button type="button" class="quiet" :disabled="busy" @click="unsharing = true">
          {{ tr('取消分享', 'Stop sharing') }}
        </button>
      </div>
    </form>
    </component>
    <ModalDialog v-if="deleting || unsharing" :busy="busy" confirmation @close="() => { deleting = false; unsharing = false }">
      <section class="modal-card compact-modal">
        <h2>
          {{
            deleting
              ? tr('确认删除', 'Confirm deletion')
              : tr('确认取消分享', 'Confirm stop sharing')
          }}
        </h2>
        <p>{{ record.name || record.id }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <button
          class="quiet"
          :disabled="busy"
          @click="
            () => {
              deleting = false
              unsharing = false
            }
          "
        >
          {{ tr('取消', 'Cancel') }}</button
        ><button :disabled="busy" @click="confirm">{{ tr('确认', 'Confirm') }}</button>
      </section>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onMounted, computed } from 'vue'
import { editedMetadata } from '../api/editedMetadata'
import { alarmCause, alarmAcknowledged } from '../api/alarmPresentation'
import { tr } from '../i18n'
import { request, listAll, errorText } from '../api/platform'
import DashboardEditor from './DashboardEditor.vue'
const props = defineProps({ section: String, workspace: String, record: Object }),
  emit = defineEmits(['saved', 'deleted'])
const existingDashboard = computed(() => props.section === 'dashboards' && !!props.record.id)
const draft = ref({
    name: '',
    description: '',
    icon: 'code',
    status: 'active',
    severity: 0,
    assignee_id: '',
    ...JSON.parse(JSON.stringify(props.record)),
    acknowledged: alarmAcknowledged(props.record),
    logic: { type: 0, value: 'return message.payload', mode: 'sandboxed', ...props.record.logic },
  }),
  tab = ref(props.section === 'dashboards' && props.record.id ? 'widgets' : 'settings'),
  error = ref(''),
  notice = ref(''),
  busy = ref(false),
  deleting = ref(false),
  unsharing = ref(false),
  metadata = ref(JSON.stringify(props.record.metadata || {}, null, 2)),
  tags = ref((props.record.tags || []).join(', ')),
  members = ref([]),
  shared = ref({ share_state: props.record.share_state || 'none', user_ids: [] }),
  shareExists = ref(false)
let sharedUserIds = []
const dashboardDraft = computed(() => {
  let meta
  try {
    meta = JSON.parse(metadata.value || '{}')
  } catch {
    meta = props.record.metadata || {}
  }
  return { ...props.record, ...draft.value, metadata: meta }
})
function applyDashboardDraft(r) {
  draft.value = { ...draft.value, metadata: r.metadata, layout: r.layout }
  metadata.value = JSON.stringify(r.metadata, null, 2)
  notice.value = tr(
    '组件草稿已应用，请回到设置创建仪表盘。',
    'Widget draft applied. Return to Settings to create the dashboard.',
  )
}
const root = `/api/platform/${props.section === 'alarms' ? 'alarms' : 'ui'}/${props.workspace}/${props.section}`,
  url = root + (props.record.id ? '/' + props.record.id : '')
onMounted(async () => {
  if (['alarms', 'dashboards'].includes(props.section))
    try {
      members.value = await listAll('tenantMembers', { tenantId: props.workspace })
    } catch (e) {
      error.value = errorText(e)
    }
})
function thumbnail(e) {
  const f = e.target.files?.[0]
  if (!f) return
  if (f.size > 5 * 1024 * 1024 || !['image/png', 'image/jpeg', 'image/webp'].includes(f.type)) {
    error.value = tr(
      '请选择不超过 5 MB 的 PNG、JPEG 或 WebP 图片。',
      'Choose PNG, JPEG or WebP up to 5 MB.',
    )
    return
  }
  const reader = new FileReader()
  reader.onload = () => (draft.value.thumbnail = reader.result)
  reader.readAsDataURL(f)
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    const meta = JSON.parse(metadata.value)
    if (!meta || Array.isArray(meta) || typeof meta !== 'object')
      throw new Error(tr('元数据必须是对象。', 'Metadata must be an object.'))
    const latest = props.record.id ? await request(url) : {}
    const merged = editedMetadata(props.record.metadata, meta, latest.metadata)
    let body
    if (props.section === 'alarms')
      body = {
        status: draft.value.status,
        severity: draft.value.severity,
        assignee_id: draft.value.assignee_id || '',
        acknowledged: draft.value.acknowledged,
        metadata: merged,
      }
    else {
      body = {
        name: draft.value.name,
        description: draft.value.description,
        tags: tags.value
          .split(/[,，]/)
          .map((s) => s.trim())
          .filter(Boolean),
        metadata: merged,
      }
      if (props.section === 'dashboards') body.thumbnail = draft.value.thumbnail || ''
      if (props.section === 'custom-nodes') {
        body.logic = draft.value.logic
        body.icon = draft.value.icon
      } else if (!props.record.id) {
        body.layout = draft.value.layout || { lg: [], md: [], sm: [], xs: [], xxs: [] }
        body.share_state = 'none'
        body.thumbnail = draft.value.thumbnail || ''
      }
    }
    const r = await request(url, { method: props.record.id ? 'PATCH' : 'POST', body })
    notice.value = tr('已保存。', 'Saved.')
    emit('saved', r || { id: props.record.id })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function openShare() {
  tab.value = 'share'
  error.value = ''
  try {
    const s = await request(url + '/share')
    sharedUserIds = [...(s.user_ids || [])]
    shared.value = { share_state: draft.value.share_state || 'none', user_ids: [...sharedUserIds] }
    shareExists.value = shared.value.share_state !== 'none'
  } catch (e) {
    if (e.status === 404) {
      shareExists.value = false
      shared.value = { share_state: 'none', user_ids: [] }
    } else error.value = errorText(e)
  }
}
async function saveShare() {
  busy.value = true
  error.value = ''
  try {
    if (shared.value.share_state === 'selected_users' && !shared.value.user_ids.length)
      throw new Error(tr('至少选择一名用户。', 'Select at least one user.'))
    const removed = sharedUserIds.filter((id) => !shared.value.user_ids.includes(id))
    if (shared.value.share_state === 'selected_users' && removed.length) {
      await request(url + '/unshare', {
        method: 'DELETE',
        body: { dashboard_id: props.record.id, user_ids: removed },
      })
      sharedUserIds = sharedUserIds.filter((id) => !removed.includes(id))
    }
    await request(url + '/share', {
      method: shareExists.value ? 'PATCH' : 'POST',
      body: {
        dashboard_id: props.record.id,
        workspace_id: props.workspace,
        share_state: shared.value.share_state,
        ...(shared.value.share_state === 'selected_users'
          ? { user_ids: shared.value.user_ids.filter((id) => !sharedUserIds.includes(id)) }
          : {}),
      },
    })
    sharedUserIds = shared.value.share_state === 'selected_users' ? [...shared.value.user_ids] : []
    shareExists.value = shared.value.share_state !== 'none'
    draft.value.share_state = shared.value.share_state
    notice.value = tr('分享设置已保存。', 'Sharing saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function confirm() {
  busy.value = true
  try {
    await request(
      url + (unsharing.value ? '/share' : ''),
      unsharing.value
        ? {
            method: 'PATCH',
            body: {
              dashboard_id: props.record.id,
              workspace_id: props.workspace,
              share_state: 'none',
            },
          }
        : { method: 'DELETE' },
    )
    if (deleting.value) emit('deleted')
    else {
      shared.value = { share_state: 'none', user_ids: [] }
      shareExists.value = false
      sharedUserIds = []
      draft.value.share_state = 'none'
    }
    deleting.value = false
    unsharing.value = false
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
