import type { ExtensionContext } from '../lib/types'
import type { Adapter, AdapterField, AdapterPreview, PreviewField } from './types'
import { fieldKey, filingExercise } from './types'

const PLACEHOLDER = /\{\{\s*([A-Za-z0-9_.]+)\s*\}\}/g

function period(filing: ExtensionContext['filing']): string {
  const label = filing.period_label.trim()
  const match = label.match(/(?:^|\s)(1T|2T|3T|4T|0[1-9]|1[0-2])(?:\s|$)/i)
  return (match?.[1] ?? label).toUpperCase()
}

function contextValue(key: string, context: ExtensionContext): string {
  const { filing, taxpayer } = context
  if (key.startsWith('casilla.')) {
    return filing.computed?.values?.[key.slice('casilla.'.length)] ?? ''
  }
  switch (key) {
    case 'taxpayer.nif':
      return taxpayer.nif
    case 'taxpayer.name':
      return taxpayer.name
    case 'taxpayer.legal_form':
      return taxpayer.legal_form
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

export function resolveValue(expression: string | undefined, context: ExtensionContext): string {
  if (!expression) return ''
  return expression.replace(PLACEHOLDER, (_, key: string) => contextValue(key, context))
}

function decimal(value: string): string {
  const trimmed = value.trim().replace(/\s/g, '')
  if (!trimmed) return ''
  if (!/^[+-]?\d+(?:[.,]\d+)?$/.test(trimmed)) return trimmed
  const sign = trimmed.startsWith('-') ? '-' : ''
  const unsigned = trimmed.replace(/^[+-]/, '').replace(',', '.')
  const [units, fraction = ''] = unsigned.split('.')
  return `${sign}${units},${(fraction + '00').slice(0, 2)}`
}

function isoDate(value: string): string {
  const match = value.trim().match(/^(\d{4})-(\d{2})-(\d{2})$/)
  return match ? `${match[3]}/${match[2]}/${match[1]}` : value
}

export function formatValue(value: string, field: AdapterField): string {
  switch (field.kind ?? 'text') {
    case 'amount':
    case 'percent':
      return decimal(value)
    case 'date':
      return isoDate(value)
    case 'nif':
      return value.trim().toUpperCase()
    default:
      return value
  }
}

function expressionFor(field: AdapterField): string {
  if (field.source) return field.source
  return field.casilla ? `{{casilla.${field.casilla}}}` : ''
}

export function adapterPageReady(adapter: Adapter): boolean {
  const required = adapter.page?.contains ?? []
  if (!required.length) return true
  const text = (document.body?.innerText || document.body?.textContent || '').toLowerCase()
  return required.every((item) => text.includes(item.toLowerCase()))
}

function findField(field: AdapterField): HTMLElement | null {
  const candidates = Array.from(document.querySelectorAll<HTMLElement>(field.selector))
  if (!candidates.length) return null
  if (field.casilla) {
    const labeled = candidates.filter((element) => element.hasAttribute('aria-label'))
    if (labeled.length) {
      return labeled.find((element) => element.getAttribute('aria-label')?.trim() === field.casilla) ?? null
    }
  }
  return candidates[0]
}

function isLocked(element: HTMLElement): boolean {
  return (
    element.hasAttribute('disabled') ||
    element.getAttribute('aria-disabled') === 'true' ||
    (element instanceof HTMLInputElement && (element.disabled || element.readOnly)) ||
    (element instanceof HTMLTextAreaElement && (element.disabled || element.readOnly)) ||
    (element instanceof HTMLSelectElement && element.disabled)
  )
}

function isCheckbox(element: HTMLElement): element is HTMLInputElement {
  return element instanceof HTMLInputElement && element.type === 'checkbox'
}

function currentValue(element: HTMLElement): string {
  if (isCheckbox(element)) return element.checked ? 'Sí' : 'No'
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement || element instanceof HTMLSelectElement) {
    return element.value
  }
  return element.textContent?.trim() ?? ''
}

function normalized(value: string, field: AdapterField): string {
  if (field.kind !== 'amount' && field.kind !== 'percent') return value.trim()
  return value.trim().replace(/\./g, '').replace(',', '.')
}

function sameValue(current: string, next: string, field: AdapterField): boolean {
  return normalized(current, field) === normalized(next, field)
}

function zeroValue(current: string, field: AdapterField): boolean {
  return normalized(current, field) === '0' || normalized(current, field) === '0.00'
}

function truthy(value: string): boolean {
  return /^(1|true|sí|si|s|x|on)$/i.test(value.trim())
}

function setNativeValue(element: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const proto = element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype
  const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set
  setter?.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
  element.dispatchEvent(new Event('change', { bubbles: true }))
}

function previewLabel(field: AdapterField, context: ExtensionContext): string {
  if (!field.casilla) return field.label
  return context.filing.computed?.lines?.find((line) => line.id === field.casilla)?.label ?? field.label
}

function noDataField(field: AdapterField, context: ExtensionContext): PreviewField {
  return {
    key: fieldKey(field),
    casilla: field.casilla,
    label: previewLabel(field, context),
    current: '',
    next: '',
    status: 'no-data',
    overwrite: false,
  }
}

export function missingData(adapter: Adapter, context: ExtensionContext): PreviewField[] {
  return adapter.fields
    .filter((field) => field.required !== false)
    .filter((field) => !resolveValue(expressionFor(field), context).trim())
    .map((field) => noDataField(field, context))
}

export function previewAdapter(adapter: Adapter, context: ExtensionContext): AdapterPreview {
  const dataMissing = missingData(adapter, context)
  if (!adapterPageReady(adapter)) {
    return { pageReady: false, fields: [], changes: [], missing: [], locked: [], dataMissing }
  }

  const fields: PreviewField[] = []
  for (const field of adapter.fields) {
    const element = findField(field)
    if (!element) {
      if (!field.optional) {
        fields.push({
          key: fieldKey(field),
          casilla: field.casilla,
          label: previewLabel(field, context),
          current: '',
          next: '',
          status: 'missing',
          overwrite: false,
        })
      }
      continue
    }

    const raw = resolveValue(expressionFor(field), context)
    const next = formatValue(raw, field)
    if (!next.trim()) continue

    const current = currentValue(element)
    const same = sameValue(current, next, field)
    if (isLocked(element)) {
      if (!same) {
        fields.push({
          key: fieldKey(field),
          casilla: field.casilla,
          label: previewLabel(field, context),
          current,
          next,
          status: 'locked',
          overwrite: false,
        })
      }
      continue
    }

    const overwrite = current !== '' && !same && !zeroValue(current, field) && current !== 'No'
    fields.push({
      key: fieldKey(field),
      casilla: field.casilla,
      label: previewLabel(field, context),
      current,
      next,
      status: same ? 'same' : 'change',
      overwrite,
    })
  }

  return {
    pageReady: true,
    fields,
    changes: fields.filter((field) => field.status === 'change'),
    missing: fields.filter((field) => field.status === 'missing'),
    locked: fields.filter((field) => field.status === 'locked'),
    dataMissing,
  }
}

function applyValue(element: HTMLElement, field: AdapterField, next: string): void {
  if (isCheckbox(element)) {
    const shouldCheck = truthy(next)
    if (element.checked !== shouldCheck) element.click()
    return
  }
  if (element instanceof HTMLSelectElement) {
    element.value = next
    element.dispatchEvent(new Event('change', { bubbles: true }))
    return
  }
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    element.focus()
    setNativeValue(element, next)
    element.blur()
    return
  }
  throw new Error(`El campo ${fieldKey(field)} no es un control de formulario`)
}

