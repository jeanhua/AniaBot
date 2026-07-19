<script setup lang="ts">
const plugins = [
  {
    icon: '🤖',
    name: 'AI 对话',
    desc: '接入任意 OpenAI 兼容大模型，支持工具调用、MCP、Skill、多模态识图与 AI 定时任务，上下文按 token 预算自动压缩。',
    cmds: ['@机器人 聊天', '#新对话', '/stop', '/clock'],
  },
  {
    icon: '🛡️',
    name: '防撤回',
    desc: '缓存群内最近 100 条消息，撤回也能通过合并转发回顾，图片/文件自动续期链接。',
    cmds: ['/explore [n]'],
  },
  {
    icon: '🔁',
    name: '复读机',
    desc: '群内 +1 文化守护者：同一消息出现 3 次自动跟读，管理员可随时开关。',
    cmds: ['/close repeat', '/enable repeat'],
  },
  {
    icon: '📰',
    name: '每日新闻',
    desc: '按 cron 表达式定时向指定群推送 60s 新闻图，也可随时手动获取。',
    cmds: ['/news', '/news force'],
  },
  {
    icon: '⚙️',
    name: '系统插件',
    desc: '启动通知、/help 插件列表、远程退出与 panic 告警，管理员的贴心助手。',
    cmds: ['/help', '/exit'],
  },
  {
    icon: '📝',
    name: '日志插件',
    desc: '在控制台美观地打印每一条收发消息，调试期的好帮手。',
    cmds: [],
  },
]

const steps = [
  {
    title: '克隆项目',
    desc: 'git clone 后 go mod tidy，需要 Go 1.26+',
  },
  {
    title: '部署 NapCat',
    desc: '启动 NapCat 并开放 WebSocket / HTTP 接口',
  },
  {
    title: '编辑 config.yaml',
    desc: '填写适配器地址、管理员 QQ 与 AI 的 api_key',
  },
  {
    title: '启动！',
    desc: 'go run cmd/main.go，@机器人 开始聊天',
  },
]
</script>

<template>
  <div class="ania-section">
    <h2 class="ania-section-title">内置插件，开箱即用</h2>
    <p class="ania-section-sub">六个内置插件覆盖常见场景，注册即用，也可以作为你开发插件的参考实现</p>
    <PluginCards :plugins="plugins" />
  </div>

  <div class="ania-section">
    <h2 class="ania-section-title">四步跑起来</h2>
    <p class="ania-section-sub">从零到一个会聊天、会定时、会使用工具的 QQ 机器人</p>
    <div class="ania-steps">
      <div v-for="(s, i) in steps" :key="s.title" class="ania-step">
        <span class="step-no">{{ i + 1 }}</span>
        <div class="step-title">{{ s.title }}</div>
        <div class="step-desc">{{ s.desc }}</div>
      </div>
    </div>
  </div>
</template>
