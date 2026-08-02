<template>
  <div class="space-y-5">
    <div v-if="error" class="bg-red-50 border border-red-200 text-red-700 text-sm rounded-xl px-4 py-3">{{ error }}</div>

    <div class="flex items-center justify-between">
      <div class="flex gap-1 bg-white border border-slate-200 rounded-lg p-1 shadow-sm">
        <button
          v-for="tab in [['groups', '群列表'], ['friends', '好友列表']]"
          :key="tab[0]"
          class="px-4 py-1.5 text-sm rounded-md transition-colors"
          :class="current === tab[0] ? 'bg-zinc-900 text-white font-medium shadow-sm' : 'text-slate-500 hover:text-slate-800'"
          @click="switchTab(tab[0])"
        >
          {{ tab[1] }}
        </button>
      </div>
      <span class="text-xs text-slate-400">共 {{ current === 'groups' ? groups.length : friends.length }} 条</span>
    </div>

    <section class="bg-white rounded-xl shadow-sm border border-slate-200/60 overflow-hidden">
      <table v-if="current === 'groups'" class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-400 bg-slate-50/60 border-b border-slate-100">
            <th class="px-6 py-3 font-medium">群 ID</th>
            <th class="px-6 py-3 font-medium">群名称</th>
            <th class="px-6 py-3 font-medium">成员数</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="g in groups" :key="g.group_id" class="border-b border-slate-50 last:border-0 hover:bg-slate-50/70 transition-colors">
            <td class="px-6 py-3 text-slate-500 font-mono text-xs">{{ g.group_id }}</td>
            <td class="px-6 py-3 text-slate-700 font-medium">{{ g.group_name }}</td>
            <td class="px-6 py-3 text-slate-600">{{ g.member_count }} / {{ g.max_member_count }}</td>
          </tr>
        </tbody>
      </table>

      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-400 bg-slate-50/60 border-b border-slate-100">
            <th class="px-6 py-3 font-medium">用户 ID</th>
            <th class="px-6 py-3 font-medium">昵称</th>
            <th class="px-6 py-3 font-medium">备注</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in friends" :key="f.user_id" class="border-b border-slate-50 last:border-0 hover:bg-slate-50/70 transition-colors">
            <td class="px-6 py-3 text-slate-500 font-mono text-xs">{{ f.user_id }}</td>
            <td class="px-6 py-3 text-slate-700 font-medium">{{ f.nickname }}</td>
            <td class="px-6 py-3 text-slate-600">{{ f.remark }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api.js'

const current = ref('groups')
const groups = ref([])
const friends = ref([])
const error = ref('')

async function load() {
  error.value = ''
  try {
    if (current.value === 'groups') {
      groups.value = await api.getGroups()
    } else {
      friends.value = await api.getFriends()
    }
  } catch (e) {
    error.value = e.message
  }
}

function switchTab(name) {
  current.value = name
  load()
}

onMounted(load)
</script>
