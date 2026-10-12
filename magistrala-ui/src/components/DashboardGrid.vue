<template>
  <div
    ref="grid"
    class="dashboard-grid"
    :style="{ height: height + 'px' }"
    @pointermove="move"
    @pointerup="end"
    @pointercancel="cancel"
  >
    <article v-for="cell in cells" :key="cell.i" class="dashboard-cell" :style="style(cell)">
      <header
        :class="{ 'grid-drag-handle': editable }"
        :tabindex="editable ? 0 : undefined"
        :aria-label="tr('移动组件', 'Move widget') + ' ' + title(cell.i)"
        @pointerdown="start($event, cell, 'move')"
        @keydown="keyMove($event, cell)"
      >
        <strong>{{ title(cell.i) }}</strong
        ><slot name="actions" :id="cell.i" />
      </header>
      <div class="dashboard-cell-body"><slot :id="cell.i" /></div>
      <button
        v-if="editable"
        class="grid-resize-handle"
        :aria-label="tr('缩放组件', 'Resize widget') + ' ' + title(cell.i)"
        @pointerdown.stop="start($event, cell, 'resize')"
        @keydown="keyResize($event, cell)"
      >
        ◢
      </button>
    </article>
  </div>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { tr } from '../i18n'
import { moveGridCell } from '../api/dashboardLayout'
const props = defineProps({ cells: Array, columns: Number, editable: Boolean, widgets: Array }),
  emit = defineEmits(['update:cells', 'width'])
const grid = ref(null),
  width = ref(900),
  preview = ref(null)
const cells = computed(() => preview.value || props.cells),
  height = computed(() => Math.max(4, ...cells.value.map((c) => c.y + c.h)) * 60)
let gesture = null,
  observer
const title = (id) =>
  props.widgets.find((w) => w.id === id)?.title ||
  props.widgets.find((w) => w.id === id)?.name ||
  id
function style(c) {
  return {
    left: (c.x / props.columns) * 100 + '%',
    top: c.y * 60 + 'px',
    width: (c.w / props.columns) * 100 + '%',
    height: c.h * 60 + 'px',
  }
}
function start(e, c, kind) {
  if (!props.editable || e.button !== 0 || (kind === 'move' && e.target.closest('button'))) return
  e.preventDefault()
  gesture = {
    id: c.i,
    kind,
    x: e.clientX,
    y: e.clientY,
    original: { ...c },
    cells: props.cells.map((c) => ({ ...c })),
  }
  grid.value.setPointerCapture(e.pointerId)
}
function move(e) {
  if (!gesture) return
  const dx = Math.round((e.clientX - gesture.x) / (width.value / props.columns)),
    dy = Math.round((e.clientY - gesture.y) / 60),
    c = gesture.original
  preview.value = moveGridCell(
    gesture.cells,
    c.i,
    gesture.kind === 'move' ? { x: c.x + dx, y: c.y + dy } : { w: c.w + dx, h: c.h + dy },
    props.columns,
  )
}
function end() {
  if (preview.value) emit('update:cells', preview.value)
  cancel()
}
function cancel() {
  gesture = null
  preview.value = null
}
const deltas = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] }
function keyMove(e, c) {
  if (!props.editable || e.target !== e.currentTarget || !deltas[e.key]) return
  e.preventDefault()
  const [x, y] = deltas[e.key]
  emit('update:cells', moveGridCell(props.cells, c.i, { x: c.x + x, y: c.y + y }, props.columns))
}
function keyResize(e, c) {
  if (!deltas[e.key]) return
  e.preventDefault()
  const [w, h] = deltas[e.key]
  emit('update:cells', moveGridCell(props.cells, c.i, { w: c.w + w, h: c.h + h }, props.columns))
}
onMounted(() => {
  observer = new ResizeObserver(() => {
    width.value = grid.value?.clientWidth || 900
    emit('width', width.value)
  })
  observer.observe(grid.value)
})
onBeforeUnmount(() => observer?.disconnect())
</script>
<style scoped>
.dashboard-grid {
  position: relative;
  min-height: 240px;
  margin: 16px -6px;
  background-image: radial-gradient(#d7dfeb 1px, transparent 1px);
  background-size: 20px 20px;
}
.dashboard-cell {
  position: absolute;
  padding: 6px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}
.dashboard-cell > header {
  background: white;
  border: 1px solid #dce3eb;
  border-bottom: 0;
  border-radius: 8px 8px 0 0;
  padding: 10px 14px;
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  min-height: 46px;
}
.grid-drag-handle {
  cursor: move;
  touch-action: none;
}
.dashboard-cell-body {
  background: white;
  border: 1px solid #dce3eb;
  border-top: 0;
  border-radius: 0 0 8px 8px;
  padding: 12px;
  overflow: auto;
  flex: 1;
  min-height: 0;
}
.grid-resize-handle {
  position: absolute;
  bottom: 8px;
  right: 8px;
  padding: 0;
  width: 22px;
  height: 22px;
  min-height: 0;
  cursor: nwse-resize;
  touch-action: none;
  background: transparent;
  color: #527386;
}
</style>
