<template>
  <div class="space-y-4">
    <div v-if="saved" class="bg-amber-50 border border-amber-200 text-amber-800 text-sm rounded-lg px-4 py-3">
      配置已保存到数据库，将在 <b>重启 Bot 后生效</b>。
    </div>

    <div class="flex items-center justify-between">
      <p class="text-sm text-slate-500">配置存储在数据库中，修改保存后重启生效。</p>
      <div class="flex gap-2">
        <button class="px-3 py-1.5 text-sm rounded border border-slate-300 text-slate-600 hover:bg-slate-50" @click="rawMode = !rawMode">
          {{ rawMode ? '表单模式' : '高级模式 (JSON)' }}
        </button>
        <button v-if="!rawMode" :disabled="!dirty || saving" class="px-4 py-1.5 text-sm rounded bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40" @click="onSave">
          {{ saving ? '保存中...' : '保存修改' }}
        </button>
      </div>
    </div>

    <!-- 表单模式 -->
    <template v-if="!rawMode">
      <section v-for="group in groups" :key="group.name" class="bg-white rounded-lg shadow-sm">
        <h2 class="px-5 py-3.5 text-sm font-semibold text-slate-700 border-b border-slate-100">{{ group.name }}</h2>
        <div class="p-5 grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-4">
          <div v-for="field in group.fields" :key="field.key" :class="{ 'lg:col-span-2': ['text', 'strings', 'ints'].includes(field.type) }">
            <label class="block text-xs font-medium text-slate-600 mb-1">
              {{ field.label }}
              <span class="text-slate-400 font-normal ml-1">{{ field.key }}</span>
            </label>

            <input v-if="field.type === 'string'" v-model="form[field.key]" type="text" :placeholder="placeholderOf(field)" :class="inputClass" />
            <input v-else-if="field.type === 'password'" v-model="form[field.key]" type="password" :placeholder="placeholderOf(field)" :class="inputClass" />
            <input v-else-if="field.type === 'int'" v-model="form[field.key]" type="number" step="1" :placeholder="placeholderOf(field)" :class="inputClass" />
            <input v-else-if="field.type === 'float'" v-model="form[field.key]" type="number" step="any" :placeholder="placeholderOf(field)" :class="inputClass" />
            <label v-else-if="field.type === 'bool'" class="inline-flex items-center gap-2 cursor-pointer select-none">
              <input type="checkbox" v-model="form[field.key]" class="w-4 h-4 accent-indigo-600" />
              <span class="text-sm text-slate-600">{{ form[field.key] ? '已启用' : '已关闭' }}</span>
            </label>
            <textarea v-else-if="field.type === 'text'" v-model="form[field.key]" rows="4" :placeholder="placeholderOf(field)" :class="inputClass + ' font-mono'" />
            <textarea v-else-if="field.type === 'strings' || field.type === 'ints'" v-model="form[field.key]" rows="3" placeholder="每行一个" :class="inputClass + ' font-mono'" />

            <p v-if="field.help" class="text-xs text-slate-400 mt-1">{{ field.help }}</p>
          </div>
        </div>
      </section>
    </template>

    <!-- 高级模式：原始 JSON -->
    <section v-else class="bg-white rounded-lg shadow-sm p-5 space-y-3">
      <p class="text-xs text-slate-500">全部配置键的扁平 JSON 视图（键为小写点分路径）。编辑后点击保存。</p>
      <textarea v-model="rawText" rows="24" class="w-full border border-slate-300 rounded px-3 py-2 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-indigo-400" />
      <div class="flex items-center gap-3">
        <button class="px-3 py-1.5 text-sm rounded border border-slate-300 text-slate-600 hover:bg-slate-50" @click="formatRaw">格式化</button>
        <button :disabled="saving" class="px-4 py-1.5 text-sm rounded bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40" @click="onSaveRaw">保存</button>
        <span v-if="rawError" class="text-sm text-red-600">{{ rawError }}</span>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '../api.js'

const MASK = '********'
const inputClass = 'w-full border border-slate-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400'

const schema = ref([])
const values = ref({})
const form = reactive({}) // field.key -> 编辑中的字符串/布尔值
const original = reactive({})
const saved = ref(false)
const saving = ref(false)
const rawMode = ref(false)
const rawText = ref('')
const rawError = ref('')

const groups = computed(() => {
  const map = new Map()
  for (const f of schema.value) {
    if (!map.has(f.group)) map.set(f.group, [])
    map.get(f.group).push(f)
  }
  return [...map.entries()].map(([name, fields]) => ({ name, fields }))
})

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

onMounted(async () => {
  const [s, v] = await Promise.all([api.getSchema(), api.getConfig()])
  schema.value = s
  values.value = v
  for (const f of s) {
    form[f.key] = toFormValue(f)
    original[f.key] = form[f.key]
  }
  rawText.value = JSON.stringify(v, null, 2)
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
