<template>
  <div class="space-y-6">
    <!-- 状态卡片 -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-white rounded-lg shadow-sm p-5">
        <p class="text-xs text-slate-500">运行时长</p>
        <p class="text-xl font-semibold text-slate-800 mt-1">{{ uptime }}</p>
      </div>
      <div class="bg-white rounded-lg shadow-sm p-5">
        <p class="text-xs text-slate-500">适配器状态</p>
        <p class="text-xl font-semibold mt-1" :class="status.adapter_status === 'connected' ? 'text-emerald-600' : 'text-amber-600'">
          {{ adapterText }}
        </p>
      </div>
      <div class="bg-white rounded-lg shadow-sm p-5">
        <p class="text-xs text-slate-500">插件数量</p>
        <p class="text-xl font-semibold text-slate-800 mt-1">{{ status.plugin_count ?? '-' }}</p>
      </div>
      <div class="bg-white rounded-lg shadow-sm p-5">
        <p class="text-xs text-slate-500">Goroutine</p>
        <p class="text-xl font-semibold text-slate-800 mt-1">{{ status.goroutines ?? '-' }}</p>
      </div>
    </div>

    <!-- 插件列表 -->
    <section class="bg-white rounded-lg shadow-sm">
      <h2 class="px-5 py-4 text-sm font-semibold text-slate-700 border-b border-slate-100">插件列表</h2>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-500 border-b border-slate-100">
            <th class="px-5 py-2.5 font-medium">名称</th>
            <th class="px-5 py-2.5 font-medium">说明</th>
            <th class="px-5 py-2.5 font-medium">作者</th>
            <th class="px-5 py-2.5 font-medium">版本</th>
            <th class="px-5 py-2.5 font-medium">仅管理员</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in plugins" :key="p.name" class="border-b border-slate-50 hover:bg-slate-50">
            <td class="px-5 py-2.5 font-medium text-slate-700">{{ p.name }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ p.help_words }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ p.author }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ p.version }}</td>
            <td class="px-5 py-2.5">
              <span v-if="p.admin_only" class="text-xs bg-amber-100 text-amber-700 px-2 py-0.5 rounded">是</span>
              <span v-else class="text-xs text-slate-400">否</span>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <!-- 定时任务执行日志 -->
    <section class="bg-white rounded-lg shadow-sm">
      <div class="px-5 py-4 border-b border-slate-100 flex items-center justify-between">
        <h2 class="text-sm font-semibold text-slate-700">AI 定时任务执行日志</h2>
        <button class="text-xs text-indigo-600 hover:underline" @click="loadLogs">刷新</button>
      </div>
      <p v-if="logs.length === 0" class="px-5 py-6 text-sm text-slate-400">暂无执行记录</p>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-500 border-b border-slate-100">
            <th class="px-5 py-2.5 font-medium">任务</th>
            <th class="px-5 py-2.5 font-medium">目标</th>
            <th class="px-5 py-2.5 font-medium">触发时间</th>
            <th class="px-5 py-2.5 font-medium">状态</th>
            <th class="px-5 py-2.5 font-medium">耗时</th>
            <th class="px-5 py-2.5 font-medium">Token</th>
            <th class="px-5 py-2.5 font-medium">错误</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in logs" :key="log.id" class="border-b border-slate-50 hover:bg-slate-50">
            <td class="px-5 py-2.5 text-slate-700 max-w-48 truncate" :title="log.task_title">{{ log.task_title }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ log.target_type === 'group' ? '群' : '好友' }} {{ log.target_id }}</td>
            <td class="px-5 py-2.5 text-slate-600 whitespace-nowrap">{{ fmtTime(log.trigger_time) }}</td>
            <td class="px-5 py-2.5">
              <span class="text-xs px-2 py-0.5 rounded" :class="statusClass(log.status)">{{ statusText(log.status) }}</span>
            </td>
            <td class="px-5 py-2.5 text-slate-600">{{ log.duration_ms ? (log.duration_ms / 1000).toFixed(1) + 's' : '-' }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ log.total_tokens || '-' }}</td>
            <td class="px-5 py-2.5 text-red-600 max-w-40 truncate" :title="log.error">{{ log.error || '' }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api.js'

const status = ref({})
const plugins = ref([])
const logs = ref([])
let timer = null

const uptime = computed(() => {
  const s = status.value.uptime_sec
  if (s == null) return '-'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d > 0) return `${d}天 ${h}小时`
  if (h > 0) return `${h}小时 ${m}分`
  return `${m}分 ${s % 60}秒`
})

const adapterText = computed(() => ({
  connected: '已连接',
  reconnecting: '重连中',
  unknown: '未知',
}[status.value.adapter_status] || status.value.adapter_status || '-'))

function fmtTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

function statusText(s) {
  return { running: '执行中', success: '成功', timeout: '超时', error: '失败' }[s] || s
}

function statusClass(s) {
  return {
    running: 'bg-blue-100 text-blue-700',
    success: 'bg-emerald-100 text-emerald-700',
    timeout: 'bg-amber-100 text-amber-700',
    error: 'bg-red-100 text-red-700',
  }[s] || 'bg-slate-100 text-slate-600'
}

async function loadStatus() {
  try { status.value = await api.getStatus() } catch { /* 忽略轮询错误 */ }
}

async function loadLogs() {
  try { logs.value = await api.getTaskLogs() } catch { /* 忽略 */ }
}

onMounted(async () => {
  loadStatus()
  loadLogs()
  try { plugins.value = await api.getPlugins() } catch { /* 忽略 */ }
  timer = setInterval(loadStatus, 10000)
})

onUnmounted(() => clearInterval(timer))
</script>
