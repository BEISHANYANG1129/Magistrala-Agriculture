<template>
  <Teleport to="body">
    <div
      ref="overlay"
      class="modal"
      :class="{ 'dialog-drawer': drawer }"
      @pointerdown="outsideDown = $event.target === $event.currentTarget"
      @click.self="outsideClick"
    >
      <div
        ref="dialog"
        class="dialog-surface"
        :role="confirmation ? 'alertdialog' : 'dialog'"
        aria-modal="true"
        :aria-labelledby="headingId"
        tabindex="-1"
      >
        <slot />
        <button
          v-if="!confirmation"
          type="button"
          class="dialog-close quiet"
          :disabled="busy"
          :aria-label="tr('关闭', 'Close')"
          @click="close"
        >
          ×
        </button>
      </div>
    </div>
  </Teleport>
</template>
<script>
const stack = []
let nextId = 0
let previousOverflow = ''
</script>
<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { dialogTrigger } from '../api/dialogFocus'
import { tr } from '../i18n'
const props = defineProps({ busy: Boolean, confirmation: Boolean, drawer: Boolean })
const emit = defineEmits(['close'])
const dialog = ref(null),
  overlay = ref(null),
  outsideDown = ref(false)
const headingId = `dialog-heading-${++nextId}`
let trigger, owner, lastFocus
const background = new Map()
const focusable = () =>
  [...dialog.value.querySelectorAll('button, a[href], input, select, textarea, [tabindex]')].filter(
    (el) =>
      !el.disabled &&
      el.tabIndex >= 0 &&
      !el.closest('[hidden], [inert]') &&
      getComputedStyle(el).display !== 'none',
  )
const blocked = () => props.busy || !!dialog.value?.querySelector('[aria-busy="true"]')
function close() {
  if (!blocked()) emit('close')
}
function outsideClick() {
  if (outsideDown.value && !props.confirmation) close()
  outsideDown.value = false
}
function keydown(event) {
  if (stack.at(-1) !== owner) return
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopImmediatePropagation()
    close()
  } else if (event.key === 'Tab') {
    const items = focusable(),
      index = items.indexOf(document.activeElement)
    if (!items.length) {
      event.preventDefault()
      dialog.value.focus()
      return
    }
    if (event.shiftKey && index <= 0) {
      event.preventDefault()
      items.at(-1).focus()
    } else if (!event.shiftKey && (index < 0 || index === items.length - 1)) {
      event.preventDefault()
      items[0].focus()
    }
  }
}
function focusin(event) {
  if (stack.at(-1) !== owner) return
  if (dialog.value.contains(event.target)) lastFocus = event.target
  else (lastFocus?.isConnected ? lastFocus : dialog.value).focus()
}
onMounted(async () => {
  trigger = dialogTrigger(document.activeElement)
  owner = { dialog: dialog.value }
  if (!stack.length) {
    previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  for (const element of document.body.children) {
    if (
      element !== overlay.value &&
      !element.contains(overlay.value) &&
      !['SCRIPT', 'STYLE'].includes(element.tagName)
    ) {
      background.set(element, element.hasAttribute('inert'))
      element.setAttribute('inert', '')
    }
  }
  stack.push(owner)
  const heading = dialog.value.querySelector('h1,h2,h3')
  if (heading) heading.id = headingId
  else dialog.value.setAttribute('aria-label', tr('对话框', 'Dialog'))
  document.addEventListener('keydown', keydown, true)
  document.addEventListener('focusin', focusin)
  await nextTick()
  if (!dialog.value || stack.at(-1) !== owner) return
  const first = props.confirmation
    ? dialog.value.querySelector('.actions button:not([disabled])')
    : focusable()[0]
  ;(first || dialog.value).focus()
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', keydown, true)
  document.removeEventListener('focusin', focusin)
  const index = stack.indexOf(owner)
  if (index >= 0) stack.splice(index, 1)
  for (const [element, wasInert] of background) {
    if (!wasInert) element.removeAttribute('inert')
  }
  if (!stack.length) document.body.style.overflow = previousOverflow
  nextTick(() => { if (trigger?.isConnected) trigger.focus() })
})
</script>
<style>
.modal {
  z-index: 1200;
}
.dialog-drawer {
  justify-content: flex-start;
  padding: 0;
}
.dialog-drawer .dialog-surface {
  width: 280px !important;
  height: 100vh;
  max-height: 100vh;
}
.dialog-drawer .sidebar {
  position: static;
  width: 280px;
  height: 100vh;
  transform: none;
  box-sizing: border-box;
}
.dialog-drawer .dialog-close {
  color: inherit;
}
@media (max-width: 1024px) {
  .has-workspace .topbar,
  .main-content {
    margin-left: 0;
  }
}
.dialog-surface {
  position: relative;
  max-width: min(900px, 96vw);
  max-height: 92vh;
  outline: none;
}
.dialog-surface > .modal-card {
  width: 100%;
  max-height: 92vh;
  margin: 0;
  box-sizing: border-box;
}
.dialog-surface:has(> .compact-modal) {
  width: min(560px, 96vw);
}
.dialog-surface:not(:has(> .compact-modal)) {
  width: min(900px, 96vw);
}
.dialog-close {
  position: absolute;
  right: 12px;
  top: 12px;
  border: 0;
  padding: 2px 8px;
  font-size: 22px;
  background: transparent;
}
.dialog-surface .modal-card > h2:first-child {
  padding-right: 32px;
}
</style>
