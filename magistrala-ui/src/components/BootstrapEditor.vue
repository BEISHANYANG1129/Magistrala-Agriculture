<template>
  <section>
    <nav class="tabs">
      <button class="quiet" @click="tab = 'configuration'">{{ tr('配置', 'Configuration') }}</button
      ><template v-if="record.id && !profileMode"
        ><button class="quiet" @click="tab = 'bindings'">{{ tr('资源绑定', 'Bindings') }}</button
        ><button class="quiet" @click="tab = 'certificates'">
          {{ tr('证书', 'Certificates') }}
        </button></template
      ><button v-if="profileMode && record.id" class="quiet" @click="tab = 'preview'">
        {{ tr('渲染预览', 'Render preview') }}
      </button>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <form v-if="tab === 'configuration'" class="panel" @submit.prevent="save">
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('名称', 'Name') }}<input v-model="draft.name" :required="profileMode" /></label
        ><template v-if="profileMode"
          ><label class="field"
            >{{ tr('描述', 'Description') }}<textarea v-model="draft.description" /></label
          ><label class="field"
            >{{ tr('模板格式', 'Template format')
            }}<select v-model="draft.content_format">
              <option v-for="v in ['go-template', 'raw', 'json', 'yaml', 'toml']" :key="v">
                {{ v }}
              </option>
            </select></label
          ><label class="field"
            >{{ tr('内容类型', 'Content type')
            }}<select v-model="draft.content_type">
              <option
                v-for="v in [
                  'application/json',
                  'application/yaml',
                  'application/toml',
                  'text/plain',
                ]"
                :key="v"
              >
                {{ v }}
              </option>
            </select></label
          ><label class="field full-width"
            >{{ tr('模板内容', 'Template content')
            }}<textarea v-model="draft.content_template" rows="12" /></label
          ><label class="field full-width"
            >{{ tr('默认变量（JSON 对象）', 'Default variables (JSON object)')
            }}<textarea v-model="contextText" rows="5" />
          </label>
          <section class="full-width">
            <h3>{{ tr('资源插槽', 'Binding slots') }}</h3>
            <div v-for="(s, i) in draft.binding_slots" :key="i" class="toolbar">
              <input v-model="s.name" :aria-label="tr('插槽名称', 'Slot name')" required /><select
                v-model="s.type"
                :aria-label="tr('资源类型', 'Resource type')"
              >
                <option value="device">{{ tr('设备', 'Device') }}</option>
                <option value="channel">{{ tr('通道', 'Channel') }}</option></select
              ><label class="check"
                ><input type="checkbox" v-model="s.required" />{{ tr('必需', 'Required') }}</label
              ><label class="field"
                >{{ tr('快照字段（逗号分隔）', 'Snapshot fields (comma separated)')
                }}<input
                  :value="(s.fields || []).join(', ')"
                  @input="
                    s.fields = $event.target.value
                      .split(',')
                      .map((v) => v.trim())
                      .filter(Boolean)
                  " /></label
              ><button type="button" class="quiet" @click="draft.binding_slots.splice(i, 1)">
                {{ tr('移除', 'Remove') }}
              </button>
            </div>
            <button
              type="button"
              class="quiet"
              @click="draft.binding_slots.push({ name: '', type: 'device', required: true })"
            >
              + {{ tr('添加插槽', 'Add slot') }}
            </button>
          </section></template
        >
        <template v-else
          ><label class="field"
            >{{ tr('外部设备编号', 'External device ID') }} *<input
              v-model="draft.external_id"
              required
              :disabled="!!record.id" /></label
          ><label class="field"
            >{{
              tr(
                '设备引导密钥（留空自动生成或保持原值）',
                'Bootstrap key (leave blank to generate or keep)',
              )
            }}<input
              type="password"
              v-model="rootKey"
              autocomplete="new-password"
              minlength="10" /></label
          ><label class="field"
            >{{ tr('配置模板', 'Configuration profile')
            }}<select v-model="draft.profile_id">
              <option value="">{{ tr('不使用模板', 'No template') }}</option>
              <option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select></label
          ><label class="field full-width"
            >{{ tr('设备配置内容', 'Device configuration content')
            }}<textarea v-model="draft.content" rows="9" /></label
          ><label class="field full-width"
            >{{ tr('模板变量（JSON 对象）', 'Template variables (JSON object)')
            }}<textarea v-model="contextText" rows="6" /></label
        ></template>
      </div>
      <div class="actions">
        <button :disabled="busy">{{ record.id ? tr('保存', 'Save') : tr('创建', 'Create') }}</button
        ><button
          v-if="record.id && !profileMode"
          type="button"
          class="quiet"
          @click="confirmAction = record.status === 'enabled' ? 'disable' : 'enable'"
        >
          {{ record.status === 'enabled' ? tr('禁用', 'Disable') : tr('启用', 'Enable') }}</button
        ><button
          v-if="record.id"
          class="quiet danger"
          type="button"
          @click="confirmAction = 'delete'"
        >
          {{ tr('删除', 'Delete') }}
        </button>
      </div>
    </form>
    <form v-if="tab === 'bindings'" class="panel" @submit.prevent="saveBindings">
      <p>
        {{
          tr(
            '将模板中的设备和通道插槽绑定到当前工作区的实际资源。',
            'Bind template slots to devices and channels in this workspace.',
          )
        }}
      </p>
      <label v-for="s in slots" :key="s.name" class="field"
        >{{ s.name }} ({{ s.type }})<select v-model="bindings[s.name]" :required="s.required">
          <option value="">{{ tr('请选择', 'Select') }}</option>
          <option v-for="r in s.type === 'device' ? devices : channels" :key="r.id" :value="r.id">
            {{ r.name }}
          </option>
        </select></label
      >
      <p v-if="!slots.length">
        {{
          tr(
            '先在配置中选择带有插槽的模板。',
            'Select a profile with binding slots in Configuration first.',
          )
        }}
      </p>
      <button :disabled="busy || !slots.length">{{ tr('保存绑定', 'Save bindings') }}</button
      ><button type="button" class="quiet" :disabled="busy" @click="refreshBindings">
        {{ tr('刷新资源快照', 'Refresh snapshots') }}
      </button>
    </form>
    <form v-if="tab === 'certificates'" class="panel" @submit.prevent="saveCertificates">
      <label
        v-for="c in [
          ['client_cert', '客户端证书', 'Client certificate'],
          ['client_key', '客户端私钥', 'Client private key'],
          ['ca_cert', 'CA 证书', 'CA certificate'],
        ]"
        :key="c[0]"
        class="field"
        >{{ tr(c[1], c[2])
        }}<textarea
          v-model="certificates[c[0]]"
          rows="6"
          spellcheck="false"
          :placeholder="
            c[0] === 'client_key'
              ? tr('留空保留现有私钥', 'Leave blank to keep existing private key')
              : ''
          "
        /></label
      ><button :disabled="busy">{{ tr('保存证书', 'Save certificates') }}</button>
    </form>
    <form v-if="tab === 'preview'" class="panel" @submit.prevent="render">
      <label class="field"
        >{{ tr('已有配置编号（可选）', 'Existing configuration ID (optional)')
        }}<input v-model="previewConfigId"
      /></label>
      <label class="field"
        >{{ tr('预览配置（JSON 对象，可选）', 'Preview configuration (JSON object, optional)')
        }}<textarea v-model="previewConfig" rows="5" />
      </label>
      <label class="field"
        >{{ tr('预览变量（JSON 对象）', 'Preview variables (JSON object)')
        }}<textarea v-model="previewContext" rows="6" /></label
      ><label class="field"
        >{{ tr('资源绑定快照（JSON 数组，可选）', 'Binding snapshots (JSON array, optional)')
        }}<textarea v-model="previewBindings" rows="5" />
      </label>
      <button :disabled="busy">{{ tr('渲染预览', 'Render preview') }}</button>
      <pre v-if="preview">{{
        typeof preview === 'string' ? preview : JSON.stringify(preview, null, 2)
      }}</pre>
    </form>
    <ModalDialog v-if="confirmAction" :busy="busy" confirmation @close="confirmAction = ''">
      <section class="modal-card compact-modal">
        <h2>{{ tr('确认操作', 'Confirm action') }}</h2>
        <p>
          {{
            confirmAction === 'delete'
              ? tr('删除此引导配置？', 'Delete this bootstrap configuration?')
              : confirmAction === 'disable'
                ? tr('禁用此引导配置？', 'Disable this enrollment?')
                : tr('启用此引导配置？', 'Enable this enrollment?')
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="quiet" @click="confirmAction = ''">{{ tr('取消', 'Cancel') }}</button
        ><button :disabled="busy" @click="applyAction">{{ tr('确认', 'Confirm') }}</button>
      </section>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, computed, onMounted } from 'vue'
import { tr } from '../i18n'
import { request, listAll, errorText } from '../api/platform'
const props = defineProps({ record: Object, workspace: String, profileMode: Boolean }),
  emit = defineEmits(['saved', 'deleted']),
  root = `/api/platform/bootstrap/${props.workspace}/devices`,
  path = props.profileMode ? '/bootstrap/profiles' : '/configs',
  url = root + path + (props.record.id ? '/' + props.record.id : '')
const draft = ref({
    name: '',
    description: '',
    external_id: '',
    content: '',
    profile_id: '',
    content_format: 'json',
    content_type: 'application/json',
    content_template: '',
    binding_slots: [],
    ...JSON.parse(JSON.stringify(props.record)),
  }),
  tab = ref('configuration'),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  contextText = ref(
    JSON.stringify(
      props.profileMode ? props.record.defaults || {} : props.record.render_context || {},
      null,
      2,
    ),
  ),
  rootKey = ref(''),
  profiles = ref([]),
  devices = ref([]),
  channels = ref([]),
  bindings = ref({}),
  certificates = ref({
    client_cert: props.record.client_cert || '',
    client_key: '',
    ca_cert: props.record.ca_cert || '',
  }),
  confirmAction = ref(''),
  previewContext = ref('{}'),
  previewConfigId = ref(''),
  previewConfig = ref('{}'),
  previewBindings = ref('[]'),
  preview = ref(null)
const slots = computed(
  () => profiles.value.find((p) => p.id === draft.value.profile_id)?.binding_slots || [],
)
function object(s) {
  const v = JSON.parse(s)
  if (!v || Array.isArray(v) || typeof v !== 'object')
    throw new Error(tr('请输入 JSON 对象。', 'Enter a JSON object.'))
  return v
}
onMounted(async () => {
  if (props.profileMode) return
  try {
    const [p, d, c] = await Promise.all([
      request(root + '/bootstrap/profiles?limit=100&offset=0'),
      listAll('entities', { tenantId: props.workspace, kind: 'device' }),
      listAll('resources', { tenantId: props.workspace, kind: 'channel' }),
    ])
    profiles.value = p.profiles || []
    devices.value = d
    channels.value = c
    if (props.record.id) {
      const b = await request(root + '/bootstrap/enrollments/' + props.record.id + '/bindings')
      bindings.value = Object.fromEntries((b.bindings || []).map((v) => [v.slot, v.resource_id]))
    }
  } catch (e) {
    error.value = errorText(e)
  }
})
async function save() {
  busy.value = true
  error.value = ''
  try {
    if (props.record.id && !props.profileMode && props.record.profile_id && !draft.value.profile_id)
      throw new Error(tr('更换模板时必须选择一个模板。', 'Select a profile to reassign it.'))
    let body
    if (props.profileMode) {
      body = {
        name: draft.value.name,
        description: draft.value.description,
        content_format: draft.value.content_format,
        content_type: draft.value.content_type,
        content_template: draft.value.content_template,
        defaults: object(contextText.value),
        binding_slots: draft.value.binding_slots,
      }
      if (new Set(body.binding_slots.map((s) => s.name)).size !== body.binding_slots.length)
        throw new Error(tr('插槽名称不能重复。', 'Slot names must be unique.'))
    } else
      body = {
        name: draft.value.name,
        content: draft.value.content,
        render_context: object(contextText.value),
        ...(rootKey.value ? { external_key: rootKey.value } : {}),
        ...(!props.record.id
          ? {
              external_id: draft.value.external_id,
              ...(draft.value.profile_id ? { profile_id: draft.value.profile_id } : {}),
            }
          : {}),
      }
    const r = await request(url, { method: props.record.id ? 'PATCH' : 'POST', body })
    rootKey.value = ''
    if (
      props.record.id &&
      !props.profileMode &&
      draft.value.profile_id !== props.record.profile_id
    ) {
      await request(root + '/bootstrap/enrollments/' + props.record.id + '/profile', {
        method: 'PATCH',
        body: { profile_id: draft.value.profile_id },
      })
    }
    emit('saved', r || { id: props.record.id })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function task(path, method, body) {
  busy.value = true
  error.value = ''
  try {
    const r = await request(path, { method, body })
    notice.value = tr('已完成。', 'Completed.')
    return r
  } catch (e) {
    error.value = errorText(e)
    throw e
  } finally {
    busy.value = false
  }
}
async function saveBindings() {
  try {
    await task(root + '/bootstrap/enrollments/' + props.record.id + '/bindings', 'PUT', {
      bindings: slots.value
        .filter((s) => bindings.value[s.name])
        .map((s) => ({ slot: s.name, type: s.type, resource_id: bindings.value[s.name] })),
    })
  } catch {}
}
async function refreshBindings() {
  try {
    await task(root + '/bootstrap/enrollments/' + props.record.id + '/bindings/refresh', 'POST')
  } catch {}
}
async function saveCertificates() {
  try {
    await task(
      root + '/configs/certs/' + props.record.id,
      'PATCH',
      Object.fromEntries(
        Object.entries(certificates.value).filter(([k, v]) => k !== 'client_key' || v),
      ),
    )
    certificates.value.client_key = ''
  } catch {}
}
async function render() {
  error.value = ''
  preview.value = null
  try {
    const snapshots = JSON.parse(previewBindings.value)
    if (
      !Array.isArray(snapshots) ||
      snapshots.some((s) => !s || typeof s !== 'object' || Array.isArray(s))
    )
      throw new Error(
        tr('绑定快照必须是 JSON 对象数组。', 'Binding snapshots must be an array of JSON objects.'),
      )
    preview.value = await task(url + '/render-preview', 'POST', {
      render_context: object(previewContext.value),
      config: object(previewConfig.value),
      ...(previewConfigId.value.trim() ? { config_id: previewConfigId.value.trim() } : {}),
      ...(snapshots.length ? { bindings: snapshots } : {}),
    })
  } catch (e) {
    error.value = errorText(e)
  }
}
async function applyAction() {
  try {
    const action = confirmAction.value
    await task(
      url + (action === 'delete' ? '' : '/' + action),
      action === 'delete' ? 'DELETE' : 'POST',
    )
    confirmAction.value = ''
    if (action === 'delete') emit('deleted')
    else emit('saved', { id: props.record.id })
  } catch {}
}
</script>
