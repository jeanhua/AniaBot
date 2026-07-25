<template>
  <div class="space-y-4">
    <div v-if="saved" class="bg-amber-50 border border-amber-200 text-amber-800 text-sm rounded-lg px-4 py-3">
      已保存，将在 <b>重启 Bot 后生效</b>。
    </div>

    <div class="flex gap-1 border-b border-slate-200">
      <button
        v-for="tab in tabs"
        :key="tab.name"
        class="px-4 py-2 text-sm rounded-t"
        :class="current === tab.name ? 'bg-white border border-b-white border-slate-200 -mb-px text-indigo-600 font-medium' : 'text-slate-500 hover:text-slate-700'"
        @click="switchTab(tab.name)"
      >
        {{ tab.label }}
      </button>
    </div>

    <section class="bg-white rounded-lg shadow-sm p-5 space-y-3">
      <p class="text-xs text-slate-500">{{ currentTab.desc }}</p>
      <textarea
        v-model="content"
        rows="20"
        spellcheck="false"
        placeholder='{"servers": [...]}'
        class="w-full border border-slate-300 rounded px-3 py-2 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-indigo-400"
      />
      <div class="flex items-center gap-3">
        <button class="px-3 py-1.5 text-sm rounded border border-slate-300 text-slate-600 hover:bg-slate-50" @click="format">格式化</button>
        <button :disabled="saving" class="px-4 py-1.5 text-sm rounded bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40" @click="onSave">
          {{ saving ? '保存中...' : '保存' }}
        </button>
        <span v-if="error" class="text-sm text-red-600">{{ error }}</span>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api.js'

const tabs = [
  { name: 'mcp', label: 'MCP 服务器', desc: '原 aniabot.mcp.json 的内容。格式: {"servers": [{name, transport(stdio/streamable/sse), command/endpoint, args, env, headers, timeout, description}]}' },
  { name: 'prompt', label: 'Prompt 覆盖', desc: '原 aniabot.prompt.json 的内容，按群/好友覆盖系统提示词。格式: {"groups": {"群号": "prompt"}, "friends": {"QQ号": "prompt"}}' },
]

const current = ref('mcp')
const content = ref('')
const error = ref('')
const saved = ref(false)
const saving = ref(false)

const currentTab = computed(() => tabs.find((t) => t.name === current.value))

async function load() {
  error.value = ''
  const data = await api.getFile(current.value)
  content.value = data.content || ''
}

function switchTab(name) {
  current.value = name
  saved.value = false
  load()
}

function format() {
  error.value = ''
  if (content.value.trim() === '') return
  try {
    content.value = JSON.stringify(JSON.parse(content.value), null, 2)
  } catch {
    error.value = 'JSON 格式错误'
  }
}

async function onSave() {
  error.value = ''
  saving.value = true
  try {
    await api.saveFile(current.value, content.value)
    saved.value = true
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
