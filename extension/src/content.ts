import { adapterForPage, applyAdapter, filingExercise, previewAdapter } from './adapters'
import type { AdapterPreview } from './adapters'
import { priorExerciseLabel } from './lib/fiscal'
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
  const adapter = adapterForPage(
    context.filing.model_code,
    filingExercise(context.filing),
    context.filing.entity,
  )
  const preview = adapter ? previewAdapter(adapter, context) : null
  const dataMissing = preview?.dataMissing ?? []
  const noDataText = dataMissing
    .slice(0, 5)
    .map((field) => (field.casilla ? `${field.casilla} (${field.label})` : field.label))
    .join(', ')
  const canFill = Boolean(
    adapter?.verified &&
      adapter.fields.length &&
      !dataMissing.length &&
      context.filing.status !== 'presented' &&
      context.filing.status !== 'not_applicable',
  )
  const adapterPending = Boolean(adapter && !adapter.verified)
  const documents = context.documents.slice(0, 8)
  const status = context.filing.status
  const exercise = priorExerciseLabel(context.filing, context.taxpayer.fiscal_year_end)
  const missingReceipt = status === 'presented' && !context.filing.receipt_document_id

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
          ${
            exercise
              ? `<span class="exercise" title="Periodo del ejercicio anterior, presentado después del cambio de ejercicio">${escapeHtml(exercise)}</span>`
              : ''
          }
        </div>

        ${
          status === 'presented'
            ? '<div class="banner ok">Este expediente ya está marcado como presentado.</div>'
            : ''
        }
        ${
          missingReceipt
            ? '<div class="banner warn">Presentado sin justificante adjunto. Añádelo en el ERP.</div>'
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
        ${
          adapterPending
            ? '<div class="banner info">El relleno asistido de este modelo está en preparación. Puedes copiar las casillas y presentarlas manualmente.</div>'
            : ''
        }
        ${
          dataMissing.length
            ? `<div class="banner warn">Faltan ${dataMissing.length} valor(es) en el ERP: ${escapeHtml(
                noDataText,
              )}${dataMissing.length > 5 ? ` y ${dataMissing.length - 5} más` : ''}. Revisa el cálculo antes de rellenar.</div>`
            : ''
        }
        <div class="banner ok" data-fill-result hidden></div>

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
          <button class="btn" data-act="copy-all" ${lines.length ? '' : 'disabled'}>Copiar todas las casillas</button>
          ${canFill ? '<button class="btn primary" data-act="fill">Revisar relleno</button>' : ''}
          <button class="btn" data-act="erp">Abrir ERP</button>
        </div>
        ${
          adapter
            ? `<p class="note">Adaptador ${escapeHtml(adapter.code)}/${escapeHtml(
                adapter.exercise ?? '',
              )} · v${escapeHtml(adapter.version)} · actualizado ${escapeHtml(
                adapter.updatedAt,
              )} · ${adapter.verified ? 'verificado' : 'en preparación'}.</p>`
            : ''
        }

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
      .banner.warn { background: rgba(251,191,36,.1); color: #fcd34d; border: 1px solid rgba(251,191,36,.22); }
      .banner.info { background: rgba(56,189,248,.1); color: #bae6fd; border: 1px solid rgba(56,189,248,.22); }
      .banner[hidden] { display: none; }
      .exercise {
        display: inline-flex; align-items: center; padding: 0 6px; border-radius: 999px;
        background: rgba(129,140,248,.15); border: 1px solid rgba(129,140,248,.3);
        color: #c7d2fe; font-size: 10px; font-weight: 700; line-height: 1.6;
      }
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
      .review {
        position: fixed; inset: 0; z-index: 2147483648;
        display: grid; place-items: center; padding: 16px;
        background: rgba(2,6,23,.72);
      }
      .review[hidden] { display: none; }
      .review-card {
        width: min(460px, 100%); max-height: calc(100vh - 32px);
        display: flex; flex-direction: column;
        background: #0f172a; color: #e2e8f0;
        border: 1px solid rgba(255,255,255,.12); border-radius: 16px;
        box-shadow: 0 24px 60px rgba(2,6,23,.6);
        font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
        font-size: 13px; line-height: 1.45; overflow: hidden;
      }
      .review-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 12px 14px; background: #020617; border-bottom: 1px solid rgba(255,255,255,.08); }
      .review-head strong { display: block; color: #f8fafc; font-size: 14px; }
      .review-sub { display: block; margin-top: 2px; color: #94a3b8; font-size: 10px; }
      .review-body { padding: 12px 14px; overflow-y: auto; }
      .review-summary { margin: 0 0 8px; color: #cbd5e1; font-size: 11px; }
      .review-row { display: grid; grid-template-columns: 34px 1fr; gap: 8px; padding: 8px 0; border-top: 1px solid rgba(255,255,255,.06); }
      .review-row:first-child { border-top: 0; }
      .review-casilla { color: #64748b; font-family: ui-monospace, monospace; font-size: 10px; padding-top: 2px; }
      .review-label { color: #cbd5e1; font-size: 11px; }
      .review-values { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-top: 3px; font-family: ui-monospace, monospace; font-size: 11px; }
      .review-old { color: #94a3b8; text-decoration: line-through; }
      .review-new { color: #6ee7b7; font-weight: 700; }
      .review-warn { color: #fcd34d; font-family: ui-sans-serif, system-ui, sans-serif; font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; }
      .review-row.overwrite { background: rgba(251,191,36,.06); }
      .review-actions { display: flex; justify-content: flex-end; gap: 6px; padding: 10px 14px; border-top: 1px solid rgba(255,255,255,.08); background: #0b1222; }
    </style>
    ${panelMarkup(context)}
    <div class="review" data-review hidden>
      <div class="review-card" role="dialog" aria-modal="true" aria-labelledby="asesoria-review-title">
        <div class="review-head">
          <div>
            <strong id="asesoria-review-title">Revisar relleno</strong>
            <span class="review-sub">Nada se envía hasta que tú firmes.</span>
          </div>
          <button class="icon-btn" data-act="review-close" title="Cerrar">×</button>
        </div>
        <div class="review-body">
          <p class="review-summary" data-review-summary></p>
          <div data-review-list></div>
        </div>
        <div class="review-actions">
          <button class="btn" data-act="review-cancel">Cancelar</button>
          <button class="btn primary" data-act="review-apply">Aplicar relleno</button>
        </div>
      </div>
    </div>
  `

  const panel = shadow.querySelector<HTMLElement>('[data-panel]')!
  const toast = shadow.querySelector<HTMLElement>('[data-toast]')!

  const review = shadow.querySelector<HTMLElement>('[data-review]')!
  const reviewList = shadow.querySelector<HTMLElement>('[data-review-list]')!
  const reviewSummary = shadow.querySelector<HTMLElement>('[data-review-summary]')!
  const reviewApply = shadow.querySelector<HTMLButtonElement>('[data-act="review-apply"]')!
  const fillResult = shadow.querySelector<HTMLElement>('[data-fill-result]')!

  let toastTimer = 0
  const flash = (message: string, isError = false) => {
    toast.textContent = message
    toast.style.color = isError ? '#fda4af' : '#34d399'
    window.clearTimeout(toastTimer)
    toastTimer = window.setTimeout(() => (toast.textContent = ''), 3_000)
  }

  const closeReview = () => {
    review.hidden = true
    reviewList.innerHTML = ''
  }

  const renderReview = (preview: AdapterPreview) => {
    const overwrites = preview.changes.filter((field) => field.overwrite).length
    const locked = preview.locked.length
    reviewSummary.textContent = [
      `Se van a escribir ${preview.changes.length} campo(s).`,
      overwrites ? `${overwrites} sobrescribe(n) un valor existente.` : '',
      locked ? `${locked} campo(s) calculados por la sede se omiten.` : '',
      'Revisa los importes y comprueba que el NIF y el periodo del formulario coinciden con el expediente antes de aplicar.',
    ]
      .filter(Boolean)
      .join(' ')
    reviewList.innerHTML = preview.changes
      .map(
        (field) => `<div class="review-row${field.overwrite ? ' overwrite' : ''}">
          <span class="review-casilla">${escapeHtml(field.casilla ?? '—')}</span>
          <div>
            <div class="review-label">${escapeHtml(field.label)}</div>
            <div class="review-values">
              <span class="review-old">${escapeHtml(field.current || 'vacío')}</span>
              <span>→</span>
              <span class="review-new">${escapeHtml(field.next)}</span>
              ${field.overwrite ? '<span class="review-warn">sobrescribe</span>' : ''}
            </div>
          </div>
        </div>`,
      )
      .join('')
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

  shadow.querySelector('[data-act="fill"]')?.addEventListener('click', () => {
    const adapter = adapterForPage(
      context.filing.model_code,
      filingExercise(context.filing),
      context.filing.entity,
    )
    if (!adapter) return
    try {
      const preview = previewAdapter(adapter, context)
      if (!preview.pageReady) {
        flash(adapter.pageHint ?? 'Abre la página correcta del formulario y vuelve a intentarlo.', true)
        return
      }
      if (preview.missing.length) {
        flash(
          `No encuentro ${preview.missing.length} campo(s). ${adapter.pageHint ?? 'Abre la página correcta y vuelve a intentarlo.'}`,
          true,
        )
        return
      }
      if (preview.dataMissing.length) {
        const labels = preview.dataMissing
          .slice(0, 5)
          .map((field) => (field.casilla ? `${field.casilla} (${field.label})` : field.label))
          .join(', ')
        flash(`Faltan ${preview.dataMissing.length} valor(es) en el ERP: ${labels}`, true)
        return
      }
      if (!preview.changes.length) {
        flash('No hay valores nuevos que aplicar')
        return
      }
      fillResult.hidden = true
      renderReview(preview)
      review.hidden = false
    } catch (error) {
      flash(error instanceof Error ? error.message : 'No se pudo preparar el relleno', true)
    }
  })

  reviewApply.addEventListener('click', async () => {
    const adapter = adapterForPage(
      context.filing.model_code,
      filingExercise(context.filing),
      context.filing.entity,
    )
    if (!adapter) return
    reviewApply.disabled = true
    try {
      const result = await applyAdapter(adapter, context)
      closeReview()
      fillResult.hidden = false
      fillResult.textContent = `Relleno aplicado en ${result.applied} campo(s)${
        result.skipped ? ` (${result.skipped} omitido(s))` : ''
      }. Revisa el formulario, fírmalo y preséntalo manualmente.`
      flash('Relleno aplicado')
    } catch (error) {
      flash(error instanceof Error ? error.message : 'No se pudo rellenar', true)
    } finally {
      reviewApply.disabled = false
    }
  })

  shadow.querySelector('[data-act="review-cancel"]')?.addEventListener('click', closeReview)
  shadow.querySelector('[data-act="review-close"]')?.addEventListener('click', closeReview)
  review.addEventListener('click', (event) => {
    if (event.target === review) closeReview()
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
