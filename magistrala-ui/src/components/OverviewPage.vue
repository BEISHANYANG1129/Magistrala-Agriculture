<template>
  <section>
    <header class="page-heading">
      <div>
        <h1>{{ tr('工作区概览', 'Workspace overview') }}</h1>
        <p>
          {{
            tr(
              '查看工作区资源，管理设备与农业应用。',
              'View workspace resources, devices and agricultural applications.',
            )
          }}
        </p>
      </div>
      <router-link class="button quiet" :to="`/domain/${route.params.id}/info`">{{
        tr('工作区设置', 'Workspace settings')
      }}</router-link>
    </header>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="overview-stats">
      <router-link
        v-for="card in cards"
        :key="card.path"
        :to="`/domain/${route.params.id}/${card.path}`"
        class="stat-card"
        ><div>
          <p>{{ tr(card.zh, card.en) }}</p>
          <span class="metric">{{ counts[card.path] ?? '—' }}</span>
        </div>
        <span class="stat-icon"><NavIcon :name="card.path" /></span
      ></router-link>
    </div>
    <h2 class="overview-section-title">{{ tr('农业应用', 'Agricultural applications') }}</h2>
    <div class="overview-apps">
      <article v-for="app in apps" :key="app.path" class="application-card">
        <NavIcon :name="app.path" />
        <h3>{{ tr(app.zh, app.en) }}</h3>
        <p>{{ tr(app.descZh, app.descEn) }}</p>
        <router-link :to="`/domain/${route.params.id}/${app.path}`"
          >{{ tr('打开应用', 'Open application') }} <span aria-hidden="true">→</span></router-link
        >
      </article>
    </div>
    <h2 class="overview-section-title">{{ tr('快捷访问', 'Quick access') }}</h2>
    <div class="overview-links">
      <div v-for="group in shortcuts" :key="group.en" class="overview-link-group">
        <h3>{{ tr(group.zh, group.en) }}</h3>
        <router-link v-for="key in group.keys" :key="key" :to="`/domain/${route.params.id}/${key}`"
          ><NavIcon :name="key" /><span>{{ sectionLabel(key) }}</span
          ><span aria-hidden="true">›</span></router-link
        >
      </div>
    </div>
  </section>
</template>
<script setup>
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { gql, errorText } from '../api/platform'
import { tr, sectionLabel } from '../i18n'
import NavIcon from './NavIcon.vue'
const route = useRoute(),
  counts = ref({}),
  error = ref('')
const shortcuts = [
  { zh: '设备管理', en: 'Device management', keys: ['devices', 'channels', 'groups', 'gateways'] },
  { zh: '数据与自动化', en: 'Data & automation', keys: ['messages', 'rules', 'alarms', 'reports'] },
  {
    zh: '工作区管理',
    en: 'Workspace management',
    keys: ['info', 'users', 'role-management', 'invitations'],
  },
]
const apps = [
  {
    path: 'field-map',
    zh: '农田分区与位置',
    en: 'Field areas & locations',
    descZh: '查看农田底图、划分区域并标记设备位置。',
    descEn: 'Organize field areas and place devices on your field map.',
  },
  {
    path: 'agent',
    zh: '区域智能分析',
    en: 'Regional intelligence',
    descZh: '按区域读取传感器数据，查看分析依据与执行建议。',
    descEn: 'Review sensor evidence and recommendations for each region.',
  },
  {
    path: 'agri-integration',
    zh: '农业数据集成',
    en: 'Agriculture integration',
    descZh: '查看农业服务连接、设备映射和数据采集情况。',
    descEn: 'Review service connections, device mappings and data collection.',
  },
]
const cards = [
  { path: 'devices', zh: '设备', en: 'Devices' },
  { path: 'channels', zh: '通道', en: 'Channels' },
  { path: 'groups', zh: '分组', en: 'Groups' },
  { path: 'users', zh: '成员', en: 'Members' },
]
watch(
  () => route.params.id,
  async (id) => {
    try {
      const data = await gql(
        'query($id:ID!){devices:entities(tenantId:$id,kind:device){total} channels:resources(tenantId:$id,kind:"channel"){total} groups:groups(tenantId:$id){total} users:tenantMembers(tenantId:$id){total}}',
        { id },
      )
      if (route.params.id === id)
        counts.value = Object.fromEntries(Object.entries(data).map(([k, v]) => [k, v.total]))
    } catch (e) {
      error.value = errorText(e)
    }
  },
  { immediate: true },
)
</script>
