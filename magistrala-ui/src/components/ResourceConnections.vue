<template>
  <section v-show="!dialogOnly">
    <div class="toolbar">
      <button @click="openConnect">+ {{ tr('连接', 'Connect') }}</button
      ><input v-model="search" :placeholder="tr('按名称搜索', 'Search by name')" /><button
        class="quiet"
        @click="load"
        :disabled="busy"
      >
        {{ tr('刷新', 'Refresh') }}
      </button>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{{ tr('名称', 'Name') }}</th>
            <th>{{ tr('连接类型', 'Connection types') }}</th>
            <th>{{ tr('状态', 'Status') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in filtered" :key="r.id">
            <td>
              <router-link
                :to="`/domain/${workspace}/${isChannel ? 'devices' : 'channels'}/${r.id}`"
                >{{ r.name }}</router-link
              >
            </td>
            <td>
              <span v-for="type in r.types" :key="type" class="badge">{{
                type === 'publish' ? tr('发布', 'Publish') : tr('订阅', 'Subscribe')
              }}</span>
            </td>
            <td>{{ tr('已连接', 'Connected') }}</td>
            <td>
              <button
                class="quiet danger"
                :disabled="busy || !r.removable"
                @click="disconnecting = r"
              >
                {{ tr('断开连接', 'Disconnect') }}
              </button>
            </td>
          </tr>
          <tr v-if="!filtered.length">
            <td colspan="4">
              {{
                busy
                  ? tr('读取中…', 'Loading…')
                  : tr(
                      '暂无直接连接。点击“连接”选择设备或通道。',
                      'No direct connections. Select Connect to add devices or channels.',
                    )
              }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <small>{{
      tr(
        '此处显示直接发布/订阅授权。通过角色或分组继承的访问权限在“访问权限”中查看。',
        'This list shows direct publish/subscribe grants. See Access for permissions inherited through roles or groups.',
      )
    }}</small>
    <ModalDialog v-if="connecting" :busy="busy" @close="connecting = false">
      <form class="modal-card compact-modal" @submit.prevent="connect">
        <h2>
          {{
            isChannel ? tr('连接设备', 'Connect device(s)') : tr('连接通道', 'Connect channel(s)')
          }}
        </h2>
        <label class="field"
          >{{ isChannel ? tr('设备', 'Devices') : tr('通道', 'Channels') }} *<select
            v-model="targets"
            multiple
            required
            size="6"
          >
            <option v-for="r in choices" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select></label
        >
        <p>
          {{
            tr(
              '选择连接类型：订阅用于读取，发布用于发送消息。',
              'Choose a type: Subscribe to read, Publish to send messages.',
            )
          }}
        </p>
        <div class="actions">
          <label class="check"
            ><input v-model="subscribe" type="checkbox" />{{ tr('订阅', 'Subscribe') }}</label
          ><label class="check"
            ><input v-model="publish" type="checkbox" />{{ tr('发布', 'Publish') }}</label
          >
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button type="button" class="quiet" :disabled="busy" @click="connecting = false">
            {{ tr('关闭', 'Close') }}</button
          ><button :disabled="busy || !targets.length || (!publish && !subscribe)">
            {{ tr('连接', 'Connect') }}
          </button>
        </div>
      </form>
    </ModalDialog>
    <ModalDialog v-if="disconnecting" :busy="busy" confirmation @close="disconnecting = null">
      <div class="modal-card compact-modal">
        <h2>{{ tr('断开连接', 'Disconnect') }}</h2>
        <p>{{ record.name }} ↔ {{ disconnecting.name }}</p>
        <p>
          {{
            tr(
              '移除该设备与通道之间的直接发布/订阅授权？',
              'Remove the direct publish/subscribe grants for this device and channel?',
            )
          }}
        </p>
        <p v-if="error" class="error">{{ error }}</p>
        <div class="actions">
          <button class="quiet" :disabled="busy" @click="disconnecting = null">
            {{ tr('取消', 'Cancel') }}</button
          ><button :disabled="busy" @click="disconnect">{{ tr('断开连接', 'Disconnect') }}</button>
        </div>
      </div>
    </ModalDialog>
  </section>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, computed, watch } from 'vue'
import { tr } from '../i18n'
import { runOperation, errorText } from '../api/platform'
import { readResources, channelPolicies, connectDevice } from '../api/resourceWorkflow'
const emit = defineEmits(['close', 'updated'])
const props = defineProps({
  workspace: String,
  section: String,
  record: Object,
  dialogOnly: Boolean,
})
const isChannel = computed(() => props.section === 'channels'),
  choices = ref([]),
  rows = ref([]),
  search = ref(''),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  connecting = ref(false),
  disconnecting = ref(null),
  targets = ref([]),
  publish = ref(false),
  subscribe = ref(false)
const filtered = computed(() =>
  rows.value.filter((r) => r.name.toLowerCase().includes(search.value.toLowerCase())),
)
let generation = 0
async function load() {
  const seq = ++generation
  busy.value = true
  error.value = ''
  try {
    const [resources, policies] = await Promise.all([
      readResources(isChannel.value ? 'devices' : 'channels', props.workspace),
      channelPolicies(
        props.workspace,
        isChannel.value ? { channel: props.record.id } : { device: props.record.id },
      ),
    ])
    if (seq !== generation) return
    choices.value = resources
    const groups = new Map()
    for (const p of policies) {
      const block = p.permissionBlock
      if (block.effect !== 'allow') continue
      const types = block.actions
        .map((a) => a.name)
        .filter((n) => ['publish', 'subscribe'].includes(n))
      if (!types.length) continue
      const id = isChannel.value ? p.subjectId : block.objectId
      if (!groups.has(id))
        groups.set(id, {
          id,
          name: resources.find((r) => r.id === id)?.name || id,
          types: [],
          policies: [],
          removable: true,
        })
      const r = groups.get(id)
      r.types = [...new Set([...r.types, ...types])]
      r.policies.push(p)
      if (
        p.managedBy === 'config' ||
        block.actions.some((a) => !['publish', 'subscribe'].includes(a.name))
      )
        r.removable = false
    }
    rows.value = [...groups.values()]
  } catch (e) {
    if (seq === generation) error.value = errorText(e)
  } finally {
    if (seq === generation) busy.value = false
  }
}
watch(
  () => [props.workspace, props.record.id],
  () => {
    connecting.value = false
    disconnecting.value = null
    load()
    if (props.dialogOnly) openConnect()
  },
  { immediate: true },
)
watch(connecting, (value, previous) => {
  if (props.dialogOnly && previous && !value) emit('close')
})
function openConnect() {
  targets.value = []
  publish.value = false
  subscribe.value = false
  error.value = ''
  connecting.value = true
}
async function connect() {
  busy.value = true
  error.value = ''
  const done = []
  try {
    const names = [...(publish.value ? ['publish'] : []), ...(subscribe.value ? ['subscribe'] : [])]
    for (const target of targets.value) {
      const existing = rows.value.find((r) => r.id === target)?.types || []
      const missing = names.filter((n) => !existing.includes(n))
      if (missing.length)
        await connectDevice(
          props.workspace,
          isChannel.value ? target : props.record.id,
          isChannel.value ? props.record.id : target,
          missing,
        )
      done.push(target)
    }
    connecting.value = false
    notice.value = tr('连接已更新。', 'Connections updated.')
    await load()
  } catch (e) {
    targets.value = targets.value.filter((t) => !done.includes(t))
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
async function disconnect() {
  busy.value = true
  error.value = ''
  try {
    for (const p of [...disconnecting.value.policies]) {
      await runOperation('deleteDirectPolicy', { id: p.id })
      disconnecting.value.policies = disconnecting.value.policies.filter((x) => x.id !== p.id)
    }
    disconnecting.value = null
    await load()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
