<template>
  <form @submit.prevent="save" :aria-busy="busy">
    <h2>{{ tr('创建', 'Create') }} {{ sectionLabel(section) }}</h2>
    <label class="field"
      >{{ tr('名称', 'Name') }} *<input v-model="name" required :disabled="!!created" autofocus
    /></label>
    <template v-if="isDevice">
      <label class="field"
        >{{ tr('设备密钥（留空自动生成）', 'Key (generated if empty)')
        }}<input v-model="key" type="password" autocomplete="new-password" :disabled="!!created"
      /></label>
      <label class="field"
        >{{ tr('序列号', 'Serial') }}<input v-model="serial" :disabled="!!created"
      /></label>
      <label class="check"
        ><input
          type="checkbox"
          v-model="gateway"
          :disabled="!!created || section === 'gateways'"
        />{{ tr('网关模式', 'Gateway mode') }}</label
      >
      <label v-if="!gateway" class="field"
        >{{ tr('网关', 'Gateways')
        }}<select v-model="gatewayIds" multiple :disabled="!!created">
          <option v-for="g in gateways" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select></label
      >
      <label class="field"
        >{{ tr('设备类型', 'Device type')
        }}<select v-model="profileId" :disabled="!!created">
          <option value="">{{ tr('不指定设备类型', 'No device type') }}</option>
          <option v-for="p in profiles" :key="p.id" :value="p.id">
            {{ p.displayName || p.name }}
          </option>
        </select></label
      >
    </template>
    <label v-if="section === 'channels'" class="field"
      >{{ tr('路由（创建后不可修改）', 'Route (cannot be changed later)')
      }}<input v-model="alias" :disabled="!!created" pattern="[a-zA-Z0-9_-]+"
    /></label>
    <label v-if="section === 'groups'" class="field"
      >{{ tr('描述', 'Description') }}<textarea v-model="description" :disabled="!!created" />
    </label>
    <label class="field"
      >{{ tr('标签（回车添加）', 'Tags (press Enter to add)')
      }}<input v-model="tagInput" @keydown.enter.prevent="addTag" :disabled="!!created"
    /></label>
    <div class="tags-editor">
      <button
        v-for="tag in tags"
        :key="tag"
        type="button"
        class="quiet"
        :disabled="!!created"
        @click="tags = tags.filter((t) => t !== tag)"
      >
        {{ tag }} ×
      </button>
    </div>
    <label class="field"
      >{{ tr('元数据', 'Metadata') }}<textarea v-model="metadata" rows="4" :disabled="!!created" />
    </label>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="created && error" class="warning">
      {{
        tr(
          '资源已创建，重试只会继续未完成的配置，不会重复创建。',
          'The resource exists. Retry finishes pending configuration without creating a duplicate.',
        )
      }}
      {{ created.id }}
    </p>
    <div class="actions">
      <button type="button" class="quiet" :disabled="busy" @click="emit('close')">
        {{ tr('关闭', 'Close') }}</button
      ><button :disabled="busy">
        {{
          busy
            ? tr('处理中…', 'Working…')
            : created
              ? tr('继续配置', 'Finish configuration')
              : tr('创建', 'Create')
        }}
      </button>
    </div>
  </form>
</template>
<script setup>
import { ref, computed, onMounted } from 'vue'
import { tr, sectionLabel } from '../i18n'
import { runOperation, listAll, errorText } from '../api/platform'
import { resourceConfig } from '../api/resourceWorkflow'
const props = defineProps({
  section: String,
  workspace: String,
  parentGroup: String,
  initial: Object,
})
const emit = defineEmits(['saved', 'close'])
const name = ref(props.initial ? `${props.initial.name} (copy)` : ''),
  key = ref(''),
  serial = ref(props.initial?.externalId || ''),
  alias = ref(''),
  description = ref(props.initial?.description || ''),
  gateway = ref(props.section === 'gateways'),
  gatewayIds = ref([]),
  profileId = ref(''),
  profiles = ref([]),
  gateways = ref([]),
  tagInput = ref(''),
  tags = ref([...(props.initial?.attributes?.tags || [])]),
  metadata = ref(JSON.stringify(props.initial?.attributes?.metadata || {}, null, 2)),
  error = ref(''),
  busy = ref(false),
  created = ref(null)
let keyDone = false,
  groupDone = false
const isDevice = computed(() => ['devices', 'gateways'].includes(props.section))
function addTag() {
  if (tagInput.value.trim() && !tags.value.includes(tagInput.value.trim()))
    tags.value.push(tagInput.value.trim())
  tagInput.value = ''
}
onMounted(async () => {
  if (!isDevice.value) return
  try {
    ;[profiles.value, gateways.value] = await Promise.all([
      listAll('profiles', { tenantId: props.workspace, objectKind: 'entity', kind: 'device' }),
      listAll('entities', {
        tenantId: props.workspace,
        kind: 'device',
        attributesContains: { is_gateway: true },
      }),
    ])
  } catch (e) {
    error.value = errorText(e)
  }
})
async function save() {
  busy.value = true
  error.value = ''
  try {
    if (!created.value) {
      addTag()
      const parsed = JSON.parse(metadata.value)
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object')
        throw new Error(tr('元数据必须是 JSON 对象。', 'Metadata must be a JSON object.'))
      const c = resourceConfig[props.section],
        input = {
          name: name.value.trim(),
          tenantId: props.workspace,
          attributes: { tags: tags.value, metadata: parsed },
        }
      if (!input.name) throw new Error(tr('请输入名称。', 'Enter a name.'))
      if (c.kind) input.kind = c.kind
      if (isDevice.value) {
        Object.assign(input.attributes, {
          is_gateway: gateway.value,
          gateways: gatewayIds.value,
          profile: 'client',
        })
        if (serial.value) input.externalId = serial.value
        if (profileId.value) input.profileId = profileId.value
      }
      if (props.section === 'channels') {
        if (alias.value) input.alias = alias.value
        Object.assign(input.attributes, { status: 'active', route: alias.value })
      }
      if (props.section === 'groups') {
        input.groupType = 'object'
        input.description = description.value
      }
      created.value = await runOperation('create' + c.type, { input })
    }
    if (isDevice.value && !keyDone) {
      await runOperation('createSharedKey', {
        entityId: created.value.id,
        input: key.value ? { key: key.value } : {},
      })
      keyDone = true
      key.value = ''
    }
    if (props.parentGroup && !groupDone) {
      const c = resourceConfig[props.section]
      if (props.section === 'groups')
        await runOperation('setGroupParent', { id: created.value.id, parentId: props.parentGroup })
      else
        await runOperation(`add${c.type}ToObjectGroup`, {
          [c.type === 'Entity' ? 'entityId' : 'resourceId']: created.value.id,
          objectGroupId: props.parentGroup,
        })
      groupDone = true
    }
    emit('saved', created.value)
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
