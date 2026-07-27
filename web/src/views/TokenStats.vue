<template>
  <div class="space-y-4">
    <!-- 顶部统计卡 -->
    <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <!-- TOTAL -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Total Tokens</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />{{ summary.requests ?? 0 }} Runs</span>
        </div>
        <div class="flex-1 py-4">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">{{ fmtTokens(summary.total_tokens) }}</div>
          <div class="tlabel mt-2">历史留存累计</div>
        </div>
        <div class="border-t border-dotted border-zinc-300 pt-2.5 flex justify-between text-[10px] tracking-[0.12em] uppercase text-zinc-500">
          <span>Prompt {{ fmtTokens(summary.prompt_tokens) }}</span>
          <span>Completion {{ fmtTokens(summary.completion_tokens) }}</span>
        </div>
      </section>

      <!-- TODAY -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Today</span>
          <span class="tpill"><span class="tdot bg-emerald-500" />{{ today.requests ?? 0 }} Runs</span>
        </div>
        <div class="flex-1 py-4">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">{{ fmtTokens(today.total_tokens) }}</div>
          <div class="tlabel mt-2">今日消耗</div>
        </div>
        <div class="border-t border-dotted border-zinc-300 pt-2.5 flex justify-between text-[10px] tracking-[0.12em] uppercase text-zinc-500">
          <span>Prompt {{ fmtTokens(today.prompt_tokens) }}</span>
          <span>Completion {{ fmtTokens(today.completion_tokens) }}</span>
        </div>
      </section>

      <!-- CACHE -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Cache Hit</span>
          <span class="tpill">
            <span class="tdot" :class="summary.cache_hit_rate > 0 ? 'bg-emerald-500' : 'bg-zinc-300'" />
            {{ summary.cache_hit_rate > 0 ? 'Active' : 'N/A' }}
          </span>
        </div>
        <div class="flex-1 py-4">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">{{ pctText(summary.cache_hit_rate) }}</div>
          <div class="tlabel mt-2">Cached / Prompt</div>
        </div>
        <div class="border-t border-dotted border-zinc-300 pt-2.5 text-[10px] tracking-[0.12em] uppercase text-zinc-500">
          命中 {{ fmtTokens(summary.cached_tokens) }} tokens
        </div>
      </section>

      <!-- AVERAGE -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Average</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />Per Run</span>
        </div>
        <div class="flex-1 py-4">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">{{ fmtTokens(Math.round(summary.avg_total_tokens || 0)) }}</div>
          <div class="tlabel mt-2">单次平均消耗</div>
        </div>
        <div class="border-t border-dotted border-zinc-300 pt-2.5 text-[10px] tracking-[0.12em] uppercase text-zinc-500">
          平均 {{ (detail.avg_iterations || 0).toFixed(1) }} 轮 LLM 调用 / 次
        </div>
      </section>
    </div>

    <!-- 30 天分来源序列 -->
    <section class="tcard p-6">
      <div class="flex items-center justify-between">
        <span class="tlabel">Daily Tokens · By Source</span>
        <div class="flex items-center gap-3">
          <span class="flex items-center gap-1.5 text-[10px] tracking-[0.12em] uppercase text-zinc-400">
            <i class="w-2 h-2 bg-zinc-700 inline-block rounded-[1px]" />对话
            <i class="w-2 h-2 bg-zinc-300 inline-block rounded-[1px] ml-2" />定时任务
          </span>
          <button class="text-[10px] tracking-[0.15em] uppercase text-zinc-500 hover:text-zinc-900 font-medium transition-colors" @click="load">刷新</button>
        </div>
      </div>

      <div class="flex items-end gap-[3px] pt-5 pb-1 h-40">
        <div v-for="d in daily" :key="d.date" class="flex-1 flex flex-col justify-end h-full min-w-0" :title="dayTip(d)">
          <div class="w-full bg-zinc-700" :style="{ height: barH(d.query?.total_tokens) }" />
          <div class="w-full bg-zinc-300 rounded-t-sm" :style="{ height: barH(d.task?.total_tokens) }" />
        </div>
      </div>
      <div class="flex items-center justify-between mt-2 text-[10px] tracking-[0.12em] uppercase text-zinc-400">
        <span>{{ daily[0]?.date?.slice(5) || '—' }}</span>
        <span>30 Days</span>
        <span>{{ daily[daily.length - 1]?.date?.slice(5) || '—' }}</span>
      </div>
      <div class="dotline my-3" />
      <div class="text-[10px] tracking-[0.14em] uppercase text-zinc-500 truncate">
        Source: query &amp; cron logs (retained, capped)
        <span class="mx-2 text-zinc-300">//</span>
        Cache metrics depend on upstream API
      </div>
    </section>

    <!-- 拆分维度 -->
    <div class="grid grid-cols-1 xl:grid-cols-3 gap-4">
      <!-- BY SOURCE -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">By Source</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />对话 / 任务</span>
        </div>
        <div class="flex h-3 rounded-sm overflow-hidden mt-5 bg-zinc-100">
          <div class="bg-zinc-800" :style="{ width: shareW(bySource.query?.total_tokens, bySource.task?.total_tokens) }" />
          <div class="bg-zinc-300" :style="{ width: shareW(bySource.task?.total_tokens, bySource.query?.total_tokens) }" />
        </div>
        <dl class="flex-1 mt-3 text-xs">
          <div class="dimrow">
            <dt class="tlabel">对话 Query</dt>
            <dd class="dimval">{{ fmtTokens(bySource.query?.total_tokens) }} <span class="text-zinc-400 font-normal">/ {{ bySource.query?.requests ?? 0 }} runs · avg {{ fmtTokens(Math.round(bySource.query?.avg_total_tokens || 0)) }}</span></dd>
          </div>
          <div class="dimrow">
            <dt class="tlabel">定时任务 Cron</dt>
            <dd class="dimval">{{ fmtTokens(bySource.task?.total_tokens) }} <span class="text-zinc-400 font-normal">/ {{ bySource.task?.requests ?? 0 }} runs · avg {{ fmtTokens(Math.round(bySource.task?.avg_total_tokens || 0)) }}</span></dd>
          </div>
        </dl>
      </section>

      <!-- BY CHAT TYPE -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">By Chat Type</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />群聊 / 私聊</span>
        </div>
        <div class="flex h-3 rounded-sm overflow-hidden mt-5 bg-zinc-100">
          <div class="bg-zinc-800" :style="{ width: shareW(byChatType.group?.total_tokens, byChatType.friend?.total_tokens) }" />
          <div class="bg-zinc-300" :style="{ width: shareW(byChatType.friend?.total_tokens, byChatType.group?.total_tokens) }" />
        </div>
        <dl class="flex-1 mt-3 text-xs">
          <div class="dimrow">
            <dt class="tlabel">群聊 Group</dt>
            <dd class="dimval">{{ fmtTokens(byChatType.group?.total_tokens) }} <span class="text-zinc-400 font-normal">/ {{ byChatType.group?.requests ?? 0 }} runs</span></dd>
          </div>
          <div class="dimrow">
            <dt class="tlabel">私聊 Friend</dt>
            <dd class="dimval">{{ fmtTokens(byChatType.friend?.total_tokens) }} <span class="text-zinc-400 font-normal">/ {{ byChatType.friend?.requests ?? 0 }} runs</span></dd>
          </div>
        </dl>
      </section>

      <!-- BY STATUS -->
      <section class="tcard p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">By Status</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />{{ statusTotal }} Finished</span>
        </div>
        <div class="flex h-3 rounded-sm overflow-hidden mt-5 bg-zinc-100">
          <div v-for="s in statusList" :key="s.key" :class="s.bar" :style="{ width: statusW(s.count) }" :title="`${s.label} ${s.count}`" />
        </div>
        <dl class="flex-1 mt-3 text-xs">
          <div v-for="s in statusList" :key="s.key" class="dimrow">
            <dt class="tlabel flex items-center gap-1.5"><span class="tdot" :class="s.dot" />{{ s.label }}</dt>
            <dd class="dimval">{{ s.count }} <span class="text-zinc-400 font-normal">{{ statusPct(s.count) }}</span></dd>
          </div>
        </dl>
      </section>
    </div>

    <!-- 小时分布 + 目标排行 -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-4">
      <!-- HOURLY -->
      <section class="tcard xl:col-span-5 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Hourly Distribution</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />24h</span>
        </div>
        <div class="flex-1 flex items-end gap-1 pt-5 pb-1 h-36">
          <div v-for="(h, i) in hourly" :key="i" class="flex-1 flex flex-col justify-end h-full" :title="`${i}:00 · ${h.total_tokens || 0} tok · ${h.requests || 0} runs`">
            <div class="w-full rounded-t-sm" :class="(h.total_tokens || 0) > 0 ? 'bg-zinc-700' : 'bg-zinc-200'" :style="{ height: hourBarH(h.total_tokens) }" />
          </div>
        </div>
        <div class="flex justify-between mt-2 text-[10px] tracking-[0.12em] uppercase text-zinc-400">
          <span>00</span><span>06</span><span>12</span><span>18</span><span>23</span>
        </div>
        <div class="dotline my-3" />
        <div class="text-[10px] tracking-[0.14em] uppercase text-zinc-500">
          Peak {{ peakHourText }}
        </div>
      </section>

      <!-- TOP TARGETS -->
      <section class="tcard xl:col-span-7 overflow-hidden flex flex-col">
        <div class="px-6 py-4 flex items-center justify-between border-b border-zinc-100">
          <h2 class="tlabel text-zinc-800!">Top Targets</h2>
          <span class="text-[10px] tracking-[0.15em] uppercase text-zinc-400">Top {{ topTargets.length }} by Tokens</span>
        </div>
        <p v-if="topTargets.length === 0" class="px-6 py-8 text-xs text-zinc-400 text-center tracking-wide">暂无消耗记录</p>
        <table v-else class="w-full text-xs">
          <thead>
            <tr class="text-left text-[10px] tracking-[0.15em] uppercase text-zinc-400 bg-zinc-50/60 border-b border-zinc-100">
              <th class="px-6 py-3 font-medium w-10">#</th>
              <th class="px-3 py-3 font-medium">目标</th>
              <th class="px-3 py-3 font-medium">占比</th>
              <th class="px-3 py-3 font-medium text-right">次数</th>
              <th class="px-6 py-3 font-medium text-right">Tokens</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(t, i) in topTargets" :key="t.chat_type + ':' + t.target_id" class="border-b border-dashed border-zinc-100 last:border-0 hover:bg-zinc-50/70 transition-colors">
              <td class="px-6 py-3 text-zinc-400">{{ i + 1 }}</td>
              <td class="px-3 py-3 text-zinc-800 font-medium whitespace-nowrap">
                <span class="text-[9px] tracking-[0.12em] uppercase border border-zinc-300 text-zinc-500 px-1.5 py-0.5 rounded mr-2">{{ t.chat_type === 'group' ? '群' : '私' }}</span>
                {{ t.target_id }}
              </td>
              <td class="px-3 py-3 w-40">
                <div class="flex items-center gap-2">
                  <div class="flex-1 h-1.5 bg-zinc-100 rounded-sm overflow-hidden">
                    <div class="h-full bg-zinc-700" :style="{ width: targetShareW(t.total_tokens) }" />
                  </div>
                  <span class="text-[10px] text-zinc-400 w-11 text-right shrink-0">{{ targetSharePct(t.total_tokens) }}</span>
                </div>
              </td>
              <td class="px-3 py-3 text-right text-zinc-600">{{ t.requests }}</td>
              <td class="px-6 py-3 text-right text-zinc-800 font-medium whitespace-nowrap" :title="`${t.total_tokens} tok (prompt ${t.prompt_tokens} / completion ${t.completion_tokens} / cached ${t.cached_tokens})`">{{ fmtTokens(t.total_tokens) }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api.js'

const detail = ref({})
let timer = null

const summary = computed(() => detail.value.summary || {})
const today = computed(() => detail.value.today || {})
const bySource = computed(() => detail.value.by_source || {})
const byChatType = computed(() => detail.value.by_chat_type || {})
const byStatus = computed(() => detail.value.by_status || {})
const topTargets = computed(() => detail.value.top_targets || [])
const hourly = computed(() => detail.value.hourly || [])
const daily = computed(() => detail.value.daily || [])

// ---- 状态分布 ----

const STATUS_META = [
  { key: 'success', label: '成功', bar: 'bg-zinc-800', dot: 'bg-emerald-500' },
  { key: 'stopped', label: '已停止', bar: 'bg-zinc-400', dot: 'bg-zinc-400' },
  { key: 'timeout', label: '超时', bar: 'bg-zinc-300', dot: 'bg-amber-500' },
  { key: 'error', label: '错误', bar: 'bg-zinc-200', dot: 'bg-red-500' },
]

const statusList = computed(() => STATUS_META.map(m => ({ ...m, count: byStatus.value[m.key] || 0 })))
const statusTotal = computed(() => statusList.value.reduce((s, x) => s + x.count, 0))

function statusW(count) {
  if (!statusTotal.value || !count) return '0%'
  return ((count / statusTotal.value) * 100).toFixed(1) + '%'
}

function statusPct(count) {
  if (!statusTotal.value) return '—'
  return ((count / statusTotal.value) * 100).toFixed(1) + '%'
}

// ---- 图表计算 ----

const dayMax = computed(() => Math.max(0, ...daily.value.map(d => d.total?.total_tokens || 0)))

function barH(v) {
  if (!dayMax.value || !v || v <= 0) return '0%'
  return ((v / dayMax.value) * 100).toFixed(1) + '%'
}

const hourMax = computed(() => Math.max(0, ...hourly.value.map(h => h.total_tokens || 0)))

function hourBarH(v) {
  if (!hourMax.value || !v || v <= 0) return '4%'
  return Math.max(4, (v / hourMax.value) * 100).toFixed(1) + '%'
}

const peakHourText = computed(() => {
  if (!hourMax.value) return '—'
  const idx = hourly.value.findIndex(h => (h.total_tokens || 0) === hourMax.value)
  return idx < 0 ? '—' : `${String(idx).padStart(2, '0')}:00 · ${fmtTokens(hourMax.value)}`
})

const targetMax = computed(() => Math.max(0, ...topTargets.value.map(t => t.total_tokens || 0)))

function targetShareW(v) {
  if (!targetMax.value || !v) return '0%'
  return ((v / targetMax.value) * 100).toFixed(1) + '%'
}

function targetSharePct(v) {
  const total = summary.value.total_tokens
  if (!total || !v) return '—'
  return ((v / total) * 100).toFixed(1) + '%'
}

function shareW(a, b) {
  const sum = (a || 0) + (b || 0)
  if (!sum) return '50%'
  return (((a || 0) / sum) * 100).toFixed(1) + '%'
}

// ---- 格式化 ----

function fmtTokens(n) {
  if (n == null || !isFinite(n) || n <= 0) return '0'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'k'
  return String(n)
}

function pctText(r) {
  return r != null && r > 0 ? (r * 100).toFixed(1) + '%' : '—'
}

function dayTip(d) {
  return `${d.date} · 共 ${d.total?.total_tokens || 0} tok（对话 ${d.query?.total_tokens || 0} / 任务 ${d.task?.total_tokens || 0}）· ${d.total?.requests || 0} runs`
}

async function load() {
  try { detail.value = await api.getTokenStatsDetail() } catch { /* 忽略轮询错误 */ }
}

function onVisible() {
  if (!document.hidden) load()
}

onMounted(() => {
  load()
  timer = setInterval(() => { if (!document.hidden) load() }, 30000)
  document.addEventListener('visibilitychange', onVisible)
})

onUnmounted(() => {
  clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisible)
})
</script>

<style scoped>
.dimrow {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.5rem 0;
  border-bottom: 1px dotted rgb(212 212 216);
}
.dimrow:last-child {
  border-bottom: 0;
}
.dimval {
  color: rgb(39 39 42);
  font-weight: 500;
  white-space: nowrap;
}
</style>
