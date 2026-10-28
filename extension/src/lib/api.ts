import type { ExtensionContext, ExtensionFiling, StoredConfig } from './types'

export const DEFAULT_API = 'http://127.0.0.1:8787'

export function normalizeUrl(url: string): string {
  return url.trim().replace(/\/+$/, '')
}

export async function getConfig(): Promise<StoredConfig> {
  const data = await chrome.storage.local.get(['apiUrl', 'token', 'tokenName'])
  return {
    apiUrl: normalizeUrl(String(data.apiUrl || DEFAULT_API)),
    token: String(data.token || ''),
    tokenName: String(data.tokenName || ''),
  }
}

export async function saveConfig(config: Partial<StoredConfig>): Promise<void> {
  await chrome.storage.local.set(config)
}

export async function clearConfig(): Promise<void> {
  await chrome.storage.local.remove(['token', 'tokenName'])
}

async function parseError(res: Response): Promise<string> {
  try {
    const data = (await res.json()) as { error?: string }
    if (data?.error) return data.error
  } catch {
    /* ignore */
  }
  return `Error ${res.status}`
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const { apiUrl, token } = await getConfig()
  if (!token) throw new Error('La extensión no está conectada al ERP')
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${token}`)
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  let res: Response
  try {
    res = await fetch(`${apiUrl}${path}`, { ...init, headers })
  } catch {
    throw new Error('No se pudo conectar con el ERP local. Comprueba que está en marcha.')
  }
  if (res.status === 401) throw new Error('El acceso de la extensión ha sido revocado. Vuelve a emparejarla.')
  if (!res.ok) throw new Error(await parseError(res))
  return (await res.json()) as T
}

export async function apiBlob(path: string): Promise<{ base64: string; mime: string }> {
  const { apiUrl, token } = await getConfig()
  if (!token) throw new Error('La extensión no está conectada al ERP')
  let res: Response
  try {
    res = await fetch(`${apiUrl}${path}`, { headers: { Authorization: `Bearer ${token}` } })
  } catch {
    throw new Error('No se pudo conectar con el ERP local')
  }
  if (!res.ok) throw new Error(await parseError(res))
  const buffer = await res.arrayBuffer()
  return { base64: toBase64(buffer), mime: res.headers.get('Content-Type') || 'application/octet-stream' }
}

function toBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(binary)
}

export async function claimPairing(apiUrl: string, code: string, name: string): Promise<StoredConfig> {
  const base = normalizeUrl(apiUrl)
  let res: Response
  try {
    res = await fetch(`${base}/api/pairing/claim`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code, name }),
    })
  } catch {
    throw new Error('No se pudo conectar con el ERP. Revisa la dirección y que esté en marcha.')
  }
  if (!res.ok) throw new Error(await parseError(res))
  const data = (await res.json()) as { token: string; name: string }
  return { apiUrl: base, token: data.token, tokenName: data.name }
}

export async function listFilings(): Promise<ExtensionFiling[]> {
  return apiFetch<ExtensionFiling[]>('/api/extension/filings?limit=100')
}

export async function getContext(): Promise<{ context: ExtensionContext | null; apiUrl: string }> {
  const { apiUrl } = await getConfig()
  const stored = await chrome.storage.session.get('activeFilingId')
  const id = String(stored.activeFilingId || '')
  if (!id) return { context: null, apiUrl }
  try {
    const context = await apiFetch<ExtensionContext>(`/api/extension/context/${id}`)
    return { context, apiUrl }
  } catch {
    return { context: null, apiUrl }
  }
}
