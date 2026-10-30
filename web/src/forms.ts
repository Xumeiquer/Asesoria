import formsData from './generated/forms.json'
import type { Filing, Taxpayer } from './types'

export interface FormRequirement {
  key: string
  casilla?: string
  label: string
  kind: string
  required: boolean
}

export interface FormAdapter {
  entity: string
  model: string
  exercise: string
  version: string
  updatedAt: string
  verified: boolean
  verifiedAgainst?: string
  pageHint?: string
  notes?: string
  page: { contains: string[] }
  requirements: FormRequirement[]
}

const adapters = (formsData.adapters ?? []) as FormAdapter[]

export const formsGeneratedAt = formsData.generatedAt

export function filingExercise(filing: Filing): string {
  return (filing.period_start || filing.period_end || filing.due_date || '').slice(0, 4)
}

export function formAdapterFor(filing: Filing): FormAdapter | undefined {
  const exercise = filingExercise(filing)
  return adapters.find(
    (adapter) =>
      adapter.entity === filing.entity &&
      adapter.model === filing.model_code &&
      (!adapter.exercise || adapter.exercise === exercise),
  )
}

function period(filing: Filing): string {
  const label = filing.period_label.trim()
  const match = label.match(/(?:^|\s)(1T|2T|3T|4T|0[1-9]|1[0-2])(?:\s|$)/i)
  return (match?.[1] ?? label).toUpperCase()
}

export function requirementValue(
  key: string,
  filing: Filing,
  taxpayer: Taxpayer | null,
): string {
  if (key.startsWith('casilla.')) {
    return filing.computed?.values?.[key.slice('casilla.'.length)] ?? ''
  }
  switch (key) {
    case 'taxpayer.nif':
      return taxpayer?.nif ?? ''
    case 'taxpayer.name':
      return taxpayer?.name ?? ''
    case 'taxpayer.legal_form':
      return taxpayer?.legal_form ?? ''
    case 'filing.id':
      return filing.id
    case 'filing.model_code':
      return filing.model_code
    case 'filing.model_name':
      return filing.model_name
    case 'filing.entity':
      return filing.entity
    case 'filing.period_label':
      return filing.period_label
    case 'filing.period':
      return period(filing)
    case 'filing.period_start':
      return filing.period_start
    case 'filing.period_end':
      return filing.period_end
    case 'filing.due_date':
      return filing.due_date
    case 'exercise':
      return filingExercise(filing)
    default:
      return ''
  }
}

export function requirementLabel(requirement: FormRequirement, filing: Filing): string {
  if (!requirement.casilla) return requirement.label
  return filing.computed?.lines?.find((line) => line.id === requirement.casilla)?.label ?? requirement.label
}

export function requirementDisplay(
  requirement: FormRequirement,
  filing: Filing,
  taxpayer: Taxpayer | null,
): string {
  const raw = requirementValue(requirement.key, filing, taxpayer)
  if (!raw.trim()) return ''
  if (!requirement.casilla) return raw
  const line = filing.computed?.lines?.find((item) => item.id === requirement.casilla)
  if (!line) return raw
  return line.kind === 'tipo' ? `${line.value} %` : `${line.value} €`
}

export function missingFormRequirements(
  adapter: FormAdapter | undefined,
  filing: Filing,
  taxpayer: Taxpayer | null,
): FormRequirement[] {
  if (!adapter) return []
  return adapter.requirements.filter(
    (requirement) => !requirementValue(requirement.key, filing, taxpayer).trim(),
  )
}
