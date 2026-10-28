import { getAdapter, runAdapter } from './adapters'
import type { ExtensionContext } from './lib/types'

const HOST_ID = 'asesoria-assistant-host'

function send<T>(message: unknown): Promise<T> {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage(message, (response: T & { error?: string }) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message))
        return
      }
      if (response && typeof response === 'object' && 'error' in response && response.error) {
        reject(new Error(String(response.error)))
        return
      }
      resolve(response)
    })
  })
}

function escapeHtml(value: string): string {
  const div = document.createElement('div')
  div.textContent = value
  return div.innerHTML
}

function formatDate(value: string): string {
  if (!value) return '—'
  try {
    return new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: 'short', year: 'numeric' }).format(
      new Date(`${value}T00:00:00`),
    )
  } catch {
    return value
  }
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    pending: 'Pendiente',
    in_progress: 'En curso',
    ready: 'Listo para presentar',
    presented: 'Presentado',
    late: 'Fuera de plazo',
    not_applicable: 'No aplica',
  }
  return map[status] ?? status
}

function casillasText(context: ExtensionContext): string {
  const lines = context.filing.computed?.lines ?? []
  const header = [
    `${context.filing.model_code} · ${context.filing.model_name}`,
    `Periodo: ${context.filing.period_label}`,
    `Vencimiento: ${formatDate(context.filing.due_date)}`,
    `Contribuyente: ${context.taxpayer.name} (${context.taxpayer.nif})`,
    '',
  ]
  const body = lines.map((line) => `${line.id}\t${line.label}\t${line.value}${line.kind === 'tipo' ? ' %' : ' €'}`)
  return [...header, ...body].join('\n')
}

async function copyText(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const area = document.createElement('textarea')
    area.value = text
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    document.execCommand('copy')
    area.remove()
  }
}

async function downloadDocument(id: string, filename: string): Promise<void> {
  const data = await send<{ base64: string; mime: string }>({ type: 'DOWNLOAD_DOC', id })
  const binary = atob(data.base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  const blob = new Blob([bytes], { type: data.mime })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 4_000)
}

function panelMarkup(context: ExtensionContext): string {
  const lines = context.filing.computed?.lines ?? []
  const warnings = context.filing.computed?.warnings ?? []
  const adapter = getAdapter(context.filing.model_code)
  const canFill = Boolean(adapter?.verified && adapter.steps.length)
  const documents = context.documents.slice(0, 8)
  const status = context.filing.status

  return `
    <div class="panel" data-panel>
      <div class="head">
        <div class="brand"><span class="dot"></span> Asesoría</div>
        <div class="head-actions">
          <button class="icon-btn" data-act="min" title="Minimizar">–</button>
          <button class="icon-btn" data-act="close" title="Cerrar">×</button>
        </div>
      </div>
      <div class="body">
        <div class="filing-head">
          <span class="code">${escapeHtml(context.filing.model_code)}</span>
          <div class="filing-title">
            <h1>${escapeHtml(context.filing.model_name)}</h1>
            <p>${escapeHtml(context.taxpayer.name)} · ${escapeHtml(context.taxpayer.nif)}</p>
          </div>
          <span class="status ${status}">${escapeHtml(statusLabel(status))}</span>
        </div>
        <div class="meta">
          <span>${escapeHtml(context.filing.period_label)}</span>
          <span>·</span>
          <span>Vence ${escapeHtml(formatDate(context.filing.due_date))}</span>
          <span>·</span>
          <span>${escapeHtml(context.filing.entity)}</span>
        </div>

        ${
          status === 'presented'
            ? '<div class="banner ok">Este expediente ya está marcado como presentado.</div>'
            : ''
        }
        ${
          warnings.length
            ? `<div class="warnings">${warnings
                .slice(0, 3)
                .map((warning) => `<p>⚠ ${escapeHtml(warning)}</p>`)
                .join('')}</div>`
            : ''
        }

        <div class="section-title">
          <span>Casillas del modelo</span>
          <span class="section-hint">${lines.length} campos</span>
        </div>
        ${
          lines.length
            ? `<div class="table-wrap">
                <table>
                  <thead><tr><th>#</th><th>Concepto</th><th class="right">Valor</th><th></th></tr></thead>
                  <tbody>
                    ${lines
                      .map(
                        (line) => `<tr>
                          <td class="casilla">${escapeHtml(line.id)}</td>
                          <td>${escapeHtml(line.label)}</td>
                          <td class="right mono">${escapeHtml(line.value)}${line.kind === 'tipo' ? ' %' : ' €'}</td>
                          <td class="right"><button class="copy-btn" data-copy="${escapeHtml(
                            `${line.id} ${line.label}: ${line.value}${line.kind === 'tipo' ? ' %' : ' €'}`,
                          )}" title="Copiar">⧉</button></td>
                        </tr>`,
                      )
                      .join('')}
                  </tbody>
                </table>
              </div>`
            : `<div class="empty">Sin cálculo todavía. Abre el ERP y pulsa «Calcular» antes de presentar.</div>`
        }

        <div class="actions">
          <button class="btn primary" data-act="copy-all" ${lines.length ? '' : 'disabled'}>Copiar todas las casillas</button>
          ${canFill ? '<button class="btn" data-act="fill">Rellenar formulario</button>' : ''}
          <button class="btn" data-act="erp">Abrir ERP</button>
        </div>

        <div class="section-title">
          <span>Documentación</span>
          <span class="section-hint">${context.documents.length} ficheros</span>
        </div>
        ${
          documents.length
            ? `<ul class="docs">${documents
                .map(
                  (doc) => `<li>
                    <span class="doc-name">${escapeHtml(doc.filename)}</span>
                    <button class="copy-btn" data-doc="${escapeHtml(doc.id)}" data-filename="${escapeHtml(
                      doc.filename,
                    )}" title="Descargar">↓</button>
                  </li>`,
                )
                .join('')}</ul>`
            : '<div class="empty small">No hay documentos subidos para este contribuyente.</div>'
        }

        <p class="note">
          La extensión nunca accede a tu certificado. Rellena y revisa el formulario; la firma se realiza
          siempre en el navegador con tu certificado electrónico.
        </p>
        <div class="toast" data-toast></div>
      </div>
    </div>
  `
}

