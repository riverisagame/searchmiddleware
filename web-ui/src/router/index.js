import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', component: () => import('../views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '仪表盘' } },
      { path: 'sync', component: () => import('../views/SyncCenter.vue'), meta: { title: '同步中心' } },
      { path: 'schedules', component: () => import('../views/Schedules.vue'), meta: { title: '定时任务' } },
      { path: 'logs', component: () => import('../views/Logs.vue'), meta: { title: '日志中心' } },
      { path: 'reconcile', component: () => import('../views/Reconcile.vue'), meta: { title: '对账中心' } },
      { path: 'indexes', component: () => import('../views/IndexEditor.vue'), meta: { title: '索引配置' } },
      { path: 'synonyms', component: () => import('../views/Synonyms.vue'), meta: { title: '同义词' } },
      { path: 'alerts', component: () => import('../views/Alerts.vue'), meta: { title: '告警中心' } },
      { path: 'users', component: () => import('../views/Users.vue'), meta: { title: '用户管理' } },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.isLoggedIn) {
    return { path: '/login' }
  }
  if (to.path === '/login' && auth.isLoggedIn) {
    return { path: '/' }
  }
  return true
})

export default router
