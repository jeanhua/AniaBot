<template>
  <div class="space-y-4">
    <!-- 适配器未连接提示 -->
    <div
      v-if="status.adapter_status && status.adapter_status !== 'connected'"
      class="tcard border-amber-300! bg-amber-50! px-5 py-3.5 flex items-center justify-between gap-4"
    >
      <span class="flex items-center gap-3 text-xs text-amber-800">
        <span class="[&>svg]:w-4 [&>svg]:h-4 text-amber-500 shrink-0" v-html="icons.warn" />
        <span class="tracking-wide">
          <span class="uppercase tracking-[0.15em] font-semibold">Link Down</span>
          <span class="mx-2 text-amber-300">//</span>
          NapCat 未连接<template v-if="status.adapter_detail">：{{ status.adapter_detail }}</template>，Bot 会持续重试
        </span>
      </span>
      <RouterLink to="/config" class="text-[10px] tracking-[0.15em] uppercase text-zinc-700 hover:underline shrink-0 font-medium">修改配置并重启 →</RouterLink>
    </div>

    <!-- 仪器面板 bento 区 -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-4">
      <!-- DEVICE CLOCK -->
      <section class="tcard xl:col-span-5 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Device Clock</span>
          <span class="tpill"><span class="tdot bg-emerald-500" />Sync OK</span>
        </div>

        <div class="flex-1 flex items-center justify-between gap-6 py-5">
          <div class="flex items-center gap-5 min-w-0">
            <div class="dotgrid w-14 h-12 shrink-0 hidden sm:block" />
            <div class="text-[56px] leading-none font-semibold tracking-tight text-zinc-900 whitespace-nowrap">
              {{ hh }}<span class="blink">:</span>{{ mm }}<span class="text-2xl text-zinc-400 font-medium ml-1">{{ ss }}</span>
            </div>
          </div>
          <dl class="text-right shrink-0 space-y-1.5">
            <div>
              <dt class="tlabel">Uptime</dt>
              <dd class="text-base font-semibold text-zinc-900 whitespace-nowrap">{{ uptime }}</dd>
            </div>
            <div class="border-t border-dotted border-zinc-300 pt-1.5">
              <dt class="tlabel">Goroutines</dt>
              <dd class="text-base font-semibold text-zinc-900">{{ status.goroutines ?? '—' }}</dd>
            </div>
            <div class="border-t border-dotted border-zinc-300 pt-1.5">
              <dt class="tlabel">Plugins</dt>
              <dd class="text-base font-semibold text-zinc-900">{{ status.plugin_count ?? '—' }}</dd>
            </div>
          </dl>
        </div>

        <div class="text-[10px] tracking-[0.18em] uppercase text-zinc-500">{{ dateLine }}</div>
        <div class="dotline my-3" />
        <div class="text-[10px] tracking-[0.14em] uppercase text-zinc-500 truncate">
          Adapter link {{ linked ? 'live' : 'down' }}
          <span class="mx-2 text-zinc-300">//</span>
          Plugin registry {{ status.plugin_count ?? 0 }} loaded
          <span class="mx-2 text-zinc-300">//</span>
          Scheduler {{ armedCount }} armed
        </div>
      </section>

      <!-- ADAPTER LINK -->
      <section class="tcard xl:col-span-4 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Adapter Link</span>
          <span class="tpill">
            <span class="tdot" :class="linked ? 'bg-emerald-500' : 'bg-amber-500'" />
            {{ linked ? 'Link Live' : 'Link Down' }}
          </span>
        </div>

        <div class="flex-1 py-5">
          <div class="text-4xl font-semibold tracking-tight" :class="linked ? 'text-zinc-900' : 'text-amber-600'">{{ adapterText }}</div>
          <div class="tlabel mt-2">NapCat · OneBot v11</div>
        </div>

        <div class="flex gap-0.75">
          <div
            v-for="i in 12"
            :key="i"
            class="seg"
            :class="i <= linkSegs ? (linked ? 'on-ok' : 'on') : ''"
          />
        </div>
        <div class="flex items-center justify-between mt-2 text-[10px] tracking-[0.12em] uppercase text-zinc-400">
          <span class="truncate" :title="status.adapter_detail">{{ status.adapter_detail || 'No detail' }}</span>
          <span class="shrink-0 ml-3">Sample 5s</span>
        </div>
      </section>

      <!-- SCHEDULER -->
      <section class="tcard xl:col-span-3 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Scheduler</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />{{ clocks.length }} Jobs</span>
        </div>

        <div class="flex-1 py-5">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">
            {{ armedCount }}<span class="text-xl text-zinc-400 font-medium">/{{ clocks.length }}</span>
          </div>
          <div class="tlabel mt-2">Armed jobs</div>
        </div>

        <div class="border-t border-dotted border-zinc-300 pt-3">
          <div class="tlabel">Next run</div>
          <div class="text-sm font-medium text-zinc-800 mt-1 truncate">{{ nextRunText }}</div>
        </div>
      </section>
    </div>

    <!-- 主机监控 -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-4">
      <!-- CPU LOAD -->
      <section class="tcard xl:col-span-4 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">CPU Load</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />{{ host.cpu_cores ?? '—' }} Cores</span>
        </div>

        <div class="flex items-end justify-between gap-4 pt-4 pb-3">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">
            {{ cpuText }}<span class="text-xl text-zinc-400 font-medium">%</span>
          </div>
          <span class="tlabel text-zinc-400! pb-1">Sample 5s</span>
        </div>

        <svg viewBox="0 0 100 32" preserveAspectRatio="none" class="w-full h-12 block">
          <line x1="0" y1="31.5" x2="100" y2="31.5" stroke="#e4e4e7" stroke-width="0.5" />
          <polyline
            v-if="cpuPoints"
            :points="cpuPoints"
            fill="none"
            stroke="#18181b"
            stroke-width="1.5"
            stroke-linejoin="round"
            stroke-linecap="round"
            vector-effect="non-scaling-stroke"
          />
          <text v-else x="50" y="18" text-anchor="middle" class="fill-zinc-300" font-size="6" letter-spacing="1">SAMPLING…</text>
        </svg>

        <div class="flex items-center justify-between mt-3 text-[10px] tracking-[0.12em] uppercase text-zinc-400">
          <span class="truncate" :title="host.cpu_model">{{ host.cpu_model || 'Unknown CPU' }}</span>
        </div>
      </section>

      <!-- MEMORY -->
      <section class="tcard xl:col-span-4 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Memory</span>
          <span class="tpill"><span class="tdot" :class="host.mem_percent >= 90 ? 'bg-red-500' : host.mem_percent >= 75 ? 'bg-amber-500' : 'bg-emerald-500'" />{{ memPctText }}</span>
        </div>

        <div class="flex-1 flex items-end justify-between gap-4 pt-4 pb-3">
          <div class="text-4xl font-semibold tracking-tight text-zinc-900">
            {{ fmtBytes(host.mem_used) }}<span class="text-xl text-zinc-400 font-medium">/{{ fmtBytes(host.mem_total) }}</span>
          </div>
          <span class="tlabel text-zinc-400! pb-1">Phys RAM</span>
        </div>

        <div class="flex gap-0.75">
          <div v-for="i in 14" :key="i" class="seg" :class="i <= memSegs ? 'on' : ''" />
        </div>

        <div class="flex items-center justify-between mt-3 text-[10px] tracking-[0.12em] uppercase text-zinc-400">
          <span>{{ fmtBytes(host.mem_total - host.mem_used) }} free</span>
          <span>Bot heap {{ fmtBytes(host.go_mem_alloc) }} · {{ host.go_version || '—' }}</span>
        </div>
      </section>

      <!-- HOST INFO -->
      <section class="tcard xl:col-span-4 p-6 flex flex-col">
        <div class="flex items-center justify-between">
          <span class="tlabel">Host Info</span>
          <span class="tpill"><span class="tdot bg-zinc-800" />{{ host.os || '—' }} · {{ host.arch || '—' }}</span>
        </div>

        <dl class="flex-1 mt-3 text-xs">
          <div class="hostrow">
            <dt class="tlabel">Hostname</dt>
            <dd class="hostval" :title="host.hostname">{{ host.hostname || '—' }}</dd>
          </div>
          <div class="hostrow">
            <dt class="tlabel">System</dt>
            <dd class="hostval" :title="host.os_version">{{ host.os_version || '—' }}</dd>
          </div>
          <div class="hostrow">
            <dt class="tlabel">Kernel</dt>
            <dd class="hostval">{{ host.kernel || '—' }}</dd>
          </div>
          <div class="hostrow">
            <dt class="tlabel">Uptime</dt>
            <dd class="hostval">{{ hostUptimeText }}</dd>
          </div>
        </dl>
      </section>
    </div>

    <!-- 插件列表 -->
    <section class="tcard overflow-hidden">
      <div class="px-6 py-4 flex items-center justify-between border-b border-zinc-100">
        <h2 class="tlabel text-zinc-800!">Plugin Registry</h2>
        <span class="text-[10px] tracking-[0.15em] uppercase text-zinc-400">{{ plugins.length }} Modules</span>
      </div>
      <table class="w-full text-xs">
        <thead>
          <tr class="text-left text-[10px] tracking-[0.15em] uppercase text-zinc-400 bg-zinc-50/60 border-b border-zinc-100">
            <th class="px-6 py-3 font-medium">名称</th>
            <th class="px-6 py-3 font-medium">说明</th>
            <th class="px-6 py-3 font-medium">作者</th>
            <th class="px-6 py-3 font-medium">版本</th>
            <th class="px-6 py-3 font-medium">仅管理员</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in plugins" :key="p.name" class="border-b border-dashed border-zinc-100 last:border-0 hover:bg-zinc-50/70 transition-colors">
            <td class="px-6 py-3 font-semibold text-zinc-800">{{ p.name }}</td>
            <td class="px-6 py-3 text-zinc-600">{{ p.help_words }}</td>
            <td class="px-6 py-3 text-zinc-600">{{ p.author }}</td>
            <td class="px-6 py-3 text-zinc-500">{{ p.version }}</td>
            <td class="px-6 py-3">
              <span v-if="p.admin_only" class="tpill py-0.5!"><span class="tdot bg-amber-500" />Admin</span>
              <span v-else class="text-[10px] tracking-[0.15em] uppercase text-zinc-400">All</span>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <!-- AI 定时任务 -->
    <section class="tcard overflow-hidden">
      <div class="px-6 py-4 border-b border-zinc-100 flex items-center justify-between">
        <h2 class="tlabel text-zinc-800!">AI Cron Jobs</h2>
        <div class="flex items-center gap-3">
          <button class="text-[10px] tracking-[0.15em] uppercase text-zinc-500 hover:text-zinc-900 font-medium transition-colors" @click="loadClocks">刷新</button>
          <button class="text-[10px] tracking-[0.15em] uppercase bg-zinc-900 text-white px-3 py-1.5 rounded-md hover:bg-zinc-700 font-medium transition-colors" @click="openCreate">新建任务</button>
        </div>
      </div>
      <p v-if="clocks.length === 0" class="px-6 py-8 text-xs text-zinc-400 text-center tracking-wide">暂无定时任务，点击右上角「新建任务」创建（也可在群聊/私聊中使用 /clock）</p>
      <table v-else class="w-full text-xs">
        <thead>
          <tr class="text-left text-[10px] tracking-[0.15em] uppercase text-zinc-400 bg-zinc-50/60 border-b border-zinc-100">
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
              class="border-b border-dashed border-zinc-100 last:border-0 hover:bg-zinc-50/70 transition-colors cursor-pointer"
              :class="{ 'bg-zinc-50/70': expanded.has(t.id) }"
              @click="toggleExpand(t.id)"
            >
              <td class="px-6 py-3 text-zinc-800 max-w-48">
                <span class="flex items-center gap-1.5">
                  <span
                    class="[&>svg]:w-3 [&>svg]:h-3 text-zinc-400 transition-transform shrink-0"
                    :class="{ 'rotate-90': expanded.has(t.id) }"
                    v-html="icons.chevron"
                  />
                  <span class="truncate font-medium" :title="t.title">{{ t.title || '(无标题)' }}</span>
                </span>
                <span v-if="t.run_once" class="text-[9px] tracking-[0.12em] uppercase border border-zinc-300 text-zinc-500 px-1.5 py-0.5 rounded ml-4">单次</span>
              </td>
              <td class="px-6 py-3 text-zinc-600 whitespace-nowrap">{{ t.target_type === 'group' ? '群' : '好友' }} {{ t.target_id }}</td>
              <td class="px-6 py-3 text-zinc-500 whitespace-nowrap">{{ t.cron }}</td>
              <td class="px-6 py-3 text-zinc-600 whitespace-nowrap">{{ t.enabled ? fmtTime(t.next_run_at) : '—' }}</td>
              <td class="px-6 py-3 text-zinc-600 whitespace-nowrap">{{ fmtTime(t.last_run_at) }}</td>
              <td class="px-6 py-3" @click.stop>
                <button
                  type="button"
                  role="switch"
                  :aria-checked="t.enabled"
                  :disabled="toggling.has(t.id)"
                  class="relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors disabled:opacity-50"
                  :class="t.enabled ? 'bg-zinc-900' : 'bg-zinc-200'"
                  @click="toggleClock(t)"
                >
                  <span
                    class="inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform"
                    :class="t.enabled ? 'translate-x-4.5' : 'translate-x-0.75'"
                  />
                </button>
              </td>
            </tr>
            <!-- 任务详情 -->
            <tr v-if="expanded.has(t.id)" class="border-b border-dashed border-zinc-100 last:border-0 bg-zinc-50/40">
              <td colspan="6" class="px-6 py-4">
                <dl class="grid grid-cols-1 sm:grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-xs">
                  <dt class="tlabel">任务内容</dt>
                  <dd class="text-zinc-700 whitespace-pre-wrap break-all">{{ t.content }}</dd>
                  <template v-if="t.note">
                    <dt class="tlabel">备注</dt>
                    <dd class="text-zinc-700 whitespace-pre-wrap break-all">{{ t.note }}</dd>
                  </template>
                  <dt class="tlabel">超时时间</dt>
                  <dd class="text-zinc-700">{{ t.timeout_sec > 0 ? t.timeout_sec + ' 秒' : '默认' }}</dd>
                  <dt class="tlabel">创建者</dt>
                  <dd class="text-zinc-700">{{ t.created_by ? 'QQ ' + t.created_by : '—' }}</dd>
                  <dt class="tlabel">创建时间</dt>
                  <dd class="text-zinc-700">{{ fmtTime(t.created_at) }}</dd>
                </dl>
                <div class="mt-4 flex items-center gap-2">
                  <button
                    class="text-[10px] tracking-[0.15em] uppercase bg-zinc-900 text-white px-3 py-1.5 rounded-md hover:bg-zinc-700 font-medium transition-colors disabled:opacity-50"
                    :disabled="toggling.has(t.id)"
                    @click="openEdit(t)"
                  >编辑</button>
                  <button
                    class="text-[10px] tracking-[0.15em] uppercase border border-zinc-300 text-zinc-600 px-3 py-1.5 rounded-md hover:bg-zinc-100 hover:text-red-600 hover:border-red-300 font-medium transition-colors disabled:opacity-50"
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
    <section class="tcard overflow-hidden">
      <div class="px-6 py-4 border-b border-zinc-100 flex items-center justify-between">
        <h2 class="tlabel text-zinc-800!">Execution Log</h2>
        <button class="text-[10px] tracking-[0.15em] uppercase text-zinc-500 hover:text-zinc-900 font-medium transition-colors" @click="loadLogs">刷新</button>
      </div>
      <p v-if="logs.length === 0" class="px-6 py-8 text-xs text-zinc-400 text-center tracking-wide">暂无执行记录</p>
      <table v-else class="w-full text-xs">
        <thead>
          <tr class="text-left text-[10px] tracking-[0.15em] uppercase text-zinc-400 bg-zinc-50/60 border-b border-zinc-100">
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
          <tr v-for="log in logs" :key="log.id" class="border-b border-dashed border-zinc-100 last:border-0 hover:bg-zinc-50/70 transition-colors">
            <td class="px-6 py-3 text-zinc-800 max-w-48 truncate font-medium" :title="log.task_title">{{ log.task_title }}</td>
            <td class="px-6 py-3 text-zinc-600">{{ log.target_type === 'group' ? '群' : '好友' }} {{ log.target_id }}</td>
            <td class="px-6 py-3 text-zinc-600 whitespace-nowrap">{{ fmtTime(log.trigger_time) }}</td>
            <td class="px-6 py-3">
              <span class="tpill py-0.5!"><span class="tdot" :class="statusDot(log.status)" />{{ statusText(log.status) }}</span>
            </td>
            <td class="px-6 py-3 text-zinc-600">{{ log.duration_ms ? (log.duration_ms / 1000).toFixed(1) + 's' : '—' }}</td>
            <td class="px-6 py-3 text-zinc-600">{{ log.total_tokens || '—' }}</td>
            <td class="px-6 py-3 text-red-600 max-w-40 truncate" :title="log.error">{{ log.error || '' }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <!-- 新建 / 编辑定时任务弹窗 -->
    <Teleport to="body">
      <div v-if="clockForm" class="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/40 p-4" @click.self="clockForm = null">
        <div class="tcard w-full max-w-lg max-h-[90vh] overflow-y-auto">
          <div class="px-6 py-4 border-b border-zinc-100 flex items-center justify-between">
            <h3 class="tlabel text-zinc-800!">{{ clockForm.id ? '编辑定时任务' : '新建定时任务' }}</h3>
            <button class="text-zinc-400 hover:text-zinc-700 transition-colors" @click="clockForm = null">✕</button>
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
                <input v-model.trim="clockForm.cron" type="text" class="form-input" placeholder="0 8 * * * 或 @every 1h" />
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
            <label class="flex items-center gap-2 text-xs text-zinc-700 select-none">
              <input v-model="clockForm.run_once" type="checkbox" class="accent-zinc-900" :disabled="!!clockForm.id" />
              单次任务（触发一次后自动删除）
            </label>
            <p v-if="clockFormError" class="text-xs text-red-600">{{ clockFormError }}</p>
            <div class="flex justify-end gap-2 pt-1">
              <button type="button" class="px-4 py-2 text-[11px] tracking-widest uppercase text-zinc-500 hover:text-zinc-800 font-medium transition-colors" @click="clockForm = null">取消</button>
              <button type="submit" class="px-4 py-2 text-[11px] tracking-widest uppercase bg-zinc-900 text-white rounded-md hover:bg-zinc-700 font-medium transition-colors disabled:opacity-50" :disabled="clockSaving">
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
  warn: '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"/></svg>',
  chevron: '<svg fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5"/></svg>',
}

