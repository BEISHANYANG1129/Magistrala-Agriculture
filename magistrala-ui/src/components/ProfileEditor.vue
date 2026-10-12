<template>
  <div>
    <nav class="tabs">
      <button
        class="quiet"
        :class="{ active: tab === 'configuration' }"
        @click="tab = 'configuration'"
      >
        {{ tr('配置', 'Configuration') }}</button
      ><button
        v-if="record.id"
        class="quiet"
        :class="{ active: tab === 'versions' }"
        @click="tab = 'versions'"
      >
        {{ tr('版本', 'Versions') }}
      </button>
    </nav>
    <p v-if="error" class="error">{{ error }}</p>
    <form v-if="tab === 'configuration'" class="panel" @submit.prevent="save">
      <div class="native-form-grid">
        <label class="field">{{ tr('名称', 'Name') }} *<input v-model="name" required /></label
        ><label class="field"
          >{{ tr('标识键', 'Key') }} *<input
            v-model="key"
            required
            :disabled="!!record.id" /></label
        ><label class="field full-width"
          >{{ tr('描述', 'Description') }}<textarea v-model="description" /></label
        ><label v-if="record.id" class="field"
          >{{ tr('状态', 'Status')
          }}<select v-model="status">
            <option value="active">{{ tr('已启用', 'Active') }}</option>
            <option value="disabled">{{ tr('已禁用', 'Disabled') }}</option>
            <option value="deprecated">{{ tr('已弃用', 'Deprecated') }}</option>
          </select></label
        >
      </div>
      <p v-if="!record.id" class="muted">
        {{
          tr(
            '先创建设备类型，再在“版本”中添加属性。',
            'Create the device type first, then add attributes in Versions.',
          )
        }}
      </p>
      <button :disabled="busy">{{ record.id ? tr('保存', 'Save') : tr('创建', 'Create') }}</button>
    </form>
    <div v-else>
      <button @click="openVersion">+ {{ tr('创建版本', 'Create version') }}</button>
      <div v-for="v in versions" :key="v.id" class="panel">
        <div class="actions">
          <h2>{{ tr('版本', 'Version') }} {{ v.version }} · {{ v.status }}</h2>
          <button
            class="quiet"
            :disabled="busy"
            @click="versionStatus = { id: v.id, status: v.status }"
          >
            {{ tr('修改状态', 'Change status') }}
          </button>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{{ tr('属性', 'Attribute') }}</th>
                <th>{{ tr('类型', 'Type') }}</th>
                <th>{{ tr('必填', 'Required') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(p, k) in v.jsonSchema?.properties || {}" :key="k">
                <td>{{ k }}</td>
                <td>{{ p.type }}</td>
                <td>{{ v.jsonSchema.required?.includes(k) ? '✓' : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <details>
          <summary>{{ tr('查看完整定义', 'View full definition') }}</summary>
          <pre>{{ JSON.stringify(v.jsonSchema, null, 2) }}</pre>
        </details>
      </div>
      <p v-if="!versions.length">{{ tr('尚无版本。', 'No versions.') }}</p>
    </div>
    <ModalDialog v-if="editingVersion" :busy="busy" @close="editingVersion = false">
      <form class="modal-card" @submit.prevent="saveVersion">
        <h2>{{ tr('创建版本', 'Create version') }}</h2>
        <label class="field"
          >{{ tr('版本号', 'Version number')
          }}<input v-model.number="number" type="number" min="1" required
        /></label>
        <div v-for="(a, i) in attributes" :key="i" class="native-form-grid">
          <label class="field"
            >{{ tr('属性名称', 'Attribute name') }}<input v-model="a.name" required /></label
          ><label class="field"
            >{{ tr('类型', 'Type')
            }}<select v-model="a.type">
              <option
                v-for="t in ['string', 'number', 'integer', 'boolean', 'object', 'array']"
                :key="t"
              >
                {{ t }}
              </option>
            </select></label
          ><label class="check"
            ><input type="checkbox" v-model="a.required" />{{ tr('必填', 'Required') }}</label
          ><button type="button" class="quiet" @click="attributes.splice(i, 1)">
            {{ tr('移除', 'Remove') }}
          </button>
        </div>
        <button
          type="button"
          class="quiet"
          @click="attributes.push({ name: '', type: 'string', required: false })"
        >
          + {{ tr('添加属性', 'Add attribute') }}
        </button>
        <details>
          <summary>{{ tr('高级 JSON Schema', 'Advanced JSON Schema') }}</summary>
          <label class="field">
            <textarea
              v-model="schemaText"
              rows="8"
              :placeholder="
                tr('留空时使用上方属性生成定义', 'Leave blank to generate from attributes above')
              "
            /></label
          ><label class="field">UI Schema<textarea v-model="uiSchema" rows="5" /></label>
        </details>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="editingVersion = false">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy">{{ tr('创建', 'Create') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="versionStatus" :busy="busy" @close="versionStatus = null">
      <form class="modal-card compact-modal" @submit.prevent="changeVersion">
        <label class="field"
          >{{ tr('状态', 'Status')
          }}<select v-model="versionStatus.status">
            <option value="draft">{{ tr('草稿', 'Draft') }}</option>
            <option value="active">{{ tr('启用', 'Active') }}</option>
            <option value="deprecated">{{ tr('弃用', 'Deprecated') }}</option>
            <option value="disabled">{{ tr('禁用', 'Disabled') }}</option>
          </select></label
        >
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" @click="versionStatus = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy">{{ tr('保存', 'Save') }}</button>
        </div>
      </form>
    </ModalDialog>
  </div>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onMounted } from 'vue'
import { tr } from '../i18n'
import { runOperation, errorText } from '../api/platform'
const props = defineProps({ record: Object, workspace: String }),
  emit = defineEmits(['saved']),
  name = ref(props.record.displayName || ''),
  key = ref(props.record.key || ''),
  description = ref(props.record.description || ''),
  status = ref(props.record.status || 'active'),
  tab = ref('configuration'),
  versions = ref([]),
  busy = ref(false),
  error = ref(''),
  editingVersion = ref(false),
  number = ref(1),
  attributes = ref([]),
  schemaText = ref(''),
  uiSchema = ref('{}'),
  versionStatus = ref(null)
async function loadVersions() {
  if (props.record.id)
    versions.value = await runOperation('profileVersions', { profileId: props.record.id })
}
onMounted(async () => {
  try {
    await loadVersions()
  } catch (e) {
    error.value = errorText(e)
  }
})
async function save() {
  busy.value = true
  error.value = ''
  try {
    const input = { displayName: name.value, description: description.value }
    const r = props.record.id
      ? await runOperation('updateProfile', {
          id: props.record.id,
          input: { ...input, status: status.value },
        })
      : await runOperation('createProfile', {
          input: {
            ...input,
            key: key.value,
            tenantId: props.workspace,
            objectKind: 'entity',
            kind: 'device',
          },
        })
    emit('saved', r)
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
function openVersion() {
  number.value = Math.max(0, ...versions.value.map((v) => v.version)) + 1
  attributes.value = []
  schemaText.value = ''
  uiSchema.value = '{}'
  editingVersion.value = true
}
async function saveVersion() {
  busy.value = true
  error.value = ''
  try {
    if (new Set(attributes.value.map((a) => a.name)).size !== attributes.value.length)
      throw new Error(tr('属性名称不能重复。', 'Attribute names must be unique.'))
    const jsonSchema = schemaText.value
      ? JSON.parse(schemaText.value)
      : {
          type: 'object',
          properties: Object.fromEntries(
            attributes.value.map((a) => [
              a.name,
              { type: a.type, ...(a.type === 'array' ? { items: {} } : {}) },
            ]),
          ),
          required: attributes.value.filter((a) => a.required).map((a) => a.name),
        }
    await runOperation('createProfileVersion', {
      profileId: props.record.id,
      input: {
        version: number.value,
        jsonSchema,
        uiSchema: JSON.parse(uiSchema.value),
        status: 'draft',
      },
    })
    editingVersion.value = false
    await loadVersions()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function changeVersion() {
  busy.value = true
  try {
    await runOperation('updateProfileVersion', {
      id: versionStatus.value.id,
      input: { status: versionStatus.value.status },
    })
    versionStatus.value = null
    await loadVersions()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
