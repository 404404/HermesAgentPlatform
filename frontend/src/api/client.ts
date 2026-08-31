const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1'

function csrfToken() {
  return document.cookie.split('; ').find((item) => item.startsWith('hep_csrf='))?.split('=')[1] || ''
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method || 'GET').toUpperCase()
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) headers.set('X-CSRF-Token', csrfToken())
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: 'include' })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(payload.error || `Request failed (${response.status})`)
  return payload.data as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body: unknown) => request<T>(path, { method: 'POST', body: JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) => request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  login: (username: string, password: string) => request<User>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
}

export type User = { id: number; username: string; display_name: string; email: string; status: string; department: string; roles: string[]; profile_count: number; runtime_status: string; last_login_at?: string; created_at: string }
export type Department = { id: number; parent_id?: number; name: string; description: string; status: string; member_count: number; knowledge_count: number; children: Department[] }
export type Profile = { id: number; user_id: number; user: string; name: string; display_name: string; description: string; status: string; model_id: number; model: string; runtime_class: string; created_at: string; updated_at: string }
export type Model = { id: number; name: string; display_name: string; provider: string; upstream_model: string; status: string; description: string; cost_class: string; data_classification: string; user_selectable: boolean }
