import { DEFAULT_API, getConfig } from './lib/api'
import { priorExerciseLabel } from './lib/fiscal'
import {
  defaultExtensionName,
  dueLabel,
  dueTone,
  escapeHtml,
  formatDate,
  formatLongDate,
  statusLabel,
  statusTone,
  todayISO,
} from './lib/format'
import {
  errorMessage,
  getState,
  listFilings,
  openErp,
  openErpFiling,
  openOptions,
  openSede,
  pair,
  unpair,
} from './lib/messages'
import type { ExtensionFiling, ExtensionState } from './lib/types'

function el<T extends Element>(selector: string): T {
  const node = document.querySelector<T>(selector)
  if (!node) throw new Error(`Elemento no encontrado: ${selector}`)
  return node
}

const unpairedEl = el<HTMLElement>('#unpaired')
const pairedEl = el<HTMLElement>('#paired')
const pairForm = el<HTMLFormElement>('#pair-form')
const apiUrlInput = el<HTMLInputElement>('#api-url')
const codeInput = el<HTMLInputElement>('#code')
const nameInput = el<HTMLInputElement>('#pair-name')
const pairError = el<HTMLElement>('#pair-error')
const pairSubmit = el<HTMLButtonElement>('#pair-submit')
const tokenName = el<HTMLElement>('#token-name')
const connectedUrl = el<HTMLElement>('#connected-url')
const listTitle = el<HTMLElement>('#list-title')
const listSubtitle = el<HTMLElement>('#list-subtitle')
const viewUpcomingBtn = el<HTMLButtonElement>('#view-upcoming')
const viewAllBtn = el<HTMLButtonElement>('#view-all')
const upcomingCount = el<HTMLElement>('#upcoming-count')
const allCount = el<HTMLElement>('#all-count')
const taxpayerSelect = el<HTMLSelectElement>('#taxpayer')
const listError = el<HTMLElement>('#list-error')
const filingsEl = el<HTMLElement>('#filings')
const emptyEl = el<HTMLElement>('#empty')
const emptyText = el<HTMLElement>('#empty-text')
const searchInput = el<HTMLInputElement>('#search')
const refreshBtn = el<HTMLButtonElement>('#refresh')
const unpairBtn = el<HTMLButtonElement>('#unpair')
const openErpBtn = el<HTMLButtonElement>('#open-erp')
const openOptionsBtn = el<HTMLButtonElement>('#open-options')
const footerLeft = el<HTMLElement>('#footer-left')
const footerVersion = el<HTMLElement>('#footer-version')

const sedeIcon = `
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M7 17 17 7" />
    <path d="M8 7h9v9" />
  </svg>`

type ListView = 'upcoming' | 'all'

interface TaxpayerOption {
  id: string
  name: string
  nif: string
}

const UPCOMING_LIMIT = 8
const OPEN_STATUSES = new Set(['pending', 'in_progress', 'ready'])

let filings: ExtensionFiling[] = []
let upcoming: ExtensionFiling[] = []
let taxpayers: TaxpayerOption[] = []
let taxpayerFilter = ''
let view: ListView = 'upcoming'
let filter = ''
let currentState: ExtensionState = { paired: false, apiUrl: DEFAULT_API, tokenName: '' }
let loading = false

function show(node: HTMLElement, visible: boolean) {
  node.classList.toggle('hidden', !visible)
}

function showError(node: HTMLElement, message: string) {
  node.textContent = message
  show(node, Boolean(message))
}

function applyState(state: ExtensionState) {
  currentState = state
  show(unpairedEl, !state.paired)
  show(pairedEl, state.paired)
  tokenName.textContent = state.tokenName || 'Extensión conectada'
  connectedUrl.textContent = state.apiUrl
  footerLeft.textContent = state.paired ? 'Datos solo en tu equipo' : 'Sin conexión'
  if (!state.paired) {
    apiUrlInput.value = state.apiUrl || apiUrlInput.value || DEFAULT_API
    nameInput.value = nameInput.value || defaultExtensionName()
  }
}

function dueClass(filing: ExtensionFiling): string {
  return `due ${dueTone(filing.filing.due_date, filing.filing.status)}`
}

function codeClass(code: string): string {
  if (code.length <= 4) return ''
  if (code.length <= 7) return 'code-sm'
  if (code.length <= 10) return 'code-xs'
  return 'code-2xs'
}

