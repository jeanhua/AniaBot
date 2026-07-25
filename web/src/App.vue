<template>
  <div v-if="!auth.checked" class="min-h-screen flex items-center justify-center bg-slate-100">
    <div class="flex items-center gap-2 text-slate-400 text-sm">
      <span class="w-4 h-4 border-2 border-slate-300 border-t-zinc-500 rounded-full animate-spin" />
      加载中...
    </div>
  </div>

  <Login v-else-if="!auth.loggedIn" />

  <Wizard v-else-if="auth.setupRequired" />

  <div v-else class="min-h-screen bg-[#f3f3f2] flex">
    <!-- 侧边导航 -->
    <aside class="w-60 bg-zinc-950 flex flex-col shrink-0 sticky top-0 h-screen">
      <div class="px-5 pt-6 pb-5 flex items-center gap-3">
        <div class="w-9 h-9 rounded-md bg-white flex items-center justify-center text-zinc-950 font-bold text-base shadow-lg">
          A
        </div>
        <div>
          <div class="text-white font-semibold leading-tight tracking-[0.2em]">ANIABOT</div>
          <div class="text-[10px] text-zinc-500 leading-tight tracking-[0.15em] uppercase mt-0.5">Console · 控制面板</div>
        </div>
      </div>
      <div class="mx-5 h-px bg-white/10" />

      <nav class="flex-1 px-3 py-3 space-y-1 overflow-y-auto">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-3 px-3 py-2.5 rounded-md text-[11px] tracking-[0.15em] transition-all"
          :class="$route.path === item.to
            ? 'bg-white text-zinc-950 font-semibold'
            : 'text-zinc-500 hover:text-zinc-200 hover:bg-white/5'"
        >
          <span v-html="item.icon" class="shrink-0 [&>svg]:w-[16px] [&>svg]:h-[16px]" />
          {{ item.label }}
          <span v-if="$route.path === item.to" class="ml-auto w-1.5 h-1.5 bg-zinc-950" />
        </RouterLink>
      </nav>

      <div class="px-3 py-4 border-t border-white/10 space-y-0.5">
        <button class="nav-foot" @click="onRestart">
          <span v-html="icons.restart" class="[&>svg]:w-4 [&>svg]:h-4" /> 重启 Bot
        </button>
        <button class="nav-foot" @click="showPwd = true">
          <span v-html="icons.key" class="[&>svg]:w-4 [&>svg]:h-4" /> 修改密码
        </button>
        <button class="nav-foot hover:!text-red-300" @click="onLogout">
          <span v-html="icons.logout" class="[&>svg]:w-4 [&>svg]:h-4" /> 退出登录
        </button>
      </div>
    </aside>

    <!-- 主内容 -->
    <main class="flex-1 min-w-0 flex flex-col">
      <header class="bg-white/85 backdrop-blur border-b border-zinc-200 px-8 py-4 sticky top-0 z-30 flex items-center justify-between">
        <h1 class="text-[11px] tracking-[0.22em] uppercase text-zinc-500 font-medium">
          <span class="text-zinc-300 mr-2">//</span>{{ $route.meta.title || '' }}
        </h1>
        <span class="tpill"><span class="tdot bg-emerald-500" />Bot Online</span>
      </header>
      <div class="p-8 flex-1">
        <RouterView v-slot="{ Component }">
          <Transition name="fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>
    </main>

    <!-- 重启中遮罩 -->
    <div v-if="restarting" class="fixed inset-0 bg-zinc-950/60 backdrop-blur-sm flex items-center justify-center z-50">
      <div class="tcard p-8 w-80 text-center space-y-3">
        <span class="mx-auto block w-8 h-8 border-[3px] border-zinc-200 border-t-zinc-800 rounded-full animate-spin" />
        <div class="text-sm font-semibold text-zinc-900 tracking-[0.15em] uppercase">Rebooting</div>
        <p class="text-xs text-zinc-500">配置修改将在重启后生效，恢复后页面自动刷新</p>
      </div>
    </div>

    <!-- 修改密码弹窗 -->
    <div v-if="showPwd" class="fixed inset-0 bg-zinc-950/50 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showPwd = false">
      <form class="tcard p-6 w-96 space-y-4" @submit.prevent="onChangePwd">
        <h2 class="text-[11px] tracking-[0.22em] uppercase text-zinc-500 font-medium">修改密码</h2>
        <input v-model="pwdForm.old" type="password" placeholder="原密码" required :class="inputClass" />
        <input v-model="pwdForm.next" type="password" placeholder="新密码（至少 6 位）" required minlength="6" :class="inputClass" />
        <p v-if="pwdForm.msg" class="text-xs" :class="pwdForm.ok ? 'text-emerald-600' : 'text-red-600'">{{ pwdForm.msg }}</p>
        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="px-4 py-2 text-[11px] tracking-[0.1em] uppercase text-zinc-500 hover:bg-zinc-100 rounded-md transition-colors" @click="showPwd = false">取消</button>
          <button type="submit" class="px-4 py-2 text-[11px] tracking-[0.1em] uppercase bg-zinc-900 text-white rounded-md hover:bg-zinc-700 transition-colors">保存</button>
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

const icons = {
  dashboard: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z"/></svg>',
  config: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/></svg>',
  files: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5"/></svg>',
  contacts: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z"/></svg>',
  chat: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 0 1-.825-.242m9.345-8.334a2.126 2.126 0 0 0-.476-.095 48.64 48.64 0 0 0-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0 0 11.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155"/></svg>',
  skills: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456ZM16.894 20.567 16.5 21.75l-.394-1.183a2.25 2.25 0 0 0-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 0 0 1.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 0 0 1.423 1.423l1.183.394-1.183.394a2.25 2.25 0 0 0-1.423 1.423Z"/></svg>',
  memory: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M17.593 3.322c1.1.128 1.907 1.077 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0 1 11.186 0Z"/></svg>',
  restart: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"/></svg>',
  key: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 0 1 3 3m3 0a6 6 0 0 1-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1 1 21.75 8.25Z"/></svg>',
  logout: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9"/></svg>',
}

const navItems = [
  { to: '/', label: '状态总览', icon: icons.dashboard },
  { to: '/msglogs', label: '消息日志', icon: icons.chat },
  { to: '/skills', label: '技能管理', icon: icons.skills },
  { to: '/memory', label: '记忆管理', icon: icons.memory },
  { to: '/config', label: '配置管理', icon: icons.config },
  { to: '/files', label: '扩展配置', icon: icons.files },
  { to: '/contacts', label: '通讯录', icon: icons.contacts },
]

const inputClass = 'w-full border border-zinc-300 rounded-md px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:border-zinc-400 transition-shadow bg-white'

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
    // 修改密码后服务端会销毁所有会话，退回登录页重新登录
    showPwd.value = false
    pwdForm.old = pwdForm.next = ''
    auth.notice = '密码已更新，请使用新密码重新登录'
    auth.loggedIn = false
  } catch (e) {
    pwdForm.ok = false
    pwdForm.msg = e.message
  }
}
</script>

<style scoped>
.nav-foot {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  width: 100%;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 11px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: rgb(113 113 122);
  transition: all 0.15s;
}
.nav-foot:hover {
  color: rgb(228 228 231);
  background: rgb(255 255 255 / 0.05);
}
</style>
