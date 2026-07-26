<template>
  <div class="flex gap-6 items-start">
    <!-- 分组导航 -->
    <aside v-if="!rawMode" class="w-52 shrink-0 sticky top-24 space-y-5 max-h-[calc(100vh-8rem)] overflow-y-auto pr-1">
      <div class="relative">
        <span class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 [&>svg]:w-4 [&>svg]:h-4" v-html="iconSearch" />
        <input
          v-model="search"
          type="text"
          placeholder="搜索配置项..."
          class="w-full bg-white border border-slate-200 rounded-lg pl-9 pr-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:border-zinc-400 transition-shadow"
        />
      </div>

      <div v-for="cat in categorized" :key="cat.name">
        <p class="px-3 mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-400">{{ cat.name }}</p>
        <nav class="space-y-0.5">
          <button
            v-for="g in cat.groups"
            :key="g.name"
            class="w-full flex items-center justify-between px-3 py-1.5 rounded-lg text-[13px] transition-colors"
            :class="activeGroup === g.name
              ? 'bg-zinc-100 text-zinc-900 font-medium'
              : 'text-slate-600 hover:bg-slate-200/60'"
            @click="jumpTo(g.name)"
          >
            <span class="truncate">{{ shortName(g.name) }}</span>
            <span class="text-[11px] text-slate-400 ml-2 shrink-0">{{ g.fields.length }}</span>
          </button>
        </nav>
      </div>
      <p v-if="categorized.length === 0" class="px-3 text-xs text-slate-400">没有匹配「{{ search }}」的配置项</p>
    </aside>

    <!-- 配置主体 -->
    <div class="flex-1 min-w-0 space-y-5">
      <Transition name="fade">
        <div v-if="saved" class="bg-emerald-50 border border-emerald-200 text-emerald-800 text-sm rounded-xl px-4 py-3 flex items-center gap-2">
          <span class="[&>svg]:w-4 [&>svg]:h-4" v-html="iconCheck" />
          配置已保存到数据库，将在 <b>重启 Bot 后生效</b>。
        </div>
      </Transition>

      <div class="flex items-center justify-between">
        <p class="text-sm text-slate-500">配置存储在数据库中，修改保存后重启生效。</p>
        <button class="px-3 py-1.5 text-sm rounded-lg border border-slate-300 text-slate-600 hover:bg-slate-50 transition-colors" @click="rawMode = !rawMode">
          {{ rawMode ? '表单模式' : '高级模式 (JSON)' }}
        </button>
      </div>

      <!-- 表单模式 -->
      <template v-if="!rawMode">
        <section
          v-for="group in filteredGroups"
          :key="group.name"
          :id="sectionId(group.name)"
          class="bg-white rounded-xl shadow-sm border border-slate-200/60 scroll-mt-24 overflow-hidden"
        >
          <h2 class="px-6 py-4 text-sm font-semibold text-slate-800 border-b border-slate-100 flex items-center gap-2.5">
            <span class="w-7 h-7 rounded-lg flex items-center justify-center text-white text-xs [&>svg]:w-4 [&>svg]:h-4" :class="groupColor(group.name)" v-html="groupIcon(group.name)" />
            {{ group.name }}
            <span class="text-xs font-normal text-slate-400">{{ group.fields.length }} 项</span>
          </h2>
          <div class="p-6 grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-5">
            <div v-for="field in group.fields" :key="field.key" :class="{ 'lg:col-span-2': ['text', 'strings', 'ints'].includes(field.type) }">
              <label class="block text-xs font-medium text-slate-700 mb-1.5">
                {{ field.label }}
                <span class="text-slate-400 font-normal ml-1">{{ field.key }}</span>
              </label>

              <input v-if="field.type === 'string'" v-model="form[field.key]" type="text" :placeholder="placeholderOf(field)" :class="inputClass" />
              <input v-else-if="field.type === 'password'" v-model="form[field.key]" type="password" :placeholder="placeholderOf(field)" :class="inputClass" />
              <input v-else-if="field.type === 'int'" v-model="form[field.key]" type="number" step="1" :placeholder="placeholderOf(field)" :class="inputClass" />
              <input v-else-if="field.type === 'float'" v-model="form[field.key]" type="number" step="any" :placeholder="placeholderOf(field)" :class="inputClass" />
              <select v-else-if="field.type === 'select'" v-model="form[field.key]" :class="inputClass">
                <option v-for="opt in field.options || []" :key="opt" :value="opt">{{ opt }}</option>
              </select>

              <label v-else-if="field.type === 'bool'" class="inline-flex items-center gap-2.5 cursor-pointer select-none py-1" @click.prevent="form[field.key] = !form[field.key]">
                <span
                  class="relative inline-flex w-9 h-5 rounded-full transition-colors duration-200"
                  :class="form[field.key] ? 'bg-zinc-900' : 'bg-slate-300'"
                >
                  <span
                    class="absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-all duration-200"
                    :class="form[field.key] ? 'left-4.5' : 'left-0.5'"
                  />
                </span>
                <span class="text-sm" :class="form[field.key] ? 'text-slate-700' : 'text-slate-400'">{{ form[field.key] ? '已启用' : '已关闭' }}</span>
              </label>

              <textarea v-else-if="field.type === 'text'" v-model="form[field.key]" rows="4" :placeholder="placeholderOf(field)" :class="inputClass + ' font-mono'" />
              <textarea v-else-if="field.type === 'strings' || field.type === 'ints'" v-model="form[field.key]" rows="3" placeholder="每行一个" :class="inputClass + ' font-mono'" />

              <p v-if="field.help" class="text-xs text-slate-400 mt-1.5">{{ field.help }}</p>
              <p
                v-if="field.key === 'bot.admin_panel.enable' && form[field.key] === false"
                class="text-xs text-amber-600 mt-1.5"
              >
                关闭并重启后将无法访问本面板。如需重新开启，可设置环境变量
                <code class="font-mono bg-amber-50 px-1 rounded">ANIA_BOT_ADMIN_PANEL_ENABLE=true</code>
                覆盖配置后重启 Bot。
              </p>
            </div>
          </div>
        </section>
      </template>

      <!-- 高级模式：原始 JSON -->
      <section v-else class="bg-white rounded-xl shadow-sm border border-slate-200/60 p-6 space-y-3">
        <p class="text-xs text-slate-500">全部配置键的扁平 JSON 视图（键为小写点分路径）。编辑后点击保存。</p>
        <textarea v-model="rawText" rows="24" spellcheck="false" class="w-full bg-zinc-950 text-slate-200 rounded-lg px-4 py-3 text-xs font-mono leading-relaxed focus:outline-none focus:ring-2 focus:ring-zinc-400" />
        <div class="flex items-center gap-3">
          <button class="px-3 py-1.5 text-sm rounded-lg border border-slate-300 text-slate-600 hover:bg-slate-50 transition-colors" @click="formatRaw">格式化</button>
          <button :disabled="saving" class="px-4 py-1.5 text-sm rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 disabled:opacity-40 transition-colors" @click="onSaveRaw">保存</button>
          <span v-if="rawError" class="text-sm text-red-600">{{ rawError }}</span>
        </div>
      </section>
    </div>

    <!-- 浮动保存条 -->
    <Transition name="fade">
      <div
        v-if="!rawMode && dirty"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 z-40 flex items-center gap-4 bg-zinc-900 text-white rounded-full pl-5 pr-2 py-2 shadow-2xl shadow-slate-900/30"
      >
        <span class="text-sm text-slate-300">有未保存的修改</span>
        <button class="text-sm text-slate-400 hover:text-white transition-colors" @click="resetForm">放弃</button>
        <button
          :disabled="saving"
          class="px-4 py-1.5 text-sm rounded-full bg-white text-zinc-900 hover:bg-zinc-200 disabled:opacity-40 transition-colors font-medium"
          @click="onSave"
        >
          {{ saving ? '保存中...' : '保存修改' }}
        </button>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api.js'

