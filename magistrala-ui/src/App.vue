<template>
  <div
    class="app-shell"
    :class="{
      'has-workspace': !!route.params.id,
      'nav-collapsed': navigationCollapsed,
      'mobile-nav-open': mobileNavigationOpen,
    }"
  >
    <header class="topbar">
      <div class="topbar-location">
        <button
          v-if="route.params.id"
          class="icon-button quiet"
          :aria-label="tr('展开或收起导航', 'Toggle navigation')"
          @click="toggleNavigation"
        >
          <NavIcon name="menu" />
        </button>
        <nav class="breadcrumbs" :aria-label="tr('页面路径', 'Breadcrumb')">
          <router-link :to="route.params.id ? `/domain/${route.params.id}/home` : '/domains'">{{
            tr('首页', 'Home')
          }}</router-link
          ><template v-if="route.params.id"
            ><span aria-hidden="true">›</span
            ><router-link
              v-if="route.params.recordId"
              :to="{path: `/domain/${route.params.id}/${currentSection}`, query: route.query}"
              >{{ sectionLabel(currentSection) }}</router-link
            ><span v-else>{{ sectionLabel(currentSection) }}</span
            ><template v-if="route.params.recordId"
              ><span aria-hidden="true">›</span
              ><router-link
                v-if="route.params.tab"
                class="breadcrumb-id"
                :to="{path: `/domain/${route.params.id}/${currentSection}/${route.params.recordId}`, query: route.query}"
                >{{ route.params.recordId }}</router-link
              ><span v-else class="breadcrumb-id">{{
                route.params.recordId === 'create' ? tr('创建', 'Create') : route.params.recordId
              }}</span></template
            ><template v-if="route.params.tab"
              ><span aria-hidden="true">›</span><span>{{ tabLabel }}</span></template
            ></template
          >
        </nav>
      </div>
      <div class="actions">
        <LanguageSwitch /><button
          v-if="!['/login', '/signup'].includes(route.path)"
          class="quiet sign-out"
          @click="logout"
        >
          {{ tr('退出登录', 'Sign out') }}</button
        ><span class="account-avatar" aria-hidden="true"><NavIcon name="account" /></span>
      </div>
    </header>
    <p v-if="sessionError" class="error" role="alert">{{ sessionError }}</p>
    <router-view />
  </div>
</template>
<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import LanguageSwitch from './components/LanguageSwitch.vue'
import { tr, sectionLabel } from './i18n'
import NavIcon from './components/NavIcon.vue'
import { navigationCollapsed, mobileNavigationOpen, mobileNavigation, toggleNavigation } from './navigation'
import { signOut, watchSession } from './api/session'
const route = useRoute(),
  router = useRouter()
const sessionError = ref('')
let stopSessionWatch
const navigationMedia = window.matchMedia('(max-width: 1024px)')
function navigationResize(e) { mobileNavigation.value = e.matches; mobileNavigationOpen.value = false }
function navigationShortcut(e) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'b' && !e.target.closest('input, textarea, select, [contenteditable="true"]')) { e.preventDefault(); toggleNavigation() }
}
const currentSection = computed(() =>
  String(route.params.section || route.path.split('/')[3] || 'home'),
)
const tabLabel = computed(
  () =>
    ({
      connections: tr('连接', 'Connections'),
      groups: tr('分组', 'Groups'),
      access: tr('访问权限', 'Access'),
      'role-management': tr('角色', 'Roles'),
      users: tr('成员', 'Members'),
      messages: tr('消息', 'Messages'),
      journals: tr('日志', 'Journals'),
      devices: tr('设备', 'Devices'),
      channels: tr('通道', 'Channels'),
      alarms: tr('告警', 'Alarms'),
    })[String(route.params.tab)] || String(route.params.tab),
)
function expired() {
  router.replace('/login')
}
async function logout() {
  sessionError.value = ''
  try {
    await signOut()
    router.replace('/login')
  } catch {
    sessionError.value = tr(
      '暂时无法退出，请恢复与平台的连接后重试。',
      'Cannot sign out right now. Reconnect to the platform and retry.',
    )
  }
}
onMounted(() => {
  navigationMedia.addEventListener('change', navigationResize)
  window.addEventListener('keydown', navigationShortcut)
  window.addEventListener('agri-session-expired', expired)
  stopSessionWatch = watchSession()
})
onBeforeUnmount(() => {
  navigationMedia.removeEventListener('change', navigationResize)
  window.removeEventListener('keydown', navigationShortcut)
  window.removeEventListener('agri-session-expired', expired)
  stopSessionWatch?.()
})
</script>
