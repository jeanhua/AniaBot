<template>
  <div class="space-y-5 max-w-6xl">
    <!-- 环境提示 -->
    <div v-if="info && info.mode === 'dev'" class="tcard p-4 border-l-2 border-l-amber-400 flex items-start gap-3">
      <span class="tdot bg-amber-400 mt-1.5 shrink-0" />
      <div class="text-xs text-zinc-600 leading-relaxed">
        当前为 <span class="font-semibold text-zinc-900">go run 开发模式</span>运行，插件市场不可用。
        请以编译后的二进制方式部署（容器内可直接使用）。
      </div>
    </div>
    <div v-else-if="info && !info.enabled" class="tcard p-4 border-l-2 border-l-amber-400 flex items-start gap-3">
      <span class="tdot bg-amber-400 mt-1.5 shrink-0" />
      <div class="text-xs text-zinc-600 leading-relaxed">
        插件市场未开启。请先在
        <RouterLink to="/config" class="font-semibold text-zinc-900 underline underline-offset-2">配置管理</RouterLink>
        的「插件市场」分组中设置 <span class="font-mono">bot.marketplace.enable=true</span>。
      </div>
    </div>
    <div v-else-if="info && !info.configured" class="tcard p-4 border-l-2 border-l-amber-400 flex items-start gap-3">
      <span class="tdot bg-amber-400 mt-1.5 shrink-0" />
      <div class="text-xs text-zinc-600 leading-relaxed">
        尚未配置源码目录。请先在
        <RouterLink to="/config" class="font-semibold text-zinc-900 underline underline-offset-2">配置管理</RouterLink>
        中设置 <span class="font-mono">bot.marketplace.source_dir</span>（或自动更新的
        <span class="font-mono">bot.update.source_dir</span>），并确保已完成一次自动更新。
      </div>
    </div>

    <!-- 市场信息 + 登录 -->
    <div class="tcard p-5">
      <div class="flex items-center justify-between mb-4 flex-wrap gap-2">
        <span class="tlabel">Marketplace / 插件市场</span>
        <div class="flex items-center gap-2">
          <span v-if="info" class="tpill"><span class="tdot" :class="info.token_set ? 'bg-emerald-500' : 'bg-zinc-300'" />{{ info.token_set ? '已登录 GitHub' : '未登录（限流 60 次/小时）' }}</span>
          <span v-if="info && info.rate_remaining >= 0" class="tpill"><span class="tdot bg-zinc-300" />剩余配额 {{ info.rate_remaining }}</span>
          <button class="text-[10px] tracking-[0.15em] uppercase text-zinc-500 hover:text-zinc-900 font-medium transition-colors" :disabled="!canBrowse || loading" @click="load(true)">{{ loading ? '刷新中...' : '刷新' }}</button>
        </div>
      </div>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
        <div>
          <div class="tlabel mb-1">插件仓库</div>
          <div class="font-mono text-sm text-zinc-900">{{ info?.repo || '—' }}</div>
        </div>
        <div>
          <div class="tlabel mb-1">分支</div>
          <div class="font-mono text-sm text-zinc-900">{{ info?.branch || '—' }}</div>
        </div>
        <div>
          <div class="tlabel mb-1">已安装</div>
          <div class="font-mono text-sm text-zinc-900">{{ info?.installed ?? '—' }} 个</div>
        </div>
        <div>
          <div class="tlabel mb-1">环境</div>
          <div class="flex items-center gap-3 text-[11px]">
            <span v-for="t in ['git', 'go']" :key="t" class="flex items-center gap-1.5">
              <span class="tdot" :class="info?.env?.[t] ? 'bg-emerald-500' : 'bg-red-400'" />
              <span class="text-zinc-500 uppercase tracking-wider">{{ t }}</span>
            </span>
          </div>
        </div>
      </div>
      <div class="pt-4 border-t border-zinc-100 flex items-end gap-3 flex-wrap">
        <div class="flex-1 min-w-64">
          <div class="tlabel mb-1">GitHub Token（可选登录，提升限流到 5000 次/小时）</div>
          <input v-model="tokenInput" type="password" placeholder="ghp_... 或留空清除" :class="inputClass" @keydown.enter="onSaveToken" />
        </div>
        <button class="text-[10px] tracking-[0.15em] uppercase bg-zinc-900 text-white px-3 py-2 rounded-md hover:bg-zinc-700 font-medium transition-colors" @click="onSaveToken">保存 Token</button>
        <button v-if="info?.enabled" class="text-[10px] tracking-[0.15em] uppercase text-zinc-500 hover:text-zinc-900 font-medium transition-colors" :disabled="status.running || status.restarting" @click="onRollback">回滚上次安装</button>
      </div>
      <p v-if="tokenMsg" class="text-xs mt-2" :class="tokenOk ? 'text-emerald-600' : 'text-red-600'">{{ tokenMsg }}</p>
    </div>

    <!-- 错误 -->
    <div v-if="listError" class="tcard p-4 border-l-2 border-l-red-400">
      <p class="text-xs text-red-600 font-mono break-all leading-relaxed">{{ listError }}</p>
      <p class="text-[10px] text-zinc-400 mt-2">可能是网络不通或触发 GitHub API 限流，可配置 Token 后重试。</p>
    </div>

    <!-- 列表 -->
    <div class="tcard p-5">
      <div class="flex items-center justify-between mb-4 gap-3 flex-wrap">
        <span class="tlabel">Plugins / 插件列表</span>
        <div class="flex items-center gap-2">
          <input v-model="keyword" placeholder="搜索名称 / 描述 / 作者 / 标签" :class="[inputClass, '!w-64']" />
          <button
            v-for="t in tabs" :key="t.key"
            class="text-[10px] tracking-[0.15em] uppercase px-3 py-1.5 rounded-md border transition-colors"
            :class="tab === t.key ? 'bg-zinc-900 text-white border-zinc-900' : 'text-zinc-500 border-zinc-200 hover:bg-zinc-50'"
            @click="tab = t.key"
          >{{ t.label }}</button>
        </div>
      </div>
      <div v-if="filtered.length === 0" class="text-xs text-zinc-400 py-10 text-center">暂无插件{{ loading ? '（加载中...）' : '' }}</div>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div
          v-for="p in filtered" :key="p.id"
          class="border border-slate-200/70 rounded-lg p-4 hover:shadow-sm transition-shadow cursor-pointer bg-white"
          @click="openDetail(p.id)"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-semibold text-zinc-900 truncate">{{ p.name }}</span>
                <span v-if="p.installed" class="tpill"><span class="tdot bg-emerald-500" />已安装 {{ p.installed_version }}</span>
                <span v-else-if="p.update_available" class="tpill"><span class="tdot bg-amber-400" />可更新</span>
              </div>
              <p class="text-xs text-zinc-500 mt-1 line-clamp-2">{{ p.description }}</p>
            </div>
          </div>
          <div class="flex items-center justify-between mt-3">
            <div class="flex items-center gap-2 text-[10px] text-zinc-400">
              <span>{{ p.author }}</span>
              <span>v{{ p.version }}</span>
              <span v-if="p.tags?.length" class="flex gap-1">
                <span v-for="t in p.tags.slice(0, 3)" :key="t" class="px-1.5 py-0.5 rounded bg-zinc-100 text-zinc-500">{{ t }}</span>
              </span>
            </div>
            <div class="flex items-center gap-2" @click.stop>
              <button
                v-if="p.installed && !p.update_available"
                class="text-[10px] tracking-[0.15em] uppercase text-zinc-400 hover:text-red-600 font-medium transition-colors"
                :disabled="status.running || status.restarting"
                @click="onUninstall(p)"
              >卸载</button>
              <button
                v-if="!p.installed || p.update_available"
                class="text-[10px] tracking-[0.15em] uppercase px-3 py-1.5 rounded-md font-medium transition-colors disabled:opacity-50"
                :class="p.installed ? 'bg-amber-500/10 text-amber-700 border border-amber-300 hover:bg-amber-500/20' : 'bg-zinc-900 text-white hover:bg-zinc-700'"
                :disabled="!canOperate || status.running || status.restarting"
                @click="onInstall(p)"
              >{{ p.installed ? '升级' : '安装' }}</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 详情抽屉 -->
    <div v-if="detail" class="fixed inset-0 bg-zinc-950/50 backdrop-blur-sm z-40 flex justify-end" @click.self="detail = null">
      <div class="w-full max-w-2xl bg-white h-full overflow-y-auto p-6 space-y-4">
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-lg font-semibold text-zinc-900">{{ detail.manifest.name }}</h2>
              <span v-if="detail.installed" class="tpill"><span class="tdot bg-emerald-500" />已安装 {{ detail.installed_version }}</span>
            </div>
            <p class="text-xs text-zinc-500 mt-1">{{ detail.manifest.description }}</p>
          </div>
          <button class="text-zinc-400 hover:text-zinc-800 text-lg leading-none" @click="detail = null">✕</button>
        </div>
        <div class="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-zinc-500">
          <span>作者：{{ detail.manifest.author }}</span>
          <span>版本：v{{ detail.manifest.version }}</span>
          <span v-if="detail.manifest.platforms?.length">平台：{{ detail.manifest.platforms.join(' / ') }}</span>
          <span v-if="detail.manifest.min_framework">最低框架：{{ detail.manifest.min_framework }}</span>
        </div>
        <div class="flex items-center gap-2">
          <button
            v-if="!detail.installed || detail.installed_version !== detail.manifest.version"
            class="text-[10px] tracking-[0.15em] uppercase px-4 py-2 rounded-md font-medium transition-colors disabled:opacity-50"
            :class="detail.installed ? 'bg-amber-500/10 text-amber-700 border border-amber-300 hover:bg-amber-500/20' : 'bg-zinc-900 text-white hover:bg-zinc-700'"
            :disabled="!canOperate || status.running || status.restarting"
            @click="onInstall(detail.manifest)"
          >{{ detail.installed ? '升级' : '安装' }}</button>
          <button
            v-if="detail.installed"
            class="text-[10px] tracking-[0.15em] uppercase text-zinc-400 hover:text-red-600 font-medium transition-colors"
            :disabled="status.running || status.restarting"
            @click="onUninstall(detail.manifest)"
          >卸载</button>
        </div>
        <div class="markdown-body bg-white border border-slate-200/70 rounded-lg px-4 py-3 max-h-[55vh] overflow-y-auto">
          <div v-if="!detail.readme" class="text-xs text-zinc-400">该插件未提供 README。</div>
          <div v-else v-html="renderedReadme" />
        </div>
      </div>
    </div>

    <!-- 进度 -->
    <div v-if="started" class="tcard p-5">
      <div class="tlabel mb-4">Progress / 任务进度</div>
      <div v-if="status.restarting" class="mb-3 text-xs text-zinc-500">操作完成，等待 Bot 重启...</div>
      <div class="flex flex-wrap items-center gap-y-2 mb-5">
        <template v-for="(p, i) in phases" :key="p.key">
          <div class="flex items-center gap-1.5">
            <span class="w-4 h-4 rounded-full border flex items-center justify-center text-[9px] font-mono" :class="phaseClass(p.key)">{{ phaseDone(p.key) ? '✓' : i + 1 }}</span>
            <span class="text-[10px] tracking-widest uppercase" :class="phaseTextClass(p.key)">{{ p.label }}</span>
          </div>
          <span v-if="i < phases.length - 1" class="mx-2 h-px w-5 bg-zinc-200" />
        </template>
      </div>
      <div ref="logEl" class="bg-zinc-950 rounded-md p-3 h-64 overflow-y-auto font-mono text-[11px] leading-relaxed text-zinc-300">
        <div v-for="(l, i) in status.logs" :key="i" :class="logLineClass(l)">{{ l }}</div>
        <div v-if="status.running" class="flex items-center gap-2 text-zinc-500 mt-1">
          <span class="w-3 h-3 border-2 border-zinc-700 border-t-zinc-300 rounded-full animate-spin" />
          执行中...
        </div>
      </div>
    </div>

    <!-- 重启中遮罩 -->
    <div v-if="rebooting" class="fixed inset-0 bg-zinc-950/60 backdrop-blur-sm flex items-center justify-center z-50">
      <div class="tcard p-8 w-80 text-center space-y-3">
        <span class="mx-auto block w-8 h-8 border-[3px] border-zinc-200 border-t-zinc-800 rounded-full animate-spin" />
        <div class="text-sm font-semibold text-zinc-900 tracking-[0.15em] uppercase">Rebooting</div>
        <p class="text-xs text-zinc-500">插件变更已应用，Bot 正在重启，恢复后页面自动刷新</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { api } from '../api.js'

