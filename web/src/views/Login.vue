<template>
  <div class="min-h-screen flex items-center justify-center bg-zinc-950 relative overflow-hidden">
    <!-- 背景光斑 -->
    <div class="absolute -top-40 -left-40 w-[500px] h-[500px] rounded-full bg-white/5 blur-[120px]" />
    <div class="absolute -bottom-40 -right-40 w-[500px] h-[500px] rounded-full bg-white/10 blur-[120px]" />

    <form class="relative bg-white/[0.06] backdrop-blur-xl border border-white/10 rounded-2xl shadow-2xl p-8 w-96 space-y-6" @submit.prevent="onSubmit">
      <div class="text-center space-y-3">
        <div class="mx-auto w-14 h-14 rounded-2xl bg-gradient-to-br from-white to-zinc-300 flex items-center justify-center text-zinc-900 font-bold text-2xl shadow-lg">
          A
        </div>
        <div>
          <h1 class="text-xl font-bold text-white">AniaBot 控制面板</h1>
          <p class="text-xs text-slate-400 mt-1.5">初始密码见首次启动时的控制台输出</p>
        </div>
      </div>

      <input
        v-model="password"
        type="password"
        placeholder="密码"
        required
        autofocus
        class="w-full bg-white/10 border border-white/10 rounded-lg px-3.5 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:border-transparent transition-shadow"
      />
      <p v-if="auth.notice" class="text-sm text-emerald-400">{{ auth.notice }}</p>
      <p v-if="error" class="text-sm text-red-400">{{ error }}</p>
      <button
        type="submit"
        :disabled="loading"
        class="w-full py-2.5 bg-white text-zinc-900 rounded-lg text-sm font-medium hover:bg-zinc-200 disabled:opacity-50 transition-all shadow-lg"
      >
        {{ loading ? '登录中...' : '登录' }}
      </button>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { api, auth } from '../api.js'

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
