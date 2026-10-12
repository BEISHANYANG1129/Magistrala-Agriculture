<template>
  <section v-show="!dialogOnly">
    <div class="toolbar">
      <button v-if="tab === 'role-management'" @click="openRole">
        + {{ tr('创建角色', 'Create role') }}</button
      ><button v-if="tab === 'users'" @click="openMember">
        + {{ tr('添加成员', 'Add member') }}</button
      ><button class="quiet" :disabled="busy" @click="load">{{ tr('刷新', 'Refresh') }}</button>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="table-wrap" v-if="tab === 'role-management'">
      <table>
        <thead>
          <tr>
            <th>{{ tr('角色', 'Role') }}</th>
            <th>{{ tr('权限', 'Permissions') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in scopedRoles" :key="r.id">
            <td>{{ r.name }}</td>
            <td>
              {{
                r.permissionBlocks
                  .filter(matches)
                  .flatMap((p) => p.actions.map((a) => label(a.name)))
                  .join(', ')
              }}
            </td>
            <td>
              <button class="quiet" :disabled="r.managedBy === 'config'" @click="openRole(r)">
                {{ tr('编辑', 'Edit') }}
              </button>
            </td>
          </tr>
          <tr v-if="!scopedRoles.length">
            <td colspan="3">
              {{ tr('此资源尚无专属角色。', 'No roles scoped to this resource.') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="table-wrap" v-else-if="tab === 'users'">
      <table>
        <thead>
          <tr>
            <th>{{ tr('成员', 'Member') }}</th>
            <th>{{ tr('角色', 'Role') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in scopedAssignments" :key="a.id">
            <td>{{ members.find((m) => m.id === a.subjectId)?.name || a.subjectId }}</td>
            <td>{{ roles.find((r) => r.id === a.roleId)?.name }}</td>
            <td>
              <button
                class="quiet danger"
                :disabled="a.managedBy === 'config'"
                @click="removing = a"
              >
                {{ tr('移除', 'Remove') }}
              </button>
            </td>
          </tr>
          <tr v-if="!scopedAssignments.length">
            <td colspan="3">
              {{ tr('没有资源角色的直接成员。', 'No direct members of resource roles.') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else>
      <h2>{{ tr('我的有效权限', 'My effective access') }}</h2>
      <p>
        {{
          tr(
            '由平台综合工作区、分组、角色及直接授权计算。',
            'Evaluated by the platform across workspace, group, role and direct grants.',
          )
        }}
      </p>
      <div class="permission-grid">
        <span v-for="a in evaluated" :key="a.name" class="badge"
          >{{ a.allowed ? '✓' : '—' }} {{ label(a.name) }}</span
        >
      </div>
    </div>
    <ModalDialog v-if="dialog === 'role'" :busy="busy" @close="dialog = ''">
      <form class="modal-card compact-modal" @submit.prevent="saveRole">
        <h2>{{ editing ? tr('编辑角色', 'Edit role') : tr('创建角色', 'Create role') }}</h2>
        <label class="field">{{ tr('名称', 'Name') }} *<input v-model="roleName" required /></label
        ><label class="field"
          >{{ tr('描述', 'Description') }}<textarea v-model="description" />
        </label>
        <p>
          {{ tr('以下权限仅适用于当前资源：', 'These permissions apply only to:') }}
          {{ record.name }}
        </p>
        <div class="permission-grid">
          <label v-for="a in actions" :key="a.id" class="check"
            ><input type="checkbox" v-model="actionIds" :value="a.id" />{{ label(a.name) }}</label
          >
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="dialog = ''">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy || !actionIds.length">{{ tr('保存', 'Save') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="dialog === 'member'" :busy="busy" @close="dialog = ''">
      <form class="modal-card compact-modal" @submit.prevent="addMember">
        <h2>{{ tr('添加成员', 'Add member') }}</h2>
        <label class="field"
          >{{ tr('成员', 'Member')
          }}<select v-model="memberId" required>
            <option value="">{{ tr('请选择', 'Select') }}</option>
            <option v-for="m in members" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select></label
        ><label class="field"
          >{{ tr('角色', 'Role')
          }}<select v-model="roleId" required>
            <option value="">{{ tr('请选择', 'Select') }}</option>
            <option v-for="r in scopedRoles" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select></label
        >
        <p v-if="!scopedRoles.length" class="warning">
          {{
            tr('请先在“角色”页创建适用于此资源的角色。', 'Create a role for this resource first.')
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="dialog = ''">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy">{{ tr('添加', 'Add') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="removing" :busy="busy" confirmation @close="removing = null">
      <div class="modal-card compact-modal">
        <h2>{{ tr('移除成员角色', 'Remove member role') }}</h2>
        <p>{{ tr('确认移除此成员的这项角色授权？', 'Remove this role assignment?') }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button class="quiet" :disabled="busy" @click="removing = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy" @click="removeMember">{{ tr('确认', 'Confirm') }}</button>
        </div>
      </div>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { tr, label } from '../i18n'
import { gql, listAll, runOperation, errorText } from '../api/platform'
const emit = defineEmits(['close', 'updated'])
const props = defineProps({
  workspace: String,
  section: String,
  record: Object,
  tab: String,
  dialogOnly: Boolean,
})
const roles = ref([]),
  assignments = ref([]),
  members = ref([]),
  actions = ref([]),
  evaluated = ref([]),
  error = ref(''),
  busy = ref(false),
  dialog = ref(''),
  editing = ref(null),
  roleName = ref(''),
  description = ref(''),
  actionIds = ref([]),
  memberId = ref(''),
  roleId = ref(''),
  removing = ref(null)
const objectKind = computed(() =>
    ['channels', 'rules', 'reports'].includes(props.section)
      ? 'resource'
      : props.section === 'groups'
        ? 'group'
        : 'entity',
  ),
  objectType = computed(() =>
    ['channels', 'rules', 'reports'].includes(props.section)
      ? 'resource:' + { channels: 'channel', rules: 'rule', reports: 'report' }[props.section]
      : props.section === 'groups'
        ? 'group:object'
        : 'entity:device',
  )
const matches = (p) =>
  p.objectId === props.record.id &&
  p.objectKind === objectKind.value &&
  p.objectType === objectType.value &&
  p.effect === 'allow' &&
  p.scopeMode === 'object'
const scopedRoles = computed(() => roles.value.filter((r) => r.permissionBlocks.some(matches))),
  scopedAssignments = computed(() =>
    assignments.value.filter((a) => scopedRoles.value.some((r) => r.id === a.roleId)),
  )
let generation = 0
async function load() {
  const seq = ++generation
  busy.value = true
  error.value = ''
  try {
    const [allRoles, allAssignments, allMembers, allActions] = await Promise.all([
      listAll('roles', { tenantId: props.workspace }),
      listAll('roleAssignments', { tenantId: props.workspace }),
      listAll('tenantMembers', { tenantId: props.workspace }),
      listAll('actions', {
        tenantId: props.workspace,
        objectKind: objectKind.value,
        objectType: objectType.value,
      }),
    ])
    const detailed = await Promise.all(allRoles.map((r) => runOperation('role', { id: r.id })))
    if (seq !== generation) return
    roles.value = detailed
    assignments.value = allAssignments
    members.value = allMembers
    actions.value = allActions
    if (props.tab === 'access') {
      const token = localStorage.getItem('token')
      const part = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
      const subjectId = JSON.parse(atob(part)).sub
      const results = await Promise.all(
        allActions.map(async (a) => ({
          name: a.name,
          allowed: (
            await runOperation('authzCheck', {
              input: {
                subjectId,
                action: a.name,
                objectKind: objectKind.value,
                objectId: props.record.id,
              },
            })
          ).allowed,
        })),
      )
      if (seq === generation) evaluated.value = results
    }
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
watch(
  () => [props.workspace, props.record.id, props.tab],
  () => {
    dialog.value = ''
    removing.value = null
    load()
    if (props.dialogOnly) openMember()
  },
  { immediate: true },
)
function openRole(r) {
  editing.value = r?.id ? r : null
  roleName.value = editing.value?.name || ''
  description.value = editing.value?.description || ''
  actionIds.value =
    editing.value?.permissionBlocks.filter(matches).flatMap((p) => p.actions.map((a) => a.id)) || []
  error.value = ''
  dialog.value = 'role'
}
watch(dialog, (value, previous) => {
  if (props.dialogOnly && previous && !value) emit('close')
})
function openMember() {
  memberId.value = ''
  roleId.value = ''
  error.value = ''
  dialog.value = 'member'
}
async function saveRole() {
  busy.value = true
  error.value = ''
  let block
  try {
    const role = editing.value
      ? await runOperation('updateRole', {
          id: editing.value.id,
          input: { name: roleName.value, description: description.value },
        })
      : await runOperation('createRole', {
          input: {
            tenantId: props.workspace,
            name: roleName.value,
            description: description.value,
          },
        })
    editing.value = editing.value || { ...role, permissionBlocks: [] }
    block = await runOperation('createPermissionBlock', {
      input: {
        tenantId: props.workspace,
        scopeMode: 'object',
        objectKind: objectKind.value,
        objectType: objectType.value,
        objectId: props.record.id,
        effect: 'allow',
        actionIds: actionIds.value,
      },
    })
    const latest = await runOperation('role', { id: role.id })
    await runOperation('replaceRolePermissionBlocks', {
      roleId: role.id,
      permissionBlockIds: [
        ...latest.permissionBlocks.filter((p) => !matches(p)).map((p) => p.id),
        block.id,
      ],
    })
    dialog.value = ''
    await load()
  } catch (e) {
    if (block)
      try {
        await runOperation('deletePermissionBlock', { id: block.id })
      } catch {}
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function addMember() {
  busy.value = true
  error.value = ''
  try {
    await runOperation('createRoleAssignment', {
      input: {
        tenantId: props.workspace,
        subjectKind: 'entity',
        subjectId: memberId.value,
        roleId: roleId.value,
      },
    })
    dialog.value = ''
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function removeMember() {
  busy.value = true
  error.value = ''
  try {
    await runOperation('deleteRoleAssignment', { id: removing.value.id })
    removing.value = null
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
