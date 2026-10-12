<template>
  <BuildingOverview v-if="type === 'building_overview'" :buildings="c.buildings" editable />
  <template v-else>
    <fieldset v-if="entityMap">
      <legend>{{ tr('地图资源', 'Map entities') }}</legend>
      <div v-for="(s, i) in c.dataSources" :key="i" class="panel">
        <div class="native-form-grid">
          <label class="field"
            >{{ tr('资源类型', 'Entity type')
            }}<select v-model="s.entityType" @change="resetEntity(s)">
              <option value="device">{{ tr('设备', 'Device') }}</option>
              <option value="channel">{{ tr('通道', 'Channel') }}</option>
              <option value="group">{{ tr('分组', 'Group') }}</option>
            </select></label
          >
          <label class="field"
            >{{ tr('资源', 'Entity')
            }}<select v-model="mapEntity(s).id" required>
              <option value="">{{ tr('请选择', 'Select') }}</option>
              <option v-for="r in entities(s.entityType)" :key="r.id" :value="r.id">
                {{ r.name }}
              </option>
            </select></label
          >
          <label class="field">{{ tr('标签', 'Label') }}<input v-model="s.label" /></label
          ><label class="field"
            >{{ tr('颜色', 'Color') }}<input v-model="s.color" type="color" /></label
          ><label class="field"
            >{{ tr('图标名称', 'Icon name') }}<input v-model="s.icon" placeholder="map-pin"
          /></label>
        </div>
        <template v-if="type === 'polygon_map'"
          ><label class="field"
            >{{ tr('子资源类型', 'Child type')
            }}<select v-model="s.childEntityType" @change="s.childEntities = []">
              <option value="group-device">{{ tr('设备', 'Device') }}</option>
              <option value="group-channel">{{ tr('通道', 'Channel') }}</option>
              <option value="group">{{ tr('分组', 'Group') }}</option>
            </select></label
          ><label class="field"
            >{{ tr('子资源', 'Children')
            }}<select
              multiple
              :value="(s.childEntities || []).map((r) => r.id)"
              @change="
                s.childEntities = Array.from($event.target.selectedOptions, (o) => ({
                  id: o.value,
                  name: o.textContent,
                }))
              "
            >
              <option v-for="r in entities(s.childEntityType)" :key="r.id" :value="r.id">
                {{ r.name }}
              </option>
            </select></label
          ></template
        >
        <button type="button" class="quiet" @click="c.dataSources.splice(i, 1)">
          {{ tr('移除数据源', 'Remove source') }}
        </button>
      </div>
      <button
        type="button"
        class="quiet"
        @click="
          c.dataSources.push({
            id: crypto.randomUUID(),
            entityType: 'group',
            entity: { id: '' },
            childEntityType: 'group-device',
            childEntities: [],
            color: '#19806c',
            icon: 'map-pin',
          })
        "
      >
        + {{ tr('添加资源', 'Add entity') }}
      </button>
      <div class="native-form-grid">
        <label v-for="field in mapFields" :key="field[0]" class="field"
          >{{ tr(field[1], field[2]) }}<input v-model="c[field[0]]" :placeholder="field[3]"
        /></label>
      </div>
    </fieldset>
    <fieldset v-if="['count_card', 'table_card'].includes(type)">
      <legend>{{ tr('统计资源', 'Resource selection') }}</legend>
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('资源类型', 'Entity')
          }}<select v-model="c.dataSource.entity">
            <option value="device">{{ tr('设备', 'Device') }}</option>
            <option value="channel">{{ tr('通道', 'Channel') }}</option>
            <option value="group">{{ tr('分组', 'Group') }}</option>
          </select></label
        ><label class="field"
          >{{ tr('状态', 'Status')
          }}<select v-model="c.dataSource.status">
            <option value="all">{{ tr('全部', 'All') }}</option>
            <option value="enabled">{{ tr('启用', 'Enabled') }}</option>
            <option value="disabled">{{ tr('禁用', 'Disabled') }}</option>
          </select></label
        ><label class="field"
          >{{ tr('标签（逗号分隔）', 'Tags (comma separated)')
          }}<input
            :value="
              Array.isArray(c.dataSource.tags) ? c.dataSource.tags.join(', ') : c.dataSource.tags
            "
            @input="
              c.dataSource.tags = $event.target.value
                .split(',')
                .map((s) => s.trim())
                .filter(Boolean)
            " /></label
        ><label class="field"
          >{{ tr('标签文字', 'Label') }}<input v-model="c.dataSource.label"
        /></label>
      </div>
    </fieldset>
    <fieldset v-if="control">
      <legend>{{ tr('控制设置', 'Control settings') }}</legend>
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('通道', 'Channel')
          }}<select v-model="c.dataSource.channel.id" required>
            <option value=""></option>
            <option v-for="r in channels" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select></label
        ><label class="field"
          >{{ tr('设备', 'Device')
          }}<select v-model="c.dataSource.client.id" required>
            <option value=""></option>
            <option v-for="r in devices" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select></label
        ><label class="field"
          >{{ tr('指标名称', 'Metric name') }}<input v-model="c.dataSource.label" required /></label
        ><label class="field">{{ tr('子主题', 'Subtopic') }}<input v-model="c.subtopic" /></label
        ><template v-if="type === 'slider_control'"
          ><label class="field"
            >{{ tr('最小值', 'Minimum')
            }}<input v-model.number="c.dataSource.minValue" type="number" step="any" /></label
          ><label class="field"
            >{{ tr('最大值', 'Maximum')
            }}<input v-model.number="c.dataSource.maxValue" type="number" step="any" /></label
          ><label class="field"
            >{{ tr('步长', 'Step')
            }}<input
              v-model.number="c.steps"
              type="number"
              min="0.001"
              step="any" /></label></template
        ><label v-else class="field"
          >{{ tr('开关样式', 'Switch style')
          }}<select v-model="c.switchStyle">
            <option value="button">{{ tr('按钮', 'Button') }}</option>
            <option value="slide">{{ tr('滑动开关', 'Slide') }}</option>
          </select></label
        >
      </div>
    </fieldset>
    <fieldset v-if="type.startsWith('alarm_')">
      <legend>{{ tr('告警筛选', 'Alarm filters') }}</legend>
      <AlarmFilters :filters="c" :workspace="workspace" :channels="channels" :devices="devices" />
      <div class="native-form-grid">
        <label v-if="type === 'alarm_count'" class="field"
          >{{ tr('标签文字', 'Label') }}<input v-model="c.label"
        /></label>
        <label v-if="type === 'alarm_count'" class="field"
          >{{ tr('颜色', 'Color')
          }}<select v-model="c.color">
            <option v-for="color in alarmColors" :key="color" :value="color">{{ color }}</option>
          </select></label
        >
        <label v-if="type === 'alarm_count'" class="field"
          >{{ tr('图标', 'Icon')
          }}<select v-model="c.icon">
            <option v-for="icon in alarmIcons" :key="icon" :value="icon">{{ icon }}</option>
          </select></label
        >
      </div>
    </fieldset>
    <fieldset v-if="type === 'alarm_table'">
      <legend>{{ tr('显示列', 'Visible columns') }}</legend>
      <div v-for="(column, index) in c.columns" :key="column.key" class="toolbar">
        <label><input type="checkbox" v-model="column.visible" />{{ column.key }}</label
        ><input v-model="column.label" :aria-label="tr('列标题', 'Column label')" /><button
          type="button"
          class="quiet"
          :disabled="index === 0"
          @click="moveColumn(index, -1)"
        >
          ↑</button
        ><button
          type="button"
          class="quiet"
          :disabled="index === c.columns.length - 1"
          @click="moveColumn(index, 1)"
        >
          ↓
        </button>
      </div>
    </fieldset>
    <fieldset v-if="telemetry">
      <legend>{{ tr('时间与聚合', 'Time and aggregation') }}</legend>
      <div class="native-form-grid">
        <label class="field"
          >{{ tr('时间窗口', 'Time window')
          }}<select v-model="c.timeWindowFormat">
            <option value="realtime">{{ tr('实时', 'Realtime') }}</option>
            <option value="history">{{ tr('历史', 'History') }}</option>
          </select></label
        ><label v-if="c.timeWindowFormat !== 'history'" class="field"
          >{{ tr('最近时长（毫秒）', 'Last duration (milliseconds)')
          }}<input v-model.number="c.last" type="number" min="1000" /></label
        ><template v-else
          ><label class="field"
            >{{ tr('开始时间', 'Start time')
            }}<input
              :value="localDate(c.startTime)"
              @input="c.startTime = $event.target.value"
              type="datetime-local"
              required /></label
          ><label class="field"
            >{{ tr('结束时间', 'End time')
            }}<input
              :value="localDate(c.stopTime)"
              @input="c.stopTime = $event.target.value"
              type="datetime-local"
              required /></label></template
        ><label class="field"
          >{{ tr('聚合', 'Aggregation')
          }}<select
            :value="c.dataType?.method || ''"
            @change="
              c.dataType = $event.target.value
                ? {
                    type: 'aggregation',
                    method: $event.target.value,
                    interval: c.dataType?.interval || '1m',
                  }
                : undefined
            "
          >
            <option value="">{{ tr('不聚合', 'None') }}</option>
            <option v-for="m in ['avg', 'min', 'max', 'sum', 'count']" :key="m">{{ m }}</option>
          </select></label
        ><label v-if="c.dataType" class="field"
          >{{ tr('聚合间隔', 'Aggregation interval')
          }}<input v-model="c.dataType.interval" placeholder="1m" required
        /></label>
      </div>
      <div v-for="(s, i) in c.dataSources" :key="i" class="native-form-grid">
        <label class="field"
          >{{ tr('指标', 'Metric') }} {{ i + 1
          }}<input v-model="s.valueName" :placeholder="c.valueName" /></label
        ><label class="field"
          >{{ tr('数据源标签', 'Source label') }} {{ i + 1 }}<input v-model="s.label" /></label
        ><label class="field"
          >{{ tr('颜色', 'Color') }}<input v-model="s.color" type="color" /></label
        ><label class="field"
          >{{ tr('单位', 'Unit')
          }}<input :value="unitText(s.unit)" @input="setUnit(s, $event.target.value)" /></label
        ><template v-if="type === 'gauge_chart'"
          ><label class="field"
            >{{ tr('最小值', 'Minimum')
            }}<input v-model.number="s.minValue" type="number" step="any" /></label
          ><label class="field"
            >{{ tr('最大值', 'Maximum')
            }}<input v-model.number="s.maxValue" type="number" step="any" /></label
        ></template>
      </div>
      <label v-if="type === 'gauge_chart'" class="field"
        >{{ tr('仪表样式', 'Gauge style')
        }}<select v-model="c.gaugeType">
          <option>Simple Gauge</option>
          <option>Temperature Gauge</option>
          <option>Speed Gauge</option>
        </select></label
      >
      <div
        class="native-form-grid"
        v-if="['line_chart', 'area_chart', 'bar_chart', 'route_map'].includes(type)"
      >
        <template v-if="type !== 'route_map'">
          <label class="field"
            >{{ tr('横轴标题', 'X axis label') }}<input v-model="c.xLabel"
          /></label>
          <label class="field"
            >{{ tr('纵轴标题', 'Y axis label') }}<input v-model="c.yLabel"
          /></label>
        </template>
        <label v-if="['line_chart', 'route_map'].includes(type)" class="field"
          >{{ tr('线宽', 'Line width')
          }}<input v-model.number="c.width" type="number" min="0.5" max="10" step="0.5"
        /></label>
        <template v-if="type === 'route_map'">
          <label class="field"
            >{{ tr('纬度字段', 'Latitude key')
            }}<input v-model="c.latitudeKey" placeholder="latitude"
          /></label>
          <label class="field"
            >{{ tr('经度字段', 'Longitude key')
            }}<input v-model="c.longitudeKey" placeholder="longitude"
          /></label>
        </template>
      </div>
    </fieldset>
    <label v-if="!control" class="field"
      >{{ tr('自动刷新间隔（秒，0 表示手动）', 'Refresh interval (seconds, 0 for manual)')
      }}<input v-model.number="c.dataRefreshInterval" type="number" min="0" max="86400"
    /></label>
  </template>
