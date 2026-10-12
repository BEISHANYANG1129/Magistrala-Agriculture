import { createRouter, createWebHistory } from 'vue-router'
import Login from '../components/Login.vue'
import Domains from '../components/Domains.vue'
import MainLayout from '../components/MainLayout.vue'
import OverviewPage from '../components/OverviewPage.vue'
import ManagementPage from '../components/ManagementPage.vue'
import { ensureSession } from '../api/session'
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/domains' },
    { path: '/login', component: Login },
    { path: '/signup', component: Login },
    { path: '/domains', component: Domains },
    {
      path: '/domain/:id',
      component: MainLayout,
      children: [
        { path: '', redirect: (to) => `/domain/${to.params.id}/home` },
        { path: 'home', component: OverviewPage },
        { path: 'agent', component: () => import('../components/AgentWorkbench.vue') },
        { path: 'field-map', component: () => import('../components/FieldMap.vue') },
        { path: 'messages', component: () => import('../components/MessagesPage.vue') },
        { path: 'agri-integration', component: () => import('../components/IntegrationPage.vue') },
        {
          path: ':section(devices|gateways|channels|groups)/:recordId?/:tab?',
          component: () => import('../components/ResourcePage.vue'),
        },
        { path: 'rules/:recordId?/:tab?', component: () => import('../components/RulesPage.vue') },
        {
          path: ':section(device-types|reports|bootstraps|bootstrap-profiles|custom-nodes|alarms|dashboards)/:recordId?/:tab?',
          component: () => import('../components/NativeCollection.vue'),
        },
        {
          path: ':section(info|users|role-management|invitations)/:recordId?/:tab?',
          component: () => import('../components/WorkspaceManagement.vue'),
        },
        { path: 'account', component: () => import('../components/AccountPage.vue') },
        { path: 'platform', component: () => import('../components/AuditPage.vue') },
        { path: ':section', component: ManagementPage },
        { path: ':section/:recordId/:tab?', component: ManagementPage },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/domains' },
  ],
})
router.beforeEach(async (to) => {
  if (['/login', '/signup'].includes(to.path)) return
  try {
    if (!(await ensureSession())) return '/login'
  } catch {
    // Preserve the current route during a temporary outage; pages show connection errors.
  }
})
export default router