export async function applyAdapter(
  adapter: Adapter,
  context: ExtensionContext,
): Promise<{ applied: number; skipped: number }> {
  if (['presented', 'not_applicable'].includes(context.filing.status)) {
    throw new Error('Este expediente está presentado o no aplica; no se puede rellenar')
  }
  if (!adapter.verified) throw new Error('Este adaptador todavía no está verificado para el ejercicio actual')
  if (!adapterPageReady(adapter)) {
    throw new Error(adapter.pageHint ?? 'Abre la página correcta del formulario y vuelve a intentarlo.')
  }

  const preview = previewAdapter(adapter, context)
  if (preview.missing.length) {
    throw new Error(
      `No encuentro ${preview.missing.length} campo(s) del formulario. ${adapter.pageHint ?? 'Abre la página correcta y vuelve a intentarlo.'}`,
    )
  }
  if (preview.dataMissing.length) {
    const labels = preview.dataMissing
      .slice(0, 5)
      .map((field) => (field.casilla ? `${field.casilla} (${field.label})` : field.label))
      .join(', ')
    const extra = preview.dataMissing.length > 5 ? ` y ${preview.dataMissing.length - 5} más` : ''
    throw new Error(
      `Faltan ${preview.dataMissing.length} valor(es) en el ERP: ${labels}${extra}. Revisa el cálculo antes de rellenar.`,
    )
  }

  let applied = 0
  let skipped = 0
  for (const change of preview.changes) {
    const field = adapter.fields.find((item) => fieldKey(item) === change.key)
    if (!field) continue
    const element = findField(field)
    if (!element || isLocked(element)) {
      skipped++
      continue
    }
    if (sameValue(currentValue(element), change.next, field)) continue
    applyValue(element, field, change.next)
    applied++
    await new Promise((resolve) => window.setTimeout(resolve, 120))
  }

  // ZK calcula totales después del blur. Damos un margen corto antes de
  // devolver el control para que el usuario vea el formulario actualizado.
  await new Promise((resolve) => window.setTimeout(resolve, 250))
  return { applied, skipped }
}
