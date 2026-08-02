import { reactive } from 'vue'

export const auth = reactive({
  loggedIn: false,
  checked: false,
  setupRequired: false,
  notice: '',
})

async function request(path, options = {}) {
  const resp = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    ...options,
  })
  const data = await resp.json().catch(() => ({}))
  if (resp.status === 401) {
    auth.loggedIn = false
    // 优先展示服务端返回的具体错误（如登录时的「密码错误」），而非笼统的会话过期
    throw new Error(data.error || '未登录或会话已过期')
  }
  if (!resp.ok) {
    throw new Error(data.error || `请求失败 (${resp.status})`)
  }
  return data
}

// 把参数对象拼成查询串，空值跳过
function qs(params = {}) {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') q.set(k, v)
  }
  const s = q.toString()
  return s ? '?' + s : ''
}

export const api = {
  async checkLogin() {
    try {
      const me = await request('/api/me')
      auth.loggedIn = true
      auth.setupRequired = me.setup_required === true
    } catch {
      auth.loggedIn = false
    } finally {
      auth.checked = true
    }
  },
  async login(password) {
    await request('/api/login', { method: 'POST', body: JSON.stringify({ password }) })
    auth.loggedIn = true
    // 登录后拉取 setup 状态
    try {
      const me = await request('/api/me')
      auth.setupRequired = me.setup_required === true
    } catch { /* 忽略 */ }
  },
  completeSetup: () => request('/api/setup/complete', { method: 'POST' }),
  async logout() {
    await request('/api/logout', { method: 'POST' })
    auth.loggedIn = false
  },
  changePassword: (new_password) =>
    request('/api/password', { method: 'PUT', body: JSON.stringify({ new_password }) }),

  getSchema: () => request('/api/config/schema'),
  getConfig: () => request('/api/config'),
  saveConfig: (updates) =>
    request('/api/config', { method: 'PUT', body: JSON.stringify(updates) }),

  // 配置预设：保存当前配置为快照，一键切换
  getPresets: () => request('/api/config/presets'),
  savePreset: (name) =>
    request('/api/config/presets', { method: 'POST', body: JSON.stringify({ name }) }),
  applyPreset: (name) =>
    request(`/api/config/presets/${encodeURIComponent(name)}/apply`, { method: 'POST' }),
  deletePreset: (name) =>
    request(`/api/config/presets/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  getFile: (name) => request(`/api/files/${name}`),
  saveFile: (name, content) =>
    request(`/api/files/${name}`, { method: 'PUT', body: JSON.stringify({ content }) }),

  getStatus: () => request('/api/status'),
  getHost: () => request('/api/host'),
  getPlugins: () => request('/api/plugins'),
  getGroups: () => request('/api/groups'),
  getFriends: () => request('/api/friends'),
  // 消息日志分页查询：{ limit, before }（均可选），返回 { items, has_more }
  getMsgLogs: (params = {}) => request(`/api/msglogs${qs(params)}`),
  // 定时任务执行日志分页查询：{ target_type, target_id, task_id, status, start, end, keyword, limit, before }（均可选）
  getTaskLogs: (params = {}) => request(`/api/tasklogs${qs(params)}`),
  // Query 日志分页查询：{ chat_type, target_id, sender, start, end, keyword, limit, before }（均可选）
  getQueryLogs: (params = {}) => request(`/api/querylogs${qs(params)}`),
  // 控制台日志分页查询：{ limit, before }（均可选），返回 { items, has_more }
  getConsoleLogs: (params = {}) => request(`/api/consolelogs${qs(params)}`),
  getClocks: () => request('/api/clocks'),
  // token 消耗监控指标：{ summary, today, daily[] }，含缓存命中率
  getTokenStats: () => request('/api/tokenstats'),
  // AI API 余额：{ enabled, value, error, updated_at, cached, ttl }；refresh=true 强制刷新服务端缓存
  getBalance: (refresh = false) => request('/api/balance' + (refresh ? '?refresh=1' : '')),
  // token 多维详细统计：{ range, summary, today, by_source, by_chat_type, by_status, top_targets[], hourly[](24h分来源), daily[](窗口内分来源), iterations, avg_iterations }
  // params.range 时间维度：today / yesterday / 7d / 30d / month / all（默认 all）/ custom（需配合 start、end=YYYY-MM-DD，跨度 1~62 天）
  getTokenStatsDetail: (params = {}) => request(`/api/tokenstats/detail${qs(params)}`),
  // 每日配额汇总：{ date, global_used, global_limit, global_remaining, global_reached, sessions[] }
  getQuota: () => request('/api/quota'),
  // 清零配额：scope 为 g:会话ID / f:用户ID（如 g:fs:oc_xxx）/ all
  resetQuota: (scope) => request('/api/quota/reset', { method: 'POST', body: JSON.stringify({ scope }) }),
  createClock: (task) =>
    request('/api/clocks', { method: 'POST', body: JSON.stringify(task) }),
  updateClock: (id, fields) =>
    request(`/api/clocks/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(fields) }),
  deleteClock: (id) =>
    request(`/api/clocks/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  getSkills: () => request('/api/skills'),
  // 上传 skill（zip 压缩包，multipart 表单，不走 JSON 请求封装）
  async uploadSkill(file) {
    const fd = new FormData()
    fd.append('file', file)
    const resp = await fetch('/api/skills', {
      method: 'POST',
      body: fd,
      credentials: 'same-origin',
    })
    if (resp.status === 401) {
      auth.loggedIn = false
      throw new Error('未登录或会话已过期')
    }
    const data = await resp.json().catch(() => ({}))
    if (!resp.ok) {
      throw new Error(data.error || `请求失败 (${resp.status})`)
    }
    return data
  },
  deleteSkill: (name) =>
    request(`/api/skills/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  getMemoryScopes: () => request('/api/memory/scopes'),
  getMemories: (scope) => request(`/api/memory/list?scope=${encodeURIComponent(scope)}`),
  createMemory: (entry) =>
    request('/api/memory', { method: 'POST', body: JSON.stringify(entry) }),
  updateMemory: (entry) =>
    request('/api/memory', { method: 'PUT', body: JSON.stringify(entry) }),
  deleteMemory: (scope, id) =>
    request(`/api/memory?scope=${encodeURIComponent(scope)}&id=${encodeURIComponent(id)}`, { method: 'DELETE' }),

  getTeamScopes: () => request('/api/team/scopes'),
  getTeams: (scope) => request(`/api/team/list?scope=${encodeURIComponent(scope)}`),
  getTeamRoles: () => request('/api/team/roles'),
  createTeam: (team) =>
    request('/api/team', { method: 'POST', body: JSON.stringify(team) }),
  updateTeam: (team) =>
    request('/api/team', { method: 'PUT', body: JSON.stringify(team) }),
  deleteTeam: (scope, name) =>
    request(`/api/team?scope=${encodeURIComponent(scope)}&name=${encodeURIComponent(name)}`, { method: 'DELETE' }),

  getKnowledgeScopes: () => request('/api/knowledge/scopes'),
  getKnowledgeDocs: (scope) => request(`/api/knowledge/list?scope=${encodeURIComponent(scope)}`),
  createKnowledge: (entry) =>
    request('/api/knowledge', { method: 'POST', body: JSON.stringify(entry) }),
  updateKnowledge: (entry) =>
    request('/api/knowledge', { method: 'PUT', body: JSON.stringify(entry) }),
  deleteKnowledge: (scope, id) =>
    request(`/api/knowledge?scope=${encodeURIComponent(scope)}&id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  importKnowledgeURL: (scope, url) =>
    request('/api/knowledge/import-url', { method: 'POST', body: JSON.stringify({ scope, url }) }),

  restart: () => request('/api/restart', { method: 'POST' }),

  getUpdateInfo: () => request('/api/update/info'),
  startUpdate: () => request('/api/update/start', { method: 'POST' }),
  getUpdateStatus: () => request('/api/update/status'),
  // 轮询等待 Bot 重启完成（会话持久化在数据库中，重启后仍有效）
  async waitUntilUp(timeoutMs = 60000) {
    const deadline = Date.now() + timeoutMs
    while (Date.now() < deadline) {
      try {
        const resp = await fetch('/api/me', { credentials: 'same-origin' })
        if (resp.ok) return true
      } catch { /* 服务尚未恢复 */ }
      await new Promise((r) => setTimeout(r, 1500))
    }
    return false
  },
}
