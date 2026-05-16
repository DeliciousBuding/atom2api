const BASE = '/api/admin'

function getSecret(): string {
  return localStorage.getItem('admin_secret') || ''
}

async function request(path: string, opts: RequestInit = {}) {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Admin-Secret': getSecret(),
    ...(opts.headers as Record<string, string> || {}),
  }
  const res = await fetch(BASE + path, { ...opts, headers })
  if (res.status === 401) {
    throw new Error('unauthorized')
  }
  return res.json()
}

export const api = {
  login: (secret: string) =>
    fetch(BASE + '/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ secret }),
    }).then(r => r.json()),

  stats: () => request('/stats'),
  tokens: () => request('/tokens'),
  addToken: (data: { label: string; access_token: string; refresh_token?: string }) =>
    request('/tokens', { method: 'POST', body: JSON.stringify(data) }),
  importEnv: () => request('/tokens/import-env', { method: 'POST' }),
  refreshToken: (id: number) => request(`/tokens/${id}/refresh`, { method: 'POST' }),
  deleteToken: (id: number) => request(`/tokens/${id}`, { method: 'DELETE' }),

  apiKeys: () => request('/apikeys'),
  createAPIKey: (data: { name: string; quota_limit?: number }) =>
    request('/apikeys', { method: 'POST', body: JSON.stringify(data) }),
  deleteAPIKey: (id: number) => request(`/apikeys/${id}`, { method: 'DELETE' }),

  usage: (params?: { limit?: number; offset?: number; model?: string }) => {
    const q = new URLSearchParams()
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.offset) q.set('offset', String(params.offset))
    if (params?.model) q.set('model', params.model)
    return request('/usage?' + q.toString())
  },

  settings: () => request('/settings'),
  updateSettings: (data: Record<string, string>) =>
    request('/settings', { method: 'PUT', body: JSON.stringify(data) }),
}