const status = ref({})
const host = ref({})
const plugins = ref([])
const logs = ref([])
const clocks = ref([])
const toggling = ref(new Set())
const expanded = ref(new Set())
const now = ref(new Date())
const cpuHistory = ref([]) // CPU 占用率历史（最近 48 个采样点）
let timer = null
let clockTimer = null

// ---- 仪器时钟 ----

const hh = computed(() => String(now.value.getHours()).padStart(2, '0'))
const mm = computed(() => String(now.value.getMinutes()).padStart(2, '0'))
const ss = computed(() => String(now.value.getSeconds()).padStart(2, '0'))

const dateLine = computed(() => {
  const d = now.value
  const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  const off = -d.getTimezoneOffset() / 60
  const tz = `UTC${off >= 0 ? '+' : '-'}${String(Math.abs(off)).padStart(2, '0')}:00`
  return `${weekdays[d.getDay()]} · ${String(d.getDate()).padStart(2, '0')} ${months[d.getMonth()]} ${d.getFullYear()} · ${tz}`
})

// ---- 状态 ----

const linked = computed(() => status.value.adapter_status === 'connected')

const linkSegs = computed(() => {
  if (linked.value) return 12
  const s = status.value.adapter_status
  if (s === 'connecting' || s === 'reconnecting') return 5
  return 0
})

