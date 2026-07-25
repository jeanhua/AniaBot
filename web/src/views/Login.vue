<template>
  <div class="min-h-screen flex items-center justify-center bg-slate-100">
    <form class="bg-white rounded-lg shadow-lg p-8 w-96 space-y-5" @submit.prevent="onSubmit">
      <h1 class="text-xl font-bold text-slate-800 text-center">AniaBot 控制面板</h1>
      <p class="text-xs text-slate-500 text-center">初始密码见首次启动时的控制台输出</p>
      <input
        v-model="password"
        type="password"
        placeholder="密码"
        required
        autofocus
        class="w-full border border-slate-300 rounded px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
      />
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      <button
        type="submit"
        :disabled="loading"
        class="w-full py-2.5 bg-indigo-600 text-white rounded text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
      >
        {{ loading ? '登录中...' : '登录' }}
      </button>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { api } from '../api.js'

const password = ref('')
const error = ref('')
const loading = ref(false)

async function onSubmit() {
  error.value = ''
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