const MASK = '********'
const inputClass = 'w-full border border-slate-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:border-zinc-400 transition-shadow'

const iconSearch = '<svg fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"/></svg>'
const iconCheck = '<svg fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"/></svg>'
const iconSpark = '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456Z"/></svg>'
const iconPuzzle = '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M14.25 6.087c0-.355.186-.676.401-.959.221-.29.349-.634.349-1.003 0-1.036-1.007-1.875-2.25-1.875s-2.25.84-2.25 1.875c0 .369.128.713.349 1.003.215.283.401.604.401.959v.431c0 .46-.335.84-.782.927a7.59 7.59 0 0 1-1.181.093h-.77c-.254 0-.487.09-.668.24-.297.246-.451.619-.371 1.014.073.361.026.74-.145 1.086-.199.402-.576.65-1.007.65H7.5c-.621 0-1.125.504-1.125 1.125v.77c0 .418.314.82.77 1.118.198.13.37.305.48.515.16.308.165.674.014.97-.168.333-.502.521-.864.521H4.875A1.875 1.875 0 0 1 3 14.25v-1.77c0-.358-.215-.68-.543-.822A1.87 1.87 0 0 0 1.875 9.75c0-1.243 1.007-2.25 2.25-2.25.369 0 .713.128 1.003.349.283.215.604.401.959.401h.413"/></svg>'
const iconCube = '<svg fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9"/></svg>'

