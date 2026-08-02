<template>
  <div class="space-y-5">
    <Transition name="fade">
      <div v-if="saved" class="bg-emerald-50 border border-emerald-200 text-emerald-800 text-sm rounded-xl px-4 py-3">
        已保存，将在 <b>重启 Bot 后生效</b>。
      </div>
    </Transition>

    <div class="flex items-center justify-between">
      <div class="flex gap-1 bg-white border border-slate-200 rounded-lg p-1 shadow-sm">
        <button
          v-for="tab in tabs"
          :key="tab.name"
          class="px-4 py-1.5 text-sm rounded-md transition-colors"
          :class="current === tab.name ? 'bg-zinc-900 text-white font-medium shadow-sm' : 'text-slate-500 hover:text-slate-800'"
          @click="switchTab(tab.name)"
        >
          {{ tab.label }}
        </button>
      </div>
      <div class="flex gap-2">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-slate-300 text-slate-600 hover:bg-slate-50 transition-colors" @click="toggleRaw">
          {{ rawMode ? '表单模式' : '源码模式 (JSON)' }}
        </button>
        <button v-if="!rawMode" :disabled="saving" class="px-4 py-1.5 text-sm rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 disabled:opacity-40 transition-colors" @click="onSave">
          {{ saving ? '保存中...' : '保存' }}
        </button>
      </div>
    </div>

    <!-- 源码模式：原始 JSON -->
    <section v-if="rawMode" class="bg-white rounded-xl shadow-sm border border-slate-200/60 p-6 space-y-3">
      <p class="text-xs text-slate-500">{{ currentTab.desc }}</p>
      <textarea
        v-model="rawText"
        rows="20"
        spellcheck="false"
        class="w-full bg-zinc-950 text-slate-200 rounded-lg px-4 py-3 text-xs font-mono leading-relaxed focus:outline-none focus:ring-2 focus:ring-zinc-400"
      />
      <div class="flex items-center gap-3">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-slate-300 text-slate-600 hover:bg-slate-50 transition-colors" @click="formatRaw">格式化</button>
        <button :disabled="saving" class="px-4 py-1.5 text-sm rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 disabled:opacity-40 transition-colors" @click="onSaveRaw">
          {{ saving ? '保存中...' : '保存' }}
        </button>
        <span v-if="error" class="text-sm text-red-600">{{ error }}</span>
      </div>
    </section>

    <!-- MCP 服务器：图形化表单 -->
    <template v-else-if="current === 'mcp'">
      <p class="text-xs text-slate-500">配置 AI 可调用的 MCP 服务器，修改保存后重启生效。</p>

      <section v-for="(srv, i) in mcpServers" :key="i" class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
        <div class="px-6 py-3.5 border-b border-slate-100 flex items-center justify-between">
          <h2 class="text-sm font-semibold text-slate-800 flex items-center gap-2">
            <span class="w-2 h-2 rounded-full" :class="srv.name ? 'bg-emerald-400' : 'bg-slate-300'" />
            {{ srv.name || `服务器 ${i + 1}` }}
          </h2>
          <button class="text-xs text-red-500 hover:text-red-700 transition-colors" @click="mcpServers.splice(i, 1)">删除</button>
        </div>
        <div class="p-6 grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-5">
          <div>
            <label :class="labelClass">名称 <span class="text-red-500">*</span></label>
            <input v-model="srv.name" type="text" placeholder="如 weather" :class="inputClass" />
          </div>
          <div>
            <label :class="labelClass">连接方式</label>
            <select v-model="srv.transport" :class="inputClass">
              <option value="stdio">stdio（本地命令）</option>
              <option value="streamable">streamable（HTTP）</option>
              <option value="sse">sse（HTTP）</option>
            </select>
          </div>
          <div>
            <label :class="labelClass">超时时间（秒）</label>
            <input v-model="srv.timeout" type="number" min="0" step="1" placeholder="默认" :class="inputClass" />
          </div>
          <div>
            <label :class="labelClass">描述</label>
            <input v-model="srv.description" type="text" placeholder="这个服务器是做什么的" :class="inputClass" />
          </div>

          <!-- stdio 字段 -->
          <template v-if="srv.transport === 'stdio'">
            <div>
              <label :class="labelClass">启动命令 <span class="text-red-500">*</span></label>
              <input v-model="srv.command" type="text" placeholder="如 npx / uvx / node" :class="inputClass" />
            </div>
            <div>
              <label :class="labelClass">命令参数</label>
              <input v-model="srv.argsText" type="text" placeholder="以空格分隔，如 -y @modelcontextprotocol/server-everything" :class="inputClass" />
            </div>
            <div class="lg:col-span-2">
              <label :class="labelClass">环境变量</label>
              <KvEditor v-model="srv.env" key-placeholder="变量名" value-placeholder="值" />
            </div>
          </template>

          <!-- HTTP 字段 -->
          <template v-else>
            <div class="lg:col-span-2">
              <label :class="labelClass">服务地址 (Endpoint) <span class="text-red-500">*</span></label>
              <input v-model="srv.endpoint" type="text" placeholder="如 http://localhost:8080/mcp" :class="inputClass" />
            </div>
            <div class="lg:col-span-2">
              <label :class="labelClass">请求头</label>
              <KvEditor v-model="srv.headers" key-placeholder="Header 名" value-placeholder="值" />
            </div>
          </template>
        </div>
      </section>

      <button class="w-full py-3 text-sm rounded-xl border-2 border-dashed border-slate-300 text-slate-400 hover:border-zinc-500 hover:text-zinc-700 hover:bg-zinc-100 transition-all" @click="addServer">
        + 添加 MCP 服务器
      </button>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
    </template>

    <!-- Prompt 覆盖：图形化表单 -->
    <template v-else>
      <p class="text-xs text-slate-500">按群聊 / 好友覆盖 AI 的系统提示词，留空的条目保存时会被忽略。修改保存后重启生效。</p>

      <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
        <div class="px-6 py-3.5 border-b border-slate-100 flex items-center justify-between">
          <h2 class="text-sm font-semibold text-slate-800">群聊覆盖</h2>
          <button class="text-xs text-zinc-700 hover:text-zinc-900 font-medium transition-colors" @click="promptGroups.push({ id: '', prompt: '' })">+ 添加</button>
        </div>
        <div class="p-6 space-y-5">
          <p v-if="promptGroups.length === 0" class="text-xs text-slate-400">暂无群聊 Prompt 覆盖</p>
          <div v-for="(item, i) in promptGroups" :key="i" class="space-y-2 bg-slate-50/60 rounded-lg p-4 border border-slate-100">
            <div class="flex items-center gap-2">
              <input v-model="item.id" type="text" placeholder="群 ID（如 123456 或 fs:oc_xxx）" :class="inputClass + ' !w-48 bg-white'" />
              <button class="text-xs text-red-500 hover:text-red-700 shrink-0 transition-colors" @click="promptGroups.splice(i, 1)">删除</button>
            </div>
            <textarea v-model="item.prompt" rows="3" placeholder="该群使用的系统提示词" :class="inputClass + ' bg-white'" />
          </div>
        </div>
      </section>

      <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
        <div class="px-6 py-3.5 border-b border-slate-100 flex items-center justify-between">
          <h2 class="text-sm font-semibold text-slate-800">好友覆盖</h2>
          <button class="text-xs text-zinc-700 hover:text-zinc-900 font-medium transition-colors" @click="promptFriends.push({ id: '', prompt: '' })">+ 添加</button>
        </div>
        <div class="p-6 space-y-5">
          <p v-if="promptFriends.length === 0" class="text-xs text-slate-400">暂无好友 Prompt 覆盖</p>
          <div v-for="(item, i) in promptFriends" :key="i" class="space-y-2 bg-slate-50/60 rounded-lg p-4 border border-slate-100">
            <div class="flex items-center gap-2">
              <input v-model="item.id" type="text" placeholder="用户 ID（如 123456 或 fs:ou_xxx）" :class="inputClass + ' !w-48 bg-white'" />
              <button class="text-xs text-red-500 hover:text-red-700 shrink-0 transition-colors" @click="promptFriends.splice(i, 1)">删除</button>
            </div>
            <textarea v-model="item.prompt" rows="3" placeholder="该好友会话使用的系统提示词" :class="inputClass + ' bg-white'" />
          </div>
        </div>
      </section>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
    </template>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { api } from '../api.js'

