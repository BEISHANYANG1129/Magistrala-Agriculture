<template>
  <ModalDialog :busy="running" @close="emit('close')">
    <section class="modal-card">
      <h2>{{ tr('批量导入', 'Upload CSV') }} · {{ sectionLabel(section) }}</h2>
      <p>
        {{
          tr(
            '选择 CSV 文件，检查预览后提交。名称必填；其他列可以留空。标签用分号分隔，元数据使用 JSON 对象。',
            'Choose a CSV file and review before submitting. Name is required; other columns are optional. Separate tags with semicolons; metadata is a JSON object.',
          )
        }}
      </p>
      <button class="quiet" @click="sample">{{ tr('下载模板', 'Download template') }}</button
      ><label class="field"
        >CSV<input
          type="file"
          accept=".csv,text/csv"
          :disabled="running || started"
          @change="readFile"
      /></label>
      <p v-if="error" class="error">{{ error }}</p>
      <div v-if="jobs.length" class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{{ tr('行', 'Row') }}</th>
              <th>{{ tr('名称', 'Name') }}</th>
              <th>{{ tr('结果', 'Result') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="j in jobs" :key="j.row">
              <td>{{ j.row }}</td>
              <td>{{ j.values.name }}</td>
              <td>
                {{ stateLabel(j.status) }} {{ j.error }}
                <small v-if="j.created">{{ j.created.id }}</small>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="started">
        {{ tr('成功', 'Succeeded') }} {{ jobs.filter((j) => j.status === 'done').length }} /
        {{ jobs.length }}
      </p>
      <p v-if="jobs.some((j) => j.status === 'uncertain')" class="warning">
        {{
          tr(
            '部分请求未返回明确结果，不能安全地自动重试。请先核对平台中已创建的资源。',
            'Some requests have an unknown outcome. Check existing platform resources before importing those rows again.',
          )
        }}
      </p>
      <div class="actions">
        <button class="quiet" :disabled="running" @click="emit('close')">
          {{ tr('关闭', 'Close') }}</button
        ><button v-if="running" class="quiet" @click="stopping = true">
          {{ tr('处理完当前行后停止', 'Stop after current row') }}</button
        ><button
          :disabled="
            running ||
            !jobs.length ||
            jobs.some((j) => j.status === 'invalid') ||
            !jobs.some((j) => ['pending', 'failed'].includes(j.status))
          "
          @click="submit"
        >
          {{
            started
              ? tr('继续未完成项', 'Continue unfinished rows')
              : tr('提交导入', 'Submit import')
          }}
        </button>
      </div>
    </section>
  </ModalDialog>
</template>
<script setup>
import ModalDialog from './ModalDialog.vue'
import { ref, onBeforeUnmount } from 'vue'
import { tr, sectionLabel } from '../i18n'
import { csvJobs, importJob } from '../api/batchWorkflow'
import { errorText } from '../api/platform'
const props = defineProps({ section: String, workspace: String, parentGroup: String }),
  emit = defineEmits(['close', 'updated'])
const jobs = ref([]),
  error = ref(''),
  running = ref(false),
  started = ref(false),
  stopping = ref(false)
const stateLabel = (s) =>
  ({
    pending: tr('待导入', 'Pending'),
    invalid: tr('格式错误', 'Invalid'),
    running: tr('处理中', 'Working'),
    done: tr('成功', 'Succeeded'),
    failed: tr('失败', 'Failed'),
    uncertain: tr('结果待核对', 'Unknown outcome'),
  })[s]
async function readFile(e) {
  error.value = ''
  jobs.value = []
  try {
    const file = e.target.files[0]
    if (!file) return
    if (file.size > 2 * 1024 * 1024)
      throw new Error(tr('文件不能超过 2 MB。', 'File must be at most 2 MB.'))
    jobs.value = csvJobs(await file.text(), props.section)
  } catch (e) {
    error.value = errorText(e)
  }
}
function sample() {
  const headers =
    props.section === 'channels' ? 'Name,Route,Tags,Metadata' : 'Name,Key,Serial,Tags,Metadata'
  const url = URL.createObjectURL(
    new Blob(
      [
        '\uFEFF' +
          headers +
          '\r\n' +
          (props.section === 'channels'
            ? 'Soil channel,soil,north;soil,{}'
            : 'Soil sensor,,SN001,north;soil,{}'),
      ],
      { type: 'text/csv;charset=utf-8' },
    ),
  )
  const a = document.createElement('a')
  a.href = url
  a.download = props.section + '.csv'
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
async function submit() {
  running.value = true
  started.value = true
  stopping.value = false
  const section = props.section,
    workspace = props.workspace,
    parent = props.parentGroup
  for (const j of jobs.value) {
    if (stopping.value) break
    if (!['pending', 'failed'].includes(j.status)) continue
    j.status = 'running'
    j.error = ''
    try {
      await importJob(j, section, workspace, parent)
      j.status = 'done'
    } catch (e) {
      j.status = e.status >= 400 && e.status < 500 ? 'failed' : 'uncertain'
      j.error = errorText(e)
      if (j.status === 'uncertain') stopping.value = true
    }
  }
  running.value = false
  emit('updated')
}
onBeforeUnmount(() => {
  stopping.value = true
})
</script>