function matches(filing: ExtensionFiling): boolean {
  if (!filter) return true
  const haystack = [
    filing.filing.model_code,
    filing.filing.model_name,
    filing.taxpayer_name,
    filing.taxpayer_nif,
    filing.filing.period_label,
  ]
    .join(' ')
    .toLocaleLowerCase('es')
  return haystack.includes(filter)
}

function filingMarkup(item: ExtensionFiling): string {
  const filing = item.filing
  const tone = statusTone(filing.status)
  const exercise = priorExerciseLabel(filing, item.taxpayer_fiscal_year_end)
  const due = dueLabel(filing.due_date, filing.status)
  const done = filing.status === 'presented' || filing.status === 'not_applicable'
  const sede = item.model_url
    ? `<button class="row-action" type="button" data-sede data-url="${escapeHtml(item.model_url)}" title="Abrir en la sede electrónica" aria-label="Abrir en la sede electrónica">${sedeIcon}</button>`
    : '<span class="row-action" aria-hidden="true"></span>'
  return `
    <div class="filing" role="listitem" tabindex="0" data-id="${escapeHtml(filing.id)}" data-model="${escapeHtml(filing.model_code)}" aria-label="${escapeHtml(`${filing.model_code} ${filing.model_name}, ${filing.period_label}, ${item.taxpayer_name}`)}">
      <span class="model ${codeClass(filing.model_code)}" title="${escapeHtml(filing.model_code)}">${escapeHtml(filing.model_code)}</span>
      <span class="filing-body">
        <span class="filing-title">${escapeHtml(filing.model_name)}</span>
        <span class="filing-sub">
          <span class="filing-sub-text" title="${escapeHtml(`${item.taxpayer_name}${item.taxpayer_nif ? ` (${item.taxpayer_nif})` : ''}`)}">${escapeHtml(item.taxpayer_name)} · ${escapeHtml(filing.period_label)}</span>
          ${
            exercise
              ? `<span class="exercise" title="Periodo del ejercicio anterior, presentado después del cambio de ejercicio">${escapeHtml(exercise)}</span>`
              : ''
          }
        </span>
      </span>
      <span class="filing-side">
        <span class="${dueClass(item)}" title="Vence el ${escapeHtml(formatLongDate(filing.due_date))}">${escapeHtml(due)}</span>
        ${done ? '' : `<span class="due-date">${escapeHtml(formatDate(filing.due_date))}</span>`}
        <span class="badge ${tone}">${escapeHtml(statusLabel(filing.status))}</span>
      </span>
      ${sede}
    </div>`
}

interface MonthGroup {
  key: string
  label: string
  items: ExtensionFiling[]
}

function groupByMonth(items: ExtensionFiling[]): MonthGroup[] {
  const groups = new Map<string, ExtensionFiling[]>()
  for (const item of items) {
    const key = item.filing.due_date.slice(0, 7)
    const list = groups.get(key) ?? []
    list.push(item)
    groups.set(key, list)
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, list]) => {
      const [year, month] = key.split('-').map(Number)
      const monthName = new Intl.DateTimeFormat('es-ES', { month: 'long' }).format(
        new Date(year, month - 1, 1),
      )
      return {
        key,
        label: `${monthName} ${year}`,
        items: list,
      }
    })
}

function upcomingMarkup(items: ExtensionFiling[]): string {
  return groupByMonth(items)
    .map(
      (group) => `
        <section class="month-group" aria-label="${escapeHtml(group.label)}">
          <div class="month-head">
            <span class="month-label">${escapeHtml(group.label)}</span>
            <span class="month-count">${group.items.length}</span>
            <span class="month-line" aria-hidden="true"></span>
          </div>
          <div class="filing-list" role="list">
            ${group.items.map(filingMarkup).join('')}
          </div>
        </section>`,
    )
    .join('')
}

function scopedFilings(): ExtensionFiling[] {
  if (!taxpayerFilter) return filings
  return filings.filter((item) => item.filing.taxpayer_id === taxpayerFilter)
}

