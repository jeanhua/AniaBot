<script setup lang="ts">
import { icons } from './icons'

defineProps<{
  plugins: Array<{
    /** 图标：icons.ts 中的短名渲染线性 SVG，其余按文本（如 emoji）渲染 */
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
        <span v-if="icons[p.icon]" class="pc-icon" v-html="icons[p.icon]" />
        <span v-else class="pc-icon pc-icon-text">{{ p.icon }}</span>
        <span class="pc-name">{{ p.name }}</span>
      </div>
      <div class="pc-desc">{{ p.desc }}</div>
      <div v-if="p.cmds?.length" class="pc-cmds">
        <span v-for="c in p.cmds" :key="c" class="pc-cmd">{{ c }}</span>
      </div>
    </div>
  </div>
</template>
