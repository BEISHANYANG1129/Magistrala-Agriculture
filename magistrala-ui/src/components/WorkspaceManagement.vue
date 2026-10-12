<template>
  <section>
    <header class="page-heading">
      <h1>{{ roleRecord?.name || sectionLabel(section) }}</h1>
      <router-link v-if="roleRecord" class="button quiet" :to="base"
        >← {{ tr('返回列表', 'Back to list') }}</router-link
      ><button v-else-if="section !== 'info'" @click="openCreate">
        +
        {{
          section === 'users'
            ? tr('添加成员', 'Add member')
            : section === 'invitations'
              ? tr('邀请', 'Invite')
              : tr('创建角色', 'Create role')
        }}
      </button>
    </header>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <WorkspaceRoleEditor
      v-if="roleRecord"
      :key="roleId + revision"
      :workspace="workspace"
      :record="roleRecord"
      @saved="roleSaved"
    />
    <template v-else-if="section === 'info' && tenant"
      ><nav class="tabs">
        <button class="quiet" @click="infoTab = 'configuration'">
          {{ tr('配置', 'Configuration') }}</button
        ><button class="quiet" @click="infoTab = 'metadata'">{{ tr('元数据', 'Metadata') }}</button>
      </nav>
      <form class="panel" @submit.prevent="saveTenant">
        <template v-if="infoTab === 'configuration'"
          ><label class="field"
            >{{ tr('名称', 'Name') }} *<input v-model="tenantName" required
          /></label>
          <div class="setting-row">
            <strong>ID</strong><span>{{ tenant.id }}</span>
          </div>
          <div class="setting-row">
            <strong>{{ tr('路由', 'Route') }}</strong
            ><span>{{ tenant.alias }}</span>
          </div>
          <label class="field"
            >{{ tr('标签（逗号分隔）', 'Tags (comma separated)') }}<input v-model="tenantTags"
          /></label>
          <div class="setting-row">
            <strong>{{ tr('状态', 'Status') }}</strong
            ><span>{{ label(tenant.status) }}</span
            ><button
              type="button"
              class="quiet"
              @click="
                () => {
                  confirmation = {
                    kind: tenant.status === 'active' ? 'disableTenant' : 'enableTenant',
                    id: tenant.id,
                    name: tenant.name,
                  }
                }
              "
            >
              {{ tenant.status === 'active' ? tr('禁用', 'Disable') : tr('启用', 'Enable') }}
            </button>
          </div></template
        ><label v-else class="field"
          >{{ tr('元数据', 'Metadata') }}<textarea v-model="tenantMetadata" rows="12" /></label
        ><button :disabled="busy">{{ tr('保存', 'Save') }}</button>
      </form></template
    >
    <template v-else-if="!roleRecord"
      ><div class="toolbar">
        <input
          v-model="search"
          :placeholder="tr('搜索名称或邮箱', 'Search name or email')"
        /><button class="quiet" @click="load">{{ tr('刷新', 'Refresh') }}</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{{ tr('名称', 'Name') }}</th>
              <th>
                {{
                  section === 'invitations'
                    ? tr('邮箱', 'Email')
                    : tr('角色 / 描述', 'Role / description')
                }}
              </th>
              <th>{{ tr('状态', 'Status') }}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in visible" :key="r.id">
              <td>
                <router-link v-if="section === 'role-management'" :to="base + '/' + r.id">{{
                  r.name
                }}</router-link
                ><span v-else>{{ r.name || r.inviteeName || r.inviteeEmail }}</span>
              </td>
              <td>
                {{
                  section === 'users'
                    ? memberRoles(r.id)
                    : section === 'invitations'
                      ? r.inviteeEmail
                      : r.description
                }}
              </td>
              <td>
                {{ section === 'invitations' ? invitationState(r) : label(r.status || 'active') }}
              </td>
              <td>
                <button
                  v-if="section === 'users'"
                  class="quiet"
                  @click="confirmation = { kind: 'removeTenantMember', id: r.id, name: r.name }"
                >
                  {{ tr('移出工作区', 'Unassign member') }}</button
                ><button
                  v-if="section === 'invitations' && !r.revokedAt && !r.acceptedAt && !r.rejectedAt"
                  class="quiet"
                  @click="
                    () => {
                      confirmation = {
                        kind: 'revokeTenantInvitation',
                        id: r.id,
                        name: r.inviteeEmail,
                      }
                    }
                  "
                >
                  {{ tr('撤回邀请', 'Revoke invitation') }}</button
                ><button
                  v-if="section === 'role-management' && r.managedBy !== 'config'"
                  class="quiet"
                  @click="confirmation = { kind: 'deleteRole', id: r.id, name: r.name }"
                >
                  {{ tr('删除', 'Delete') }}
                </button>
              </td>
            </tr>
            <tr v-if="!visible.length">
              <td colspan="4">
                {{ loading ? tr('读取中…', 'Loading…') : tr('暂无记录。', 'No records.') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <TablePagination v-model:page="page" v-model:size="size" :total="filtered.length" /></template
    >
    <ModalDialog v-if="dialog" :busy="busy" @close="dialog = false">
      <form class="modal-card" @submit.prevent="submit">
        <h2>
          {{ section === 'users' ? tr('添加成员', 'Add member') : tr('邀请成员', 'Invite member') }}
        </h2>
        <template v-if="section === 'users'"
          ><label class="field"
            >{{ tr('查找用户', 'Search for a user')
            }}<input v-model="userQuery" @input="findUsers" /></label
          ><label class="field"
            >{{ tr('用户', 'User') }} *<select v-model="userId" required>
              <option value="">{{ tr('请选择', 'Select') }}</option>
              <option v-for="u in candidates" :key="u.id" :value="u.id">
                {{ u.name }} · {{ u.alias || u.id }}
              </option>
            </select></label
          ><label class="check"
            ><input type="checkbox" v-model="attachRole" />{{
              tr('现在附加角色', 'Attach a role now')
            }}</label
          >
          <p class="muted">
            {{
              tr(
                '不附加角色时，成员需要后续授权才能访问资源。',
                'Without a role, the member needs a later grant to access resources.',
              )
            }}
          </p></template
        ><label v-else class="field"
          >{{ tr('邮箱', 'Email') }} *<input type="email" v-model="email" required /></label
        ><label v-if="attachRole || section === 'invitations'" class="field"
          >{{ tr('角色', 'Role')
          }}<select v-model="role" :required="attachRole">
            <option value="">{{ tr('暂不分配', 'No role yet') }}</option>
            <option v-for="r in roles" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select></label
        >
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="dialog = false">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="busy">
            {{
              section === 'users' ? tr('添加成员', 'Add member') : tr('发送邀请', 'Send invitation')
            }}
          </button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="confirmation" :busy="busy" confirmation @close="confirmation = null">
      <section class="modal-card compact-modal">
        <h2>{{ tr('确认操作', 'Confirm action') }}</h2>
        <p>{{ confirmation.name }}</p>
        <p>
          {{
            confirmation.kind === 'removeTenantMember'
              ? tr(
                  '移除该成员在当前工作区的成员关系？',
                  'Remove this member from the current workspace?',
                )
              : confirmation.kind === 'revokeTenantInvitation'
                ? tr('撤回这条邀请？', 'Revoke this invitation?')
                : confirmation.kind === 'deleteRole'
                  ? tr('删除此角色？', 'Delete this role?')
                  : tr('修改此工作区的启用状态？', 'Change the enabled state of this workspace?')
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="quiet" :disabled="busy" @click="confirmation = null">
          {{ tr('取消', 'Cancel') }}</button
        ><button :disabled="busy" @click="confirm">{{ tr('确认', 'Confirm') }}</button>
      </section>
    </ModalDialog>
  </section>
</template>
<script setup>
import TablePagination from './TablePagination.vue'
import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { editedMetadata } from '../api/editedMetadata'
import { tr, label, sectionLabel } from '../i18n'
import { runOperation, listAll, errorText } from '../api/platform'
import WorkspaceRoleEditor from './WorkspaceRoleEditor.vue'
const route = useRoute(),
  router = useRouter(),
  workspace = computed(() => String(route.params.id)),
  section = computed(() => String(route.params.section)),
  base = computed(() => `/domain/${workspace.value}/${section.value}`),
  roleId = computed(() => String(route.params.recordId || '')),
  records = ref([]),
  roles = ref([]),
  assignments = ref([]),
  tenant = ref(null),
  roleRecord = ref(null),
  revision = ref(0),
  tenantName = ref(''),
  tenantTags = ref(''),
  tenantMetadata = ref('{}'),
  infoTab = ref('configuration'),
  error = ref(''),
  notice = ref(''),
  busy = ref(false),
  loading = ref(false),
  search = ref(''),
  page = ref(1),
  size = ref(10),
  dialog = ref(false),
  confirmation = ref(null),
  candidates = ref([]),
  userQuery = ref(''),
  userId = ref(''),
  email = ref(''),
  role = ref(''),
  attachRole = ref(false)
const filtered = computed(() =>
    records.value.filter((r) =>
      (r.name || r.inviteeEmail || r.inviteeName || '')
        .toLowerCase()
        .includes(search.value.toLowerCase()),
    ),
  ),
  visible = computed(() => filtered.value.slice((page.value - 1) * size.value, page.value * size.value))
let generation = 0,
  searchGeneration = 0
function memberRoles(id) {
  return assignments.value
    .filter((a) => a.subjectId === id)
    .map((a) => roles.value.find((r) => r.id === a.roleId)?.name || a.roleId)
    .join(', ')
}
function invitationState(r) {
  return r.revokedAt
    ? tr('已撤回', 'Revoked')
    : r.acceptedAt
      ? tr('已接受', 'Accepted')
      : r.rejectedAt
        ? tr('已拒绝', 'Rejected')
        : tr('待接受', 'Pending')
}
async function load() {
  const seq = ++generation
  loading.value = true
  error.value = ''
  try {
    if (section.value === 'info') {
      const t = await runOperation('tenant', { id: workspace.value })
      if (seq !== generation) return
      tenant.value = t
      tenantName.value = t.name
      tenantTags.value = (t.tags || []).join(', ')
      tenantMetadata.value = JSON.stringify(t.attributes?.metadata || {}, null, 2)
    } else if (section.value === 'role-management' && roleId.value) {
      const r = roleId.value === 'create' ? {} : await runOperation('role', { id: roleId.value })
      if (seq !== generation) return
      if (r.tenantId && r.tenantId !== workspace.value)
        throw new Error('Role is outside this workspace')
      roleRecord.value = r
      revision.value++
    } else {
      const d = await listAll(
        section.value === 'users'
          ? 'tenantMembers'
          : section.value === 'invitations'
            ? 'tenantInvitations'
            : 'roles',
        { tenantId: workspace.value },
      )
      if (seq !== generation) return
      records.value = d
      if (section.value === 'users') {
        const [r, a] = await Promise.all([
          listAll('roles', { tenantId: workspace.value }),
          listAll('roleAssignments', { tenantId: workspace.value }),
        ])
        if (seq === generation) {
          roles.value = r
          assignments.value = a
        }
      }
    }
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) loading.value = false
  }
}
watch(
  () => [workspace.value, section.value, roleId.value],
  () => {
    roleRecord.value = null
    tenant.value = null
    records.value = []
    dialog.value = false
    confirmation.value = null
    page.value = 1
    load()
  },
  { immediate: true },
)
watch(search, () => (page.value = 1))
async function openCreate() {
  if (section.value === 'role-management') {
    router.push(base.value + '/create')
    return
  }
  userQuery.value = ''
  candidates.value = []
  userId.value = ''
  email.value = ''
  role.value = ''
  attachRole.value = false
  error.value = ''
  dialog.value = true
  try {
    roles.value = await listAll('roles', { tenantId: workspace.value })
  } catch (e) {
    error.value = errorText(e)
  }
}
async function findUsers() {
  const seq = ++searchGeneration
  if (userQuery.value.trim().length < 2) {
    candidates.value = []
    return
  }
  try {
    const d = await runOperation('tenantAssignableEntities', {
      tenantId: workspace.value,
      q: userQuery.value.trim(),
      limit: 20,
      offset: 0,
    })
    if (seq === searchGeneration) candidates.value = d.items || []
  } catch (e) {
    if (seq === searchGeneration) error.value = errorText(e)
  }
}
async function submit() {
  busy.value = true
  error.value = ''
  try {
    if (section.value === 'users')
      await runOperation('addTenantMember', {
        tenantId: workspace.value,
        entityId: userId.value,
        ...(attachRole.value && role.value ? { roleId: role.value } : {}),
      })
    else
      await runOperation('createTenantInvitation', {
        tenantId: workspace.value,
        input: { inviteeEmail: email.value, ...(role.value ? { roleId: role.value } : {}) },
      })
    dialog.value = false
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function saveTenant() {
  busy.value = true
  try {
    const m = JSON.parse(tenantMetadata.value)
    if (!m || typeof m !== 'object' || Array.isArray(m))
      throw new Error(tr('元数据必须是 JSON 对象。', 'Metadata must be a JSON object.'))
    const latest = await runOperation('tenant', { id: workspace.value })
    await runOperation('updateTenant', {
      id: workspace.value,
      input: {
        name: tenantName.value,
        tags: tenantTags.value
          .split(/[,，]/)
          .map((v) => v.trim())
          .filter(Boolean),
        attributes: {
          ...latest.attributes,
          metadata: editedMetadata(
            tenant.value.attributes?.metadata,
            m,
            latest.attributes?.metadata,
          ),
        },
      },
    })
    await load()
    notice.value = tr('已保存。', 'Saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function confirm() {
  busy.value = true
  try {
    const c = confirmation.value
    await runOperation(
      c.kind,
      c.kind === 'removeTenantMember'
        ? { tenantId: workspace.value, entityId: c.id }
        : c.kind === 'revokeTenantInvitation'
          ? { tenantId: workspace.value, invitationId: c.id }
          : { id: c.id },
    )
    confirmation.value = null
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
function roleSaved(r) {
  if (roleId.value === 'create') router.replace(base.value + '/' + r.id)
  else load()
}
</script>
