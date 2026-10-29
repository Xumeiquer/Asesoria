const BASE = '/api'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

let sessionPromise: Promise<void> | null = null

export function ensureSession(): Promise<void> {
  if (!sessionPromise) {
    sessionPromise = fetch(`${BASE}/session`, { credentials: 'include' }).then((res) => {
      if (!res.ok) throw new ApiError(res.status, 'No se pudo iniciar la sesión local')
    })
  }
  return sessionPromise
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  await ensureSession()
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(`${BASE}${path}`, { ...init, headers, credentials: 'include' })
  if (!res.ok) {
    let message = `Error ${res.status}`
    try {
      const data = await res.json()
      if (data?.error) message = data.error
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  del: <T = void>(path: string) => request<T>(path, { method: 'DELETE' }),
  upload: <T>(path: string, form: FormData) => request<T>(path, { method: 'POST', body: form }),
}

export function euros(cents: number | null | undefined): string {
  if (cents === null || cents === undefined) return '—'
  return new Intl.NumberFormat('es-ES', { style: 'currency', currency: 'EUR' }).format(cents / 100)
}

export function centsFromEuros(value: string): number {
  let normalized = value.trim().replace(/[€\s]/g, '')
  if (!normalized) return 0
  if (normalized.includes(',') && normalized.includes('.')) {
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  } else if (normalized.includes(',')) {
    normalized = normalized.replace(',', '.')
  } else if (normalized.includes('.')) {
    // Spanish input treats 1.234 and 1.234.567 as thousands separators, while
    // 1234.56 and 1.5 are decimals.
    const parts = normalized.split('.')
    const last = parts[parts.length - 1]
    if (
      parts.length > 2 ||
      (parts.length === 2 && last.length === 3 && parts[0].length > 0 && parts[0].length <= 3)
    ) {
      normalized = parts.join('')
    }
  }
  const n = Number(normalized)
  if (!Number.isFinite(n)) return 0
  return Math.round(n * 100)
}

export function dateLabel(value: string): string {
  if (!value) return '—'
  const date = new Date(value.length <= 10 ? `${value}T00:00:00` : value)
  return new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: 'short', year: 'numeric' }).format(date)
}

export function dateTimeLabel(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  return new Intl.DateTimeFormat('es-ES', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

export function bytes(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

export function daysUntil(date: string): number {
  const target = new Date(`${date}T00:00:00`)
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return Math.round((target.getTime() - today.getTime()) / 86_400_000)
}
