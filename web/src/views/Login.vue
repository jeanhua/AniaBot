<template>
  <div class="min-h-screen flex items-center justify-center bg-zinc-950 relative overflow-hidden select-none">
    <!-- 背景光斑 + 点阵网格 -->
    <div class="absolute -top-40 -left-40 w-125 h-125 rounded-full bg-white/5 blur-[120px]" />
    <div class="absolute -bottom-40 -right-40 w-125 h-125 rounded-full bg-white/10 blur-[120px]" />
    <div
      class="absolute inset-0 pointer-events-none"
      style="background-image: radial-gradient(rgba(255, 255, 255, 0.07) 1px, transparent 1px); background-size: 28px 28px;"
    />

    <!-- 终端窗口式登录卡片，呼应控制台输出 -->
    <form
      class="relative w-135 max-w-[92vw] bg-zinc-900/80 backdrop-blur-xl border border-white/10 rounded-xl shadow-2xl overflow-hidden"
      @submit.prevent="onSubmit"
    >
      <!-- 终端标题栏 -->
      <div class="flex items-center gap-1.5 px-4 py-3 bg-white/4 border-b border-white/10" aria-hidden="true">
        <span class="w-3 h-3 rounded-full bg-[#ff5f57]" />
        <span class="w-3 h-3 rounded-full bg-[#febc2e]" />
        <span class="w-3 h-3 rounded-full bg-[#28c840]" />
        <span class="ml-3 font-mono text-xs text-zinc-500">ania@bot: ~/console</span>
      </div>

      <div class="p-7 space-y-6">
        <!-- 控制台同款 ASCII 标识 -->
        <pre
          aria-hidden="true"
          class="font-mono text-center select-none bg-linear-to-b from-zinc-100 via-zinc-300 to-zinc-600 bg-clip-text text-transparent text-[6px] min-[480px]:text-[8px] sm:text-[10px] leading-[1.2]"
        >{{ LOGO }}</pre>

        <div class="flex items-center gap-3">
          <span class="h-px flex-1 bg-white/10" />
          <h1 class="text-[11px] tracking-[0.35em] text-zinc-400 font-medium whitespace-nowrap">AniaBot 控制面板</h1>
          <span class="h-px flex-1 bg-white/10" />
        </div>

        <!-- 模拟启动日志 -->
        <div class="font-mono text-[11px] leading-relaxed space-y-1" aria-hidden="true">
          <p class="text-zinc-600"># 初始密码见首次启动时的控制台输出</p>
          <p><span class="text-emerald-400">[ OK ]</span> <span class="text-zinc-400">插件系统就绪</span></p>
          <p><span class="text-emerald-400">[ OK ]</span> <span class="text-zinc-400">Web 控制面板已启动</span></p>
          <p><span class="text-amber-400">[ .. ]</span> <span class="text-zinc-400">等待管理员认证<span class="cursor-blink">▋</span></span></p>
        </div>

        <!-- 终端提示符式密码输入 -->
        <div class="flex items-center gap-2.5 font-mono">
          <span class="text-emerald-400 text-sm select-none" aria-hidden="true">➜</span>
          <input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            required
            autofocus
            aria-label="密码"
            class="flex-1 min-w-0 bg-transparent border-b border-white/15 focus:border-emerald-400/70 px-1 py-1.5 text-sm text-white placeholder-zinc-600 focus:outline-none transition-colors"
          />
        </div>

        <p v-if="auth.notice" class="font-mono text-xs text-emerald-400">✓ {{ auth.notice }}</p>
        <p v-if="error" class="font-mono text-xs text-red-400">✗ {{ error }}</p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full py-2.5 bg-white text-zinc-900 rounded-lg text-sm font-medium hover:bg-zinc-200 disabled:opacity-50 transition-all shadow-lg"
        >
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { api, auth } from '../api.js'

const LOGO = `
   █████████               ███            ███████████            █████   
  ███░░░░░███             ░░░            ░░███░░░░░███          ░░███    
 ░███    ░███  ████████   ████   ██████   ░███    ░███  ██████  ███████  
 ░███████████ ░░███░░███ ░░███  ░░░░░███  ░██████████  ███░░███░░░███░   
 ░███░░░░░███  ░███ ░███  ░███   ███████  ░███░░░░░███░███ ░███  ░███    
 ░███    ░███  ░███ ░███  ░███  ███░░███  ░███    ░███░███ ░███  ░███ ███
 █████   █████ ████ █████ █████░░████████ ███████████ ░░██████   ░░█████ 
░░░░░   ░░░░░ ░░░░ ░░░░░ ░░░░░  ░░░░░░░░ ░░░░░░░░░░░   ░░░░░░     ░░░░░  
`

const password = ref('')
const error = ref('')
const loading = ref(false)

async function onSubmit() {
  error.value = ''
  auth.notice = ''
  loading.value = true
  try {
    await api.login(password.value)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* 终端光标闪烁 */
.cursor-blink {
  display: inline-block;
  margin-left: 2px;
  animation: cursor-blink 1.1s steps(1) infinite;
}
@keyframes cursor-blink {
  50% {
    opacity: 0;
  }
}
</style>
