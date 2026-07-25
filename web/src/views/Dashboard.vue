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

    <!-- AI 定时任务 -->
    <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
      <div class="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
        <h2 class="text-sm font-semibold text-slate-800">AI 定时任务</h2>
        <div class="flex items-center gap-3">
          <button class="text-xs text-zinc-700 hover:text-zinc-900 font-medium transition-colors" @click="loadClocks">刷新</button>
          <button class="text-xs bg-zinc-800 text-white px-2.5 py-1 rounded-md hover:bg-zinc-700 font-medium transition-colors" @click="openCreate">新建任务</button>
        </div>
      </div>
      <p v-if="clocks.length === 0" class="px-6 py-8 text-sm text-slate-400 text-center">暂无定时任务，点击右上角「新建任务」创建（也可在群聊/私聊中使用 /clock）</p>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-400 bg-slate-50/60 border-b border-slate-100">
            <th class="px-6 py-3 font-medium">任务</th>
            <th class="px-6 py-3 font-medium">目标</th>
            <th class="px-6 py-3 font-medium">Cron</th>
            <th class="px-6 py-3 font-medium">下次执行</th>
            <th class="px-6 py-3 font-medium">上次执行</th>
            <th class="px-6 py-3 font-medium">启用</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="t in clocks" :key="t.id">
            <tr
              class="border-b border-slate-50 last:border-0 hover:bg-slate-50/70 transition-colors cursor-pointer"
              :class="{ 'bg-slate-50/70': expanded.has(t.id) }"
              @click="toggleExpand(t.id)"
            >
              <td class="px-6 py-3 text-slate-700 max-w-48">
                <span class="flex items-center gap-1.5">
                  <span
                    class="[&>svg]:w-3.5 [&>svg]:h-3.5 text-slate-400 transition-transform shrink-0"
                    :class="{ 'rotate-90': expanded.has(t.id) }"
                    v-html="icons.chevron"
                  />
                  <span class="truncate" :title="t.title">{{ t.title || '(无标题)' }}</span>
                </span>
                <span v-if="t.run_once" class="text-xs bg-zinc-100 text-zinc-500 px-1.5 py-0.5 rounded ml-5">单次</span>
              </td>
              <td class="px-6 py-3 text-slate-600 whitespace-nowrap">{{ t.target_type === 'group' ? '群' : '好友' }} {{ t.target_id }}</td>
              <td class="px-6 py-3 text-slate-500 font-mono text-xs whitespace-nowrap">{{ t.cron }}</td>
              <td class="px-6 py-3 text-slate-600 whitespace-nowrap">{{ t.enabled ? fmtTime(t.next_run_at) : '-' }}</td>
              <td class="px-6 py-3 text-slate-600 whitespace-nowrap">{{ fmtTime(t.last_run_at) }}</td>
              <td class="px-6 py-3" @click.stop>
                <button
                  type="button"
                  role="switch"
                  :aria-checked="t.enabled"
                  :disabled="toggling.has(t.id)"
                  class="relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors disabled:opacity-50"
                  :class="t.enabled ? 'bg-zinc-800' : 'bg-zinc-200'"
                  @click="toggleClock(t)"
                >
                  <span
                    class="inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform"
                    :class="t.enabled ? 'translate-x-[1.125rem]' : 'translate-x-[0.1875rem]'"
                  />
                </button>
              </td>
            </tr>
            <!-- 任务详情 -->
            <tr v-if="expanded.has(t.id)" class="border-b border-slate-100 last:border-0 bg-slate-50/40">
              <td colspan="6" class="px-6 py-4">
                <dl class="grid grid-cols-1 sm:grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
                  <dt class="text-slate-400">任务内容</dt>
                  <dd class="text-slate-700 whitespace-pre-wrap break-all">{{ t.content }}</dd>
                  <template v-if="t.note">
                    <dt class="text-slate-400">备注</dt>
                    <dd class="text-slate-700 whitespace-pre-wrap break-all">{{ t.note }}</dd>
                  </template>
                  <dt class="text-slate-400">超时时间</dt>
                  <dd class="text-slate-700">{{ t.timeout_sec > 0 ? t.timeout_sec + ' 秒' : '默认' }}</dd>
                  <dt class="text-slate-400">创建者</dt>
                  <dd class="text-slate-700">{{ t.created_by ? 'QQ ' + t.created_by : '-' }}</dd>
                  <dt class="text-slate-400">创建时间</dt>
                  <dd class="text-slate-700">{{ fmtTime(t.created_at) }}</dd>
                </dl>
                <div class="mt-4 flex items-center gap-2">
                  <button
                    class="text-xs bg-zinc-800 text-white px-3 py-1.5 rounded-md hover:bg-zinc-700 font-medium transition-colors disabled:opacity-50"
                    :disabled="toggling.has(t.id)"
                    @click="openEdit(t)"
                  >编辑</button>
                  <button
                    class="text-xs border border-zinc-200 text-zinc-600 px-3 py-1.5 rounded-md hover:bg-zinc-100 hover:text-red-600 hover:border-red-200 font-medium transition-colors disabled:opacity-50"
                    :disabled="toggling.has(t.id)"
                    @click="removeClock(t)"
                  >删除</button>
                </div>
              </td>
            </tr>
          </template>
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

    <!-- 新建 / 编辑定时任务弹窗 -->
    <Teleport to="body">
      <div v-if="clockForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" @click.self="clockForm = null">
        <div class="bg-white rounded-xl shadow-xl border border-zinc-200 w-full max-w-lg max-h-[90vh] overflow-y-auto">
          <div class="px-6 py-4 border-b border-zinc-100 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-zinc-800">{{ clockForm.id ? '编辑定时任务' : '新建定时任务' }}</h3>
            <button class="text-zinc-400 hover:text-zinc-600 transition-colors" @click="clockForm = null">✕</button>
          </div>
          <form class="px-6 py-5 space-y-4" @submit.prevent="saveClock">
            <div>
              <label class="form-label">任务标题</label>
              <input v-model.trim="clockForm.title" type="text" class="form-input" placeholder="如：每日晨报" />
            </div>
            <div>
              <label class="form-label">任务内容 <span class="text-red-500">*</span></label>
              <textarea v-model.trim="clockForm.content" rows="3" class="form-input" placeholder="触发时发送给 AI 的内容"></textarea>
            </div>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="form-label">Cron 表达式 <span class="text-red-500">*</span></label>
                <input v-model.trim="clockForm.cron" type="text" class="form-input font-mono" placeholder="0 8 * * * 或 @every 1h" />
              </div>
              <div>
                <label class="form-label">超时时间（秒，0 为默认）</label>
                <input v-model.number="clockForm.timeout_sec" type="number" min="0" class="form-input" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="form-label">触发对象 <span class="text-red-500">*</span></label>
                <select v-model="clockForm.target_type" class="form-input" :disabled="!!clockForm.id">
                  <option value="group">群聊</option>
                  <option value="friend">好友</option>
                </select>
              </div>
              <div>
                <label class="form-label">{{ clockForm.target_type === 'group' ? '群号' : 'QQ 号' }} <span class="text-red-500">*</span></label>
                <input v-model.trim="clockForm.target_id" type="text" class="form-input" :disabled="!!clockForm.id" placeholder="数字" />
              </div>
            </div>
            <div>
              <label class="form-label">备注</label>
              <input v-model.trim="clockForm.note" type="text" class="form-input" placeholder="可选，触发时附带给 AI" />
            </div>
            <label class="flex items-center gap-2 text-sm text-zinc-700 select-none">
              <input v-model="clockForm.run_once" type="checkbox" class="accent-zinc-800" :disabled="!!clockForm.id" />
              单次任务（触发一次后自动删除）
            </label>
            <p v-if="clockFormError" class="text-sm text-red-600">{{ clockFormError }}</p>
            <div class="flex justify-end gap-2 pt-1">
              <button type="button" class="px-4 py-2 text-sm text-zinc-600 hover:text-zinc-800 font-medium transition-colors" @click="clockForm = null">取消</button>
              <button type="submit" class="px-4 py-2 text-sm bg-zinc-800 text-white rounded-md hover:bg-zinc-700 font-medium transition-colors disabled:opacity-50" :disabled="clockSaving">
                {{ clockSaving ? '保存中…' : '保存' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
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
  chevron: '<svg fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5"/></svg>',
}

const status = ref({})
const plugins = ref([])
const logs = ref([])
const clocks = ref([])
const toggling = ref(new Set())
const expanded = ref(new Set())
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
  const d = new Date(t)
  if (isNaN(d) || d.getFullYear() < 2000) return '-' // Go 零值时间
  return d.toLocaleString('zh-CN', { hour12: false })
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

async function loadClocks() {
  try { clocks.value = await api.getClocks() } catch { /* 忽略 */ }
}

function toggleExpand(id) {
  const s = new Set(expanded.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  expanded.value = s
}

// 乐观更新开关状态，失败时回滚
async function toggleClock(t) {
  if (toggling.value.has(t.id)) return
  toggling.value = new Set(toggling.value).add(t.id)
  const prev = t.enabled
  t.enabled = !prev
  try {
    await api.updateClock(t.id, { enabled: t.enabled })
  } catch (e) {
    t.enabled = prev
    alert(e.message || '操作失败')
  } finally {
    const s = new Set(toggling.value)
    s.delete(t.id)
    toggling.value = s
  }
}

// ---- 新建 / 编辑 / 删除 ----

const clockForm = ref(null) // 非 null 时显示弹窗；id 为空表示新建
const clockFormError = ref('')
const clockSaving = ref(false)

function blankClockForm() {
  return {
    id: '',
    title: '',
    content: '',
    cron: '',
    target_type: 'group',
    target_id: '',
    timeout_sec: 0,
    note: '',
    run_once: false,
  }
}

function openCreate() {
  clockFormError.value = ''
  clockForm.value = blankClockForm()
}

function openEdit(t) {
  clockFormError.value = ''
  clockForm.value = {
    id: t.id,
    title: t.title,
    content: t.content,
    cron: t.cron,
    target_type: t.target_type,
    target_id: t.target_id,
    timeout_sec: t.timeout_sec || 0,
    note: t.note || '',
    run_once: t.run_once,
  }
}

async function saveClock() {
  const f = clockForm.value
  if (!f.content) { clockFormError.value = '任务内容不能为空'; return }
  if (!f.cron) { clockFormError.value = 'Cron 表达式不能为空'; return }
  if (!f.id) {
    if (!f.target_id) { clockFormError.value = '目标 ID 不能为空'; return }
    if (!/^\d+$/.test(f.target_id)) { clockFormError.value = '目标 ID 必须是数字'; return }
  }
  clockFormError.value = ''
  clockSaving.value = true
  try {
    if (f.id) {
      await api.updateClock(f.id, {
        title: f.title,
        content: f.content,
        cron: f.cron,
        note: f.note,
        timeout_sec: f.timeout_sec || 0,
      })
    } else {
      await api.createClock({
        title: f.title,
        content: f.content,
        cron: f.cron,
        target_type: f.target_type,
        target_id: f.target_id,
        enabled: true,
        run_once: f.run_once,
        timeout_sec: f.timeout_sec || 0,
        note: f.note,
      })
    }
    clockForm.value = null
    await loadClocks()
  } catch (e) {
    clockFormError.value = e.message || '保存失败'
  } finally {
    clockSaving.value = false
  }
}

async function removeClock(t) {
  if (!confirm(`确定删除定时任务「${t.title || t.id}」吗？`)) return
  toggling.value = new Set(toggling.value).add(t.id)
  try {
    await api.deleteClock(t.id)
    await loadClocks()
  } catch (e) {
    alert(e.message || '删除失败')
  } finally {
    const s = new Set(toggling.value)
    s.delete(t.id)
    toggling.value = s
  }
}

onMounted(async () => {
  loadStatus()
  loadLogs()
  loadClocks()
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
.form-label {
  display: block;
  font-size: 0.75rem;
  color: rgb(113 113 122);
  margin-bottom: 0.375rem;
}
.form-input {
  width: 100%;
  border: 1px solid rgb(228 228 231);
  border-radius: 0.5rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: rgb(39 39 42);
  outline: none;
  transition: border-color 0.15s;
  background: white;
}
.form-input:focus {
  border-color: rgb(113 113 122);
}
.form-input:disabled {
  background: rgb(244 244 245);
  color: rgb(161 161 170);
}
</style>
