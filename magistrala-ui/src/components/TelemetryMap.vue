<template>
  <div class="telemetry-map-widget">
    <div class="actions">
      <button class="quiet" @click="fit">{{ tr('适应数据', 'Fit data') }}</button
      ><label v-if="!floorPlan" class="check"
        ><input v-model="online" type="checkbox" />{{
          tr('显示在线底图（OpenStreetMap）', 'Show online basemap (OpenStreetMap)')
        }}</label
      >
    </div>
    <div
      ref="host"
      class="telemetry-map"
      :aria-label="tr('设备位置地图', 'Device locations map')"
    ></div>
    <p v-if="!points.length && !shapes.length" class="notice">
      {{
        tr(
          '没有有效的位置记录。需要纬度、经度字段，或包含 latitude / longitude 的位置对象。',
          'No valid positions. Provide latitude and longitude metrics, or a position object containing latitude / longitude.',
        )
      }}
    </p>
    <p v-if="floorPlan && !config.image" class="notice">
      {{
        tr(
          '请在组件设置中添加平面图。X 为横向，Y 为纵向，单位使用图片像素。',
          'Add a floor plan in widget settings. X and Y use image pixels.',
        )
      }}
    </p>
    <p v-if="error" class="error">{{ error }}</p>
  </div>
</template>
<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { tr, language } from '../i18n'
import { mapPoints } from '../api/mapPoints'
const props = defineProps({
  rows: Array,
  shapes: { type: Array, default: () => [] },
  type: String,
  config: { type: Object, default: () => ({}) },
})
const host = ref(null),
  online = ref(false),
  error = ref(''),
  floorPlan = computed(() => props.type === 'building_overview')
const points = computed(() =>
  mapPoints(props.rows, { ...props.config, floorPlan: floorPlan.value }),
)
let zoom,
  map,
  layer,
  tiles,
  observer,
  image,
  drawVersion = 0
function fit() {
  if (!map) return
  const bounds = layer?.getBounds()
  if (bounds?.isValid()) map.fitBounds(bounds.pad(0.2), { maxZoom: 16 })
  else if (image) map.fitBounds(image.getBounds())
  else map.setView([0, 0], 2)
}
async function draw() {
  const version = ++drawVersion
  if (!map) return
  const markerFactory = points.value.length ? (await import('../api/mapMarker')).mapMarker : null
  if (!map || version !== drawVersion) return
  layer.clearLayers()
  image?.remove()
  image = null
  if (floorPlan.value && props.config.image) {
    image = L.imageOverlay(props.config.image, [
      [0, 0],
      [Number(props.config.height) || 800, Number(props.config.width) || 1200],
    ]).addTo(map)
  }
  for (const shape of props.shapes) {
    const style = { color: shape.color || '#19806c', fillOpacity: 0.18 }
    const object =
      shape.type === 'polygon'
        ? L.polygon(shape.coordinates, style)
        : shape.type === 'rectangle'
          ? L.rectangle(shape.bounds, style)
          : L.circle(shape.center, { ...style, radius: shape.radius * 1000 })
    const label = document.createElement('span')
    label.textContent = shape.name || shape.type
    object.bindPopup(label).addTo(layer)
  }
  const groups = new Map()
  for (const p of points.value) {
    const list = groups.get(p.publisher) || []
    list.push(p)
    groups.set(p.publisher, list)
    const label = document.createElement('span')
    label.textContent = (p.label || p.name || p.publisher) + ' · ' + p.lat + ', ' + p.lng
    L.marker([p.lat, p.lng], {
      icon: L.divIcon({
        className: 'telemetry-marker',
        html: markerFactory(p.icon || 'map-pin', p.color),
        iconSize: [28, 28],
        iconAnchor: [14, 14],
      }),
    })
      .bindPopup(label)
      .addTo(layer)
  }
  for (const positions of groups.values()) {
    const coords = positions.map((p) => [p.lat, p.lng])
    if (props.type === 'route_map' && coords.length > 1)
      L.polyline(coords, {
        color: positions[0].color || '#2167b6',
        weight: Number(props.config.width) || 3,
      }).addTo(layer)
    else if (props.type === 'polygon_map' && coords.length > 2)
      L.polygon(coords, { color: '#19806c', fillOpacity: 0.18 }).addTo(layer)
  }
  fit()
}
function basemap() {
  tiles?.remove()
  tiles = null
  error.value = ''
  if (map && online.value && !floorPlan.value) {
    tiles = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '© OpenStreetMap contributors',
    }).addTo(map)
    tiles.on(
      'tileerror',
      () =>
        (error.value = tr(
          '底图暂时不可用，位置数据仍可查看。',
          'Basemap unavailable. Position data remains visible.',
        )),
    )
  }
}
function updateZoom() {
  if (!map) return
  zoom?.remove()
  zoom = L.control
    .zoom({ zoomInTitle: tr('放大', 'Zoom in'), zoomOutTitle: tr('缩小', 'Zoom out') })
    .addTo(map)
}
watch(language, updateZoom)
function init() {
  map?.remove()
  map = L.map(host.value, {
    crs: floorPlan.value ? L.CRS.Simple : L.CRS.EPSG3857,
    scrollWheelZoom: false,
    zoomControl: false,
  }).setView([0, 0], 2)
  updateZoom()
  layer = L.featureGroup().addTo(map)
  draw()
  basemap()
}
onMounted(() => {
  init()
  observer = new ResizeObserver(() => map?.invalidateSize())
  observer.observe(host.value)
})
watch(() => [props.rows, props.config, props.shapes], draw, { deep: true })
watch(
  () => props.type,
  () => host.value && init(),
)
watch(online, basemap)
onBeforeUnmount(() => {
  drawVersion++
  observer?.disconnect()
  map?.remove()
  map = null
})
</script>
<style scoped>
.telemetry-map {
  flex: 1;
  min-height: 160px;
  width: 100%;
  background: #edf2f4;
  z-index: 0;
}
.telemetry-map-widget {
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}
.telemetry-map-widget .actions {
  margin-bottom: 8px;
}
.telemetry-map :deep(.map-marker-icon) {
  display: block;
  background: white;
  border-radius: 50%;
  box-shadow: 0 1px 4px #0003;
}
</style>
