<template>
  <div class="topology-card">
    <h2>物理拓扑展示</h2>
    <p v-if="error" role="alert">{{ error }}</p>
    <div v-if="loading" class="topology-loading">加载中...</div>
    <div v-else>
      <div class="topology-bg-wrap" v-if="bgUrl">
        <img
          :src="bgUrl"
          class="topology-bg"
          ref="bgImg"
          :style="{ width: fixedWidth + 'px', height: displayHeight + 'px', display: 'block' }"
          @load="onBgLoad"
        />
        <svg
          v-if="bgLoaded"
          :width="displayWidth"
          :height="displayHeight"
          class="topology-svg"
          style="position:absolute;left:0;top:0;"
        >
          <g v-for="part in partitions" :key="part.id">
            <polygon
              v-if="part.shape === 'polygon'"
              :points="polygonPoints(part.points)"
              :fill="part.color"
              fill-opacity="0.25"
              stroke="black"
              stroke-width="2"
            />
            <rect
              v-else-if="part.shape === 'rect'"
              v-bind="rectAttrs(part)"
              :fill="part.color"
              fill-opacity="0.25"
              stroke="black"
              stroke-width="2"
            />
            <text
              :x="labelX(part)"
              :y="labelY(part)"
              font-size="18"
              fill="#174e8a"
              font-weight="bold"
            >
              {{ part.name }}
            </text>
          </g>
          <g v-for="client in clientsWithPosition" :key="client.id">
            <circle
              :cx="client.metadata.position.x * displayWidth"
              :cy="client.metadata.position.y * displayHeight"
              r="10"
              fill="#e53935"
              fill-opacity="0.8"
              stroke="#fff"
              stroke-width="2"
            />
            <text
              :x="client.metadata.position.x * displayWidth"
              :y="client.metadata.position.y * displayHeight - 14"
              text-anchor="middle"
              font-size="14"
              fill="#e53935"
              font-weight="bold"
            >
              {{ client.name }}
            </text>
          </g>
        </svg>
      </div>
      <div v-else class="topology-empty">未设置底图</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { graphql } from '../api/atom'

const route = useRoute()
const domainId = route.params.id
const channelId = route.params.channelId
const error = ref('')

const loading = ref(true)
const bgUrl = ref('')
const partitions = ref([])
const bgLoaded = ref(false)
const bgImg = ref(null)

// 固定显示宽度
const fixedWidth = 1000
const displayWidth = ref(fixedWidth)
const displayHeight = ref(600)

// client 列表
const clients = ref([])

// 获取 channel 信息，解析 metadata
async function fetchChannel() {
  loading.value = true
  try {
    const data = await graphql('query($id: ID!) { resource(id: $id) { id attributes } }', { id: String(channelId) })
    const meta = data.resource?.attributes || {}
    bgUrl.value = meta.background || ''
    partitions.value = meta.partitions || []
  } catch (caught) { error.value = caught instanceof Error ? caught.message : '拓扑加载失败' }
  loading.value = false
}

// 获取 connect 到本 channel 的所有 client
async function fetchClients() {
  try {
    const data = await graphql('query($id: ID!) { entities(tenantId: $id, kind: "device", limit: 100) { items { id name attributes } } }', { id: String(domainId) })
    clients.value = (data.entities?.items || []).map(item => ({ ...item, metadata: item.attributes || {} }))
  } catch (caught) { error.value = caught instanceof Error ? caught.message : '设备加载失败' }
}

// 图片加载后获取等比例缩放后的显示高度
function onBgLoad() {
  if (bgImg.value) {
    const ratio = bgImg.value.naturalHeight / bgImg.value.naturalWidth
    displayWidth.value = fixedWidth
    displayHeight.value = Math.round(fixedWidth * ratio)
    bgLoaded.value = true
  }
}

// 多边形分区点格式化（比例转像素）
function polygonPoints(points) {
  return points.map(pt => [
    pt[0] * displayWidth.value,
    pt[1] * displayHeight.value
  ].join(',')).join(' ')
}

// 矩形分区属性（比例转像素）
function rectAttrs(part) {
  const x = part.points[0][0] * displayWidth.value
  const y = part.points[0][1] * displayHeight.value
  const width = (part.points[2][0] - part.points[0][0]) * displayWidth.value
  const height = (part.points[2][1] - part.points[0][1]) * displayHeight.value
  return { x, y, width, height }
}

// 分区标签位置（比例转像素）
function labelX(part) {
  if (part.shape === 'rect') {
    return ((part.points[0][0] + part.points[2][0]) / 2) * displayWidth.value
  }
  const xs = part.points.map(p => p[0] * displayWidth.value)
  return xs.reduce((a, b) => a + b, 0) / xs.length
}
function labelY(part) {
  if (part.shape === 'rect') {
    return ((part.points[0][1] + part.points[2][1]) / 2) * displayHeight.value
  }
  const ys = part.points.map(p => p[1] * displayHeight.value)
  return ys.reduce((a, b) => a + b, 0) / ys.length
}

// 只取有 position 的 client
const clientsWithPosition = computed(() =>
  clients.value.filter(
    c => c.metadata && c.metadata.position && typeof c.metadata.position.x === 'number' && typeof c.metadata.position.y === 'number'
  )
)

onMounted(async () => {
  await fetchChannel()
  await fetchClients()
})
</script>

<style scoped>
.topology-card {
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 0 24px #eee;
  padding: 0;
  min-width: 0;
  position: relative;
  width: 100%;
  height: 100%;
}
.topology-loading {
  color: #174e8a;
  font-size: 1.2em;
  text-align: center;
  padding: 32px 0;
}
.topology-bg-wrap {
  position: relative;
  width: fit-content;
  margin: 0 auto;
}
.topology-bg {
  display: block;
  border-radius: 12px;
}
.topology-svg {
  position: absolute;
  left: 0; top: 0;
  pointer-events: none;
}
.topology-empty {
  color: #888;
  font-size: 1.2em;
  text-align: center;
  padding: 32px 0;
}
</style>