const phases = [
  { key: 'env', label: '环境检查' },
  { key: 'fetch', label: '下载源码' },
  { key: 'verify', label: '校验' },
  { key: 'copy', label: '写入目录' },
  { key: 'generate', label: '生成注册' },
  { key: 'deps', label: '拉取依赖' },
  { key: 'build', label: '编译' },
  { key: 'swap', label: '替换二进制' },
]
const phaseOrder = phases.map((p) => p.key)

const info = ref(null)
const loading = ref(false)
const listError = ref('')
const plugins = ref([])
const keyword = ref('')
const tab = ref('all')
const tabs = [
  { key: 'all', label: '全部' },
  { key: 'installed', label: '已安装' },
  { key: 'updatable', label: '可更新' },
]
const tokenInput = ref('')
const tokenMsg = ref('')
const tokenOk = ref(false)
const detail = ref(null)
const started = ref(false)
const rebooting = ref(false)
const logEl = ref(null)
const status = reactive({ running: false, restarting: false, action: '', plugin_id: '', phase: '', logs: [], error: '', errKind: '' })

const inputClass = 'w-full border border-zinc-300 rounded-md px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:border-zinc-400 transition-shadow bg-white'
const canBrowse = computed(() => info.value?.enabled && info.value?.mode === 'binary' && info.value?.configured && !status.running && !status.restarting && !rebooting.value)
const canOperate = computed(() => info.value?.enabled && info.value?.mode === 'binary' && info.value?.configured && !status.running && !status.restarting && !rebooting.value)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return plugins.value.filter((p) => {
    if (tab.value === 'installed' && !p.installed) return false
    if (tab.value === 'updatable' && !p.update_available) return false
    if (!kw) return true
    return [p.name, p.description, p.author, (p.tags || []).join(' ')].join(' ').toLowerCase().includes(kw)
  })
})

