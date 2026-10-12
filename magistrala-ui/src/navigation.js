import { ref } from 'vue'
export const mobileNavigation = ref(window.matchMedia('(max-width: 1024px)').matches)
export const navigationCollapsed = ref(false)
export const mobileNavigationOpen = ref(false)
export function toggleNavigation() {
  if (mobileNavigation.value)
    mobileNavigationOpen.value = !mobileNavigationOpen.value
  else navigationCollapsed.value = !navigationCollapsed.value
}
export const navigationGroups = [
  { zh: '', en: '', keys: ['home', 'dashboards'] },
  {
    zh: '工作区管理',
    en: 'Workspace management',
    keys: ['info', 'users', 'role-management', 'invitations'],
  },
  {
    zh: '业务操作',
    en: 'Operations',
    keys: ['groups', 'channels', 'device-menu', 'messages', 'automation-menu', 'alarms', 'reports'],
  },
  { zh: '农业应用', en: 'Agriculture', keys: ['field-map', 'agent', 'agri-integration'] },
  { zh: '系统管理', en: 'Administration', keys: ['account', 'platform'] },
]
