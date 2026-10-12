<template>
  <section>
    <nav class="tabs">
      <button class="quiet" @click="tab = 'permissions'">
        {{ tr('配置与权限', 'Configuration & permissions') }}</button
      ><button v-if="currentId" class="quiet" @click="tab = 'members'">
        {{ tr('成员', 'Members') }}
      </button>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <form v-if="tab === 'permissions'" @submit.prevent="save">
      <section class="step-panel">
        <h2>{{ tr('角色信息', 'Role details') }}</h2>
        <label class="field"
          >{{ tr('角色名称', 'Role name') }} *<input v-model="name" required /></label
        ><label class="field"
          >{{ tr('描述', 'Description') }}<textarea v-model="description" />
        </label>
      </section>
      <section class="step-panel">
        <h2>{{ tr('权限', 'Permissions') }}</h2>
        <div v-for="(b, i) in blocks" :key="b.id || i" class="panel">
          <div class="actions">
            <strong
              >{{ tr('权限', 'Permission') }} {{ i + 1 }} ·
              {{ b.effect === 'deny' ? tr('拒绝', 'Deny') : tr('允许', 'Allow') }}</strong
            ><button type="button" class="quiet" :disabled="busy" @click="blocks.splice(i, 1)">
              {{ tr('移除此权限', 'Remove permission') }}
            </button>
          </div>
          <p>{{ scopeLabel(b) }} · {{ b.actions?.map((a) => label(a.name)).join(', ') }}</p>
        </div>
        <div class="native-form-grid">
          <label class="field"
            >{{ tr('适用于', 'Applies to')
            }}<select v-model="target" @change="loadActions">
              <option v-for="t in targets" :key="t.key" :value="t.key">{{ tr(t.zh, t.en) }}</option>
            </select></label
          >
          <div class="field">
            <span>{{ tr('允许的操作', 'Allowed actions') }}</span>
            <div class="permission-grid">
              <label v-for="a in actions" :key="a.id" class="check"
                ><input type="checkbox" v-model="chosenActions" :value="a.id" />{{
                  label(a.name)
                }}</label
              >
            </div>
          </div>
        </div>
        <button
          type="button"
          class="quiet"
          :disabled="!chosenActions.length || busy"
          @click="addBlock"
        >
          + {{ tr('添加权限', 'Add permission') }}
        </button>
      </section>
      <button :disabled="busy || !blocks.length">{{ tr('保存角色', 'Save role') }}</button>
    </form>
    <section v-else class="panel">
      <h2>{{ tr('角色成员', 'Role members') }}</h2>
      <form class="toolbar" @submit.prevent="assign">
        <select v-model="member" required :aria-label="tr('成员', 'Member')">
          <option value="">{{ tr('选择工作区成员', 'Select workspace member') }}</option>
          <option
            v-for="m in members.filter((m) => !assignments.some((a) => a.subjectId === m.id))"
            :key="m.id"
            :value="m.id"
          >
            {{ m.name }}
          </option></select
        ><button :disabled="busy">{{ tr('分配角色', 'Assign role') }}</button>
      </form>
      <div v-for="a in assignments" :key="a.id" class="setting-row">
        <span>{{ members.find((m) => m.id === a.subjectId)?.name || a.subjectId }}</span
        ><button class="quiet" @click="removing = a">
          {{ tr('移除角色分配', 'Remove assignment') }}
        </button>
      </div>
    </section>
    <ModalDialog v-if="removing" :busy="busy" confirmation @close="removing = null">
      <section class="modal-card compact-modal">
        <h2>{{ tr('确认移除角色分配', 'Confirm removal of role assignment') }}</h2>
        <p>{{ members.find((m) => m.id === removing.subjectId)?.name || removing.subjectId }}</p>
        <button class="quiet" @click="removing = null">{{ tr('取消', 'Cancel') }}</button
        ><button :disabled="busy" @click="unassign">{{ tr('确认', 'Confirm') }}</button>
      </section>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onMounted } from 'vue'
