<template>
  <section class="buildings">
    <div v-if="buildings.length" class="toolbar">
      <label
        >{{ tr('建筑', 'Building')
        }}<select v-model="selected">
          <option value="">{{ tr('全部', 'All') }}</option>
          <option v-for="b in buildings" :key="b.id" :value="b.id">{{ b.name }}</option>
        </select></label
      >
      <label
        ><input type="checkbox" v-model="missingOnly" />{{
          tr('仅显示未读取', 'Missing only')
        }}</label
      >
      <strong>{{ totals.read }} / {{ totals.total }} {{ tr('已读取', 'read') }}</strong>
    </div>
    <p v-if="!buildings.length">
      {{ tr('暂无建筑，请添加建筑和楼层。', 'No buildings. Add a building and floors.') }}
    </p>
    <article
      v-for="(b, bi) in buildings"
      v-show="!selected || selected === b.id"
      :key="b.id"
      class="panel"
      :class="{ estimated: b.estimated }"
      :draggable="editable && !selected"
      @dragstart.self="dragged = { building: b.id }"
      @dragover.prevent
      @drop.prevent.stop="dropBuilding(bi)"
    >
      <div class="toolbar">
        <input
          v-if="editable"
          v-model="b.name"
          :aria-label="tr('建筑名称', 'Building name')"
          required
        />
        <h3 v-else>{{ b.name }}</h3>
        <input
          v-if="editable"
          v-model="b.model"
          :aria-label="tr('设备型号', 'Device model')"
          :placeholder="tr('设备型号', 'Device model')"
        /><span v-else>{{ b.model }}</span>
        <span>{{ count(b).read }} / {{ count(b).total }} {{ tr('已读取', 'read') }}</span>
        <span v-if="b.estimated">{{ tr('估算建筑', 'Estimated building') }}</span>
        <template v-if="editable"
          ><button
            type="button"
            class="quiet"
            :disabled="bi === 0"
            :aria-label="tr('上移建筑', 'Move building up')"
            @click="reorder(buildings, bi, bi - 1)"
          >
            ↑</button
          ><button
            type="button"
            class="quiet"
            :disabled="bi === buildings.length - 1"
            :aria-label="tr('下移建筑', 'Move building down')"
            @click="reorder(buildings, bi, bi + 1)"
          >
            ↓
          </button></template
        >
        <button v-if="editable" type="button" class="quiet danger" @click="buildings.splice(bi, 1)">
          {{ tr('移除建筑', 'Remove building') }}
        </button>
      </div>
      <section
        v-for="(f, fi) in b.floors"
        :key="f.name"
        class="floor"
        :draggable="editable"
        @dragstart.stop="dragged = { building: b.id, floor: fi }"
        @dragover.prevent
        @drop.prevent.stop="dropFloor(b, fi)"
      >
        <div class="toolbar">
          <input
            v-if="editable"
            :value="f.name"
            @change="renameFloor(b, f, $event.target.value)"
            :aria-label="tr('楼层名称', 'Floor name')"
            required
          />
          <h4 v-else>{{ f.name }}</h4>
          <span
            >{{ f.devices.filter((d) => d.read).length }} / {{ f.devices.length }}
            {{ tr('已读取', 'read') }}</span
          >
          <button
            v-if="editable"
            type="button"
            class="quiet"
            @click="
              f.devices.push({
                id: '',
                apartment: '',
                read: false,
                floor: f.name,
                buildingId: b.id,
              })
            "
          >
            + {{ tr('设备', 'Device') }}
          </button>
          <button
            v-if="editable"
            type="button"
            class="quiet danger"
            :disabled="!emptyFloor(b, f)"
            @click="removeFloor(b, fi)"
          >
            {{ tr('删除空楼层', 'Delete empty floor') }}
          </button>
          <template v-if="editable"
            ><button
              type="button"
              class="quiet"
              :disabled="fi === 0"
              :aria-label="tr('上移楼层', 'Move floor up')"
              @click="reorder(b.floors, fi, fi - 1)"
            >
              ↑</button
            ><button
              type="button"
              class="quiet"
              :disabled="fi === b.floors.length - 1"
              :aria-label="tr('下移楼层', 'Move floor down')"
              @click="reorder(b.floors, fi, fi + 1)"
            >
              ↓
            </button></template
          >
        </div>
        <div
          v-for="(d, di) in f.devices"
          v-show="!missingOnly || !d.read"
          :key="di"
          class="toolbar device"
          :class="{ read: d.read, missing: !d.read }"
        >
          <input
            v-if="editable"
            v-model="d.id"
            :aria-label="tr('设备编号', 'Device ID')"
            required
          /><button
            v-else
            type="button"
            class="quiet"
            @click="details = { building: b, floor: f, device: d }"
          >
            {{ d.id }}
          </button>
          <input
            v-if="editable"
            v-model="d.apartment"
            :aria-label="tr('房间', 'Apartment')"
            :placeholder="tr('房间', 'Apartment')"
          /><span v-else>{{ d.apartment }}</span>
          <label
            ><input type="checkbox" v-model="d.read" :disabled="!editable" />{{
              tr('已读取', 'Read')
            }}</label
          >
          <select
            v-if="editable"
            :value="f.name"
            @change="moveDevice(b, f, di, $event.target.value)"
            :aria-label="tr('移动到楼层', 'Move to floor')"
          >
            <option v-for="floor in b.floors" :key="floor.name">{{ floor.name }}</option>
          </select>
          <button v-if="editable" type="button" class="quiet" @click="f.devices.splice(di, 1)">
            {{ tr('移除', 'Remove') }}
          </button>
        </div>
      </section>
      <div class="toolbar" v-if="editable">
        <button type="button" class="quiet" @click="addFloor(b)">+ {{ tr('楼层', 'Floor') }}</button
        ><button
          type="button"
          class="quiet"
          :disabled="!b.floors.length"
          @click="b.gateways.push({ id: '', floor: b.floors[0].name, buildingId: b.id })"
        >
          + {{ tr('网关', 'Gateway') }}
        </button>
      </div>
      <div v-for="(g, gi) in b.gateways" :key="gi" class="toolbar gateway">
        <span>{{ tr('网关', 'Gateway') }}</span
        ><input
          v-if="editable"
          v-model="g.id"
          :aria-label="tr('网关编号', 'Gateway ID')"
          required
        /><strong v-else>{{ g.id }}</strong
        ><select v-if="editable" v-model="g.floor" :aria-label="tr('网关楼层', 'Gateway floor')">
          <option v-for="f in b.floors" :key="f.name">{{ f.name }}</option></select
        ><span v-else>{{ g.floor }}</span
        ><button v-if="editable" type="button" class="quiet" @click="b.gateways.splice(gi, 1)">
          {{ tr('移除', 'Remove') }}
        </button>
      </div>
    </article>
    <section v-if="details" class="panel" :aria-label="tr('设备详情', 'Device details')">
      <h3>{{ tr('设备详情', 'Device details') }}</h3>
      <dl>
        <dt>{{ tr('设备编号', 'Device ID') }}</dt>
        <dd>{{ details.device.id }}</dd>
        <dt>{{ tr('建筑', 'Building') }}</dt>
        <dd>{{ details.building.name }}</dd>
        <dt>{{ tr('楼层', 'Floor') }}</dt>
        <dd>{{ details.floor.name }}</dd>
        <dt>{{ tr('房间', 'Apartment') }}</dt>
        <dd>{{ details.device.apartment || '—' }}</dd>
        <dt>{{ tr('设备型号', 'Model') }}</dt>
        <dd>{{ details.building.model || '—' }}</dd>
        <dt>{{ tr('状态', 'Status') }}</dt>
        <dd>{{ details.device.read ? tr('已读取', 'Read') : tr('未读取', 'Not read') }}</dd>
      </dl>
      <button type="button" class="quiet" @click="details.device.read = !details.device.read">
        {{ tr('切换读取状态', 'Toggle status') }}
      </button>
      <button type="button" class="quiet" @click="details = null">
        {{ tr('关闭详情', 'Close details') }}
      </button>
    </section>
    <button
      v-if="editable"
      type="button"
      class="quiet"
      @click="
        buildings.push({
          id: crypto.randomUUID(),
          name: tr('新建筑', 'New building'),
          model: '',
          floors: [],
          gateways: [],
        })
      "
    >
      + {{ tr('添加建筑', 'Add building') }}
    </button>
    <p v-if="error" class="error">{{ error }}</p>
  </section>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { tr } from '../i18n'
