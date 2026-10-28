import type { ExtensionContext } from '../lib/types'

export type StepAction = 'wait' | 'set' | 'select' | 'click' | 'pause'

export interface Step {
  action: StepAction
  selector?: string
  value?: string
  timeout?: number
  message?: string
}

export interface Adapter {
  code: string
  version: string
  /** A false value means the selectors have not been verified against the live form. */
  verified: boolean
  startUrl?: string
  steps: Step[]
}

/**
 * Adapters are intentionally declarative and versioned. No adapter is shipped as
 * "verified" yet: the AEAT form DOM must be validated for each exercise before
 * enabling automatic filling. The assistant panel always works as a fallback.
 */
export const adapters: Record<string, Adapter> = {}

export function getAdapter(code: string): Adapter | undefined {
  return adapters[code]
}

function resolve(value: string | undefined, context: ExtensionContext): string {
  if (!value) return ''
  return value.replace(/\{\{casilla\.([A-Za-z0-9]+)\}\}/g, (_, id: string) => {
    return context.filing.computed?.values?.[id] ?? ''
  })
}

function waitFor(selector: string, timeout = 10_000): Promise<Element> {
  return new Promise((resolve, reject) => {
    const existing = document.querySelector(selector)
    if (existing) return resolve(existing)
    const observer = new MutationObserver(() => {
      const el = document.querySelector(selector)
      if (el) {
        observer.disconnect()
        clearTimeout(timer)
        resolve(el)
      }
    })
    observer.observe(document.documentElement, { childList: true, subtree: true })
    const timer = window.setTimeout(() => {
      observer.disconnect()
      reject(new Error(`No se encontró el campo ${selector}`))
    }, timeout)
  })
}

function setNativeValue(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype
  const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set
  setter?.call(el, value)
  el.dispatchEvent(new Event('input', { bubbles: true }))
  el.dispatchEvent(new Event('change', { bubbles: true }))
}

export async function runAdapter(
  adapter: Adapter,
  context: ExtensionContext,
  onProgress: (text: string) => void,
): Promise<void> {
  if (!adapter.verified) throw new Error('Este adaptador todavía no está verificado para el ejercicio actual')
  for (const step of adapter.steps) {
    if (step.action === 'pause') {
      onProgress(step.message ?? 'Revisa el formulario y firma con tu certificado.')
      return
    }
    if (!step.selector) throw new Error('Paso sin selector')
    onProgress(`Ejecutando ${step.action}…`)
    const element = await waitFor(step.selector, step.timeout)
    switch (step.action) {
      case 'wait':
        break
      case 'set': {
        if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
          setNativeValue(element, resolve(step.value, context))
        }
        break
      }
      case 'select': {
        if (element instanceof HTMLSelectElement) {
          element.value = resolve(step.value, context)
          element.dispatchEvent(new Event('change', { bubbles: true }))
        }
        break
      }
      case 'click': {
        ;(element as HTMLElement).click()
        break
      }
    }
    await new Promise((r) => setTimeout(r, 120))
  }
  onProgress('Formulario completado. Revisa los datos y firma con tu certificado.')
}
