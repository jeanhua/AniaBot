<script setup lang="ts">
import { icons } from './icons'

const features = [
  {
    icon: 'bolt',
    title: '高性能',
    desc: '基于 Go 语言开发，工作池 + 协程模型处理消息事件，轻松应对高并发群聊场景。',
  },
  {
    icon: 'grid',
    title: '插件驱动',
    desc: '一切功能皆为插件。中间件链式执行、生命周期管理、panic 自动恢复，扩展只需实现一个接口。',
  },
  {
    icon: 'chat',
    title: 'AI 对话引擎',
    desc: '内置完整 Agent 能力：工具调用、MCP 协议、Skill 系统、多模态识图、token 预算上下文窗口。',
  },
  {
    icon: 'clock',
    title: 'AI 定时任务',
    desc: 'AI 与用户均可动态创建 cron 任务，持久化保存、重启不丢，到点自动以完整工具链执行。',
  },
  {
    icon: 'link',
    title: '多平台接入',
    desc: 'QQ（OneBot v11，NapCat WebSocket / HTTP）+ 飞书（官方 SDK 长连接 / Webhook）+ Telegram（Bot API 长轮询）+ Discord（Gateway WebSocket），可同时在线、按需扩展新平台。',
  },
  {
    icon: 'database',
    title: '双层存储',
    desc: '缓存层（Redis / 内存）+ 持久化层（SQLite / MySQL），插件数据自动按命名空间隔离。',
  },
]

const plugins = [
  {
    icon: 'chat',
    name: 'AI 对话',
    desc: '接入任意 OpenAI 兼容大模型，支持工具调用、MCP、Skill、多模态识图与 AI 定时任务，上下文按 token 预算自动压缩。',
    cmds: ['@机器人 聊天', '#新对话', '/stop', '/clock'],
  },
  {
    icon: 'shield',
    name: '防撤回',
    desc: '缓存群内最近 100 条消息，撤回也能通过合并转发回顾，图片/文件自动续期链接。',
    cmds: ['/explore [n]'],
  },
  {
    icon: 'repeat',
    name: '复读机',
    desc: '群内 +1 文化守护者：同一消息出现 3 次自动跟读，管理员可随时开关。',
    cmds: ['/close repeat', '/enable repeat'],
  },
  {
    icon: 'news',
    name: '每日新闻',
    desc: '按 cron 表达式定时向指定群推送 60s 新闻图，也可随时手动获取。',
    cmds: ['/news', '/news force'],
  },
  {
    icon: 'gear',
    name: '系统插件',
    desc: '启动通知、/help 插件列表、远程退出与 panic 告警，管理员的贴心助手。',
    cmds: ['/help', '/exit'],
  },
  {
    icon: 'log',
    name: '日志插件',
    desc: '在控制台美观地打印每一条收发消息，调试期的好帮手。',
    cmds: [],
  },
]

const steps = [
  {
    title: '克隆项目',
    desc: 'git clone 后 go mod tidy，需要 Go 1.25+',
  },
  {
    title: '部署平台端',
    desc: 'QQ：部署 NapCat 开放 WebSocket / HTTP 接口；飞书：创建应用并开通权限；Telegram：@BotFather 建机器人填 Token；Discord：Developer Portal 建应用填 Token（均无需额外协议端）',
  },
  {
    title: '面板中配置',
    desc: '启动后在 Web 控制面板勾选平台、填写连接配置、管理员 ID 与 AI 的 api_key',
  },
  {
    title: '启动！',
    desc: 'go run cmd/main.go，@机器人 开始聊天',
  },
]
</script>

<template>
  <div class="ania-section ania-features-section">
    <div class="ania-feature-grid">
      <div v-for="f in features" :key="f.title" class="ania-feature">
        <span class="ft-icon" v-html="icons[f.icon]" />
        <div class="ft-title">{{ f.title }}</div>
        <div class="ft-desc">{{ f.desc }}</div>
      </div>
    </div>
  </div>

  <div class="ania-section">
    <h2 class="ania-section-title">内置插件，开箱即用</h2>
    <p class="ania-section-sub">六个内置插件覆盖常见场景，注册即用，也可以作为你开发插件的参考实现</p>
    <PluginCards :plugins="plugins" />
  </div>

  <div class="ania-section">
    <h2 class="ania-section-title">四步跑起来</h2>
    <p class="ania-section-sub">从零到一个会聊天、会定时、会使用工具的多平台机器人</p>
    <div class="ania-steps">
      <div v-for="(s, i) in steps" :key="s.title" class="ania-step">
        <span class="step-no">{{ i + 1 }}</span>
        <div class="step-title">{{ s.title }}</div>
        <div class="step-desc">{{ s.desc }}</div>
      </div>
    </div>
  </div>
</template>

