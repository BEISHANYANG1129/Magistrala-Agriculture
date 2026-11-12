<template>
  <section class="report">
    <h3>
      {{
        report.task === 'environment'
          ? tr('环境观测报告', 'Environment report')
          : tr('叶片检查报告', 'Leaf inspection report')
      }}
    </h3>
    <p v-for="(claim, index) in report.claims" :key="index">
      {{ claim.text }}
      <small v-if="claim.citations?.length" class="references">
        <a v-for="id in claim.citations" :key="id" :href="'#' + anchor(id)">{{
          citationLabel(id)
        }}</a>
      </small>
    </p>
    <ul v-if="report.limitations?.length">
      <li v-for="(limit, index) in report.limitations" :key="index">{{ limit }}</li>
    </ul>
    <div v-for="(window, index) in windows" :key="index" class="report-window">
      <h4>
        {{ index === 0 ? tr('主时段', 'Primary window') : tr('对照时段', 'Comparison window') }}
      </h4>
      <p>
        {{ date(window.start) }} – {{ date(window.end) }} · {{ window.rows }}
        {{ tr('条记录', 'records') }}
      </p>
      <div class="report-table">
        <table>
          <thead>
            <tr>
              <th>{{ tr('指标', 'Metric') }}</th>
              <th>{{ tr('范围', 'Range') }}</th>
              <th>{{ tr('均值', 'Mean') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="metric in window.metrics" :id="anchor(metric.id)" :key="metric.id">
              <td>{{ metric.label || metric.key }}</td>
              <td>{{ number(metric.min) }}–{{ number(metric.max) }} {{ metric.unit }}</td>
              <td>{{ number(metric.mean) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div v-if="report.workflow?.vision" :id="anchor(report.image_hash)">
      <h4>{{ tr('视觉工具初步观察', 'Preliminary visual findings') }}</h4>
      <small>{{ report.workflow.vision.model }} · {{ report.workflow.vision.tool_version }}</small>
      <ul>
        <li v-for="(item, index) in report.workflow.vision.visible" :key="index">{{ item }}</li>
      </ul>
    </div>
    <div v-if="report.workflow?.quality?.length">
      <h4>{{ tr('质量诊断', 'Quality findings') }}</h4>
      <ul>
        <li v-for="(finding, i) in report.workflow.quality" :key="i">
          <strong>{{ finding.window ? finding.window + ' · ' : '' }}{{ finding.label }}</strong>
          <span v-if="finding.notes?.length"> · {{ finding.notes.join(' ') }}</span>
        </li>
      </ul>
    </div>
    <div v-if="report.workflow?.verification?.length">
      <h4>{{ tr('逐条复核', 'Claim verification') }}</h4>
      <ul>
        <li v-for="check in report.workflow.verification" :key="check.index">
          {{ check.index + 1 }} ·
          {{
            check.verdict === 'approved'
              ? tr('通过', 'Approved')
              : check.verdict === 'rejected'
                ? tr('未通过', 'Rejected')
                : tr('无法确认', 'Unconfirmed')
          }}
          · {{ check.reason }}
        </li>
      </ul>
    </div>
    <div v-for="source in report.knowledge" :id="anchor(source.id)" :key="source.id">
      <h4>{{ source.title }}</h4>
      <p>{{ source.text }}</p>
      <small>{{ source.source }}</small>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { tr } from '../i18n'
const props = defineProps({ report: { type: Object, required: true } })
const windows = computed(() => [props.report.evidence, props.report.comparison].filter(Boolean))
const date = (value) => new Date(value).toLocaleString()
const number = (value) =>
  value == null ? '—' : Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })
const anchor = (id) => 'task-report-' + props.report.id + '-' + id
function citationLabel(id) {
  return (
    props.report.knowledge?.find((source) => source.id === id)?.title ||
    windows.value.flatMap((window) => window.metrics).find((metric) => metric.id === id)?.label ||
    (id === props.report.image_hash ? tr('图片依据', 'Image evidence') : id)
  )
}
</script>

<style scoped>
.report {
  border-top: 1px solid var(--border);
  padding-top: 10px;
  font-size: 13px;
  line-height: 1.65;
}
h3,
h4 {
  margin: 10px 0 6px;
}
p {
  margin: 6px 0;
}
ul {
  margin: 4px 0 10px;
  padding-inline-start: 20px;
}
.references {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
a {
  color: #3d6b51;
}
.report-table {
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th,
td {
  border-bottom: 1px solid var(--border);
  text-align: left;
  padding: 6px;
}
</style>
