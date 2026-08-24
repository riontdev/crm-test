import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: 'Ingresar', public: true },
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
      meta: { title: 'Dashboard', requiresAuth: true },
    },
    {
      path: '/inbox',
      name: 'inbox',
      component: () => import('@/views/InboxView.vue'),
      meta: { title: 'Bandeja', requiresAuth: true },
    },
    {
      path: '/inbox/:id',
      name: 'conversation',
      component: () => import('@/views/ConversationView.vue'),
      props: true,
      meta: { title: 'Conversación', requiresAuth: true },
    },
    {
      path: '/agents',
      name: 'agents',
      component: () => import('@/views/AgentsView.vue'),
      meta: { title: 'Agentes IA', requiresAuth: true },
    },
    {
      path: '/channels',
      name: 'channels',
      component: () => import('@/views/ChannelsView.vue'),
      meta: { title: 'Canales', requiresAuth: true },
    },
    {
      path: '/templates',
      name: 'templates',
      component: () => import('@/views/TemplatesView.vue'),
      meta: { title: 'Plantillas', requiresAuth: true },
    },
    {
      path: '/reports',
      name: 'reports',
      component: () => import('@/views/ReportsView.vue'),
      meta: { title: 'Reportes', requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/SettingsView.vue'),
      meta: { title: 'Configuración', requiresAuth: true },
    },
    {
      path: '/users',
      name: 'users',
      component: () => import('@/views/UsersView.vue'),
      meta: { title: 'Usuarios', requiresAuth: true, requiresAdmin: true },
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue'),
      meta: { title: 'No encontrado', requiresAuth: true },
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.initialized) await auth.init()

  if (to.meta.public) {
    if (auth.isAuthenticated && to.name === 'login') return { path: '/' }
    return true
  }

  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { path: '/inbox' }
  }

  return true
})

router.afterEach((to) => {
  const title = to.meta.title
  document.title = `${typeof title === 'string' ? title : ''} · SocialCRM`
})

export default router