const uptime = computed(() => {
  const s = status.value.uptime_sec
  if (s == null) return '—'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const pad = n => String(n).padStart(2, '0')
  if (d > 0) return `${d}d ${pad(h)}:${pad(m)}`
  return `${pad(h)}:${pad(m)}:${pad(s % 60)}`
})

const adapterText = computed(() => ({
  connected: '已连接',
  connecting: '连接中',
  reconnecting: '重连中',
  setup_pending: '待配置',
  not_started: '未连接',
  unknown: '未知',
}[status.value.adapter_status] || status.value.adapter_status || '—'))

const armedCount = computed(() => clocks.value.filter(t => t.enabled).length)

// ---- 主机监控 ----

const cpuText = computed(() => {
  const p = host.value.cpu_percent
  return p != null && p >= 0 ? p.toFixed(1) : '—'
})

const memPctText = computed(() => {
  const p = host.value.mem_percent
  return p != null && host.value.mem_total ? p.toFixed(1) + '%' : '—'
})

const memSegs = computed(() => {
  const p = host.value.mem_percent
  return p != null && host.value.mem_total ? Math.round((p / 100) * 14) : 0
})

const hostUptimeText = computed(() => {
  const s = host.value.uptime_sec
  if (!s) return '—'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const pad = n => String(n).padStart(2, '0')
  return d > 0 ? `${d}d ${pad(h)}:${pad(m)}` : `${pad(h)}:${pad(m)}`
})