function refreshTaxpayers() {
  const byID = new Map<string, TaxpayerOption>()
  for (const item of filings) {
    const id = item.filing.taxpayer_id
    if (!id || byID.has(id)) continue
    byID.set(id, {
      id,
      name: item.taxpayer_name || 'Contribuyente sin nombre',
      nif: item.taxpayer_nif,
    })
  }
  taxpayers = [...byID.values()].sort((a, b) => a.name.localeCompare(b.name, 'es'))

  if (taxpayerFilter && !taxpayers.some((taxpayer) => taxpayer.id === taxpayerFilter)) {
    taxpayerFilter = ''
  }
  // Con un único contribuyente no hace falta obligar a elegir «Todos».
  if (!taxpayerFilter && taxpayers.length === 1) taxpayerFilter = taxpayers[0].id

  taxpayerSelect.innerHTML = [
    '<option value="">Todos los contribuyentes</option>',
    ...taxpayers.map(
      (taxpayer) =>
        `<option value="${escapeHtml(taxpayer.id)}">${escapeHtml(taxpayer.name)}${
          taxpayer.nif ? ` · ${escapeHtml(taxpayer.nif)}` : ''
        }</option>`,
    ),
  ].join('')
  taxpayerSelect.value = taxpayerFilter
}

function refreshUpcoming() {
  const today = todayISO()
  upcoming = scopedFilings()
    .filter((item) => {
      const filing = item.filing
      return Boolean(filing.due_date) && filing.due_date >= today && OPEN_STATUSES.has(filing.status)
    })
    .sort(
      (a, b) =>
        a.filing.due_date.localeCompare(b.filing.due_date) ||
        a.filing.model_code.localeCompare(b.filing.model_code),
    )
    .slice(0, UPCOMING_LIMIT)
}

function updateViewButtons(scopedCount: number) {
  const isUpcoming = view === 'upcoming'
  listTitle.textContent = isUpcoming ? 'Próximos vencimientos' : 'Expedientes abiertos'
  viewUpcomingBtn.classList.toggle('is-active', isUpcoming)
  viewAllBtn.classList.toggle('is-active', !isUpcoming)
  viewUpcomingBtn.setAttribute('aria-pressed', String(isUpcoming))
  viewAllBtn.setAttribute('aria-pressed', String(!isUpcoming))
  upcomingCount.textContent = String(upcoming.length)
  allCount.textContent = String(scopedCount)
}

function renderFilings() {
  const scoped = scopedFilings()
  const source = view === 'upcoming' ? upcoming : scoped
  const visible = source.filter(matches)
  listError.classList.add('hidden')
  updateViewButtons(scoped.length)

  const time = new Intl.DateTimeFormat('es-ES', { hour: '2-digit', minute: '2-digit' }).format(new Date())
  if (!filings.length) {
    listSubtitle.textContent = 'Sin expedientes pendientes'
  } else if (!scoped.length) {
    listSubtitle.textContent = 'Sin expedientes para el contribuyente'
  } else if (view === 'upcoming') {
    listSubtitle.textContent = upcoming.length
      ? `${upcoming.length} vencimiento${upcoming.length === 1 ? '' : 's'} · actualizado ${time}`
      : 'Sin vencimientos próximos'
  } else {
    listSubtitle.textContent = `${scoped.length} expediente${scoped.length === 1 ? '' : 's'} · actualizado ${time}`
  }

  emptyText.textContent = !filings.length
    ? 'No hay expedientes abiertos. Crea o importa uno desde el ERP.'
    : !scoped.length
      ? 'El contribuyente seleccionado no tiene expedientes abiertos.'
      : filter
        ? 'Ningún expediente coincide con la búsqueda.'
        : view === 'upcoming'
          ? 'No hay vencimientos próximos desde hoy. Cambia a «Todos» para ver los expedientes abiertos.'
          : 'No hay expedientes abiertos. Crea o importa uno desde el ERP.'
  show(emptyEl, visible.length === 0)
  show(filingsEl, visible.length > 0)

  filingsEl.innerHTML =
    view === 'upcoming'
      ? upcomingMarkup(visible)
      : `<div class="filing-list" role="list">${visible.map(filingMarkup).join('')}</div>`
}

function setView(next: ListView) {
  if (view === next) return
  view = next
  renderFilings()
}

async function loadFilings() {
  if (loading || !currentState.paired) return
  loading = true
  refreshBtn.classList.add('spinning')
  try {
    const list = await listFilings()
    filings = list
      .slice()
      .sort(
        (a, b) =>
          a.filing.due_date.localeCompare(b.filing.due_date) ||
          a.filing.model_code.localeCompare(b.filing.model_code),
      )
    refreshTaxpayers()
    refreshUpcoming()
    renderFilings()
  } catch (error) {
    filings = []
    upcoming = []
    taxpayerFilter = ''
    refreshTaxpayers()
    renderFilings()
    showError(listError, errorMessage(error))
  } finally {
    loading = false
    refreshBtn.classList.remove('spinning')
  }
}

