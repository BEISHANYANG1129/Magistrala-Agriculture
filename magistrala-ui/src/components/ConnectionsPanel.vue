<template>
  <div class="panel">
    <h2>{{ tr('设备与通道连接', 'Device–channel connections') }}</h2>
    <p>
      {{
        tr(
          '连接决定设备可以向哪个通道发布消息或订阅消息。',
          'Connections determine which channels a device may publish to or subscribe to.',
        )
      }}
    </p>
    <form @submit.prevent="connect">
      <div class="form-grid">
        <label class="field"
          >{{ tr('设备', 'Device')
          }}<select v-model="device" :aria-label="tr('设备', 'Device')" required>
            <option value="">{{ tr('选择设备', 'Select device') }}</option>
            <option v-for="d in devices" :key="d.id" :value="d.id">{{ d.name }}</option>
          </select></label
        ><label class="field"
          >{{ tr('通道', 'Channel')
          }}<select v-model="channel" :aria-label="tr('通道', 'Channel')" required>
            <option value="">{{ tr('选择通道', 'Select channel') }}</option>
            <option v-for="c in channels" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select></label
        >
      </div>
      <div class="actions">
        <label class="check"
          ><input v-model="publish" type="checkbox" />{{ tr('允许发布', 'Allow publish') }}</label
        ><label class="check"
          ><input v-model="subscribe" type="checkbox" />{{
            tr('允许订阅', 'Allow subscribe')
          }}</label
        ><button :disabled="busy || (!publish && !subscribe)">
          {{ tr('建立连接', 'Connect') }}</button
        ><button type="button" class="quiet" :disabled="busy || !channel" @click="loadPolicies">
          {{ tr('查看直接连接', 'Load direct connections') }}
        </button>
      </div>
    </form>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="notice" class="success">{{ notice }}</p>
    <div class="table-wrap" v-if="policies.length">
      <table>
        <thead>
          <tr>
            <th>{{ tr('设备 / 主体', 'Device / subject') }}</th>
            <th>{{ tr('权限', 'Permissions') }}</th>
            <th>{{ tr('操作', 'Actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in policies" :key="p.id">
            <td>{{ devices.find((d) => d.id === p.subjectId)?.name || p.subjectId }}</td>
            <td>
              {{ p.permissionBlock?.actions?.map((a) => a.name).join(', ') || p.permissionBlockId }}
            </td>
            <td>
              <button
                class="quiet"
                :disabled="busy || p.managedBy === 'config'"
                @click="disconnect(p)"
              >
                {{ tr('断开此直接授权', 'Remove this direct grant') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <small>{{
      tr(
        '这里列出直接授权。通过角色或分组继承的权限，请在角色与权限页面管理。',
        'This lists direct grants. Manage permissions inherited from roles or groups under Roles & permissions.',
      )
    }}</small>
  </div>
</template>
<script setup>
import { ref, watch } from 'vue'
import { tr } from '../i18n'
import { runOperation, listAll, errorText, gql } from '../api/platform'
const props = defineProps({ workspace: String, record: Object, section: String })
const devices = ref([]),
  channels = ref([]),
  device = ref(''),
  channel = ref(''),
  publish = ref(true),
  subscribe = ref(false),
  busy = ref(false),
  error = ref(''),
  notice = ref(''),
  policies = ref([])
watch(
  () => [props.workspace, props.record?.id],
  async () => {
    device.value = props.section === 'channels' ? '' : props.record?.id || ''
    channel.value = props.section === 'channels' ? props.record?.id || '' : ''
    try {
      ;[devices.value, channels.value] = await Promise.all([
        listAll('entities', { tenantId: props.workspace, kind: 'device' }),
        listAll('resources', { tenantId: props.workspace, kind: 'channel' }),
      ])
      if (channel.value) await loadPolicies()
    } catch (e) {
      error.value = errorText(e)
    }
  },
  { immediate: true },
)
async function loadPolicies() {
  error.value = ''
  try {
    const d = await gql(
      'query($workspace:ID!,$channel:ID!){directPolicies(tenantId:$workspace,objectId:$channel,objectKind:"resource",limit:100){items{id subjectId permissionBlockId managedBy permissionBlock{actions{name}}}}}',
      { workspace: props.workspace, channel: channel.value },
    )
    policies.value = d.directPolicies.items
  } catch (e) {
    error.value = errorText(e)
  }
}
async function connect() {
  busy.value = true
  error.value = ''
  notice.value = ''
  let block
  try {
    const actions = await listAll('actions', {
      tenantId: props.workspace,
      objectKind: 'resource',
      objectType: 'resource:channel',
    })
    const names = [...(publish.value ? ['publish'] : []), ...(subscribe.value ? ['subscribe'] : [])]
    const ids = names.map((n) => actions.find((a) => a.name === n)?.id)
    if (ids.some((id) => !id))
      throw new Error(
        tr(
          '平台缺少所需的发布或订阅权限定义。',
          'Required publish or subscribe action is not configured.',
        ),
      )
    block = await runOperation('createPermissionBlock', {
      input: {
        tenantId: props.workspace,
        scopeMode: 'object',
        objectKind: 'resource',
        objectType: 'resource:channel',
        objectId: channel.value,
        effect: 'allow',
        actionIds: ids,
      },
    })
    await runOperation('createDirectPolicy', {
      input: {
        tenantId: props.workspace,
        subjectKind: 'entity',
        subjectId: device.value,
        permissionBlockId: block.id,
      },
    })
    notice.value = tr('连接已创建。', 'Connection created.')
    await loadPolicies()
  } catch (e) {
    error.value =
      errorText(e) +
      (block
        ? tr(
            ' 已创建权限块，若授权失败可在角色与权限中检查：',
            ' Permission block was created; inspect it under Roles & permissions if the grant failed: ',
          ) + block.id
        : '')
  } finally {
    busy.value = false
  }
}
async function disconnect(p) {
  if (!window.confirm(tr('确认移除此直接授权？', 'Remove this direct grant?'))) return
  busy.value = true
  try {
    await runOperation('deleteDirectPolicy', { id: p.id })
    await loadPolicies()
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
</script>