const cpuPoints = computed(() => {
  const h = cpuHistory.value
  if (h.length < 2) return ''
  const max = Math.max(10, ...h)
  const n = h.length
  return h
    .map((v, i) => `${((i / (n - 1)) * 100).toFixed(2)},${(31 - (v / max) * 29).toFixed(2)}`)
    .join(' ')
})

function fmtBytes(b) {
  if (b == null || !isFinite(b) || b <= 0) return '0M'
  const gb = b / 1073741824
  if (gb >= 10) return gb.toFixed(0) + 'G'
  if (gb >= 1) return gb.toFixed(1) + 'G'
  return Math.round(b / 1048576) + 'M'
}

const nextRunText = computed(() => {
  const next = clocks.value
    .filter(t => t.enabled && t.next_run_at)
    .map(t => new Date(t.next_run_at))
    .filter(d => !isNaN(d) && d.getFullYear() >= 2000)
    .sort((a, b) => a - b)[0]
  if (!next) return '—'
  const title = clocks.value.find(t => t.enabled && t.next_run_at && +new Date(t.next_run_at) === +next)?.title
  return `${next.toLocaleString('zh-CN', { hour12: false })}${title ? ' · ' + title : ''}`
})

function fmtTime(t) {
  if (!t) return '—'
  const d = new Date(t)
  if (isNaN(d) || d.getFullYear() < 2000) return '—' // Go 零值时间
  return d.toLocaleString('zh-CN', { hour12: false })
}

