<template>
  <div v-if="!auth.checked" class="min-h-screen flex items-center justify-center bg-slate-100">
    <p class="text-slate-500">加载中...</p>
  </div>

  <Login v-else-if="!auth.loggedIn" />

  <Wizard v-else-if="auth.setupRequired" />

  <div v-else class="min-h-screen bg-slate-100 flex">
    <!-- 侧边导航 -->
    <aside class="w-56 bg-slate-900 text-slate-200 flex flex-col shrink-0">
      <div class="px-5 py-5 text-lg font-bold text-white border-b border-slate-700">
        AniaBot 控制面板
      </div>
      <nav class="flex-1 py-3">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="block px-5 py-2.5 text-sm hover:bg-slate-800 transition-colors"
          :class="{ 'bg-slate-800 text-white border-r-2 border-indigo-400': $route.path === item.to }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
      <div class="px-5 py-4 border-t border-slate-700 space-y-2">
        <button class="block text-xs text-slate-400 hover:text-white" @click="onRestart">重启 Bot</button>
        <button class="block text-xs text-slate-400 hover:text-white" @click="showPwd = true">修改密码</button>
        <button class="block text-xs text-slate-400 hover:text-white" @click="onLogout">退出登录</button>
      </div>
    </aside>

    <!-- 主内容 -->
    <main class="flex-1 min-w-0">
      <header class="bg-white border-b border-slate-200 px-6 py-4">
        <h1 class="text-lg font-semibold text-slate-800">{{ $route.meta.title || '' }}</h1>
      </header>
      <div class="p-6">
        <RouterView />
      </div>
    </main>

    <!-- 重启中遮罩 -->
    <div v-if="restarting" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white rounded-lg shadow-xl p-8 w-80 text-center space-y-3">
        <div class="text-base font-semibold text-slate-800">正在重启 Bot...</div>
        <p class="text-sm text-slate-500">配置修改将在重启后生效，恢复后页面自动刷新</p>
      </div>
    </div>

    <!-- 修改密码弹窗 -->
    <div v-if="showPwd" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="showPwd = false">
      <form class="bg-white rounded-lg shadow-xl p-6 w-96 space-y-4" @submit.prevent="onChangePwd">
        <h2 class="text-base font-semibold text-slate-800">修改密码</h2>
        <input v-model="pwdForm.old" type="password" placeholder="原密码" required
          class="w-full border border-slate-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
        <input v-model="pwdForm.next" type="password" placeholder="新密码（至少 6 位）" required minlength="6"
          class="w-full border border-slate-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400" />
        <p v-if="pwdForm.msg" class="text-sm" :class="pwdForm.ok ? 'text-emerald-600' : 'text-red-600'">{{ pwdForm.msg }}</p>
        <div class="flex justify-end gap-2">
          <button type="button" class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded" @click="showPwd = false">取消</button>
          <button type="submit" class="px-4 py-2 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700">保存</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, auth } from './api.js'
import Login from './views/Login.vue'
import Wizard from './views/Wizard.vue'

const router = useRouter()
const navItems = [
  { to: '/', label: '状态总览' },
  { to: '/config', label: '配置管理' },
  { to: '/files', label: '扩展配置' },
  { to: '/contacts', label: '通讯录' },
]

const showPwd = ref(false)
const pwdForm = reactive({ old: '', next: '', msg: '', ok: false })
const restarting = ref(false)

onMounted(() => api.checkLogin())

async function onLogout() {
  await api.logout()
  router.push('/')
}

async function onRestart() {
  if (!confirm('确定要重启 Bot 吗？配置修改将在重启后生效。')) return
  restarting.value = true
  try {
    await api.restart()
  } catch { /* 进程可能已开始退出 */ }
  const up = await api.waitUntilUp()
  if (up) {
    location.reload()
  } else {
    restarting.value = false
    alert('等待重启超时，请检查 Bot 运行状态')
  }
}

async function onChangePwd() {
  pwdForm.msg = ''
  try {
    await api.changePassword(pwdForm.old, pwdForm.next)
    pwdForm.ok = true
    pwdForm.msg = '密码已更新'
    pwdForm.old = pwdForm.next = ''
  } catch (e) {
    pwdForm.ok = false
    pwdForm.msg = e.message
  }
}
</script>
