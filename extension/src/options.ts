import { DEFAULT_API, getConfig } from './lib/api'
import { defaultExtensionName } from './lib/format'
import {
  errorMessage,
  getState,
  listFilings,
  openErp,
  pair,
  unpair,
} from './lib/messages'
import type { ExtensionState } from './lib/types'

function el<T extends Element>(selector: string): T {
  const node = document.querySelector<T>(selector)
  if (!node) throw new Error(`Elemento no encontrado: ${selector}`)
  return node
}

const statusState = el<HTMLElement>('#status-state')
const statusUrl = el<HTMLElement>('#status-url')
const statusName = el<HTMLElement>('#status-name')
const statusVersion = el<HTMLElement>('#status-version')
const testBtn = el<HTMLButtonElement>('#test')
const unpairBtn = el<HTMLButtonElement>('#unpair')
const testResult = el<HTMLElement>('#test-result')
const pairCard = el<HTMLElement>('#pair-card')
const pairForm = el<HTMLFormElement>('#pair-form')
const apiUrlInput = el<HTMLInputElement>('#api-url')
const codeInput = el<HTMLInputElement>('#code')
const nameInput = el<HTMLInputElement>('#pair-name')
const pairError = el<HTMLElement>('#pair-error')
const pairSubmit = el<HTMLButtonElement>('#pair-submit')
const openErpBtn = el<HTMLButtonElement>('#open-erp')
const footerVersion = el<HTMLElement>('#footer-version')

let currentState: ExtensionState = { paired: false, apiUrl: DEFAULT_API, tokenName: '' }

function show(node: HTMLElement, visible: boolean) {
  node.classList.toggle('hidden', !visible)
}

function setResult(message: string, tone: 'ok' | 'error' | 'none' = 'none') {
  testResult.textContent = message
  testResult.classList.toggle('hidden', !message)
  testResult.classList.toggle('alert-ok', tone === 'ok')
  testResult.classList.toggle('alert-error', tone === 'error')
}

function applyState(state: ExtensionState) {
  currentState = state
  statusState.textContent = state.paired ? 'Conectada' : 'Sin conectar'
  statusState.classList.toggle('ok', state.paired)
  statusUrl.textContent = state.apiUrl || '—'
  statusName.textContent = state.paired ? state.tokenName || 'Extensión' : '—'
  show(pairCard, !state.paired)
  show(unpairBtn, state.paired)
  if (!state.paired) {
    apiUrlInput.value = state.apiUrl || apiUrlInput.value || DEFAULT_API
  }
}

async function loadState() {
  try {
    const state = await getState()
    applyState(state)
  } catch (error) {
    statusState.textContent = 'No se pudo consultar'
    statusState.classList.remove('ok')
    setResult(errorMessage(error), 'error')
  }
}

pairForm.addEventListener('submit', async (event) => {
  event.preventDefault()
  pairError.classList.add('hidden')
  const apiUrl = apiUrlInput.value.trim() || DEFAULT_API
  const code = codeInput.value.replace(/\D/g, '')
  const name = nameInput.value.trim() || defaultExtensionName()

  if (apiUrlInput.value.trim() && !/^https?:\/\//i.test(apiUrlInput.value.trim())) {
    pairError.textContent = 'La dirección debe empezar por http:// o https://'
    show(pairError, true)
    apiUrlInput.focus()
    return
  }
  if (code.length !== 6) {
    pairError.textContent = 'El código debe tener 6 dígitos'
    show(pairError, true)
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
    setResult('Extensión emparejada correctamente.', 'ok')
  } catch (error) {
    pairError.textContent = errorMessage(error)
    show(pairError, true)
  } finally {
    pairSubmit.disabled = false
    pairSubmit.textContent = 'Conectar con el ERP'
  }
})

codeInput.addEventListener('input', () => {
  const digits = codeInput.value.replace(/\D/g, '').slice(0, 6)
  if (codeInput.value !== digits) codeInput.value = digits
})

testBtn.addEventListener('click', async () => {
  if (!currentState.paired) {
    setResult('Conecta primero la extensión con el ERP.', 'error')
    return
  }
  testBtn.disabled = true
  testBtn.textContent = 'Comprobando…'
  try {
    const filings = await listFilings()
    setResult(
      filings.length
        ? `Conexión correcta. Hay ${filings.length} expediente${filings.length === 1 ? '' : 's'} abierto${filings.length === 1 ? '' : 's'}.`
        : 'Conexión correcta. No hay expedientes abiertos en este momento.',
      'ok',
    )
  } catch (error) {
    setResult(errorMessage(error), 'error')
  } finally {
    testBtn.disabled = false
    testBtn.textContent = 'Comprobar conexión'
  }
})

unpairBtn.addEventListener('click', async () => {
  if (!window.confirm('¿Desconectar esta extensión del ERP? Podrás volver a emparejarla con un código nuevo.')) return
  try {
    await unpair()
    const config = await getConfig()
    applyState({ paired: false, apiUrl: config.apiUrl, tokenName: '' })
    setResult('La extensión se ha desconectado del ERP.', 'ok')
  } catch (error) {
    setResult(errorMessage(error), 'error')
  }
})

openErpBtn.addEventListener('click', () => {
  void openErp().catch((error) => setResult(errorMessage(error), 'error'))
})

async function init() {
  footerVersion.textContent = `v${chrome.runtime.getManifest().version}`
  statusVersion.textContent = `v${chrome.runtime.getManifest().version}`
  try {
    const config = await getConfig()
    apiUrlInput.value = config.apiUrl || DEFAULT_API
    const stored = await chrome.storage.local.get('extensionName')
    nameInput.value = String(stored.extensionName || '') || defaultExtensionName()
  } catch {
    apiUrlInput.value = DEFAULT_API
    nameInput.value = defaultExtensionName()
  }
  await loadState()
}

void init()
