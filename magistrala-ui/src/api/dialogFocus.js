let pendingTrigger

// A menu item disappears before its dialog mounts. Preserve its stable trigger.
export function rememberDialogTrigger(element) {
  pendingTrigger = element
  setTimeout(() => { if (pendingTrigger === element) pendingTrigger = undefined }, 0)
}
export function dialogTrigger(fallback) {
  const element = pendingTrigger || fallback
  pendingTrigger = undefined
  return element
}
