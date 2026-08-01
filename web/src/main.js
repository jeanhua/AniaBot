import { createApp } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

const routes = [
  { path: '/', name: 'dashboard', component: () => import('./views/Dashboard.vue'), meta: { title: '状态总览' } },
  { path: '/console', name: 'console', component: () => import('./views/ConsoleLog.vue'), meta: { title: '控制台日志' } },
  { path: '/msglogs', name: 'msglogs', component: () => import('./views/MsgLog.vue'), meta: { title: '消息日志' } },
  { path: '/querylogs', name: 'querylogs', component: () => import('./views/QueryLog.vue'), meta: { title: 'Query 日志' } },
  { path: '/tokenstats', name: 'tokenstats', component: () => import('./views/TokenStats.vue'), meta: { title: 'Token 统计' } },
  { path: '/quota', name: 'quota', component: () => import('./views/Quota.vue'), meta: { title: '配额管理' } },
  { path: '/tasklogs', name: 'tasklogs', component: () => import('./views/TaskLog.vue'), meta: { title: '定时任务' } },
  { path: '/skills', name: 'skills', component: () => import('./views/Skills.vue'), meta: { title: '技能管理' } },
  { path: '/memory', name: 'memory', component: () => import('./views/Memory.vue'), meta: { title: '记忆管理' } },
  { path: '/knowledge', name: 'knowledge', component: () => import('./views/Knowledge.vue'), meta: { title: '知识库' } },
  { path: '/team', name: 'team', component: () => import('./views/Team.vue'), meta: { title: 'Agent 团队' } },
  { path: '/config', name: 'config', component: () => import('./views/Config.vue'), meta: { title: '配置管理' } },
  { path: '/files', name: 'files', component: () => import('./views/Files.vue'), meta: { title: '扩展配置' } },
  { path: '/contacts', name: 'contacts', component: () => import('./views/Contacts.vue'), meta: { title: '通讯录' } },
  { path: '/update', name: 'update', component: () => import('./views/Update.vue'), meta: { title: '自动更新' } },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

createApp(App).use(router).mount('#app')
