<template>
  <div class="space-y-4">
    <!-- 筛选与操作栏 -->
    <div class="flex items-center justify-between flex-wrap gap-3">
      <div class="flex items-center gap-1 bg-white border border-slate-200/60 rounded-lg p-1 shadow-sm">
        <button
          v-for="t in typeTabs"
          :key="t.value"
          class="px-3 py-1.5 text-xs rounded-md transition-all"
          :class="filter === t.value
            ? 'bg-zinc-900 text-white font-medium shadow-sm'
            : 'text-slate-500 hover:text-slate-800 hover:bg-slate-100'"
          @click="filter = t.value"
        >
          {{ t.label }}
        </button>
      </div>
      <div class="flex items-center gap-3">
        <label class="flex items-center gap-1.5 text-xs text-slate-500 select-none cursor-pointer">
          <input v-model="autoRefresh" type="checkbox" class="accent-zinc-800" />
          自动刷新
        </label>
        <button class="text-xs text-zinc-700 hover:text-zinc-900 font-medium transition-colors" @click="load">刷新</button>
      </div>
    </div>

    <!-- 日志列表（旧在上、新在下，自动滚到底部） -->
    <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
      <ul ref="listEl" class="h-[60vh] overflow-y-auto px-5 py-3 space-y-3">
        <li v-if="filtered.length === 0" class="py-12 text-sm text-slate-400 text-center list-none">
          暂无消息记录（日志保存在内存中，重启后清空，最多保留最近 500 条）
        </li>
        <li v-for="log in filtered" :key="log.id" class="flex items-start gap-3">
          <span class="text-xs text-slate-400 font-mono whitespace-nowrap pt-0.5 w-16 shrink-0">{{ fmtTime(log.time) }}</span>
          <span class="text-xs px-2 py-0.5 rounded-full whitespace-nowrap shrink-0" :class="typeClass(log.type)">
            {{ typeText(log) }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="text-xs text-slate-400 mb-0.5">
              <template v-if="log.type === 'notice'">
                <span class="text-zinc-600 font-medium">{{ log.title }}</span>
                <template v-if="log.group_id"> · 群 {{ log.group_id }}</template>
              </template>
              <template v-else>
                <span v-if="log.nickname" class="text-zinc-600 font-medium">{{ log.nickname }}</span>
                <span v-if="log.user_id"> ({{ log.user_id }})</span>
                <template v-if="log.group_id"> · 群 {{ log.group_id }}</template>
              </template>
            </div>
            <p class="text-sm text-slate-700 whitespace-pre-wrap break-all leading-relaxed">{{ log.text }}</p>
          </div>
        </li>
      </ul>

      <!-- 有新消息提示（用户上翻查看历史时） -->
      <div v-if="hasNew" class="border-t border-slate-100 px-5 py-2 flex justify-center">
        <button
          class="text-xs bg-zinc-900 text-white px-3 py-1.5 rounded-full hover:bg-zinc-700 font-medium transition-colors shadow-sm"
          @click="scrollToBottom(true)"
        >
          ↓ 有新消息，回到底部
        </button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api.js'

const typeTabs = [
  { value: 'all', label: '全部' },
  { value: 'group', label: '群消息' },
  { value: 'friend', label: '好友消息' },
  { value: 'notice', label: '通知' },
]

const logs = ref([])
const filter = ref('all')
const autoRefresh = ref(true)
const listEl = ref(null)
const hasNew = ref(false)
let timer = null

// 接口返回新在前，展示时翻转为旧在上、新在下
const filtered = computed(() => {
  const list = filter.value === 'all' ? logs.value : logs.value.filter((l) => l.type === filter.value)
  return [...list].reverse()
})

function fmtTime(t) {
  if (!t) return '-'
  const d = new Date(t)
  if (isNaN(d)) return '-'
  const now = new Date()
  const hm = d.toLocaleTimeString('zh-CN', { hour12: false })
  if (d.toDateString() === now.toDateString()) return hm
  return `${d.getMonth() + 1}/${d.getDate()} ${hm}`
}

function typeText(log) {
  return { group: '群消息', friend: '好友', notice: '通知' }[log.type] || log.type
}

function typeClass(type) {
  return {
    group: 'bg-zinc-900 text-white',
    friend: 'bg-zinc-100 text-zinc-700 border border-zinc-200',
    notice: 'bg-white text-zinc-500 border border-zinc-300',
  }[type] || 'bg-slate-100 text-slate-600'
}

function nearBottom() {
  const el = listEl.value
  if (!el) return true
  return el.scrollHeight - el.scrollTop - el.clientHeight < 60
}

function scrollToBottom(smooth = false) {
  const el = listEl.value
  if (!el) return
  el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
  hasNew.value = false
}

async function load() {
  const stick = nearBottom()
  const prevLatest = logs.value[0]?.id
  try { logs.value = await api.getMsgLogs() } catch { return }
  const gotNew = logs.value[0]?.id !== prevLatest
  if (!gotNew) return
  if (stick) {
    await nextTick()
    scrollToBottom()
  } else {
    hasNew.value = true
  }
}

// 实时刷新：标签页隐藏时暂停，恢复可见时立即刷新
function onVisible() {
  if (!document.hidden && autoRefresh.value) load()
}

onMounted(async () => {
  await load()
  await nextTick()
  scrollToBottom()
  timer = setInterval(() => { if (!document.hidden && autoRefresh.value) load() }, 3000)
  document.addEventListener('visibilitychange', onVisible)
})

onUnmounted(() => {
  clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisible)
})
</script>
