<template>
  <div class="space-y-4">
    <div v-if="error" class="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3">{{ error }}</div>

    <div class="flex gap-1 border-b border-slate-200">
      <button
        v-for="tab in [['groups', '群列表'], ['friends', '好友列表']]"
        :key="tab[0]"
        class="px-4 py-2 text-sm rounded-t"
        :class="current === tab[0] ? 'bg-white border border-b-white border-slate-200 -mb-px text-indigo-600 font-medium' : 'text-slate-500 hover:text-slate-700'"
        @click="switchTab(tab[0])"
      >
        {{ tab[1] }}
      </button>
    </div>

    <section class="bg-white rounded-lg shadow-sm">
      <table v-if="current === 'groups'" class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-500 border-b border-slate-100">
            <th class="px-5 py-2.5 font-medium">群号</th>
            <th class="px-5 py-2.5 font-medium">群名称</th>
            <th class="px-5 py-2.5 font-medium">成员数</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="g in groups" :key="g.group_id" class="border-b border-slate-50 hover:bg-slate-50">
            <td class="px-5 py-2.5 text-slate-700">{{ g.group_id }}</td>
            <td class="px-5 py-2.5 text-slate-700">{{ g.group_name }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ g.member_count }} / {{ g.max_member_count }}</td>
          </tr>
        </tbody>
      </table>

      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-500 border-b border-slate-100">
            <th class="px-5 py-2.5 font-medium">QQ 号</th>
            <th class="px-5 py-2.5 font-medium">昵称</th>
            <th class="px-5 py-2.5 font-medium">备注</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in friends" :key="f.user_id" class="border-b border-slate-50 hover:bg-slate-50">
            <td class="px-5 py-2.5 text-slate-700">{{ f.user_id }}</td>
            <td class="px-5 py-2.5 text-slate-700">{{ f.nickname }}</td>
            <td class="px-5 py-2.5 text-slate-600">{{ f.remark }}</td>
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
