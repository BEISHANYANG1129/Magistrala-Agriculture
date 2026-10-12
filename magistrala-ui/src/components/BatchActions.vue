<template>
  <ModalDialog :busy="busy" confirmation @close="emit('close')">
    <section class="modal-card">
      <h2>{{ title }} · {{ records.length }}</h2>
      <p>
        {{
          tr(
            '仅处理下列已选记录。成功的项目不会在重试时再次提交。',
            'Only the selected records below will be processed. Successful items are not submitted again on retry.',
          )
        }}
      </p>
      <div class="table-wrap">
        <table>
          <tbody>
            <tr v-for="j in jobs" :key="j.record.id">
              <td>{{ j.record.name || j.record.id }}</td>
              <td>
                {{
                  j.status === 'done'
                    ? tr('成功', 'Succeeded')
                    : j.status === 'uncertain'
                      ? tr(
                          '结果待核对；请刷新列表确认后再操作',
                          'Unknown outcome; reload the list before acting again',
                        )
                      : j.status === 'failed'
                        ? tr('失败', 'Failed')
                        : tr('待处理', 'Pending')
                }}
                {{ j.error }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="actions">
        <button class="quiet" :disabled="busy" @click="emit('close')">
          {{ tr('关闭', 'Close') }}</button
        ><button v-if="busy" class="quiet" @click="stop = true">
          {{ tr('处理完当前项后停止', 'Stop after current item') }}</button
        ><button
          :disabled="
            busy ||
            jobs.some((j) => j.status === 'uncertain') ||
            jobs.every((j) => j.status === 'done')
          "
          @click="run"
        >
          {{ tr('确认', 'Confirm') }} {{ title }}
        </button>
      </div>
    </section>
  </ModalDialog>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onBeforeUnmount } from 'vue'
import { tr } from '../i18n'
import { errorText } from '../api/platform'
const props = defineProps({ records: Array, title: String, perform: Function }),
  emit = defineEmits(['close', 'updated'])
const jobs = ref(props.records.map((record) => ({ record, status: 'pending', error: '' }))),
  busy = ref(false),
  stop = ref(false)
async function run() {
  busy.value = true
  stop.value = false
  const perform = props.perform
  for (const j of jobs.value) {
    if (stop.value) break
    if (j.status === 'done') continue
    try {
      await perform(j.record)
      j.status = 'done'
      j.error = ''
    } catch (e) {
      j.status = e.status >= 400 && e.status < 500 ? 'failed' : 'uncertain'
      j.error = errorText(e)
      stop.value = true
    }
  }
  busy.value = false
  emit(
    'updated',
    jobs.value.filter((j) => j.status === 'done').map((j) => j.record.id),
  )
}
onBeforeUnmount(() => {
  stop.value = true
})
</script>