function statusText(s) {
  return { running: '执行中', success: '成功', timeout: '超时', error: '失败' }[s] || s
}

function statusDot(s) {
  return {
    running: 'bg-blue-500',
    success: 'bg-emerald-500',
    timeout: 'bg-amber-500',
    error: 'bg-red-500',
  }[s] || 'bg-zinc-400'
}

async function loadStatus() {
  try { status.value = await api.getStatus() } catch { /* 忽略轮询错误 */ }
}

async function loadHost() {
  try {
    const h = await api.getHost()
    host.value = h
    if (h.cpu_percent != null && h.cpu_percent >= 0) {
      const hist = [...cpuHistory.value, h.cpu_percent]
      cpuHistory.value = hist.length > 48 ? hist.slice(hist.length - 48) : hist
    }
  } catch { /* 忽略轮询错误 */ }
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

// 实时刷新：状态 / 定时任务 / 执行日志统一轮询；标签页隐藏时暂停，恢复可见时立即刷新
function poll() {
  loadStatus()
  loadHost()
  loadLogs()
  loadClocks()
}

function onVisible() {
  if (!document.hidden) poll()
}

onMounted(async () => {
  poll()
  try { plugins.value = await api.getPlugins() } catch { /* 忽略 */ }
  timer = setInterval(() => { if (!document.hidden) poll() }, 5000)
  clockTimer = setInterval(() => { now.value = new Date() }, 1000)
  document.addEventListener('visibilitychange', onVisible)
})

onUnmounted(() => {
  clearInterval(timer)
  clearInterval(clockTimer)
  document.removeEventListener('visibilitychange', onVisible)
})
</script>

<style scoped>
.hostrow {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.5rem 0;
  border-bottom: 1px dotted rgb(212 212 216);
}
.hostrow:last-child {
  border-bottom: 0;
}
.hostval {
  color: rgb(39 39 42);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.form-label {
  display: block;
  font-size: 10px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: rgb(113 113 122);
  margin-bottom: 0.375rem;
}
.form-input {
  width: 100%;
  border: 1px solid rgb(212 212 216);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.75rem;
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
