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
  if (resp.status === 401) {
    auth.loggedIn = false
    throw new Error('未登录或会话已过期')
  }
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    throw new Error(data.error || `请求失败 (${resp.status})`)
  }
  return data
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
  changePassword: (old_password, new_password) =>
    request('/api/password', { method: 'PUT', body: JSON.stringify({ old_password, new_password }) }),

  getSchema: () => request('/api/config/schema'),
  getConfig: () => request('/api/config'),
  saveConfig: (updates) =>
    request('/api/config', { method: 'PUT', body: JSON.stringify(updates) }),

  getFile: (name) => request(`/api/files/${name}`),
  saveFile: (name, content) =>
    request(`/api/files/${name}`, { method: 'PUT', body: JSON.stringify({ content }) }),

  getStatus: () => request('/api/status'),
  getHost: () => request('/api/host'),
  getPlugins: () => request('/api/plugins'),
  getGroups: () => request('/api/groups'),
  getFriends: () => request('/api/friends'),
  getTaskLogs: () => request('/api/tasklogs'),
  getMsgLogs: () => request('/api/msglogs'),
  // Query 日志条件查询：{ chat_type, target_id, sender, start, end, keyword, limit }（均可选）
  getQueryLogs: (params = {}) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== '') qs.set(k, v)
    }
    const s = qs.toString()
    return request(`/api/querylogs${s ? '?' + s : ''}`)
  },
  getClocks: () => request('/api/clocks'),
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

  restart: () => request('/api/restart', { method: 'POST' }),
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
