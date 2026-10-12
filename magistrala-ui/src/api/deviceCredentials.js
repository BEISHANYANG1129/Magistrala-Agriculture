import { runOperation } from './platform'
import { readResource } from './resourceWorkflow'
export function activeSharedKeys(credentials) {
  return (credentials.items || []).filter(
    (c) =>
      c.kind === 'shared_key' &&
      !['revoked', 'expired', 'disabled'].includes(c.status) &&
      (!c.expiresAt || new Date(c.expiresAt) > new Date()),
  )
}
// Keep the new credential in this in-memory transaction before revoking the selected old key.
// On a failed revocation a retry must never create yet another credential.
export async function replaceDeviceKey(state, record, workspace, key, credentialId) {
  if (state.unknown)
    throw new Error(
      'Previous result is unknown. Reload credentials before retrying. / 上次结果待核对，请重新读取凭据。',
    )
  await readResource('devices', record.id, workspace)
  if (!state.created) {
    const current = activeSharedKeys(await runOperation('credentials', { entityId: record.id }))
    const old = current.find((c) => c.id === credentialId)
    if (credentialId && (!old || old.managedBy === 'config'))
      throw new Error(
        'The selected credential changed or is configuration managed. / 所选凭据已变更或由配置管理。',
      )
    state.oldId = credentialId
    try {
      state.created = await runOperation('createSharedKey', {
        entityId: record.id,
        input: { key, description: 'Device key' },
      })
    } catch (e) {
      if (!e.status || e.status >= 500) state.unknown = true
      throw e
    }
    if (!state.created?.credentialId) {
      state.unknown = true
      throw new Error(
        'Credential result incomplete. Reload before retrying. / 凭据返回不完整，请先重新读取。',
      )
    }
  }
  if (state.oldId && !state.revoked) {
    await runOperation('revokeCredential', { entityId: record.id, credentialId: state.oldId })
    state.revoked = true
  }
  return state.created
}
