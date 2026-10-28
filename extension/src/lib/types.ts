export interface ComputedLine {
  id: string
  label: string
  value: string
  kind: 'base' | 'tipo' | 'cuota' | 'resultado'
}

export interface Filing {
  id: string
  taxpayer_id: string
  model_code: string
  model_name: string
  entity: string
  period_label: string
  period_start: string
  period_end: string
  due_date: string
  status: string
  computed?: {
    model?: string
    version?: string
    lines?: ComputedLine[]
    values?: Record<string, string>
    warnings?: string[]
  }
  data?: Record<string, unknown>
}

export interface Taxpayer {
  id: string
  name: string
  nif: string
  legal_form: string
}

export interface Model {
  code: string
  name: string
  entity: string
  url: string
  documents: string[]
  notes: string
}

export interface DocumentItem {
  id: string
  filename: string
  kind: string
  mime: string
  size: number
  download_url: string
}

export interface ExtensionFiling {
  filing: Filing
  taxpayer_name: string
  taxpayer_nif: string
  model_url: string
  calculator: boolean
}

export interface ExtensionContext {
  filing: Filing
  taxpayer: Taxpayer
  model: Model
  documents: DocumentItem[]
}

export interface StoredConfig {
  apiUrl: string
  token: string
  tokenName: string
}

export interface ExtensionState {
  paired: boolean
  apiUrl: string
  tokenName: string
}
