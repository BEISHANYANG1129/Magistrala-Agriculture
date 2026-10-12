<template>
  <section :class="['flow-editor', { 'flow-expanded': expanded }]">
    <div class="toolbar">
      <button class="quiet" @click="emit('add', 'input')">
        + {{ tr('添加输入', 'Add input') }}</button
      ><button class="quiet" @click="emit('add', 'logic')">
        + {{ tr('添加逻辑', 'Add logic') }}</button
      ><button class="quiet" @click="emit('add', 'output')">
        + {{ tr('添加输出', 'Add output') }}</button
      ><select v-model="direction" :aria-label="tr('布局方向', 'Layout direction')">
        <option value="horizontal">{{ tr('水平', 'Horizontal') }}</option>
        <option value="vertical">{{ tr('垂直', 'Vertical') }}</option></select
      ><button class="quiet" @click="layout">{{ tr('自动布局', 'Auto layout') }}</button
      ><button class="quiet" @click="fit">{{ tr('适应画布', 'Fit view') }}</button
      ><button class="quiet" @click="expanded = !expanded">
        {{ expanded ? tr('退出放大', 'Exit expanded view') : tr('放大画布', 'Expand canvas') }}
      </button>
    </div>
    <p v-if="canvasError" class="error" role="alert">{{ canvasError }}</p>
    <div v-if="connecting || selectedEdge" class="toolbar">
      <span v-if="connecting">{{
        tr('选择另一个节点的输入端口完成连线。', 'Choose another node’s input port to connect.')
      }}</span>
      <button v-if="selectedEdge" class="quiet danger" @click="removeEdge">
        {{ tr('删除选中连线', 'Delete selected connection') }}
      </button>
      <button
        class="quiet"
        @click="
          () => {
            connecting = ''
            selectedEdge = ''
          }
        "
      >
        {{ tr('取消选择', 'Clear selection') }}
      </button>
    </div>
    <div
      ref="viewport"
      class="flow-viewport"
      @pointerdown="startPan"
      @pointermove="move"
      @pointerup="end"
      @pointercancel="end"
      @wheel.prevent="wheel"
    >
      <div
        class="flow-world"
        :style="{ transform: `translate(${view.x}px,${view.y}px) scale(${view.zoom})` }"
      >
        <svg class="flow-edges" width="4000" height="3000">
          <defs>
            <marker
              id="flow-arrow"
              markerUnits="userSpaceOnUse"
              markerWidth="10"
              markerHeight="10"
              refX="8"
              refY="3"
              orient="auto"
            >
              <path d="M0,0 L0,6 L9,3 z" fill="#8899b5" />
            </marker>
          </defs>
          <path
            v-for="e in edges"
            :key="e.key"
            :d="e.path"
            fill="none"
            :stroke="selectedEdge === e.key ? '#19806c' : '#8899b5'"
            class="flow-edge-path"
            tabindex="0"
            role="button"
            :aria-label="tr('选择连线', 'Select connection') + ' ' + e.key"
            @pointerdown.stop="selectedEdge = e.key"
            @keydown.enter="selectedEdge = e.key"
            @keydown.delete.prevent="
              () => {
                selectedEdge = e.key
                removeEdge()
              }
            "
            stroke-width="2"
            marker-end="url(#flow-arrow)"
          />
        </svg>
        <article
          v-for="n in nodes"
          :key="n.key"
          class="flow-node"
          :style="{ left: pos(n.key).x + 'px', top: pos(n.key).y + 'px' }"
        >
          <div
            class="flow-node-handle"
            tabindex="0"
            :aria-label="tr('移动节点', 'Move node') + ' ' + n.title"
            @pointerdown.stop="startNode($event, n.key)"
            @keydown="keyMove($event, n.key)"
          >
            <span class="badge">{{
              n.kind === 'input'
                ? tr('输入', 'Input')
                : n.kind === 'logic'
                  ? tr('逻辑', 'Logic')
                  : tr('输出', 'Output')
            }}</span
            ><span aria-hidden="true">⠿</span>
          </div>
          <button
            v-if="n.kind !== 'input'"
            class="flow-port flow-port-in"
            :aria-label="tr('输入端口', 'Input port') + ' ' + n.title"
            @pointerup.stop="finishConnection(n.key)"
            @click.stop="finishConnection(n.key)"
          >
            ●
          </button>
          <button
            v-if="n.kind !== 'output'"
            class="flow-port flow-port-out"
            :class="{ active: connecting === n.key }"
            :aria-label="tr('输出端口', 'Output port') + ' ' + n.title"
            @pointerdown.stop="connecting = n.key"
            @click.stop="connecting = n.key"
          >
            ●
          </button>
          <h3>{{ n.title }}</h3>
          <pre v-if="n.kind === 'logic'">{{ n.text }}</pre>
          <p v-else>{{ n.text }}</p>
          <div class="actions">
            <button class="quiet" @click="emit('edit', n.kind, n.index)">
              {{ tr('编辑', 'Edit') }}</button
            ><button class="quiet danger" @click="emit('remove', n.kind, n.index)">
              {{ tr('移除', 'Remove') }}
            </button>
          </div>
        </article>
      </div>
      <p v-if="!nodes.length" class="flow-empty">
        {{
          tr(
            '从上方添加输入、逻辑和输出。拖动节点标题移动，拖动画布平移，滚轮缩放。',
            'Add input, logic and outputs above. Drag node headers to move, drag the background to pan, and use the wheel to zoom.',
          )
        }}
      </p>
      <div class="flow-controls">
        <button class="quiet" :aria-label="tr('缩小', 'Zoom out')" @click="zoomAt(view.zoom / 1.2)">
          −</button
        ><span>{{ Math.round(view.zoom * 100) }}%</span
        ><button class="quiet" :aria-label="tr('放大', 'Zoom in')" @click="zoomAt(view.zoom * 1.2)">
          +</button
        ><button class="quiet" @click="fit">⌖</button>
      </div>
    </div>
    <small>{{
      tr(
        '拖动端口或依次点击输出、输入端口连接节点。连线和布局随规则保存；执行仍由规则的逻辑和输出配置决定。',
        'Drag between ports or click an output then an input port. Connections and layout are saved with the rule; execution follows its logic and output configuration.',
      )
    }}</small>
  </section>
