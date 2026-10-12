export const permissionTargets = [
  { key: 'workspace', zh: '工作区本身', en: 'Workspace itself' },
  ...['device', 'channel', 'group', 'rule', 'report', 'alarm'].map((key, i) => ({
    key,
    zh: ['所有设备', '所有通道', '所有分组', '所有规则', '所有报表', '所有告警'][i],
    en: 'All ' + key + 's',
  })),
]

// Atom's tenant scope only targets the workspace object, not its contents.
export function permissionScope(key) {
  if (key === 'workspace') return { scopeMode: 'tenant' }
  if (!permissionTargets.some((t) => t.key === key)) throw new Error('Unknown permission scope')
  return {
    scopeMode: 'object_type',
    objectKind: key === 'device' ? 'entity' : key === 'group' ? 'group' : 'resource',
    objectType:
      key === 'device' ? 'entity:device' : key === 'group' ? 'group:object' : 'resource:' + key,
  }
}
export function actionFilter(key) {
  const s = permissionScope(key)
  return key === 'workspace'
    ? { objectKind: 'tenant' }
    : { objectKind: s.objectKind, objectType: s.objectType }
}
