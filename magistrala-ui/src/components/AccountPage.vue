<template>
  <section>
    <h1>{{ tr('账户与令牌', 'Account & tokens') }}</h1>
    <nav class="tabs">
      <button class="quiet" @click="tab = 'profile'">{{ tr('个人资料', 'Profile') }}</button
      ><button class="quiet" @click="tab = 'tokens'">{{ tr('访问令牌', 'Access tokens') }}</button
      ><button class="quiet" @click="tab = 'password'">
        {{ tr('修改密码', 'Change password') }}
      </button>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <form v-if="tab === 'profile'" class="panel" @submit.prevent="saveProfile">
      <label class="field">{{ tr('姓名', 'Name') }}<input v-model="name" required /></label>
      <div class="setting-row">
        <strong>{{ tr('用户名', 'Username') }}</strong
        ><span>{{ profile?.alias || '—' }}</span>
      </div>
      <button :disabled="busy">{{ tr('保存', 'Save') }}</button>
    </form>
    <form v-if="tab === 'password'" class="panel" @submit.prevent="password">
      <label class="field"
        >{{ tr('当前密码', 'Current password')
        }}<input
          v-model="currentPassword"
          type="password"
          autocomplete="current-password"
          required /></label
      ><label class="field"
        >{{ tr('新密码', 'New password')
        }}<input
          v-model="newPassword"
          type="password"
          autocomplete="new-password"
          required /></label
      ><label class="field"
        >{{ tr('再次输入新密码', 'Confirm new password')
        }}<input
          v-model="confirmPassword"
          type="password"
          autocomplete="new-password"
          required /></label
      ><button :disabled="busy">{{ tr('更新密码', 'Update password') }}</button>
    </form>
    <section v-if="tab === 'tokens'">
      <button @click="openToken()">+ {{ tr('创建访问令牌', 'Create access token') }}</button>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{{ tr('名称', 'Name') }}</th>
              <th>{{ tr('状态', 'Status') }}</th>
              <th>{{ tr('到期时间', 'Expires') }}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in tokens" :key="t.credentialId">
              <td>{{ t.name }}</td>
              <td>{{ t.status }}</td>
              <td>{{ t.expiresAt || '—' }}</td>
              <td>
                <button class="quiet" @click="openToken(t)">{{ tr('权限', 'Permissions') }}</button
                ><button v-if="t.status !== 'revoked'" class="quiet danger" @click="revoking = t">
                  {{ tr('撤销', 'Revoke') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <ModalDialog v-if="tokenDialog" :busy="busy" @close="tokenDialog = false">
      <form class="modal-card" @submit.prevent="saveToken">
        <h2>
          {{
            editing
              ? tr('修改令牌权限', 'Edit token permissions')
              : tr('创建访问令牌', 'Create access token')
          }}
        </h2>
        <template v-if="!editing"
          ><label class="field"
            >{{ tr('名称', 'Name') }} *<input v-model="tokenName" required /></label
          ><label class="field"
            >{{ tr('描述', 'Description') }}<input v-model="description" /></label
          ><label class="field"
            >{{ tr('到期时间', 'Expires at')
            }}<input type="datetime-local" v-model="expires" /></label
        ></template>
        <h3>{{ tr('权限范围', 'Permission scope') }}</h3>
        <div v-for="(p, i) in permissions" :key="i" class="panel">
          <p>
            {{ p.tenantId || tr('全局', 'Global') }} · {{ p.scopeMode }} ·
            {{ p.actions.join(', ') }}
          </p>
          <button type="button" class="quiet" @click="permissions.splice(i, 1)">
            {{ tr('移除权限', 'Remove permission') }}
          </button>
        </div>
        <p>
          {{ tr('向令牌添加当前工作区的操作权限：', 'Add permissions for the current workspace:') }}
        </p>
        <label class="field"
          >{{ tr('适用于', 'Applies to')
          }}<select v-model="target" @change="loadActions">
            <option v-for="t in permissionTargets" :key="t.key" :value="t.key">
              {{ tr(t.zh, t.en) }}
            </option>
          </select></label
        >
        <div class="permission-grid">
          <label v-for="a in actions" :key="a.id" class="check"
            ><input type="checkbox" v-model="chosen" :value="a.name" />{{ label(a.name) }}</label
          >
        </div>
        <button type="button" class="quiet" :disabled="!chosen.length" @click="addPermission">
          + {{ tr('添加当前工作区权限', 'Add workspace permissions') }}
        </button>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="tokenDialog = false">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="busy || !permissions.length">{{ tr('保存', 'Save') }}</button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="createdToken" confirmation @close="() => { createdToken = ''; load() }">
      <section class="modal-card">
        <h2>{{ tr('令牌已创建', 'Token created') }}</h2>
        <p>
          {{
            tr(
              '令牌仅在这里显示一次，请妥善保存。',
              'The token is shown only once. Store it securely.',
            )
          }}
        </p>
        <textarea
          readonly
          :value="createdToken"
          rows="4"
          :aria-label="tr('新令牌', 'New token')"
        /><button
          @click="
            () => {
              createdToken = ''
              load()
            }
          "
        >
          {{ tr('已保存，关闭', 'Saved, close') }}
        </button>
      </section>
    </ModalDialog>
    <ModalDialog v-if="revoking" :busy="busy" confirmation @close="revoking = null">
      <section class="modal-card compact-modal">
        <h2>{{ tr('确认撤销令牌', 'Confirm token revocation') }}</h2>
        <p>{{ revoking.name }}</p>
        <p v-if="error" class="error">{{ error }}</p>
        <button class="quiet" @click="revoking = null">{{ tr('取消', 'Cancel') }}</button
        ><button :disabled="busy" @click="revoke">{{ tr('撤销', 'Revoke') }}</button>
      </section>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { permissionTargets, permissionScope, actionFilter } from '../api/permissionScopes'
import { tr, label } from '../i18n'
import { runOperation, listAll, errorText } from '../api/platform'
const route = useRoute(),
  workspace = String(route.params.id),
  tab = ref('profile'),
  profile = ref(null),
  name = ref(''),
  tokens = ref([]),
  error = ref(''),
  notice = ref(''),
  busy = ref(false),
  currentPassword = ref(''),
  newPassword = ref(''),
  confirmPassword = ref(''),
  tokenDialog = ref(false),
  editing = ref(null),
  tokenName = ref(''),
  description = ref(''),
  expires = ref(''),
  permissions = ref([]),
  actions = ref([]),
  target = ref('workspace'),
  chosen = ref([]),
  revoking = ref(null),
  createdToken = ref('')
let subject = ''
onMounted(async () => {
  try {
    const payload = localStorage
      .getItem('token')
      .split('.')[1]
      .replace(/-/g, '+')
      .replace(/_/g, '/')
    subject = JSON.parse(atob(payload)).sub
    profile.value = await runOperation('entity', { id: subject })
    name.value = profile.value.name
    await load()
    await loadActions()
  } catch (e) {
    error.value = errorText(e)
  }
})
async function loadActions() {
  chosen.value = []
  actions.value = []
  const selected = target.value
  try {
    const all = await listAll('actions', { tenantId: workspace, ...actionFilter(selected) })
    if (target.value === selected)
      actions.value = all.filter((a, i) => all.findIndex((b) => b.name === a.name) === i)
  } catch (e) {
    error.value = errorText(e)
  }
}
async function load() {
  try {
    tokens.value = await listAll('accessTokens', { subjectId: subject })
  } catch (e) {
    error.value = errorText(e)
  }
}
async function saveProfile() {
  busy.value = true
  try {
    await runOperation('updateEntity', { id: subject, input: { name: name.value } })
    notice.value = tr('个人资料已保存。', 'Profile saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function password() {
  busy.value = true
  error.value = ''
  try {
    if (newPassword.value !== confirmPassword.value)
      throw new Error(tr('两次新密码不一致。', 'New passwords do not match.'))
    await runOperation('changeOwnPassword', {
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
    })
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    notice.value = tr('密码已修改。', 'Password changed.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
function openToken(t) {
  editing.value = t || null
  tokenName.value = ''
  description.value = ''
  expires.value = ''
  chosen.value = []
  permissions.value = JSON.parse(JSON.stringify(t?.permissions || []))
  tokenDialog.value = true
  error.value = ''
}
function addPermission() {
  permissions.value.push({
    tenantId: workspace,
    ...permissionScope(target.value),
    actions: [...new Set(chosen.value)],
  })
  chosen.value = []
}
async function saveToken() {
  busy.value = true
  error.value = ''
  try {
    const permissionsInput = permissions.value.map((p) =>
      Object.fromEntries(
        Object.entries(p).filter(([k]) =>
          [
            'actions',
            'scopeMode',
            'tenantId',
            'objectKind',
            'objectType',
            'objectId',
            'conditions',
          ].includes(k),
        ),
      ),
    )
    if (editing.value)
      await runOperation('replaceAccessTokenPermissions', {
        credentialId: editing.value.credentialId,
        permissions: permissionsInput,
      })
    else {
      const exp = expires.value ? new Date(expires.value) : null
      if (exp && exp <= new Date())
        throw new Error(tr('到期时间必须在未来。', 'Expiry must be in the future.'))
      const r = await runOperation('createAccessToken', {
        input: {
          name: tokenName.value,
          description: description.value,
          permissions: permissionsInput,
          scoped: true,
          ...(exp ? { expiresAt: exp.toISOString() } : {}),
        },
      })
      createdToken.value = r.token
    }
    tokenDialog.value = false
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function revoke() {
  busy.value = true
  try {
    await runOperation('revokeAccessToken', { credentialId: revoking.value.credentialId })
    revoking.value = null
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