async function launch(action: () => Promise<unknown>) {
  try {
    await action()
    window.close()
  } catch (error) {
    showError(listError, errorMessage(error))
  }
}

async function init() {
  footerVersion.textContent = `v${chrome.runtime.getManifest().version}`
  try {
    const config = await getConfig()
    apiUrlInput.value = config.apiUrl || DEFAULT_API
    const stored = await chrome.storage.local.get('extensionName')
    nameInput.value = String(stored.extensionName || '') || defaultExtensionName()
    const state = await getState()
    applyState(state)
    if (state.paired) await loadFilings()
  } catch (error) {
    applyState({ paired: false, apiUrl: DEFAULT_API, tokenName: '' })
    showError(pairError, errorMessage(error))
  }
}

pairForm.addEventListener('submit', async (event) => {
  event.preventDefault()
  pairError.classList.add('hidden')
  const apiUrl = apiUrlInput.value.trim() || DEFAULT_API
  const code = codeInput.value.replace(/\D/g, '')
  const name = nameInput.value.trim() || defaultExtensionName()

  if (apiUrlInput.value.trim() && !/^https?:\/\//i.test(apiUrlInput.value.trim())) {
    showError(pairError, 'La dirección debe empezar por http:// o https://')
    apiUrlInput.focus()
    return
  }
  if (code.length !== 6) {
    showError(pairError, 'El código debe tener 6 dígitos')
    codeInput.focus()
    return
  }

  pairSubmit.disabled = true
  pairSubmit.textContent = 'Conectando…'
  try {
    const state = await pair(apiUrl, code, name)
    await chrome.storage.local.set({ extensionName: name })
    codeInput.value = ''
    applyState(state)
    await loadFilings()
  } catch (error) {
    showError(pairError, errorMessage(error))
  } finally {
    pairSubmit.disabled = false
    pairSubmit.textContent = 'Conectar con el ERP'
  }
})

codeInput.addEventListener('input', () => {
  const digits = codeInput.value.replace(/\D/g, '').slice(0, 6)
  if (codeInput.value !== digits) codeInput.value = digits
})

searchInput.addEventListener('input', () => {
  filter = searchInput.value.trim().toLocaleLowerCase('es')
  renderFilings()
})

viewUpcomingBtn.addEventListener('click', () => setView('upcoming'))
viewAllBtn.addEventListener('click', () => setView('all'))

taxpayerSelect.addEventListener('change', () => {
  taxpayerFilter = taxpayerSelect.value
  refreshUpcoming()
  renderFilings()
})

refreshBtn.addEventListener('click', () => {
  void loadFilings()
})

unpairBtn.addEventListener('click', async () => {
  if (!window.confirm('¿Desconectar esta extensión del ERP? Podrás volver a emparejarla con un código nuevo.')) return
  try {
    await unpair()
    filings = []
    upcoming = []
    taxpayerFilter = ''
    refreshTaxpayers()
    filter = ''
    searchInput.value = ''
    const config = await getConfig()
    applyState({ paired: false, apiUrl: config.apiUrl, tokenName: '' })
    showError(listError, '')
  } catch (error) {
    showError(listError, errorMessage(error))
  }
})

filingsEl.addEventListener('click', (event) => {
  const target = event.target as HTMLElement
  const row = target.closest<HTMLElement>('.filing')
  if (!row) return
  const id = row.dataset.id
  if (!id) return
  const sede = target.closest<HTMLElement>('[data-sede]')
  if (sede) {
    event.stopPropagation()
    const url = sede.dataset.url || ''
    if (url) void launch(() => openSede(id, url))
    return
  }
  void launch(() => openErpFiling(id))
})

filingsEl.addEventListener('keydown', (event) => {
  if (event.key !== 'Enter' && event.key !== ' ') return
  const row = (event.target as HTMLElement).closest<HTMLElement>('.filing')
  if (!row?.dataset.id) return
  event.preventDefault()
  void launch(() => openErpFiling(row.dataset.id ?? ''))
})

openErpBtn.addEventListener('click', () => {
  void launch(openErp)
})

openOptionsBtn.addEventListener('click', () => {
  void openOptions().catch((error) => showError(pairError, errorMessage(error)))
})

void init()
