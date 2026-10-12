<template>
  <div>
    <div class="tabs">
      <button class="quiet" :class="{ active: subtab === 'settings' }" @click="subtab = 'settings'">
        {{ tr('配置', 'Configuration') }}</button
      ><button
        class="quiet"
        :class="{ active: subtab === 'metadata' }"
        @click="subtab = 'metadata'"
      >
        {{ tr('元数据', 'Metadata') }}
      </button>
      <template v-if="isDevice">
        <button
          class="quiet"
          :class="{ active: subtab === 'gateways' }"
          @click="subtab = 'gateways'"
        >
          {{ tr('网关', 'Gateways') }}
        </button>
        <button
          class="quiet"
          :class="{ active: subtab === 'device-type' }"
          @click="subtab = 'device-type'"
        >
          {{ tr('设备类型', 'Device Type') }}
        </button>
      </template>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <div class="panel" v-if="subtab === 'settings'">
      <div class="setting-row">
        <strong>{{ tr('名称', 'Name') }}</strong
        ><span>{{ record.name }}</span
        ><button class="quiet" @click="edit('name', record.name)">{{ tr('编辑', 'Edit') }}</button>
      </div>
      <div class="setting-row">
        <strong>ID</strong><span class="mono">{{ record.id }}</span
        ><button class="quiet" @click="copy(record.id)">{{ tr('复制', 'Copy') }}</button>
      </div>
      <div v-if="isDevice" class="setting-row">
        <strong>{{ tr('设备密钥', 'Device key') }}</strong
        ><span class="mono">{{ secret || '••••••••••••' }}</span
        ><button class="quiet" :disabled="busy" @click="reveal">
          {{ secret ? tr('隐藏', 'Hide') : tr('显示', 'Reveal') }}</button
        ><button class="quiet" :disabled="busy" @click="keyEditor = true">
          {{ tr('编辑', 'Edit') }}
        </button>
      </div>
      <div v-if="isDevice" class="setting-row">
        <strong>{{ tr('序列号', 'Serial') }}</strong
        ><span>{{ record.externalId || '—' }}</span
        ><button class="quiet" @click="edit('externalId', record.externalId || '')">
          {{ tr('编辑', 'Edit') }}
        </button>
      </div>
      <div v-if="section === 'channels'" class="setting-row">
        <strong>{{ tr('路由', 'Route') }}</strong
        ><span>{{ record.alias || record.attributes?.route || '—' }}</span>
      </div>
      <div v-if="section === 'groups'" class="setting-row">
        <strong>{{ tr('描述', 'Description') }}</strong
        ><span>{{ record.description || '—' }}</span
        ><button class="quiet" @click="edit('description', record.description || '')">
          {{ tr('编辑', 'Edit') }}
        </button>
      </div>
      <div class="setting-row">
        <strong>{{ tr('标签', 'Tags') }}</strong
        ><span
          ><span v-for="tag in tagsOf(record)" :key="tag" class="badge">{{ tag }}</span></span
        ><button class="quiet" @click="edit('tags', tagsOf(record).join(', '))">
          {{ tr('编辑', 'Edit') }}
        </button>
      </div>
      <div v-if="isDevice" class="setting-row">
        <strong>{{ tr('网关模式', 'Gateway mode') }}</strong
        ><span>{{
          record.attributes?.is_gateway ? tr('已开启', 'Enabled') : tr('未开启', 'Disabled')
        }}</span
        ><button
          class="quiet"
          :disabled="busy"
          @click="confirmAction = { kind: 'gateway', value: !record.attributes?.is_gateway }"
        >
          {{ tr('修改', 'Change') }}
        </button>
      </div>
      <div class="setting-row">
        <strong>{{ tr('元数据', 'Metadata') }}</strong
        ><span /><button class="quiet" @click="subtab = 'metadata'">
          {{ tr('查看元数据', 'View metadata') }}
        </button>
      </div>
      <div class="setting-row">
        <strong>{{ tr('状态', 'Status') }}</strong
        ><span class="badge">{{
          statusOf(record) === 'active'
            ? tr('已启用', 'Enabled')
            : statusOf(record) === 'deleted'
              ? tr('已删除', 'Deleted')
              : tr('已禁用', 'Disabled')
        }}</span
        ><button
          v-if="!record.deletedAt"
          class="quiet"
          :disabled="busy"
          @click="confirmAction = { kind: 'status', value: statusOf(record) !== 'active' }"
        >
          {{ statusOf(record) === 'active' ? tr('禁用', 'Disable') : tr('启用', 'Enable') }}
        </button>
      </div>
      <div v-if="section === 'groups'" class="setting-row">
        <strong>{{ tr('上级分组', 'Parent group') }}</strong
        ><span>{{ groups.find((g) => g.id === record.parentId)?.name || tr('无', 'None') }}</span
        ><button class="quiet" @click="edit('parentId', record.parentId || '')">
          {{ tr('修改', 'Change') }}
        </button>
      </div>
      <div class="settings-footer">
        <small>{{ tr('创建时间', 'Created') }}: {{ record.createdAt }}</small
        ><button
          v-if="!record.deletedAt"
          class="quiet danger"
          :disabled="busy || record.managedBy === 'config'"
          @click="confirmAction = { kind: 'delete' }"
        >
          {{ tr('删除', 'Delete') }}</button
        ><button v-else :disabled="busy" @click="confirmAction = { kind: 'restore' }">
          {{ tr('恢复', 'Restore') }}
        </button>
      </div>
    </div>
    <div v-else-if="subtab === 'gateways'" class="panel">
      <div class="actions">
        <h2>{{ tr('网关配置', 'Gateway Configuration') }}</h2>
        <button class="quiet" @click="edit('gateways', [...(record.attributes?.gateways || [])])">
          {{ tr('添加网关', 'Add Gateways') }}
        </button>
      </div>
      <table>
        <thead>
          <tr>
            <th>{{ tr('名称', 'Name') }}</th>
            <th>ID</th>
            <th>{{ tr('网关链接', 'Gateway link') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="gid in record.attributes?.gateways || []" :key="gid">
            <td>{{ gateways.find((g) => g.id === gid)?.name || gid }}</td>
            <td>{{ gid }}</td>
            <td>
              <router-link :to="`/domain/${workspace}/gateways/${gid}`">{{
                tr('打开', 'Open')
              }}</router-link>
            </td>
          </tr>
          <tr v-if="!record.attributes?.gateways?.length">
            <td colspan="3">{{ tr('尚未分配网关。', 'No gateways assigned.') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <DeviceTypeBinding
      v-else-if="subtab === 'device-type'"
      :record="record"
      :workspace="workspace"
      :profiles="profiles"
      @updated="emit('updated')"
    />
    <div v-else class="panel">
      <div class="actions">
        <h2>{{ tr('元数据', 'Metadata') }}</h2>
        <button
          class="quiet"
          @click="edit('metadata', JSON.stringify(record.attributes?.metadata || {}, null, 2))"
        >
          {{ tr('编辑', 'Edit') }}
        </button>
      </div>
      <pre>{{ JSON.stringify(record.attributes?.metadata || {}, null, 2) }}</pre>
      <ResourceGeometry
        :record="record"
        :section="section"
        :workspace="workspace"
        @saved="emit('updated')"
      />
    </div>
    <DeviceKeyEditor
      v-if="keyEditor"
      :record="record"
      :workspace="workspace"
      @close="keyEditor = false"
      @updated="secret = ''"
    />
    <ModalDialog v-if="field" :busy="busy" @close="field = ''">
      <form class="modal-card compact-modal" @submit.prevent="save">
        <h2>{{ tr('编辑', 'Edit') }} · {{ fieldTitle }}</h2>
        <label class="field"
          >{{ fieldTitle
          }}<select v-if="field === 'parentId'" v-model="value">
            <option value="">{{ tr('无上级分组', 'No parent group') }}</option>
            <option v-for="g in groups.filter((g) => g.id !== record.id)" :key="g.id" :value="g.id">
              {{ g.name }}
            </option></select
          ><select v-else-if="field === 'gateways'" v-model="value" multiple>
            <option
              v-for="g in gateways.filter((g) => g.id !== record.id)"
              :key="g.id"
              :value="g.id"
            >
              {{ g.name }}
            </option></select
          ><select v-else-if="field === 'profileId'" v-model="value">
            <option value="">{{ tr('不指定设备类型', 'No device type') }}</option>
            <option v-for="p in profiles" :key="p.id" :value="p.id">
              {{ p.displayName || p.name }}
            </option></select
          ><textarea
            v-else-if="['description', 'metadata'].includes(field)"
            v-model="value"
            rows="8" /><input v-else v-model="value" :required="field === 'name'"
        /></label>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="field = ''">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy">{{ tr('保存', 'Save') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="confirmAction" :busy="busy" confirmation @close="confirmAction = null">
      <div class="modal-card compact-modal">
        <h2>{{ tr('确认操作', 'Confirm action') }}</h2>
        <p>{{ record.name }} · {{ record.id }}</p>
        <p>
          {{
            confirmAction.kind === 'delete'
              ? tr(
                  '删除后，该资源将无法正常使用。确认删除此资源？',
                  'This resource will no longer be usable. Delete it?',
                )
              : tr('确认修改此资源的配置或状态？', 'Apply this change to the resource?')
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button class="quiet" :disabled="busy" @click="confirmAction = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy" @click="applyAction">{{ tr('确认', 'Confirm') }}</button>
        </div>
      </div>
    </ModalDialog>
  </div>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import ResourceGeometry from './ResourceGeometry.vue'
import { ref, computed, watch } from 'vue'
import DeviceTypeBinding from './DeviceTypeBinding.vue'
import DeviceKeyEditor from './DeviceKeyEditor.vue'
import { tr } from '../i18n'
import { runOperation, listAll, errorText } from '../api/platform'
import {
  resourceConfig,
  readResource,
  statusOf,
  tagsOf,
  setResourceStatus,
  deviceKey,
} from '../api/resourceWorkflow'
const props = defineProps({ section: String, workspace: String, record: Object }),
  emit = defineEmits(['updated', 'deleted'])
const keyEditor = ref(false)
const subtab = ref('settings'),
  field = ref(''),
  value = ref(''),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  secret = ref(''),
  confirmAction = ref(null),
  groups = ref([]),
  gateways = ref([]),
  profiles = ref([])
const isDevice = computed(() => ['devices', 'gateways'].includes(props.section)),
  fieldTitle = computed(
    () =>
      ({
        gateways: tr('网关', 'Gateways'),
        profileId: tr('设备类型', 'Device type'),
        name: tr('名称', 'Name'),
        externalId: tr('序列号', 'Serial'),
        description: tr('描述', 'Description'),
        tags: tr('标签（逗号分隔）', 'Tags (comma separated)'),
        metadata: tr('元数据', 'Metadata'),
        parentId: tr('上级分组', 'Parent group'),
      })[field.value],
  )
watch(
  () => props.record.id,
  async () => {
    secret.value = ''
    field.value = ''
    error.value = ''
    notice.value = ''
    confirmAction.value = null
    if (isDevice.value)
      try {
        ;[gateways.value, profiles.value] = await Promise.all([
          listAll('entities', {
            tenantId: props.workspace,
            kind: 'device',
            attributesContains: { is_gateway: true },
          }),
          listAll('profiles', { tenantId: props.workspace, objectKind: 'entity', kind: 'device' }),
        ])
      } catch (e) {
        error.value = errorText(e)
      }
    if (props.section === 'groups')
      try {
        groups.value = await listAll('groups', { tenantId: props.workspace })
      } catch (e) {
        error.value = errorText(e)
      }
  },
  { immediate: true },
)
function edit(k, v) {
  field.value = k
  value.value = v
  error.value = ''
}
async function copy(v) {
  try {
    await navigator.clipboard.writeText(v)
    notice.value = tr('已复制。', 'Copied.')
  } catch (e) {
    error.value = errorText(e)
  }
}
async function reveal() {
  if (secret.value) {
    secret.value = ''
    return
  }
  busy.value = true
  try {
    const target = props.record.id
    const result = await deviceKey(target)
    if (props.record.id === target) secret.value = result
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    const latest = await readResource(props.section, props.record.id, props.workspace)
    if (field.value === 'parentId') {
      await runOperation(value.value ? 'setGroupParent' : 'removeGroupParent', {
        id: latest.id,
        ...(value.value ? { parentId: value.value } : {}),
      })
    } else {
      const input = {}
      if (['tags', 'metadata', 'gateways'].includes(field.value)) {
        const v =
          field.value === 'gateways'
            ? value.value
            : field.value === 'tags'
              ? value.value
                  .split(/[,，]/)
                  .map((s) => s.trim())
                  .filter(Boolean)
              : JSON.parse(value.value)
        if (field.value === 'metadata' && (!v || typeof v !== 'object' || Array.isArray(v)))
          throw new Error(tr('请输入 JSON 对象。', 'Enter a JSON object.'))
        input.attributes = { ...latest.attributes, [field.value]: v }
      } else input[field.value] = field.value === 'profileId' ? value.value || null : value.value
      await runOperation('update' + resourceConfig[props.section].type, { id: latest.id, input })
    }
    field.value = ''
    emit('updated')
    notice.value = tr('已保存。', 'Saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function applyAction() {
  busy.value = true
  error.value = ''
  try {
    const a = confirmAction.value
    if (a.kind === 'status')
      await setResourceStatus(props.section, props.record, a.value, props.workspace)
    else if (a.kind === 'gateway') {
      const latest = await readResource(props.section, props.record.id, props.workspace)
      await runOperation('updateEntity', {
        id: latest.id,
        input: { attributes: { ...latest.attributes, is_gateway: a.value } },
      })
    } else
      await runOperation(
        (a.kind === 'delete' ? 'delete' : 'restore') + resourceConfig[props.section].type,
        { id: props.record.id },
      )
    confirmAction.value = null
    emit(a.kind === 'delete' ? 'deleted' : 'updated')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
