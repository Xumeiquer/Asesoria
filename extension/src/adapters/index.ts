import { aeat130 } from './aeat130'
import { aeat303 } from './aeat303'
import type { Adapter } from './types'

export * from './types'
export { adapterPageReady, applyAdapter, formatValue, previewAdapter, resolveValue } from './engine'

/**
 * Adaptadores declarativos, versionados por modelo y ejercicio. Un adaptador
 * solo se activa cuando `verified` es true; el panel siempre mantiene la copia
 * manual de casillas como alternativa.
 */
const adapters: Adapter[] = [aeat303, aeat130]

export function getAdapter(code: string, exercise?: string): Adapter | undefined {
  return adapters.find(
    (adapter) => adapter.code === code && (!exercise || !adapter.exercise || adapter.exercise === exercise),
  )
}

export function listAdapters(): Adapter[] {
  return [...adapters]
}
