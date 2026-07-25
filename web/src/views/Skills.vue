<template>
  <div class="space-y-4">
    <!-- 操作栏 -->
    <div class="flex items-center justify-between flex-wrap gap-3">
      <div class="text-xs text-slate-500">
        <template v-if="dir">
          Skills 目录：<span class="font-mono text-zinc-700">{{ dir }}</span>
          <span class="ml-2 text-slate-400">上传/删除立即生效，无需重启</span>
        </template>
      </div>
      <div class="flex items-center gap-3">
        <button class="text-xs text-zinc-700 hover:text-zinc-900 font-medium transition-colors" @click="load">刷新</button>
        <button
          class="text-xs bg-zinc-900 text-white px-3.5 py-2 rounded-lg hover:bg-zinc-700 font-medium transition-colors shadow-sm disabled:opacity-50"
          :disabled="uploading"
          @click="fileInput?.click()"
        >
          {{ uploading ? '上传中...' : '上传 Skill（zip）' }}
        </button>
        <input ref="fileInput" type="file" accept=".zip" class="hidden" @change="onUpload" />
      </div>
    </div>

    <!-- 白名单提示 -->
    <div v-if="whitelist.length" class="bg-amber-50 border border-amber-200/70 rounded-xl px-4 py-3 text-xs text-amber-700">
      当前启用了 Skills 白名单（{{ whitelist.join('、') }}），不在白名单中的 skill 不会被加载。可在「配置管理」中调整。
    </div>

    <!-- 上传结果提示 -->
    <p v-if="msg" class="text-xs" :class="msgOk ? 'text-emerald-600' : 'text-red-600'">{{ msg }}</p>

    <!-- Skill 列表 -->
    <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
      <ul class="divide-y divide-slate-100">
        <li v-if="skills.length === 0" class="py-12 text-sm text-slate-400 text-center list-none">
          暂无已加载的 skill，点击右上角上传 zip 压缩包（需包含 SKILL.md）
        </li>
        <li v-for="s in skills" :key="s.name" class="px-5 py-4 flex items-start gap-4">
          <div class="w-9 h-9 rounded-lg bg-zinc-100 flex items-center justify-center shrink-0 text-zinc-500">
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456Z"/></svg>
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-sm font-semibold text-slate-800">{{ s.name }}</span>
              <span class="text-[11px] font-mono text-slate-400">{{ s.location }}</span>
            </div>
            <p v-if="s.description" class="text-xs text-slate-500 mt-0.5 leading-relaxed">{{ s.description }}</p>
            <div v-if="(s.refs?.length || s.extras?.length)" class="flex items-center gap-1.5 mt-2 flex-wrap">
              <span v-for="f in s.refs" :key="'r' + f" class="text-[11px] px-2 py-0.5 rounded-full bg-zinc-100 text-zinc-600 font-mono">{{ f }}</span>
              <span v-for="f in s.extras" :key="'e' + f" class="text-[11px] px-2 py-0.5 rounded-full bg-zinc-900/5 text-zinc-500 font-mono border border-zinc-200/60">{{ f }}</span>
            </div>
          </div>
          <button
            class="text-xs text-red-500 hover:text-red-600 hover:bg-red-50 px-2.5 py-1.5 rounded-lg font-medium transition-colors shrink-0"
            @click="onDelete(s)"
          >
            删除
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'

const skills = ref([])
const dir = ref('')
const whitelist = ref([])
const fileInput = ref(null)
const uploading = ref(false)
const msg = ref('')
const msgOk = ref(false)

async function load() {
  try {
    const data = await api.getSkills()
    skills.value = data.skills || []
    dir.value = data.dir || ''
    whitelist.value = data.whitelist || []
  } catch (e) {
    msgOk.value = false
    msg.value = e.message
  }
}

async function onUpload(e) {
  const file = e.target.files?.[0]
  e.target.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.zip')) {
    msgOk.value = false
    msg.value = '请上传 zip 格式的压缩包'
    return
  }
  uploading.value = true
  msg.value = ''
  try {
    await api.uploadSkill(file)
    msgOk.value = true
    msg.value = `「${file.name}」上传成功`
    await load()
  } catch (err) {
    msgOk.value = false
    msg.value = err.message
  } finally {
    uploading.value = false
  }
}

async function onDelete(s) {
  if (!confirm(`确定要删除 skill「${s.name}」吗？将同时删除其目录下的所有文件。`)) return
  msg.value = ''
  try {
    await api.deleteSkill(s.name)
    msgOk.value = true
    msg.value = `「${s.name}」已删除`
    await load()
  } catch (e) {
    msgOk.value = false
    msg.value = e.message
  }
}

onMounted(load)
</script>
