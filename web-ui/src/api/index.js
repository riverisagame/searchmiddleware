// 后端 API 封装：自动携带 JWT，统一错误处理
const TOKEN_KEY = 'sm_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(t) {
  localStorage.setItem(TOKEN_KEY, t)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) headers['Authorization'] = 'Bearer ' + token

  const resp = await fetch(path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  let data = null
  try { data = await resp.json() } catch { /* 非 JSON 响应 */ }

  if (resp.status === 401) {
    clearToken()
    window.location.hash = '#/login'
    throw new Error('登录已过期，请重新登录')
  }
  if (!resp.ok || (data && data.code !== 0)) {
    throw new Error((data && (data.msg || data.error)) || `HTTP ${resp.status}`)
  }
  return data ? data.data : null
}

export const api = {
  login: (username, password) => request('POST', '/api/v1/auth/login', { username, password }),
  health: async () => {
    const resp = await fetch('/health')
    if (!resp.ok) throw new Error('HTTP ' + resp.status)
    return resp.json()
  },
  search: (params) => request('GET', '/api/v1/search?' + new URLSearchParams(params)),

  listIndexes: () => request('GET', '/api/v1/indexes'),
  createIndex: (name, content) => request('POST', '/api/v1/indexes', { name, content }),
  updateIndex: (name, content) => request('PUT', '/api/v1/indexes/' + name, { content }),
  deleteIndex: (name) => request('DELETE', '/api/v1/indexes/' + name),
  syncIndex: (name, type, ids) => request('POST', `/api/v1/indexes/${name}/sync`, { type, ids }),
  reconcile: (name, type) => request('POST', `/api/v1/indexes/${name}/reconcile?type=${type}`),
  fixReconcile: (name, id) => request('POST', `/api/v1/indexes/${name}/reconcile/${id}/fix`),
  listReconcile: (name) => request('GET', `/api/v1/indexes/${name}/reconcile`),

  listRuns: () => request('GET', '/api/v1/runs'),
  listLogs: (params) => request('GET', '/api/v1/logs?' + new URLSearchParams(params)),
  listAlerts: () => request('GET', '/api/v1/alerts'),

  listSchedules: () => request('GET', '/api/v1/schedules'),
  createSchedule: (s) => request('POST', '/api/v1/schedules', s),
  deleteSchedule: (id) => request('DELETE', '/api/v1/schedules/' + id),

  listSynonyms: () => request('GET', '/api/v1/synonyms'),
  createSynonym: (s) => request('POST', '/api/v1/synonyms', s),
  deleteSynonym: (id) => request('DELETE', '/api/v1/synonyms/' + id),
  syncSynonymsToZinc: () => request('POST', '/api/v1/synonyms/sync'),

  listUsers: () => request('GET', '/api/v1/users'),
  createUser: (u) => request('POST', '/api/v1/users', u),
  sqlTest: (datasource, sql) => request('POST', '/api/v1/sql/test', { datasource, sql }),
}
