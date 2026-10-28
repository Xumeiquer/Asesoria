import type { ExtensionFiling, ExtensionState } from './types'

interface ErrorResponse {
  error?: string
}

export function send<T>(message: Record<string, unknown>): Promise<T> {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage(message, (response: T & ErrorResponse) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message))
        return
      }
      if (response && typeof response === 'object' && response.error) {
        reject(new Error(String(response.error)))
        return
      }
      resolve(response)
    })
  })
}

export function getState(): Promise<ExtensionState> {
  return send<ExtensionState>({ type: 'STATE' })
}

export function listFilings(): Promise<ExtensionFiling[]> {
  return send<ExtensionFiling[]>({ type: 'LIST_FILINGS' })
}

export function pair(apiUrl: string, code: string, name: string): Promise<ExtensionState> {
  return send<ExtensionState>({ type: 'PAIR', apiUrl, code, name })
}

export function unpair(): Promise<{ ok: boolean }> {
  return send<{ ok: boolean }>({ type: 'UNPAIR' })
}

export function openErp(): Promise<{ ok: boolean }> {
  return send<{ ok: boolean }>({ type: 'OPEN_ERP' })
}

export function openErpFiling(id: string): Promise<{ ok: boolean }> {
  return send<{ ok: boolean }>({ type: 'OPEN_ERP_FILING', id })
}

export function openSede(id: string, url: string): Promise<{ ok: boolean }> {
  return send<{ ok: boolean }>({ type: 'OPEN_FILING', id, url })
}

export function openOptions(): Promise<{ ok: boolean }> {
  return send<{ ok: boolean }>({ type: 'OPEN_OPTIONS' })
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