const inputClass = 'w-full border border-slate-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:border-zinc-400 transition-shadow'
const labelClass = 'block text-xs font-medium text-slate-600 mb-1.5'

// 键值对编辑器（环境变量 / 请求头）
const KvEditor = defineComponent({
  props: {
    modelValue: { type: Array, default: () => [] }, // [{k, v}]
    keyPlaceholder: { type: String, default: '键' },
    valuePlaceholder: { type: String, default: '值' },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    const update = (i, field, val) => {
      const next = props.modelValue.map((row, j) => (j === i ? { ...row, [field]: val } : row))
      emit('update:modelValue', next)
    }
    const remove = (i) => emit('update:modelValue', props.modelValue.filter((_, j) => j !== i))
    const add = () => emit('update:modelValue', [...props.modelValue, { k: '', v: '' }])
    return () =>
      h('div', { class: 'space-y-2' }, [
        ...props.modelValue.map((row, i) =>
          h('div', { class: 'flex items-center gap-2', key: i }, [
            h('input', {
              class: inputClass,
              type: 'text',
              placeholder: props.keyPlaceholder,
              value: row.k,
              onInput: (e) => update(i, 'k', e.target.value),
            }),
            h('input', {
              class: inputClass,
              type: 'text',
              placeholder: props.valuePlaceholder,
              value: row.v,
              onInput: (e) => update(i, 'v', e.target.value),
            }),
            h('button', { class: 'text-xs text-red-500 hover:text-red-700 shrink-0 transition-colors', onClick: () => remove(i) }, '删除'),
          ])
        ),
        h(
          'button',
          { class: 'text-xs text-zinc-700 hover:text-zinc-900 transition-colors', onClick: add },
          '+ 添加一行'
        ),
      ])
  },
})

