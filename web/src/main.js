import { createApp } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

const routes = [
  { path: '/', name: 'dashboard', component: () => import('./views/Dashboard.vue'), meta: { title: '状态总览' } },
  { path: '/config', name: 'config', component: () => import('./views/Config.vue'), meta: { title: '配置管理' } },
  { path: '/files', name: 'files', component: () => import('./views/Files.vue'), meta: { title: '文件编辑' } },
  { path: '/contacts', name: 'contacts', component: () => import('./views/Contacts.vue'), meta: { title: '通讯录' } },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

createApp(App).use(router).mount('#app')
