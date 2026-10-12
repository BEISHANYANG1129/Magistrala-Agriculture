<template>
  <section class="panel">
    <h3>{{ tr('位置与围栏', 'Location and perimeter') }}</h3>
    <p>
      {{
        tr(
          '位置供标记地图使用，围栏供区域地图使用。点击保存才会更新此资源。',
          'Marker maps use the location; polygon maps use the perimeter. Save to update this resource.',
        )
      }}
    </p>
    <form @submit.prevent="save">
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('编辑内容', 'Edit')
          }}<select v-model="kind" @change="load">
            <option value="location">{{ tr('位置', 'Location') }}</option>
            <option value="perimeter">{{ tr('围栏', 'Perimeter') }}</option>
          </select></label
        ><label class="field"
          >{{ tr('元数据字段', 'Metadata key') }}<input v-model="key" @change="load" required
        /></label>
        <label v-if="kind === 'perimeter'" class="field"
          >{{ tr('形状', 'Shape')
          }}<select v-model="shape">
            <option value="polygon">{{ tr('多边形', 'Polygon') }}</option>
            <option value="rectangle">{{ tr('矩形', 'Rectangle') }}</option>
            <option value="circle">{{ tr('圆形', 'Circle') }}</option>
          </select></label
        >
      </div>
      <div v-if="kind === 'location' || shape === 'circle'" class="native-form-grid">
        <label class="field"
          >{{ tr('纬度', 'Latitude')
          }}<input
            v-model.number="latitude"
            type="number"
            min="-90"
            max="90"
            step="any"
            required /></label
        ><label class="field"
          >{{ tr('经度', 'Longitude')
          }}<input
            v-model.number="longitude"
            type="number"
            min="-180"
            max="180"
            step="any"
            required /></label
        ><label v-if="kind === 'perimeter'" class="field"
          >{{ tr('半径（公里）', 'Radius (kilometers)')
          }}<input v-model.number="radius" type="number" min="0.001" step="any" required
        /></label>
      </div>
      <template v-else
        ><p>
          {{
            tr(
              '在地图点击添加坐标；拖动顶点调整位置。',
              'Click the map to add coordinates; drag a vertex to adjust it.',
            )
          }}
        </p>
        <div v-for="(p, i) in coordinates" :key="i" class="toolbar">
          <label
            >{{ tr('纬度', 'Latitude')
            }}<input
              v-model.number="p[0]"
              type="number"
              min="-90"
              max="90"
              step="any"
              required /></label
          ><label
            >{{ tr('经度', 'Longitude')
            }}<input
              v-model.number="p[1]"
              type="number"
              min="-180"
              max="180"
              step="any"
              required /></label
          ><button type="button" class="quiet" @click="coordinates.splice(i, 1)">
            {{ tr('移除', 'Remove') }}
          </button>
        </div>
        <button type="button" class="quiet" @click="coordinates.push([0, 0])">
          + {{ tr('坐标', 'Coordinate') }}
        </button></template
      >
      <div ref="host" class="geometry-map"></div>
      <div class="toolbar">
        <button type="button" class="quiet" @click="fit">{{ tr('适应形状', 'Fit shape') }}</button
        ><label
          ><input v-model="online" type="checkbox" />{{ tr('在线底图', 'Online basemap') }}</label
        ><button :disabled="busy">{{ tr('保存位置 / 围栏', 'Save location / perimeter') }}</button>
      </div>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="notice" class="success">{{ notice }}</p>
    </form>
  </section>
</template>
<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { tr } from '../i18n'
import { readResource, resourceConfig } from '../api/resourceWorkflow'
import { runOperation, errorText } from '../api/platform'
import { metadataValue, perimeterOf } from '../api/widgetConfig'
import { editedMetadata } from '../api/editedMetadata'
const props = defineProps({ record: Object, section: String, workspace: String }),
  emit = defineEmits(['saved'])
const host = ref(),
  kind = ref('location'),
  key = ref('location'),
  shape = ref('polygon'),
  latitude = ref(0),
  longitude = ref(0),
  radius = ref(1),
  coordinates = ref([]),
  online = ref(false),
  error = ref(''),
  notice = ref(''),
  busy = ref(false)
const clone = (v) => JSON.parse(JSON.stringify(v))
let map,
  layer,
  tiles,
  observer,
  baseline = {}