const props = defineProps({ buildings: { type: Array, default: () => [] }, editable: Boolean })
const crypto = window.crypto,
  error = ref(''),
  selected = ref(''),
  missingOnly = ref(false),
  dragged = ref(null),
  details = ref(null)
function count(b) {
  const devices = b.floors.flatMap((f) => f.devices)
  return { total: devices.length, read: devices.filter((d) => d.read).length }
}
const totals = computed(() =>
  props.buildings
    .filter((b) => !selected.value || b.id === selected.value)
    .reduce(
      (sum, b) => {
        const n = count(b)
        return { total: sum.total + n.total, read: sum.read + n.read }
      },
      { total: 0, read: 0 },
    ),
)
watch(
  () => props.buildings.map((b) => b.id),
  (ids) => {
    if (!ids.includes(selected.value)) selected.value = ''
  },
)
function reorder(items, from, to) {
  if (from < 0 || to < 0 || from >= items.length || to >= items.length || from === to) return
  items.splice(to, 0, items.splice(from, 1)[0])
}
function dropBuilding(index) {
  if (props.editable && dragged.value && dragged.value.floor === undefined)
    reorder(
      props.buildings,
      props.buildings.findIndex((b) => b.id === dragged.value.building),
      index,
    )
  dragged.value = null
}
function dropFloor(b, index) {
  if (props.editable && dragged.value?.building === b.id && dragged.value.floor !== undefined)
    reorder(b.floors, dragged.value.floor, index)
  dragged.value = null
}
function emptyFloor(b, f) {
  return !f.devices.length && !b.gateways.some((g) => g.floor === f.name)
}
function renameFloor(b, f, name) {
  name = name.trim()
  error.value = ''
  if (!name.trim() || b.floors.some((x) => x !== f && x.name === name)) {
    error.value = tr('楼层名称为空或重复。', 'Empty or duplicate floor name.')
    return
  }
  b.gateways.filter((g) => g.floor === f.name).forEach((g) => (g.floor = name))
  f.devices.forEach((d) => (d.floor = name))
  f.name = name
}
function removeFloor(b, i) {
  if (!emptyFloor(b, b.floors[i])) return
  b.floors.splice(i, 1)
}
function addFloor(b) {
  let n = b.floors.length + 1
  while (b.floors.some((f) => f.name === 'Floor ' + n)) n++
  b.floors.push({ name: 'Floor ' + n, devices: [] })
}
function moveDevice(b, f, i, name) {
  const target = b.floors.find((x) => x.name === name)
  if (target && target !== f) {
    const [d] = f.devices.splice(i, 1)
    d.floor = name
    target.devices.push(d)
  }
}
</script>
<style scoped>
.floor {
  border: 1px solid #dfe5eb;
  padding: 10px;
  margin: 8px 0;
  border-radius: 6px;
}
.estimated {
  border-color: #d89b32;
}
.device {
  padding: 8px;
  border-left: 4px solid;
  border-radius: 4px;
  margin: 6px 0;
}
.device.read {
  border-color: #19806c;
  background: #19806c0c;
}
.device.missing {
  border-color: #d89b32;
  background: #d89b320c;
}
.gateway {
  padding: 8px;
  border-left: 4px solid #397bc3;
}
.buildings input:not([type='checkbox']),
.buildings select {
  max-width: 180px;
}
.buildings .toolbar {
  flex-wrap: wrap;
}
</style>