</template>
<script setup>
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { tr } from '../i18n'
import { ruleCanvasState } from '../api/ruleFlow'
const props = defineProps({ draft: Object, channels: Array, outputLabel: Function }),
  emit = defineEmits(['add', 'edit', 'remove', 'layout', 'edges'])
const viewport = ref(null),
  expanded = ref(false),
  positions = ref({}),
  view = reactive({ x: 30, y: 30, zoom: 1 })
const connections = ref([]),
  connecting = ref(''),
  selectedEdge = ref(''),
  canvasError = ref(''),
  direction = ref('horizontal')
let gesture = null
const nodes = computed(() => [
  ...(props.draft.input_channel
    ? [
        {
          key: 'input',
          kind: 'input',
          title: tr('通道订阅', 'Channel subscriber'),
          text:
            props.channels.find((c) => c.id === props.draft.input_channel)?.name ||
            props.draft.input_channel,
        },
      ]
    : []),
  ...(props.draft.logic?.value
    ? [
        {
          key: 'logic',
          kind: 'logic',
          title: props.draft.logic.type === 1 ? 'Go' : 'Lua',
          text: props.draft.logic.value.slice(0, 140),
        },
      ]
    : []),
  ...(props.draft.outputs || []).map((o, i) => ({
    key: 'output-' + (o.id || i),
    kind: 'output',
    index: i,
    title: props.outputLabel(o.type),
    text: props.channels.find((c) => c.id === o.channel)?.name || o.channel || '',
  })),
])
function defaultPos(key) {
  const index = nodes.value.findIndex((n) => n.key === key)
  const n = nodes.value[index]
  return {
    x: n?.kind === 'input' ? 20 : n?.kind === 'logic' ? 360 : 700,
    y: n?.kind === 'output' ? 20 + (n.index || 0) * 230 : 20,
  }
}
function pos(key) {
  return positions.value[key] || defaultPos(key)
}
const edges = computed(() => {
  return connections.value
    .filter(
      (e) =>
        nodes.value.some((n) => n.key === e.source) && nodes.value.some((n) => n.key === e.target),
    )
    .map(({ id, source: a, target: b }) => {
      const p = pos(a),
        q = pos(b)
      return {
        key: id,
        path: `M ${p.x + 260},${p.y + 90} C ${p.x + 310},${p.y + 90} ${q.x - 50},${q.y + 90} ${q.x},${q.y + 90}`,
      }
    })
})
watch(
  () => props.draft,
  () => {
    try {
      const state = ruleCanvasState(props.draft)
      positions.value = state.positions
      connections.value = state.edges
      canvasError.value = ''
    } catch (e) {
      canvasError.value = e.message
      positions.value = {}
      connections.value = []
    }
    connecting.value = ''
    selectedEdge.value = ''
    nextTick(fit)
  },
  { immediate: true },
)
function persist() {
  emit('layout', { ...positions.value })
}
function startNode(e, key) {
  if (e.button !== 0) return
  gesture = { kind: 'node', key, x: e.clientX, y: e.clientY, start: { ...pos(key) } }
  viewport.value.setPointerCapture(e.pointerId)
}
function startPan(e) {
  if (e.button !== 0 || e.target.closest('button,article')) return
  gesture = { kind: 'pan', x: e.clientX, y: e.clientY, start: { x: view.x, y: view.y } }
  viewport.value.setPointerCapture(e.pointerId)
}
function move(e) {
  if (!gesture) return
  const dx = e.clientX - gesture.x,
    dy = e.clientY - gesture.y
  if (gesture.kind === 'pan') {
    view.x = gesture.start.x + dx
    view.y = gesture.start.y + dy
  } else
    positions.value[gesture.key] = {
      x: Math.max(0, Math.min(3500, gesture.start.x + dx / view.zoom)),
      y: Math.max(0, Math.min(2600, gesture.start.y + dy / view.zoom)),
    }
}
function end() {
  if (gesture?.kind === 'node') persist()
  gesture = null
}
function keyMove(e, key) {
  const delta = { ArrowLeft: [-15, 0], ArrowRight: [15, 0], ArrowUp: [0, -15], ArrowDown: [0, 15] }[
    e.key
  ]
  if (!delta) return
  e.preventDefault()
  const p = pos(key)
  positions.value[key] = { x: Math.max(0, p.x + delta[0]), y: Math.max(0, p.y + delta[1]) }
  persist()
}
function zoomAt(
  z,
  x = viewport.value?.clientWidth / 2 || 400,
  y = viewport.value?.clientHeight / 2 || 250,
) {
  z = Math.max(0.25, Math.min(2, z))
  const ratio = z / view.zoom
  view.x = x - (x - view.x) * ratio
  view.y = y - (y - view.y) * ratio
  view.zoom = z
}
function wheel(e) {
  const r = viewport.value.getBoundingClientRect()
  zoomAt(view.zoom * (e.deltaY > 0 ? 0.9 : 1.1), e.clientX - r.left, e.clientY - r.top)
}
function fit() {
  const points = nodes.value.map((n) => pos(n.key))
  if (!points.length) {
    view.x = 30
    view.y = 30
    view.zoom = 1
    return
  }
  const minX = Math.min(...points.map((p) => p.x)),
    minY = Math.min(...points.map((p) => p.y)),
    width = Math.max(...points.map((p) => p.x + 280)) - minX,
    height = Math.max(...points.map((p) => p.y + 220)) - minY
  view.zoom = Math.max(
    0.25,
    Math.min(
      1,
      (viewport.value?.clientWidth - 60 || 900) / width,
      (viewport.value?.clientHeight - 60 || 440) / height,
    ),
  )
  view.x = 30 - minX * view.zoom
  view.y = 30 - minY * view.zoom
}
function layout() {
  positions.value = Object.fromEntries(
    nodes.value.map((n) => {
      const p = defaultPos(n.key)
      return [n.key, direction.value === 'vertical' ? { x: p.y, y: p.x } : p]
    }),
  )
  persist()
  fit()
}
function finishConnection(target) {
  const source = connecting.value
  if (!source || source === target) return
  if (!connections.value.some((e) => e.source === source && e.target === target))
    connections.value.push({ id: crypto.randomUUID(), source, target })
  connecting.value = ''
  emit('edges', [...connections.value])
}
function removeEdge() {
  connections.value = connections.value.filter((e) => e.id !== selectedEdge.value)
  selectedEdge.value = ''
  emit('edges', [...connections.value])
}
watch(
  () => nodes.value.map((n) => n.key).join('|'),
  () => {
    const keys = new Set(nodes.value.map((n) => n.key))
    connections.value = connections.value.filter((e) => keys.has(e.source) && keys.has(e.target))
  },
)
watch(expanded, () => nextTick(fit))
</script>

<style scoped>
.flow-port {
  position: absolute;
  top: 85px;
  border-radius: 50%;
  width: 22px;
  height: 22px;
  min-height: 22px;
  padding: 0;
  z-index: 3;
  touch-action: none;
  background: #fff;
  color: #19624f;
  border: 2px solid #19624f;
}
.flow-port-in {
  left: -12px;
}
.flow-port-out {
  right: -12px;
}
.flow-port.active {
  background: #19624f;
  color: white;
}
.flow-edge-path {
  pointer-events: stroke;
  cursor: pointer;
  stroke-width: 5;
}
.flow-edge-path:focus {
  outline: none;
  stroke: #19806c;
  stroke-width: 7;
}
</style>