</template>
<script setup>
import { computed, watch } from 'vue'
import { tr } from '../i18n'
import { widgetType, entitySection } from '../api/widgetConfig'
import { unitText } from '../api/chartPresentation'
import BuildingOverview from './BuildingOverview.vue'
import AlarmFilters from './AlarmFilters.vue'
import { alarmReferences } from '../api/alarmFilters'
import { completeAlarmColumns, alarmColors, alarmIcons } from '../api/alarmPresentation'
const props = defineProps({
  widget: Object,
  workspace: String,
  channels: Array,
  devices: Array,
  groups: Array,
})
const crypto = window.crypto,
  c = computed(() => props.widget.chart),
  type = computed(() => widgetType(props.widget)),
  control = computed(() => type.value.endsWith('_control')),
  entityMap = computed(() => ['map', 'polygon_map'].includes(type.value)),
  telemetry = computed(
    () =>
      !control.value &&
      ![
        'map',
        'polygon_map',
        'building_overview',
        'count_card',
        'table_card',
        'alarm_count',
        'alarm_table',
      ].includes(type.value),
  )
const mapFields = [
  ['locationKey', '位置字段', 'Location key', 'location'],
  ['latitudeKey', '纬度字段', 'Latitude key', 'latitude'],
  ['longitudeKey', '经度字段', 'Longitude key', 'longitude'],
  ['polygonKey', '围栏字段', 'Polygon key', 'perimeter'],
]
function setUnit(source, value) {
  source.unit =
    type.value === 'gauge_chart' || typeof source.unit === 'object'
      ? { label: value, value }
      : value
}
function localDate(value) {
  if (!value) return ''
  const d = new Date(value)
  return Number.isFinite(d.getTime())
    ? new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
    : ''
}
function moveColumn(i, delta) {
  const cols = c.value.columns
  ;[cols[i], cols[i + delta]] = [cols[i + delta], cols[i]]
  cols.forEach((col, index) => (col.order = index))
}
function mapEntity(s) {
  const key = type.value === 'map' ? 'entityDetails' : 'entity'
  return (s[key] ||= s.entityDetails || s.entity || { id: '' })
}
function resetEntity(s) {
  s[type.value === 'map' ? 'entityDetails' : 'entity'] = { id: '' }
  s.childEntities = []
}
function entities(t) {
  return props[entitySection(t)] || []
}
watch(
  type,
  () => {
    c.value.dataSource ||= {}
    c.value.dataSource.channel ||= { id: '' }
    c.value.dataSource.client ||= { id: '' }
    c.value.channel ||= { id: '' }
    c.value.client ||= { id: '' }
    if (type.value.startsWith('alarm_'))
      for (const [field] of alarmReferences) c.value[field] ||= { id: '' }
    c.value.buildings ||= []
    if (type.value === 'alarm_count') {
      c.value.color ||= 'blue'
      c.value.icon ||= 'bell'
    }
    if (type.value === 'alarm_table') c.value.columns = completeAlarmColumns(c.value.columns)
    c.value.dataSources ||= []
    if (entityMap.value) for (const s of c.value.dataSources) mapEntity(s)
  },
  { immediate: true },
)
</script>
