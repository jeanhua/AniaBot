<template>
  <div class="min-h-screen bg-slate-100 flex items-center justify-center p-4">
    <div class="bg-white rounded-lg shadow-lg w-full max-w-lg">
      <!-- 步骤指示 -->
      <div class="flex border-b border-slate-100">
        <div
          v-for="(label, i) in ['欢迎', '连接 NapCat', 'AI 配置', '完成']"
          :key="i"
          class="flex-1 py-3 text-center text-xs"
          :class="i === step ? 'text-indigo-600 font-semibold border-b-2 border-indigo-600' : i < step ? 'text-emerald-600' : 'text-slate-400'"
        >
          {{ i + 1 }}. {{ label }}
        </div>
      </div>

      <div class="p-8">
        <!-- 步骤 0: 欢迎 -->
        <div v-if="step === 0" class="space-y-4 text-center">
          <h1 class="text-xl font-bold text-slate-800">欢迎使用 AniaBot 🎉</h1>
          <p class="text-sm text-slate-500 leading-relaxed">
            这是首次启动，接下来用两步完成最基本的配置：<br />
            <b>连接 NapCat</b>（QQ 协议端）和 <b>AI 对话模型</b>。<br />
            其余配置（插件、MCP、Prompt 覆盖等）可稍后在控制面板中完善。
          </p>
          <p class="text-xs text-slate-400">所有配置保存在数据库中，也可随时跳过，之后在「配置管理」中修改。</p>
        </div>

        <!-- 步骤 1: 连接 NapCat -->
        <div v-else-if="step === 1" class="space-y-4">
          <h2 class="text-base font-semibold text-slate-800">连接 NapCat</h2>
          <div>
            <label class="block text-xs font-medium text-slate-600 mb-1">WebSocket 地址</label>
            <input v-model="form.wsAddress" type="text" placeholder="ws://localhost:4455" :class="inputClass" />
            <p class="text-xs text-slate-400 mt-1">NapCat 的 WebSocket 服务端地址；Docker 部署时把 localhost 换成内网 IP</p>
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-600 mb-1">Access Token（可选）</label>
            <input v-model="form.token" type="password" placeholder="NapCat 端设置了 token 时填写" :class="inputClass" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-600 mb-1">管理员 QQ</label>
            <input v-model="form.adminId" type="number" placeholder="你的 QQ 号，接收启动/异常通知" :class="inputClass" />
          </div>
        </div>

        <!-- 步骤 2: AI 配置 -->
        <div v-else-if="step === 2" class="space-y-4">
          <h2 class="text-base font-semibold text-slate-800">AI 对话模型</h2>
          <div>
            <label class="block text-xs font-medium text-slate-600 mb-1">Base URL</label>
            <input v-model="form.baseUrl" type="text" placeholder="https://api.deepseek.com" :class="inputClass" />
            <p class="text-xs text-slate-400 mt-1">任意兼容 OpenAI 规范的 API 地址</p>
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-600 mb-1">API Key</label>
            <input v-model="form.apiKey" type="password" placeholder="sk-..." :class="inputClass" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-600 mb-1">模型</label>
            <input v-model="form.model" type="text" placeholder="deepseek-chat" :class="inputClass" />
          </div>
        </div>

        <!-- 步骤 3: 完成 -->
        <div v-else class="space-y-4 text-center">
          <template v-if="!restarting">
            <h1 class="text-xl font-bold text-slate-800">配置完成 ✅</h1>
            <p class="text-sm text-slate-500 leading-relaxed">
              配置已保存到数据库，<b>重启后生效</b>。<br />
              插件、MCP 服务器、Prompt 覆盖等更多配置可在「配置管理」与「扩展配置」页继续完善。
            </p>
          </template>
          <template v-else>
            <h1 class="text-xl font-bold text-slate-800">正在重启 Bot...</h1>
            <p class="text-sm text-slate-500">恢复后页面自动刷新</p>
          </template>
        </div>

        <p v-if="error" class="text-sm text-red-600 mt-4">{{ error }}</p>

        <!-- 操作按钮 -->
        <div class="flex justify-between mt-8" v-if="!restarting">
          <button v-if="step > 0 && step < 3" class="px-4 py-2 text-sm text-slate-500 hover:bg-slate-100 rounded" @click="step--">上一步</button>
          <button v-else class="px-4 py-2 text-sm text-slate-400 hover:text-slate-600" @click="onSkip">跳过引导</button>

          <button v-if="step < 2" class="px-5 py-2 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700" @click="step++">下一步</button>
          <button v-else-if="step === 2" class="px-5 py-2 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700" @click="onSave">保存并继续</button>
          <button v-else class="px-5 py-2 text-sm bg-emerald-600 text-white rounded hover:bg-emerald-700" @click="onRestart">重启 Bot 生效</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api, auth } from '../api.js'

const inputClass = 'w-full border border-slate-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400'

const step = ref(0)
const error = ref('')
const restarting = ref(false)
const form = reactive({
  wsAddress: '',
  token: '',
  adminId: '',
  baseUrl: '',
  apiKey: '',
  model: '',
})

// 预填当前默认/已迁移值
onMounted(async () => {
  try {
    const cfg = await api.getConfig()
    form.wsAddress = cfg['bot.adapter.ws.address'] || ''
    form.baseUrl = cfg['plugin.ai_chat_bot.base_url'] || ''
    form.model = cfg['plugin.ai_chat_bot.model'] || ''
    const adminId = cfg['bot.admin_id']
    if (adminId) form.adminId = String(adminId)
  } catch { /* 忽略，使用空表单 */ }
})

async function onSave() {
  error.value = ''
  const updates = {}
  if (form.wsAddress.trim()) updates['bot.adapter.ws.address'] = form.wsAddress.trim()
  if (form.token.trim()) updates['bot.adapter.token'] = form.token.trim()
  const adminId = parseInt(form.adminId, 10)
  if (!Number.isNaN(adminId) && adminId > 0) updates['bot.admin_id'] = adminId
  if (form.baseUrl.trim()) updates['plugin.ai_chat_bot.base_url'] = form.baseUrl.trim()
  if (form.apiKey.trim()) updates['plugin.ai_chat_bot.api_key'] = form.apiKey.trim()
  if (form.model.trim()) updates['plugin.ai_chat_bot.model'] = form.model.trim()

  try {
    if (Object.keys(updates).length > 0) {
      await api.saveConfig(updates)
    }
    await api.completeSetup()
    step.value = 3
  } catch (e) {
    error.value = e.message
  }
}

async function onSkip() {
  try { await api.completeSetup() } catch { /* 忽略 */ }
  auth.setupRequired = false
}

async function onRestart() {
  restarting.value = true
  try {
    await api.restart()
  } catch { /* 进程可能已开始退出 */ }
  auth.setupRequired = false
  const up = await api.waitUntilUp()
  if (up) {
    location.reload()
  } else {
    restarting.value = false
    error.value = '等待重启超时，请检查 Bot 运行状态'
  }
}
</script>