const tabs = [
  { name: 'mcp', label: 'MCP 服务器', desc: '格式: {"servers": [{name, transport(stdio/streamable/sse), command/endpoint, args, env, headers, timeout, description}]}' },
  { name: 'prompt', label: 'Prompt 覆盖', desc: '格式: {"groups": {"群ID": "prompt"}, "friends": {"用户ID": "prompt"}}（QQ 为数字，其他平台带前缀）' },
]

const current = ref('mcp')
const error = ref('')
const saved = ref(false)
const saving = ref(false)
const rawMode = ref(false)
const rawText = ref('')

// 图形化表单模型
const mcpServers = ref([])
const promptGroups = ref([])
const promptFriends = ref([])

const currentTab = computed(() => tabs.find((t) => t.name === current.value))

// ---- 解析：JSON -> 表单模型 ----

function kvToRows(obj) {
  return Object.entries(obj || {}).map(([k, v]) => ({ k, v }))
}

function parseMcp(content) {
  if (!content.trim()) return []
  const data = JSON.parse(content)
  return (data.servers || []).map((s) => {
    const transport = (s.transport || 'stdio').toLowerCase()
    const isHttp = transport === 'streamable' || transport === 'streamable-http' || transport === 'sse'
    return {
      name: s.name || '',
      transport: isHttp ? (transport === 'sse' ? 'sse' : 'streamable') : 'stdio',
      command: s.command || '',
      argsText: (s.args || []).join(' '),
      env: kvToRows(s.env),
      endpoint: s.endpoint || '',
      headers: kvToRows(s.headers),
      timeout: s.timeout ? String(s.timeout) : '',
      description: s.description || '',
    }
  })
}

function parsePrompt(content) {
  if (!content.trim()) return { groups: [], friends: [] }
  const data = JSON.parse(content)
  return {
    groups: Object.entries(data.groups || {}).map(([id, prompt]) => ({ id, prompt })),
    friends: Object.entries(data.friends || {}).map(([id, prompt]) => ({ id, prompt })),
  }
}

// ---- 序列化：表单模型 -> JSON ----

