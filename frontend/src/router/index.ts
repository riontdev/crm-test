import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/inbox',
    },
    {
      path: '/inbox',
      name: 'inbox',
      component: () => import('@/views/InboxView.vue'),
      meta: { title: 'Bandeja' },
    },
    {
      path: '/inbox/:id',
      name: 'conversation',
      component: () => import('@/views/ConversationView.vue'),
      props: true,
      meta: { title: 'Conversación' },
    },
    {
      path: '/agents',
      name: 'agents',
      component: () => import('@/views/AgentsView.vue'),
      meta: { title: 'Agentes IA' },
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFoundView.vue'),
      meta: { title: 'No encontrado' },
    },
  ],
})

router.afterEach((to) => {
  const title = to.meta.title
  document.title = `${typeof title === 'string' ? title : ''} · SocialCRM`
})

export default router
