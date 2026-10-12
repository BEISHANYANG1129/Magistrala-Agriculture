<template>
  <div class="channel-chart-card">
    <p v-if="notice" role="status">{{ notice }}</p>
    <div class="chart-toolbar">
      <label>选择主题：</label>
      <select v-model="selectedSubtopic">
        <option v-for="sub in subtopics" :key="sub" :value="sub">{{ sub }}</option>
      </select>
      <label style="margin-left:20px;">开始日期：</label>
      <input type="date" v-model="startDate" />
      <label style="margin-left:10px;">结束日期：</label>
      <input type="date" v-model="endDate" />
    </div>
    <div ref="chartRef" style="width: 100%; height: 400px;"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import * as echarts from 'echarts'

const route = useRoute()
const domainId = route.params.id
const channelId = route.params.channelId
const userToken = localStorage.getItem('token')
const notice = ref('')

const messages = ref([])
const chartRef = ref(null)
let chartInstance = null

const selectedSubtopic = ref('')
const subtopics = ref([])
const startDate = ref('')
const endDate = ref('')

// 获取当前 subtopic 的所有消息的日期区间
function updateDateRange() {
  const filtered = messages.value.filter(msg => msg.subtopic === selectedSubtopic.value && typeof msg.value === 'number')
  if (filtered.length === 0) return
  const times = filtered.map(m => Number(m.time) / 1e6)
  const minDate = new Date(Math.min(...times))
  const maxDate = new Date(Math.max(...times))
  startDate.value = minDate.toISOString().slice(0, 10)
  endDate.value = maxDate.toISOString().slice(0, 10)
}

// 只展示日期区间内的数据
function renderChart() {
  if (!chartRef.value) return
  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value)
  }
  const filtered = messages.value.filter(msg => {
    if (msg.subtopic !== selectedSubtopic.value || typeof msg.value !== 'number') return false
    const t = Number(msg.time) / 1e6
    const d = new Date(t)
    const dateStr = d.toISOString().slice(0, 10)
    return (!startDate.value || dateStr >= startDate.value) && (!endDate.value || dateStr <= endDate.value)
  })
  const xData = filtered.map(m => new Date(Number(m.time) / 1e6))
  const yData = filtered.map(m => m.value)
  chartInstance.setOption({
    title: { text: selectedSubtopic.value + '变化趋势' },
    tooltip: {},
    xAxis: {
      type: 'time',
      axisLabel: {
        rotate: 45,
        formatter: function (value) {
          const date = new Date(value)
          // 显示“yyyy-MM-dd HH:mm”
          return date.getFullYear() + '-' +
            String(date.getMonth() + 1).padStart(2, '0') + '-' +
            String(date.getDate()).padStart(2, '0') + ' ' +
            String(date.getHours()).padStart(2, '0') + ':' +
            String(date.getMinutes()).padStart(2, '0')
        }
      }
    },
    yAxis: { type: 'value' },
    series: [{
      data: xData.map((x, i) => [x, yData[i]]),
      type: 'line',
      smooth: true,
      name: selectedSubtopic.value
    }]
  })
}

async function fetchMessages() {
  const url = `/Messages/${domainId}/channels/${channelId}/messages?offset=0&limit=1000`
  const res = await fetch(url, {
    headers: {
      'Authorization': `Bearer ${userToken}` // 使用usertoken
    }
  })
  if (res.ok) {
    const data = await res.json()
    messages.value = data.messages || []
    // 提取所有 subtopic
    const set = new Set()
    messages.value.forEach(msg => {
      if (msg.subtopic) set.add(msg.subtopic)
    })
    subtopics.value = Array.from(set)
    if (!selectedSubtopic.value && subtopics.value.length > 0) {
      selectedSubtopic.value = subtopics.value[0]
    }
    renderChart()
  } else {
    notice.value = '当前核心暂未提供此页面所需的历史消息接口。'
    renderChart()
  }
}

onMounted(async () => {
  await fetchMessages()
  updateDateRange()
})

watch(selectedSubtopic, () => {
  updateDateRange()
  renderChart()
})
watch([startDate, endDate], renderChart)
</script>

<style scoped>
.channel-chart-card {
  width: 90%;
  max-width: 900px;
  margin: 0 auto;
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 0 24px #eee;
  padding: 32px 32px 24px 32px;
  align-self: flex-start;
  position: relative;
  overflow: auto;
}
.chart-toolbar {
  display: flex;
  align-items: center;
  gap: 10px; /* 控件间距缩小 */
  margin-bottom: 24px;
  font-size: 1.08em;
  flex-wrap: wrap;
}
.chart-toolbar label {
  font-weight: 500;
  color: #2a3a4a;
  margin-right: 2px; /* label和控件间距更小 */
}
.chart-toolbar select,
.chart-toolbar input[type="date"] {
  padding: 4px 10px;
  border: 1px solid #d0d6e1;
  border-radius: 6px;
  font-size: 1em;
  background: #f8fafc;
  transition: border-color 0.2s;
  margin-right: 10px; /* 控件之间间距 */
}
.chart-toolbar select:last-child,
.chart-toolbar input[type="date"]:last-child {
  margin-right: 0;
}
</style>
