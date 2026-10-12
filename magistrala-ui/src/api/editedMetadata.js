// Apply only keys edited in this form. Keep newer keys owned by other modules.
export function editedMetadata(original = {}, draft = {}, latest = {}) {
  const next = { ...latest }
  for (const key of new Set([...Object.keys(original), ...Object.keys(draft)])) {
    const before = JSON.stringify(original[key]),
      after = JSON.stringify(draft[key]),
      current = JSON.stringify(latest[key])
    if (before === after) continue
    if (current !== before && current !== after)
      throw new Error(
        'Metadata changed elsewhere; reload before editing / 元数据已被其他操作修改，请重新读取',
      )
    if (Object.hasOwn(draft, key)) next[key] = draft[key]
    else delete next[key]
  }
  return next
}
