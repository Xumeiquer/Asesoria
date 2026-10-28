import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('@/pages/DashboardPage.vue') },
    { path: '/calendario', name: 'calendar', component: () => import('@/pages/CalendarPage.vue') },
    { path: '/modelos', name: 'models', component: () => import('@/pages/ModelsPage.vue') },
    {
      path: '/expedientes/:id',
      name: 'filing',
      component: () => import('@/pages/FilingPage.vue'),
      props: true,
    },
    { path: '/documentos', name: 'documents', component: () => import('@/pages/DocumentsPage.vue') },
    { path: '/facturas', name: 'invoices', component: () => import('@/pages/InvoicesPage.vue') },
    { path: '/contribuyentes', name: 'taxpayers', component: () => import('@/pages/TaxpayersPage.vue') },
    { path: '/ajustes', name: 'settings', component: () => import('@/pages/SettingsPage.vue') },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

export default router