function load() {
  baseline = clone(props.record.attributes?.metadata || {})
  if (['location', 'perimeter'].includes(key.value)) key.value = kind.value
  const v = metadataValue(baseline[key.value]) || {}
  latitude.value = v.latitude ?? v.center?.[0] ?? 0
  longitude.value = v.longitude ?? v.center?.[1] ?? 0
  shape.value = v.type || 'polygon'
  radius.value = v.radius ?? 1
  coordinates.value = clone(v.coordinates || v.bounds || [])
  notice.value = ''
  draw()
  fit()
}
function draw() {
  if (!map) return
  layer.clearLayers()
  const pairs =
    kind.value === 'location' || shape.value === 'circle'
      ? [[latitude.value, longitude.value]]
      : coordinates.value
  pairs.forEach((p, i) => {
    if (!p.every(Number.isFinite)) return
    L.marker(p, {
      draggable: true,
      icon: L.divIcon({ html: '●', className: 'geometry-point', iconSize: [18, 18] }),
    })
      .on('dragend', (e) => {
        const v = e.target.getLatLng()
        if (kind.value === 'location' || shape.value === 'circle') {
          latitude.value = v.lat
          longitude.value = v.lng
        } else coordinates.value[i] = [v.lat, v.lng]
      })
      .addTo(layer)
  })
  if (kind.value === 'perimeter') {
    const style = { color: '#19806c', fillOpacity: 0.16 }
    if (shape.value === 'polygon' && pairs.length >= 3) L.polygon(pairs, style).addTo(layer)
    if (shape.value === 'rectangle' && pairs.length === 2) L.rectangle(pairs, style).addTo(layer)
    if (shape.value === 'circle' && radius.value > 0)
      L.circle(pairs[0], { ...style, radius: radius.value * 1000 }).addTo(layer)
  }
}
function fit() {
  const b = layer?.getBounds()
  if (b?.isValid()) map.fitBounds(b.pad(0.2), { maxZoom: 14 })
}
async function save() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    if (!key.value.trim() || key.value.includes('.'))
      throw new Error(tr('字段名不能为空且不能包含点。', 'Use a nonempty key without dots.'))
    const value =
      kind.value === 'location'
        ? { latitude: latitude.value, longitude: longitude.value }
        : shape.value === 'circle'
          ? { type: 'circle', center: [latitude.value, longitude.value], radius: radius.value }
          : shape.value === 'rectangle'
            ? { type: 'rectangle', bounds: coordinates.value }
            : { type: 'polygon', coordinates: coordinates.value }
    if (kind.value === 'perimeter' && !perimeterOf({ metadata: { perimeter: { value } } }))
      throw new Error(
        tr(
          '形状坐标无效；多边形至少三个点，矩形正好两个点。',
          'Invalid geometry: polygons need at least three points, rectangles exactly two.',
        ),
      )
    const latest = await readResource(props.section, props.record.id, props.workspace),
      metadata = editedMetadata(
        baseline,
        {
          ...baseline,
          [key.value]: {
            ...(baseline[key.value]?.value ? baseline[key.value] : {}),
            type: kind.value,
            value,
          },
        },
        latest.attributes?.metadata,
      )
    const saved = await runOperation('update' + resourceConfig[props.section].type, {
      id: latest.id,
      input: { attributes: { ...latest.attributes, metadata } },
    })
    baseline = clone(metadata)
    emit('saved', saved)
    notice.value = tr('位置配置已保存。', 'Geometry saved.')
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
onMounted(() => {
  map = L.map(host.value, { scrollWheelZoom: false }).setView([0, 0], 2)
  layer = L.featureGroup().addTo(map)
  map.on('click', (e) => {
    const p = [e.latlng.lat, e.latlng.lng]
    if (kind.value === 'location' || shape.value === 'circle') {
      latitude.value = p[0]
      longitude.value = p[1]
    } else if (shape.value !== 'rectangle' || coordinates.value.length < 2)
      coordinates.value.push(p)
  })
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => map.invalidateSize())
    observer.observe(host.value)
  }
  load()
})
watch(() => props.record, load)
watch([latitude, longitude, radius, shape, coordinates], draw, { deep: true })
watch(online, (v) => {
  tiles?.remove()
  if (v)
    tiles = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© OpenStreetMap contributors',
    }).addTo(map)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  map?.remove()
})
</script>
<style scoped>
.geometry-map {
  height: 300px;
  margin: 12px 0;
  z-index: 0;
  background: #edf2f4;
}
.toolbar {
  flex-wrap: wrap;
}
.toolbar input[type='number'] {
  max-width: 170px;
}
</style>