const renderedReadme = computed(() => {
  if (!detail.value?.readme) return ''
  return DOMPurify.sanitize(marked.parse(detail.value.readme, { gfm: true, breaks: true }))
})

let timer = null
onMounted(() => {
  load()
  pollStatus()
  timer = setInterval(() => {
    if (!document.hidden) pollStatus()
  }, 1500)
})
onUnmounted(() => clearInterval(timer))

async function load(refresh = false) {
  loading.value = true
  listError.value = ''
  try {
    info.value = await api.getMarketplaceInfo()
    if (info.value.enabled && info.value.mode === 'binary' && info.value.configured) {
      const data = await api.getMarketplacePlugins(refresh)
      plugins.value = data.plugins || []
    }
  } catch (e) {
    listError.value = e.message
  } finally {
    loading.value = false
  }
}

async function openDetail(id) {
  try {
    detail.value = await api.getMarketplaceDetail(id)
  } catch (e) {
    listError.value = e.message
  }
}

async function onSaveToken() {
  tokenMsg.value = ''
  try {
    await api.setMarketplaceToken(tokenInput.value)
    tokenOk.value = true
    tokenMsg.value = tokenInput.value ? 'Token 已保存，立即生效' : 'Token 已清除'
    tokenInput.value = ''
    info.value = await api.getMarketplaceInfo()
  } catch (e) {
    tokenOk.value = false
    tokenMsg.value = e.message
  }
}