function rowsToKv(rows) {
  const out = {}
  for (const { k, v } of rows) {
    if (k.trim()) out[k.trim()] = v
  }
  return out
}

function serializeMcp() {
  const servers = []
  for (const [i, s] of mcpServers.value.entries()) {
    if (!s.name.trim()) throw new Error(`第 ${i + 1} 个服务器缺少名称`)
    const entry = { name: s.name.trim(), transport: s.transport }
    const timeout = parseInt(s.timeout, 10)
    if (!Number.isNaN(timeout) && timeout > 0) entry.timeout = timeout
    if (s.description.trim()) entry.description = s.description.trim()
    if (s.transport === 'stdio') {
      if (!s.command.trim()) throw new Error(`服务器「${entry.name}」缺少启动命令`)
      entry.command = s.command.trim()
      const args = s.argsText.split(/\s+/).filter(Boolean)
      if (args.length) entry.args = args
      const env = rowsToKv(s.env)
      if (Object.keys(env).length) entry.env = env
    } else {
      if (!s.endpoint.trim()) throw new Error(`服务器「${entry.name}」缺少服务地址`)
      entry.endpoint = s.endpoint.trim()
      const headers = rowsToKv(s.headers)
      if (Object.keys(headers).length) entry.headers = headers
    }
    servers.push(entry)
  }
  return JSON.stringify({ servers }, null, 2)
}

function serializePrompt() {
  const groups = {}
  for (const { id, prompt } of promptGroups.value) {
    if (id.trim() && prompt.trim()) groups[id.trim()] = prompt
  }
  const friends = {}
  for (const { id, prompt } of promptFriends.value) {
    if (id.trim() && prompt.trim()) friends[id.trim()] = prompt
  }
  return JSON.stringify({ groups, friends }, null, 2)
}

// ---- 加载 / 切换 / 保存 ----

async function load() {
  error.value = ''
  const data = await api.getFile(current.value)
  rawText.value = data.content || ''
  try {
    if (current.value === 'mcp') {
      mcpServers.value = parseMcp(rawText.value)
    } else {
      const { groups, friends } = parsePrompt(rawText.value)
      promptGroups.value = groups
      promptFriends.value = friends
    }
  } catch {
    // 已有内容不是合法 JSON，回退到源码模式让用户修复
    rawMode.value = true
    error.value = '现有内容不是合法 JSON，已切换到源码模式'
  }
}

function switchTab(name) {
  current.value = name
  saved.value = false
  rawMode.value = false
  load()
}

function toggleRaw() {
  if (!rawMode.value) {
    // 表单 -> 源码：先用当前表单内容刷新源码，校验失败则保留磁盘上的原文
    try {
      rawText.value = current.value === 'mcp' ? serializeMcp() : serializePrompt()
      error.value = ''
    } catch {
      /* 表单未通过校验，保留原始内容 */
    }
  }
  rawMode.value = !rawMode.value
}

function formatRaw() {
  error.value = ''
  if (rawText.value.trim() === '') return
  try {
    rawText.value = JSON.stringify(JSON.parse(rawText.value), null, 2)
  } catch {
    error.value = 'JSON 格式错误'
  }
}

async function save(content) {
  saving.value = true
  try {
    await api.saveFile(current.value, content)
    saved.value = true
  } catch (e) {
    error.value = e.message
    throw e
  } finally {
    saving.value = false
  }
}

async function onSave() {
  error.value = ''
  try {
    const content = current.value === 'mcp' ? serializeMcp() : serializePrompt()
    await save(content)
    rawText.value = content
  } catch (e) {
    if (!error.value) error.value = e.message
  }
}

async function onSaveRaw() {
  error.value = ''
  try {
    if (rawText.value.trim() !== '') JSON.parse(rawText.value)
  } catch {
    error.value = 'JSON 格式错误'
    return
  }
  try {
    await save(rawText.value)
  } catch { /* error 已设置 */ }
}

function addServer() {
  mcpServers.value.push({
    name: '',
    transport: 'stdio',
    command: '',
    argsText: '',
    env: [],
    endpoint: '',
    headers: [],
    timeout: '',
    description: '',
  })
}

onMounted(load)
</script>