const schema = ref([])
const values = ref({})
const form = reactive({}) // field.key -> 编辑中的字符串/布尔值
const original = reactive({})
const saved = ref(false)
const saving = ref(false)
const rawMode = ref(false)
const rawText = ref('')
const rawError = ref('')
const search = ref('')
const activeGroup = ref('')

const groups = computed(() => {
  const map = new Map()
  for (const f of schema.value) {
    if (!map.has(f.group)) map.set(f.group, [])
    map.get(f.group).push(f)
  }
  return [...map.entries()].map(([name, fields]) => ({ name, fields }))
})

// 分组归类：框架基础 / AI 对话 / 插件
const FRAMEWORK = new Set(['Bot 基础', 'Web 面板', 'NapCat 适配器', '缓存存储'])
const CAT_ORDER = ['框架基础', 'AI 对话', '插件']

function categoryOf(groupName) {
  if (FRAMEWORK.has(groupName)) return '框架基础'
  if (groupName.startsWith('AI 对话')) return 'AI 对话'
  return '插件'
}

// 搜索过滤后的分组，按分类排序（与左侧导航一致，插件在最后）
const filteredGroups = computed(() => {
  const q = search.value.trim().toLowerCase()
  const list = !q
    ? groups.value
    : groups.value
      .map((g) => ({
        name: g.name,
        fields: g.fields.filter((f) =>
          f.label.toLowerCase().includes(q) ||
          f.key.toLowerCase().includes(q) ||
          (f.help || '').toLowerCase().includes(q)
        ),
      }))
      .filter((g) => g.fields.length > 0)
  return [...list].sort((a, b) => CAT_ORDER.indexOf(categoryOf(a.name)) - CAT_ORDER.indexOf(categoryOf(b.name)))
})

const categorized = computed(() => {
  const map = new Map(CAT_ORDER.map((n) => [n, []]))
  for (const g of filteredGroups.value) {
    map.get(categoryOf(g.name)).push(g)
  }
  return CAT_ORDER.map((name) => ({ name, groups: map.get(name) })).filter((c) => c.groups.length > 0)
})

function shortName(groupName) {
  return groupName.replace('AI 对话 · ', '')
}

function sectionId(name) {
  return 'grp-' + name.replace(/[^\w一-龥]+/g, '-')
}