async function onInstall(p) {
  const verb = p.installed ? '升级' : '安装'
  if (!confirm(`确定要${verb}插件「${p.name}」吗？\n\n将下载插件源码、重新编译并重启 Bot（约需几分钟）。\n\n⚠️ 安装插件等于在本机执行插件代码，请确认来源可信。`)) return
  started.value = true
  try {
    await api.installMarketplacePlugin(p.id)
    await pollStatus()
  } catch (e) {
    listError.value = e.message
  }
}

async function onUninstall(p) {
  if (!confirm(`确定要卸载插件「${p.name}」吗？将重新编译并重启 Bot。`)) return
  started.value = true
  try {
    await api.uninstallMarketplacePlugin(p.id)
    await pollStatus()
  } catch (e) {
    listError.value = e.message
  }
}

async function onRollback() {
  if (!confirm('确定要回滚到上次插件操作前的版本吗？将恢复旧二进制并重启 Bot。')) return
  started.value = true
  try {
    await api.rollbackMarketplace()
    await pollStatus()
  } catch (e) {
    listError.value = e.message
  }
}

async function pollStatus() {
  try {
    const s = await api.getMarketplaceStatus()
    const wasRunning = status.running
    Object.assign(status, s)
    if (s.running) started.value = true
    if (started.value && (wasRunning || s.phase === 'done') && !s.running && s.phase === 'done' && !rebooting.value) {
      waitReboot()
    }
    nextTick(scrollLog)
  } catch { /* 服务可能正在重启 */ }
}

async function waitReboot() {
  rebooting.value = true
  const up = await api.waitUntilUp(120000)
  if (up) {
    location.reload()
  } else {
    rebooting.value = false
    alert('等待重启超时，请检查 Bot 运行状态')
  }
}

function scrollLog() {
  if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
}

function phaseIndex(key) {
  if (key === 'done' || key === 'restart') return phaseOrder.length
  return phaseOrder.indexOf(key)
}
function phaseDone(key) {
  return phaseIndex(status.phase) > phaseIndex(key)
}
function phaseClass(key) {
  if (status.error && status.phase === key) return 'border-red-400 text-red-500'
  if (phaseDone(key)) return 'border-emerald-500 bg-emerald-500 text-white'
  if (status.phase === key) return 'border-zinc-900 text-zinc-900'
  return 'border-zinc-300 text-zinc-400'
}
function phaseTextClass(key) {
  if (status.error && status.phase === key) return 'text-red-600 font-semibold'
  if (status.phase === key && status.running) return 'text-zinc-900 font-semibold'
  if (phaseDone(key)) return 'text-zinc-600'
  return 'text-zinc-400'
}
function logLineClass(l) {
  if (l.startsWith('✗')) return 'text-red-400'
  if (l.startsWith('==')) return 'text-zinc-100 font-semibold mt-2'
  if (l.startsWith('$')) return 'text-zinc-500'
  return ''
}
</script>