function mount(context: ExtensionContext): void {
  if (document.getElementById(HOST_ID)) return
  const host = document.createElement('div')
  host.id = HOST_ID
  const shadow = host.attachShadow({ mode: 'open' })
  shadow.innerHTML = `
    <style>
      :host { all: initial; }
      * { box-sizing: border-box; }
      .panel {
        position: fixed; top: 16px; right: 16px;
        width: min(400px, calc(100vw - 24px));
        max-height: calc(100vh - 32px);
        display: flex; flex-direction: column;
        background: #0f172a; color: #e2e8f0;
        border: 1px solid rgba(255,255,255,.1);
        border-radius: 18px;
        box-shadow: 0 24px 60px rgba(2,6,23,.5);
        font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
        font-size: 13px; line-height: 1.45;
        overflow: hidden; z-index: 2147483647;
      }
      .head {
        display: flex; align-items: center; justify-content: space-between;
        padding: 10px 12px; background: #020617; border-bottom: 1px solid rgba(255,255,255,.08);
      }
      .brand { display: flex; align-items: center; gap: 8px; font-weight: 800; color: #34d399; letter-spacing: .02em; }
      .dot { width: 8px; height: 8px; border-radius: 999px; background: #34d399; box-shadow: 0 0 0 4px rgba(52,211,153,.15); }
      .head-actions { display: flex; gap: 4px; }
      .icon-btn {
        width: 26px; height: 26px; border: 0; border-radius: 8px; background: transparent;
        color: #94a3b8; font-size: 17px; line-height: 1; cursor: pointer;
      }
      .icon-btn:hover { background: rgba(255,255,255,.08); color: #fff; }
      .body { padding: 14px; overflow-y: auto; }
      .panel.min .body { display: none; }
      .filing-head { display: flex; align-items: flex-start; gap: 10px; }
      .code {
        display: grid; place-items: center; min-width: 48px; height: 38px; padding: 0 8px;
        border-radius: 10px; background: #34d399; color: #052e1b; font-weight: 800; font-size: 13px;
      }
      .filing-title { flex: 1; min-width: 0; }
      .filing-title h1 { margin: 0; font-size: 14px; font-weight: 700; color: #f8fafc; }
      .filing-title p { margin: 2px 0 0; font-size: 11px; color: #94a3b8; }
      .status { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; padding: 3px 7px; border-radius: 999px; background: rgba(148,163,184,.15); color: #cbd5e1; white-space: nowrap; }
      .status.presented { background: rgba(52,211,153,.15); color: #6ee7b7; }
      .status.ready { background: rgba(56,189,248,.15); color: #7dd3fc; }
      .status.late { background: rgba(251,113,133,.15); color: #fda4af; }
      .meta { margin-top: 10px; display: flex; flex-wrap: wrap; gap: 5px; font-size: 11px; color: #94a3b8; }
      .banner { margin-top: 10px; padding: 8px 10px; border-radius: 10px; font-size: 12px; }
      .banner.ok { background: rgba(52,211,153,.12); color: #a7f3d0; border: 1px solid rgba(52,211,153,.25); }
      .warnings { margin-top: 10px; padding: 8px 10px; border-radius: 10px; background: rgba(251,191,36,.1); border: 1px solid rgba(251,191,36,.22); color: #fcd34d; font-size: 11px; }
      .warnings p { margin: 2px 0; }
      .section-title { display: flex; align-items: center; justify-content: space-between; margin: 14px 0 6px; font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: .08em; color: #64748b; }
      .section-hint { font-weight: 600; letter-spacing: 0; text-transform: none; color: #475569; }
      .table-wrap { overflow: auto; border: 1px solid rgba(255,255,255,.07); border-radius: 10px; max-height: 240px; }
      table { width: 100%; border-collapse: collapse; }
      th { position: sticky; top: 0; background: #0b1222; text-align: left; font-size: 9px; text-transform: uppercase; letter-spacing: .06em; color: #64748b; padding: 6px 8px; }
      td { padding: 6px 8px; border-top: 1px solid rgba(255,255,255,.06); vertical-align: top; color: #cbd5e1; }
      td.casilla { color: #64748b; font-family: ui-monospace, monospace; font-size: 10px; }
      .right { text-align: right; }
      .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; white-space: nowrap; color: #f1f5f9; }
      .copy-btn { border: 0; background: transparent; color: #64748b; cursor: pointer; border-radius: 6px; padding: 2px 5px; font-size: 12px; }
      .copy-btn:hover { background: rgba(255,255,255,.08); color: #e2e8f0; }
      .actions { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
      .btn { border: 1px solid rgba(255,255,255,.12); background: rgba(255,255,255,.04); color: #cbd5e1; border-radius: 9px; padding: 7px 10px; font-size: 11px; font-weight: 700; cursor: pointer; }
      .btn:hover { background: rgba(255,255,255,.09); color: #fff; }
      .btn.primary { background: #34d399; border-color: #34d399; color: #052e1b; }
      .btn.primary:hover { background: #6ee7b7; }
      .btn:disabled { opacity: .45; cursor: not-allowed; }
      .docs { list-style: none; margin: 0; padding: 0; border: 1px solid rgba(255,255,255,.07); border-radius: 10px; overflow: hidden; }
      .docs li { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 7px 9px; border-top: 1px solid rgba(255,255,255,.06); }
      .docs li:first-child { border-top: 0; }
      .doc-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #cbd5e1; font-size: 11px; }
      .empty { padding: 14px; border: 1px dashed rgba(255,255,255,.12); border-radius: 10px; color: #64748b; font-size: 11px; text-align: center; }
      .empty.small { padding: 9px; }
      .note { margin: 12px 0 0; font-size: 10px; line-height: 1.5; color: #64748b; }
      .toast { margin-top: 8px; font-size: 11px; font-weight: 700; color: #34d399; min-height: 15px; text-align: right; }
    </style>
    ${panelMarkup(context)}
  `

  const panel = shadow.querySelector<HTMLElement>('[data-panel]')!
  const toast = shadow.querySelector<HTMLElement>('[data-toast]')!

  let toastTimer = 0
  const flash = (message: string, isError = false) => {
    toast.textContent = message
    toast.style.color = isError ? '#fda4af' : '#34d399'
    window.clearTimeout(toastTimer)
    toastTimer = window.setTimeout(() => (toast.textContent = ''), 3_000)
  }

  shadow.querySelector('[data-act="close"]')?.addEventListener('click', async () => {
    await send({ type: 'CLEAR_ACTIVE' }).catch(() => undefined)
    host.remove()
  })
  shadow.querySelector('[data-act="min"]')?.addEventListener('click', () => {
    panel.classList.toggle('min')
  })

  shadow.querySelectorAll<HTMLElement>('[data-copy]').forEach((button) => {
    button.addEventListener('click', async () => {
      await copyText(button.dataset.copy ?? '')
      flash('Copiado')
    })
  })

  shadow.querySelector('[data-act="copy-all"]')?.addEventListener('click', async () => {
    await copyText(casillasText(context))
    flash('Casillas copiadas al portapapeles')
  })

  shadow.querySelector('[data-act="erp"]')?.addEventListener('click', () => {
    send({ type: 'OPEN_ERP' }).catch((error: Error) => flash(error.message, true))
  })

  shadow.querySelector('[data-act="fill"]')?.addEventListener('click', async () => {
    const adapter = getAdapter(context.filing.model_code)
    if (!adapter) return
    try {
      await runAdapter(adapter, context, (text) => flash(text))
    } catch (error) {
      flash(error instanceof Error ? error.message : 'No se pudo rellenar', true)
    }
  })

  shadow.querySelectorAll<HTMLElement>('[data-doc]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        await downloadDocument(button.dataset.doc ?? '', button.dataset.filename ?? 'documento')
        flash('Documento descargado')
      } catch (error) {
        flash(error instanceof Error ? error.message : 'No se pudo descargar', true)
      }
    })
  })

  document.documentElement.appendChild(host)
}

async function boot() {
  try {
    const response = await send<{ context: ExtensionContext | null }>({ type: 'GET_CONTEXT' })
    if (response?.context) mount(response.context)
  } catch {
    /* The extension may not be paired; stay silent on public pages. */
  }
}

void boot()
