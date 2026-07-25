<template>
  <div class="space-y-6">
    <!-- 适配器未连接提示 -->
    <div
      v-if="status.adapter_status && status.adapter_status !== 'connected'"
      class="bg-amber-50 border border-amber-200 text-amber-700 rounded-xl px-5 py-3.5 text-sm flex items-center justify-between gap-4"
    >
      <span class="flex items-center gap-2">
        <span class="[&>svg]:w-5 [&>svg]:h-5 text-amber-500" v-html="icons.warn" />
        <span>
          NapCat 未连接<template v-if="status.adapter_detail">：{{ status.adapter_detail }}</template>。
          Bot 会持续重试，不会退出。
        </span>
      </span>
      <RouterLink to="/config" class="text-zinc-700 hover:underline shrink-0 font-medium">修改配置并重启 →</RouterLink>
    </div>

    <!-- 状态卡片 -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="stat-card">
        <div class="stat-icon bg-zinc-100 text-zinc-700" v-html="icons.clock" />
        <div>
          <p class="text-xs text-slate-500">运行时长</p>
          <p class="text-xl font-semibold text-slate-800 mt-0.5 whitespace-nowrap">{{ uptime }}</p>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" :class="status.adapter_status === 'connected' ? 'bg-emerald-50 text-emerald-600' : 'bg-amber-50 text-amber-600'" v-html="icons.link" />
        <div class="min-w-0">
          <p class="text-xs text-slate-500">适配器状态</p>
          <p class="text-xl font-semibold mt-0.5" :class="status.adapter_status === 'connected' ? 'text-emerald-600' : 'text-amber-600'">
            {{ adapterText }}
          </p>
          <p v-if="status.adapter_detail" class="text-xs text-slate-400 truncate" :title="status.adapter_detail">
            {{ status.adapter_detail }}
          </p>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon bg-zinc-100 text-zinc-600" v-html="icons.puzzle" />
        <div>
          <p class="text-xs text-slate-500">插件数量</p>
          <p class="text-xl font-semibold text-slate-800 mt-0.5">{{ status.plugin_count ?? '-' }}</p>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon bg-zinc-100 text-zinc-600" v-html="icons.cpu" />
        <div>
          <p class="text-xs text-slate-500">Goroutine</p>
          <p class="text-xl font-semibold text-slate-800 mt-0.5">{{ status.goroutines ?? '-' }}</p>
        </div>
      </div>
    </div>

    <!-- 插件列表 -->
    <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
      <h2 class="px-6 py-4 text-sm font-semibold text-slate-800 border-b border-slate-100">插件列表</h2>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-400 bg-slate-50/60 border-b border-slate-100">
            <th class="px-6 py-3 font-medium">名称</th>
            <th class="px-6 py-3 font-medium">说明</th>
            <th class="px-6 py-3 font-medium">作者</th>
            <th class="px-6 py-3 font-medium">版本</th>
            <th class="px-6 py-3 font-medium">仅管理员</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in plugins" :key="p.name" class="border-b border-slate-50 last:border-0 hover:bg-slate-50/70 transition-colors">
            <td class="px-6 py-3 font-medium text-slate-700">{{ p.name }}</td>
            <td class="px-6 py-3 text-slate-600">{{ p.help_words }}</td>
            <td class="px-6 py-3 text-slate-600">{{ p.author }}</td>
            <td class="px-6 py-3 text-slate-500 font-mono text-xs">{{ p.version }}</td>
            <td class="px-6 py-3">
              <span v-if="p.admin_only" class="text-xs bg-amber-100 text-amber-700 px-2 py-0.5 rounded-full">是</span>
              <span v-else class="text-xs text-slate-400">否</span>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <!-- 定时任务执行日志 -->
    <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
      <div class="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
        <h2 class="text-sm font-semibold text-slate-800">AI 定时任务执行日志</h2>
        <button class="text-xs text-zinc-700 hover:text-zinc-900 font-medium transition-colors" @click="loadLogs">刷新</button>
      </div>
      <p v-if="logs.length === 0" class="px-6 py-8 text-sm text-slate-400 text-center">暂无执行记录</p>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-400 bg-slate-50/60 border-b border-slate-100">
            <th class="px-6 py-3 font-medium">任务</th>
            <th class="px-6 py-3 font-medium">目标</th>
            <th class="px-6 py-3 font-medium">触发时间</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">耗时</th>
            <th class="px-6 py-3 font-medium">Token</th>
            <th class="px-6 py-3 font-medium">错误</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in logs" :key="log.id" class="border-b border-slate-50 last:border-0 hover:bg-slate-50/70 transition-colors">
            <td class="px-6 py-3 text-slate-700 max-w-48 truncate" :title="log.task_title">{{ log.task_title }}</td>
            <td class="px-6 py-3 text-slate-600">{{ log.target_type === 'group' ? '群' : '好友' }} {{ log.target_id }}</td>
            <td class="px-6 py-3 text-slate-600 whitespace-nowrap">{{ fmtTime(log.trigger_time) }}</td>
            <td class="px-6 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full" :class="statusClass(log.status)">{{ statusText(log.status) }}</span>
            </td>
            <td class="px-6 py-3 text-slate-600">{{ log.duration_ms ? (log.duration_ms / 1000).toFixed(1) + 's' : '-' }}</td>
            <td class="px-6 py-3 text-slate-600">{{ log.total_tokens || '-' }}</td>
            <td class="px-6 py-3 text-red-600 max-w-40 truncate" :title="log.error">{{ log.error || '' }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api.js'

const icons = {
  clock: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/></svg>',
  link: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244"/></svg>',
  puzzle: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M14.25 6.087c0-.355.186-.676.401-.959.221-.29.349-.634.349-1.003 0-1.036-1.007-1.875-2.25-1.875s-2.25.84-2.25 1.875c0 .369.128.713.349 1.003.215.283.401.604.401.959v.431c0 .46-.335.84-.782.927a7.59 7.59 0 0 1-1.181.093h-.77c-.254 0-.487.09-.668.24-.297.246-.451.619-.371 1.014.073.361.026.74-.145 1.086-.199.402-.576.65-1.007.65H7.5c-.621 0-1.125.504-1.125 1.125v.77c0 .418.314.82.77 1.118.198.13.37.305.48.515.16.308.165.674.014.97-.168.333-.502.521-.864.521H4.875A1.875 1.875 0 0 1 3 14.25v-1.77c0-.358-.215-.68-.543-.822A1.87 1.87 0 0 0 1.875 9.75c0-1.243 1.007-2.25 2.25-2.25.369 0 .713.128 1.003.349.283.215.604.401.959.401h.413"/></svg>',
  cpu: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M8.25 3v1.5M4.5 8.25H3m18 0h-1.5M4.5 12H3m18 0h-1.5m-15 3.75H3m18 0h-1.5M8.25 19.5V21M12 3v1.5m0 15V21m3.75-18v1.5m0 15V21m-9-1.5h10.5a2.25 2.25 0 0 0 2.25-2.25V6.75a2.25 2.25 0 0 0-2.25-2.25H6.75A2.25 2.25 0 0 0 4.5 6.75v10.5a2.25 2.25 0 0 0 2.25 2.25Zm.75-12h9v9h-9v-9Z"/></svg>',
  warn: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"/></svg>',
}

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
  connecting: '连接中',
  reconnecting: '重连中',
  setup_pending: '等待首次配置',
  not_started: '未连接',
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

<style scoped>
.stat-card {
  display: flex;
  align-items: flex-start;
  gap: 0.875rem;
  background: white;
  border-radius: 0.75rem;
  border: 1px solid rgb(226 232 240 / 0.6);
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.04);
  padding: 1.25rem;
}
.stat-icon {
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 0.625rem;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.stat-icon :deep(svg) {
  width: 1.25rem;
  height: 1.25rem;
}
</style>