import { permissionTargets, permissionScope, actionFilter } from '../api/permissionScopes'
import { tr, label } from '../i18n'
import { runOperation, listAll, errorText } from '../api/platform'
const props = defineProps({ record: Object, workspace: String }),
  emit = defineEmits(['saved']),
  currentId = ref(props.record.id || ''),
  name = ref(props.record.name || ''),
  description = ref(props.record.description || ''),
  blocks = ref(JSON.parse(JSON.stringify(props.record.permissionBlocks || []))),
  target = ref('workspace'),
  actions = ref([]),
  chosenActions = ref([]),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  tab = ref('permissions'),
  members = ref([]),
  assignments = ref([]),
  member = ref(''),
  removing = ref(null)
const roleStamp = (r) =>
  JSON.stringify([
    r.name || '',
    r.description || '',
    (r.permissionBlocks || []).map((b) => b.id).sort(),
  ])
let baseline = roleStamp(props.record)
const targets = permissionTargets
const scope = permissionScope
function scopeLabel(b) {
  if (b.scopeMode === 'tenant') return tr('工作区本身', 'Workspace itself')
  return [b.scopeMode, b.objectType || b.objectKind, b.objectId || b.groupId]
    .filter(Boolean)
    .join(' · ')
}
async function loadActions() {
  chosenActions.value = []
  const selected = target.value
  actions.value = []
  try {
    const found = await listAll('actions', {
      tenantId: props.workspace,
      ...actionFilter(selected),
    })
    if (target.value === selected) actions.value = found
  } catch (e) {
    error.value = errorText(e)
  }
}
async function loadMembers() {
  members.value = await listAll('tenantMembers', { tenantId: props.workspace })
  if (currentId.value)
    assignments.value = await listAll('roleAssignments', {
      tenantId: props.workspace,
      roleId: currentId.value,
    })
}
onMounted(async () => {
  try {
    await Promise.all([loadActions(), loadMembers()])
  } catch (e) {
    error.value = errorText(e)
  }
})
function addBlock() {
  blocks.value.push({
    ...scope(target.value),
    effect: 'allow',
    actions: actions.value.filter((a) => chosenActions.value.includes(a.id)),
  })
  chosenActions.value = []
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    if (currentId.value) {
      const latest = await runOperation('role', { id: currentId.value })
      if (roleStamp(latest) !== baseline)
        throw new Error(
          tr(
            '角色已被其他操作修改，请重新打开后再编辑。',
            'Role changed elsewhere. Reopen it before editing.',
          ),
        )
    }
    if (!currentId.value) {
      const r = await runOperation('createRole', {
        input: { tenantId: props.workspace, name: name.value, description: description.value },
      })
      currentId.value = r.id
      baseline = roleStamp({ ...r, name: name.value, description: description.value })
    } else
      await runOperation('updateRole', {
        id: currentId.value,
        input: { name: name.value, description: description.value },
      })
    const stamp = JSON.parse(baseline)
    baseline = JSON.stringify([name.value, description.value, stamp[2]])
    for (const b of blocks.value)
      if (!b.id) {
        const { actions, ...scoping } = b
        const created = await runOperation('createPermissionBlock', {
          input: { ...scoping, tenantId: props.workspace, actionIds: actions.map((a) => a.id) },
        })
        b.id = created.id
      }
    await runOperation('replaceRolePermissionBlocks', {
      roleId: currentId.value,
      permissionBlockIds: blocks.value.map((b) => b.id),
    })
    notice.value = tr('角色已保存。', 'Role saved.')
    emit('saved', { id: currentId.value })
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function assign() {
  busy.value = true
  try {
    await runOperation('createRoleAssignment', {
      input: {
        tenantId: props.workspace,
        subjectKind: 'entity',
        subjectId: member.value,
        roleId: currentId.value,
      },
    })
    member.value = ''
    await loadMembers()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function unassign() {
  busy.value = true
  try {
    await runOperation('deleteRoleAssignment', { id: removing.value.id })
    removing.value = null
    await loadMembers()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
