import type { Filing } from '../lib/types'

export type FieldKind = 'text' | 'amount' | 'percent' | 'date' | 'nif' | 'select' | 'checkbox'

export interface AdapterField {
  /** Casilla del modelo. Si se omite, usa `source`. */
  casilla?: string
  label: string
  /** Selector CSS estable. No usar IDs dinámicos de ZK. */
  selector: string
  kind?: FieldKind
  /** Expresión con placeholders. Por defecto: {{casilla.<casilla>}}. */
  source?: string
  /** Un campo opcional puede no existir según el perfil fiscal. */
  optional?: boolean
}

export interface Adapter {
  code: string
  /** Ejercicio al que aplica. Si se omite, el adaptador es transversal. */
  exercise?: string
  version: string
  /** Solo se activa después de probar los selectores contra el formulario. */
  verified: boolean
  verifiedAgainst?: string
  /** Textos que deben aparecer en la página para considerar que es la correcta. */
  page?: { contains: string[] }
  pageHint?: string
  fields: AdapterField[]
  notes?: string
}

export interface PreviewField {
  key: string
  casilla?: string
  label: string
  current: string
  next: string
  status: 'change' | 'same' | 'missing' | 'locked'
  /** El valor actual no está vacío y se va a sustituir. */
  overwrite: boolean
}

export interface AdapterPreview {
  pageReady: boolean
  fields: PreviewField[]
  changes: PreviewField[]
  missing: PreviewField[]
  locked: PreviewField[]
}

export function fieldKey(field: AdapterField): string {
  return field.casilla ?? field.label
}

export function filingExercise(filing: Filing): string {
  const source = filing.period_start || filing.period_end
  return source ? source.slice(0, 4) : ''
}
