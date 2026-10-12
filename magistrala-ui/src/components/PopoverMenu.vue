<template>
  <span class="popover-menu">
    <button
      ref="trigger"
      type="button"
      class="quiet popover-trigger"
      :aria-label="label"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-controls="id"
      @click="toggle"
      @keydown="triggerKey"
    >
      <slot name="trigger" />
    </button>
    <Teleport to="body">
      <div
        v-if="open"
        :id="id"
        ref="panel"
        class="popover-panel"
        role="menu"
        :aria-label="label"
        tabindex="-1"
        :style="position"
        @keydown="navigate"
        @click.capture="prepareDialog"
        @click="select"
      >
        <slot />
      </div>
    </Teleport>
  </span>
</template>
<script>
let serial = 0
let closeActive
</script>
<script setup>
import { rememberDialogTrigger } from '../api/dialogFocus'
import { ref, nextTick, onBeforeUnmount } from 'vue'
defineProps({ label: String })
const id = `popover-${++serial}`,
  trigger = ref(null),
  panel = ref(null),
  open = ref(false)
const position = ref({})
const items = () =>
  [...panel.value.querySelectorAll('a[href], button, input, select, [tabindex="0"]')].filter(
    (el) => !el.disabled,
  )
function place() {
  if (!panel.value) return
  const anchor = trigger.value.getBoundingClientRect(),
    box = panel.value.getBoundingClientRect()
  position.value = {
    left: `${Math.max(8, Math.min(anchor.right - box.width, window.innerWidth - box.width - 8))}px`,
    top: `${Math.max(8, anchor.bottom + box.height + 8 > window.innerHeight ? anchor.top - box.height - 4 : anchor.bottom + 4)}px`,
  }
}
function close(restore = false) {
  open.value = false
  document.removeEventListener('pointerdown', outside, true)
  window.removeEventListener('resize', place)
  window.removeEventListener('scroll', place, true)
  if (closeActive === close) closeActive = undefined
  if (restore) trigger.value?.focus()
}
async function show(last = false) {
  closeActive?.()
  closeActive = close
  open.value = true
  document.addEventListener('pointerdown', outside, true)
  window.addEventListener('resize', place)
  window.addEventListener('scroll', place, true)
  await nextTick()
  place()
  const choices = items()
  ;(last ? choices.at(-1) : choices[0] || panel.value)?.focus()
}
function toggle() {
  if (open.value) close(true)
  else show()
}
function outside(e) {
  if (!panel.value?.contains(e.target) && !trigger.value?.contains(e.target)) close()
}
function triggerKey(e) {
  if (['ArrowDown', 'ArrowUp'].includes(e.key)) {
    e.preventDefault()
    show(e.key === 'ArrowUp')
  }
  if (e.key === 'Escape') {
    e.preventDefault()
    close(true)
  }
}
function navigate(e) {
  const choices = items(),
    index = choices.indexOf(document.activeElement)
  if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    close(true)
  } else if (e.key === 'Tab') close()
  else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(e.key)) {
    e.preventDefault()
    const next =
      e.key === 'Home'
        ? 0
        : e.key === 'End'
          ? choices.length - 1
          : (index + (e.key === 'ArrowDown' ? 1 : -1) + choices.length) % choices.length
    choices[next]?.focus()
  }
}
function prepareDialog(e) { if (e.target.closest('a[href],button')) rememberDialogTrigger(trigger.value) }
function select(e) {
  if (e.target.closest('a[href],button') && !e.target.closest('[data-keep-open]')) close(true)
}
onBeforeUnmount(() => close())
</script>
<style>
.popover-menu {
  display: inline-block;
}
.popover-trigger {
  white-space: nowrap;
}
.popover-panel {
  position: fixed;
  z-index: 1300;
  min-width: 160px;
  max-width: calc(100vw - 16px);
  max-height: calc(100vh - 16px);
  overflow-y: auto;
  background: var(--surface, #fff);
  color: var(--text, #172333);
  border: 1px solid var(--line, #e2e5e9);
  border-radius: 6px;
  box-shadow: 0 4px 12px #0002;
  padding: 4px;
}
.popover-panel > div {
  display: grid;
  gap: 2px;
}
.popover-panel a,
.popover-panel button,
.popover-panel label {
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  padding: 6px 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 14px;
  text-decoration: none;
}
.popover-panel a:hover,
.popover-panel button:hover,
.popover-panel label:hover,
.popover-panel :focus-visible {
  background: #f1f3f5;
  outline: none;
}
.popover-panel label {
  cursor: pointer;
}
</style>
