import { adapterPageReady } from './engine'
import { generatedAdapters } from './generated'
import type { Adapter } from './types'

export * from './types'
export {
  adapterPageReady,
  applyAdapter,
  formatValue,
  missingData,
  previewAdapter,
  resolveValue,
} from './engine'

/**
 * Catálogo generado en tiempo de compilación desde forms/**.yaml|json.
 * Los adaptadores se ordenan por organismo, modelo, ejercicio y revisión
 * descendente: la primera coincidencia es la revisión más reciente.
 */
const adapters: Adapter[] = [...generatedAdapters]

export function listAdapters(code?: string, exercise?: string, entity?: string): Adapter[] {
  return adapters.filter(
    (adapter) =>
      (!code || adapter.code === code) &&
      (!exercise || !adapter.exercise || adapter.exercise === exercise) &&
      (!entity || adapter.entity === entity),
  )
}

export function getAdapter(code: string, exercise?: string, entity?: string): Adapter | undefined {
  return listAdapters(code, exercise, entity)[0]
}

/**
 * Devuelve la revisión más reciente cuya página coincide con el documento
 * actual. Si ninguna coincide, devuelve la más reciente para que el panel
 * pueda mostrar su `pageHint`.
 */
export function adapterForPage(code: string, exercise?: string, entity?: string): Adapter | undefined {
  const candidates = listAdapters(code, exercise, entity)
  return candidates.find((adapter) => adapterPageReady(adapter)) ?? candidates[0]
}
