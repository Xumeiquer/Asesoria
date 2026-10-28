export type LegalForm = 'autonomo' | 'sl'
export type FilingStatus = 'pending' | 'in_progress' | 'ready' | 'presented' | 'late' | 'not_applicable'

export interface Taxpayer {
  id: string
  nif: string
  name: string
  legal_form: LegalForm
  email: string
  phone: string
  vat_regime: string
  irpf_method: string
  has_employees: boolean
  has_rentals: boolean
  economic_activity: boolean
  cnae: string
  fiscal_year_end: string
  iban: string
  reta_base_cents: number
  reta_base_from: string
  reta_net_income_cents: number
  reta_net_income_year: number
  created_at: string
  updated_at: string
}

export interface RetaBracket {
  number: number
  table: 'reducida' | 'general'
  min_income_cents: number
  max_income_cents: number
  min_base_cents: number
  max_base_cents: number
}

export interface RetaBracketsResponse {
  year: number
  brackets: RetaBracket[]
}

export interface RetaBaseChange {
  id: string
  taxpayer_id: string
  base_cents: number
  base_from: string
  net_income_cents: number
  net_income_year: number
  created_at: string
}

export interface ComputedLine {
  id: string
  label: string
  value: string
  kind: 'base' | 'tipo' | 'cuota' | 'resultado'
}

export interface Computed {
  model: string
  version: string
  lines: ComputedLine[]
  values: Record<string, string>
  warnings: string[]
  calculated_at: string
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
  status: FilingStatus
  data: Record<string, unknown>
  computed: Partial<Computed>
  notes: string
  presented_at: string | null
  receipt_document_id: string | null
  created_at: string
  updated_at: string
}

export interface DocumentItem {
  id: string
  taxpayer_id: string | null
  filing_id: string | null
  kind: string
  filename: string
  mime: string
  size: number
  sha256: string
  doc_date: string
  amount_cents: number | null
  counterparty_nif: string
  metadata: Record<string, unknown>
  text_excerpt?: string
  ocr_status: string
  review_status: 'ok' | 'review'
  library_path?: string
  source_path?: string
  created_at: string
}

export interface ScanProblem {
  path: string
  error: string
}

export interface ScanResult {
  root: string
  started_at: string
  finished_at: string
  scanned: number
  imported: number
  duplicates: number
  skipped: number
  errors: number
  needs_review: number
  truncated: boolean
  problems?: ScanProblem[]
}

export interface LibraryConfig {
  root: string
  configured: boolean
  writable: boolean
  ocr_lang: string
  years: number[]
  tesseract: boolean
  pdf_text: boolean
  pdf_images: boolean
  last_scan?: ScanResult
}

export interface Invoice {
  id: string
  taxpayer_id: string
  direction: 'issued' | 'received'
  number: string
  invoice_date: string
  counterparty_nif: string
  counterparty_name: string
  concept: string
  base_cents: number
  vat_rate: number
  vat_cents: number
  retention_rate: number
  retention_cents: number
  total_cents: number
  category: string
  document_id: string | null
  created_at: string
}

export interface Schedule {
  type: string
  due_months?: number[]
  due_day?: number
  dues?: Array<{ month: number; day: number; period_start: string; period_end: string; period_year_offset: number }>
}

export interface Model {
  code: string
  name: string
  entity: string
  category: string
  periodicity: string
  updated_at?: string
  description: string
  url: string
  auto: boolean
  obsolete?: boolean
  applies: Record<string, unknown>
  schedule?: Schedule
  documents: string[]
  notes: string
  applicable: boolean
  calculator: boolean
}

export interface Dashboard {
  taxpayers: number
  due_soon: number
  overdue: number
  presented: number
  pending: number
  documents: number
  invoices: number
  upcoming: Filing[]
  by_entity: Record<string, number>
}

export interface AppMeta {
  version: string
  catalog_version: string
  catalog_updated_at: string
  data_dir: string
  paired_tokens: number
  calculators: string[]
  dev: boolean
}

export interface PairingCode {
  code: string
  created_at: string
  expires_at: string
  used_at: string | null
}

export interface ApiToken {
  id: string
  name: string
  created_at: string
  last_used_at: string | null
}