function jumpTo(name) {
  activeGroup.value = name
  document.getElementById(sectionId(name))?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function groupColor(name) {
  const cat = categoryOf(name)
  return {
    '框架基础': 'bg-zinc-500',
    'AI 对话': 'bg-zinc-900',
    '插件': 'bg-zinc-700',
  }[cat]
}

function groupIcon(name) {
  const cat = categoryOf(name)
  return { '框架基础': iconCube, 'AI 对话': iconSpark, '插件': iconPuzzle }[cat]
}

const dirty = computed(() => schema.value.some((f) => form[f.key] !== original[f.key]))

function valueOf(key) {
  return values.value[key.toLowerCase()]
}

function placeholderOf(field) {
  const v = valueOf(field.key)
  if (field.sensitive && v === MASK) return '已设置（留空保持不变）'
  if (v === undefined || v === null || v === '') return '未设置'
  return ''
}

function toFormValue(field) {
  const v = valueOf(field.key)
  if (field.type === 'bool') return v === true
  if (v === undefined || v === null) return ''
  if (field.type === 'strings' || field.type === 'ints') return Array.isArray(v) ? v.join('\n') : ''
  if (field.sensitive) return '' // 敏感字段不回显，留空表示不修改
  return String(v)
}

function fromFormValue(field) {
  const raw = form[field.key]
  if (field.type === 'bool') return raw === true
  if (field.type === 'int') { const n = parseInt(raw, 10); return Number.isNaN(n) ? 0 : n }
  if (field.type === 'float') { const n = parseFloat(raw); return Number.isNaN(n) ? 0 : n }
  if (field.type === 'strings') return raw.split('\n').map((s) => s.trim()).filter(Boolean)
  if (field.type === 'ints') return raw.split('\n').map((s) => parseInt(s.trim(), 10)).filter((n) => !Number.isNaN(n))
  return raw
}

function resetForm() {
  for (const f of schema.value) form[f.key] = original[f.key]
}

onMounted(async () => {
  const [s, v] = await Promise.all([api.getSchema(), api.getConfig()])
  schema.value = s
  values.value = v
  for (const f of s) {
    form[f.key] = toFormValue(f)
    original[f.key] = form[f.key]
  }
  rawText.value = JSON.stringify(v, null, 2)
  if (groups.value.length) activeGroup.value = groups.value[0].name
})

async function onSave() {
  const updates = {}
  for (const f of schema.value) {
    if (form[f.key] === original[f.key]) continue
    if (f.sensitive && form[f.key] === '') continue // 未填写 = 不修改
    updates[f.key] = fromFormValue(f)
  }
  if (Object.keys(updates).length === 0) return
  saving.value = true
  try {
    await api.saveConfig(updates)
    saved.value = true
    // 重新加载，同步掩码与原始值
    values.value = await api.getConfig()
    for (const f of schema.value) {
      form[f.key] = toFormValue(f)
      original[f.key] = form[f.key]
    }
    rawText.value = JSON.stringify(values.value, null, 2)
  } catch (e) {
    alert(e.message)
  } finally {
    saving.value = false
  }
}

function formatRaw() {
  rawError.value = ''
  try {
    rawText.value = JSON.stringify(JSON.parse(rawText.value), null, 2)
  } catch {
    rawError.value = 'JSON 格式错误'
  }
}

async function onSaveRaw() {
  rawError.value = ''
  let parsed
  try {
    parsed = JSON.parse(rawText.value)
  } catch {
    rawError.value = 'JSON 格式错误'
    return
  }
  saving.value = true
  try {
    await api.saveConfig(parsed)
    saved.value = true
    values.value = await api.getConfig()
    for (const f of schema.value) {
      form[f.key] = toFormValue(f)
      original[f.key] = form[f.key]
    }
  } catch (e) {
    rawError.value = e.message
  } finally {
    saving.value = false
  }
}
</script>
