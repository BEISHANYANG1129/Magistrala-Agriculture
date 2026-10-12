<template>
  <component
    :is="mobileNavigation ? ModalDialog : 'div'"
    v-if="!mobileNavigation || mobileNavigationOpen"
    :drawer="mobileNavigation"
    @close="mobileNavigationOpen = false"
  >
    <aside class="sidebar" :aria-label="tr('主导航', 'Main navigation')">
      <router-link class="brand" to="/domains" @click="mobileNavigationOpen = false"
        ><span class="brand-symbol" aria-hidden="true">▰</span
        ><span class="nav-label"
          >Magistrala <small>{{ tr('农业平台', 'Agriculture') }}</small></span
        ></router-link
      >
      <div class="workspace-picker">
        <label class="workspace-caption" for="workspace-select">{{
          tr('当前工作区', 'Workspace')
        }}</label
        ><select id="workspace-select" :value="workspace" @change="switchWorkspace">
          <option v-if="!workspaces.some((w) => w.id === workspace)" :value="workspace">
            {{ workspaceName }}
          </option>
          <option v-for="w in workspaces" :key="w.id" :value="w.id">{{ w.name }}</option>
        </select>
      </div>
      <nav>
        <div v-for="group in navigationGroups" :key="group.en" class="nav-group">
          <p v-if="group.en" class="nav-group-title">{{ tr(group.zh, group.en) }}</p>
          <template v-for="key in group.keys" :key="key">
            <template v-if="submenus[key]">
              <button
                class="nav-branch"
                :aria-expanded="expanded[key]"
                @click="expanded[key] = !expanded[key]"
                :title="tr(submenus[key].zh, submenus[key].en)"
              >
                <NavIcon :name="submenus[key].icon" /><span class="nav-label">{{
                  tr(submenus[key].zh, submenus[key].en)
                }}</span
                ><span class="nav-label">{{ expanded[key] ? '⌄' : '›' }}</span>
              </button>
              <div v-if="expanded[key]" class="nav-children">
                <router-link
                  v-for="child in submenus[key].keys"
                  :key="child"
                  :to="`/domain/${workspace}/${child}`"
                  active-class="active"
                  :title="sectionLabel(child)"
                  @click="mobileNavigationOpen = false"
                  ><NavIcon :name="child" /><span class="nav-label">{{
                    sectionLabel(child)
                  }}</span></router-link
                >
              </div>
            </template>
            <router-link
              v-else
              :to="`/domain/${workspace}/${key}`"
              active-class="active"
              :title="sectionLabel(key)"
              @click="mobileNavigationOpen = false"
              ><NavIcon :name="key" /><span class="nav-label">{{
                sectionLabel(key)
              }}</span></router-link
            >
          </template>
        </div>
      </nav>
      <router-link class="switch-workspace" to="/domains" @click="mobileNavigationOpen = false"
        ><NavIcon name="arrow" /><span class="nav-label">{{
          tr('所有工作区', 'All workspaces')
        }}</span></router-link
      >
    </aside>
  </component>
</template>
<script setup>
import { computed, ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tr, sectionLabel } from '../i18n'
import { gql, listAll } from '../api/platform'
import ModalDialog from './ModalDialog.vue'
import { navigationGroups, mobileNavigationOpen, mobileNavigation } from '../navigation'
import NavIcon from './NavIcon.vue'
const submenus = {
  'device-menu': {
    zh: '设备管理',
    en: 'Device Management',
    icon: 'devices',
    keys: ['devices', 'gateways', 'device-types', 'bootstraps'],
  },
  'automation-menu': {
    zh: '自动化',
    en: 'Automation',
    icon: 'rules',
    keys: ['rules', 'custom-nodes'],
  },
}
const expanded = ref({ 'device-menu': true, 'automation-menu': true })
const route = useRoute(),
  router = useRouter(),
  workspace = computed(() => String(route.params.id || '')),
  workspaceName = ref(''),
  workspaces = ref([])
watch(
  workspace,
  async (id) => {
    workspaceName.value = id
    try {
      const data = await gql('query($id:ID!){tenant(id:$id){name}}', { id })
      if (workspace.value === id) workspaceName.value = data.tenant.name
    } catch {}
  },
  { immediate: true },
)
onMounted(async () => {
  try {
    workspaces.value = await listAll('tenants', {})
  } catch {}
})
function switchWorkspace(e) {
  mobileNavigationOpen.value = false
  router.push(`/domain/${e.target.value}/home`)
}
</script>
