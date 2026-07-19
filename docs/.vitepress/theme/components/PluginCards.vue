<script setup lang="ts">
defineProps<{
  plugins: Array<{
    icon: string
    name: string
    desc: string
    cmds?: string[]
  }>
}>()
</script>

<template>
  <div class="ania-plugin-grid">
    <div v-for="p in plugins" :key="p.name" class="ania-plugin-card">
      <div class="pc-head">
        <span class="pc-icon">{{ p.icon }}</span>
        <span class="pc-name">{{ p.name }}</span>
      </div>
      <div class="pc-desc">{{ p.desc }}</div>
      <div v-if="p.cmds?.length" class="pc-cmds">
        <span v-for="c in p.cmds" :key="c" class="pc-cmd">{{ c }}</span>
      </div>
    </div>
  </div>
</template>
