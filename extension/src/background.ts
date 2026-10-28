import {
  apiBlob,
  apiFetch,
  claimPairing,
  clearConfig,
  getConfig,
  getContext,
  listFilings,
  saveConfig,
} from './lib/api'
import type { ExtensionContext, ExtensionFiling, ExtensionState } from './lib/types'

type Message = { type: string; [key: string]: unknown }

async function state(): Promise<ExtensionState> {
  const config = await getConfig()
  return { paired: Boolean(config.token), apiUrl: config.apiUrl, tokenName: config.tokenName }
}

async function handle(message: Message): Promise<unknown> {
  switch (message.type) {
    case 'STATE':
      return state()

    case 'PAIR': {
      const config = await claimPairing(String(message.apiUrl), String(message.code), String(message.name))
      await saveConfig(config)
      return { paired: true, apiUrl: config.apiUrl, tokenName: config.tokenName }
    }

    case 'UNPAIR': {
      await clearConfig()
      await chrome.storage.session.remove('activeFilingId')
      return { ok: true }
    }

    case 'LIST_FILINGS': {
      const filings = await listFilings()
      return filings
    }

    case 'OPEN_FILING': {
      const id = String(message.id || '')
      const url = String(message.url || '')
      if (!id || !url) throw new Error('Expediente o URL no válidos')
      await chrome.storage.session.set({ activeFilingId: id })
      await chrome.tabs.create({ url })
      return { ok: true }
    }

    case 'GET_CONTEXT': {
      return getContext()
    }

    case 'CLEAR_ACTIVE': {
      await chrome.storage.session.remove('activeFilingId')
      return { ok: true }
    }

    case 'DOWNLOAD_DOC': {
      const id = String(message.id || '')
      if (!id) throw new Error('Documento no válido')
      return apiBlob(`/api/documents/${encodeURIComponent(id)}/download`)
    }

    case 'OPEN_ERP': {
      const config = await getConfig()
      await chrome.tabs.create({ url: config.apiUrl })
      return { ok: true }
    }

    case 'OPEN_ERP_FILING': {
      const id = String(message.id || '')
      if (!id) throw new Error('Expediente no válido')
      const config = await getConfig()
      await chrome.storage.session.set({ activeFilingId: id })
      await chrome.tabs.create({ url: `${config.apiUrl}/expedientes/${encodeURIComponent(id)}` })
      return { ok: true }
    }

    case 'OPEN_OPTIONS': {
      chrome.runtime.openOptionsPage()
      return { ok: true }
    }

    case 'PING':
      return { ok: true }

    default:
      throw new Error('Mensaje no soportado')
  }
}

chrome.runtime.onMessage.addListener((message: Message, _sender, sendResponse) => {
  handle(message)
    .then((result) => sendResponse(result))
    .catch((error: unknown) => sendResponse({ error: error instanceof Error ? error.message : String(error) }))
  return true
})

chrome.runtime.onInstalled.addListener(async () => {
  const config = await getConfig()
  await saveConfig({ apiUrl: config.apiUrl })
})

export type { ExtensionContext, ExtensionFiling }
