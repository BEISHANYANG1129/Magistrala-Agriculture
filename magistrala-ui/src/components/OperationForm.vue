<template>
  <form :aria-busy="busy" @submit.prevent="submit" class="operation-form">
    <h3>{{ label(op.name) }}</h3>
    <p v-if="dangerous" class="warning">
      {{
        tr(
          '此操作会删除、撤销或停用所选对象，请核对目标。',
          'This operation deletes, revokes or disables the selected object. Check the target.',
        )
      }}
    </p>
    <FormFields :schema="schema" v-model="values" :choices="choices" />
    <details v-if="op.kind === 'rest' && op.body">
      <summary>{{ tr('高级：完整请求内容', 'Advanced: complete request body') }}</summary>
      <textarea
        rows="10"
        :value="JSON.stringify(values.body || {}, null, 2)"
        @change="setBody($event)"
      ></textarea>
    </details>
    <label v-if="dangerous" class="check"
      ><input type="checkbox" v-model="confirmed" required />{{
        tr('已核对目标，确认执行', 'I have checked the target and confirm this action')
      }}</label
    >
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <div class="actions">
      <button :disabled="busy || (dangerous && !confirmed)">
        {{
          busy
            ? tr('处理中…', 'Working…')
            : op.kind === 'query' || op.method === 'GET'
              ? tr('查询', 'Load')
              : tr('提交', 'Submit')
        }}</button
      ><button type="button" class="quiet" @click="emit('close')">{{ tr('关闭', 'Close') }}</button>
    </div>
    <div v-if="result !== undefined" class="success">
      {{ tr('操作成功', 'Operation succeeded') }}
      <pre>{{ JSON.stringify(result, null, 2) }}</pre>
    </div>
  </form>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import FormFields from './FormFields.vue'
import { formSchema, execute } from '../api/catalog'
import { errorText } from '../api/platform'
import { tr, label } from '../i18n'
const props = defineProps({ op: Object, initial: Object, choices: Object })
const emit = defineEmits(['done', 'close'])
const values = ref({}),
  busy = ref(false),
  error = ref(''),
  result = ref(),
  confirmed = ref(false)
const schema = computed(() => formSchema(props.op))
const dangerous = computed(
  () =>
    /delete|purge|revoke|remove|disable|freeze|retire|rotate|clear/i.test(props.op.name) ||
    props.op.method === 'DELETE',
)
watch(
  () => props.op,
  () => {
    values.value = JSON.parse(JSON.stringify(props.initial || {}))
    error.value = ''
    result.value = undefined
    confirmed.value = false
  },
  { immediate: true },
)
async function submit() {
  busy.value = true
  error.value = ''
  try {
    result.value = await execute(props.op, values.value)
    emit('done', result.value)
  } catch (e) {
    error.value = errorText(e)
  } finally {
    busy.value = false
  }
}
function setBody(event) {
  try {
    values.value.body = JSON.parse(event.target.value)
    event.target.setCustomValidity('')
  } catch {
    event.target.setCustomValidity(tr('JSON 格式错误', 'Invalid JSON'))
  }
}
</script>
