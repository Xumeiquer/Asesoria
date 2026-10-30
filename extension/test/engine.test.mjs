import assert from 'node:assert/strict'
import test from 'node:test'

import { build } from 'esbuild'
import { parseHTML } from 'linkedom'

const fixture = `<!doctype html>
<html>
  <body>
    <main>IVA devengado. Total cuota devengada.</main>
    <input aria-label="101" />
    <input aria-label=" 01" />
    <input aria-label=" 03" />
    <input aria-label=" 04" />
    <input aria-label=" 06" />
    <input aria-label=" 07" disabled />
    <input aria-label=" 08" />
  </body>
</html>`

const { window, document } = parseHTML(fixture)
globalThis.window = window
globalThis.document = document
globalThis.Event = window.Event
globalThis.HTMLElement = window.HTMLElement
globalThis.HTMLInputElement = window.HTMLInputElement
globalThis.HTMLTextAreaElement = window.HTMLTextAreaElement
globalThis.HTMLSelectElement = window.HTMLSelectElement
window.setTimeout = setTimeout

const bundled = await build({
  entryPoints: ['src/adapters/index.ts'],
  bundle: true,
  format: 'esm',
  platform: 'browser',
  target: 'es2022',
  write: false,
})
const moduleUrl = `data:text/javascript;base64,${Buffer.from(bundled.outputFiles[0].text).toString('base64')}`
const engine = await import(moduleUrl)

const context = {
  filing: {
    id: 'filing-1',
    taxpayer_id: 'taxpayer-1',
    model_code: '303',
    model_name: 'IVA. Autoliquidación',
    entity: 'AEAT',
    period_label: '3T',
    period_start: '2026-07-01',
    period_end: '2026-09-30',
    due_date: '2026-10-20',
    status: 'ready',
    computed: {
      values: {
        '01': '1000.00',
        '03': '40.00',
        '04': '500.00',
        '06': '50.00',
        '07': '2000.00',
        '09': '420.00',
      },
    },
  },
  taxpayer: {
    id: 'taxpayer-1',
    name: 'Persona de prueba',
    nif: '12345678z',
    legal_form: 'autonomo',
  },
  model: {},
  documents: [],
}

test('resuelve placeholders y formatos oficiales', () => {
  assert.equal(engine.resolveValue('{{casilla.01}}', context), '1000.00')
  assert.equal(engine.resolveValue('{{taxpayer.nif}}', context), '12345678z')
  assert.equal(engine.resolveValue('{{filing.period}}', context), '3T')
  assert.equal(engine.resolveValue('{{exercise}}', context), '2026')
  assert.equal(engine.formatValue('1000.00', { kind: 'amount' }), '1000,00')
  assert.equal(engine.formatValue('0', { kind: 'amount' }), '0,00')
  assert.equal(engine.formatValue('21', { kind: 'percent' }), '21,00')
  assert.equal(engine.formatValue('2026-10-20', { kind: 'date' }), '20/10/2026')
  assert.equal(engine.formatValue('12345678z', { kind: 'nif' }), '12345678Z')
})

test('previsualiza sin escribir y aplica solo tras la confirmación', async () => {
  const adapter = {
    code: 'TEST',
    version: '1',
    verified: true,
    page: { contains: ['IVA devengado'] },
    fields: [
      { casilla: '01', label: 'Base al 4%', selector: 'input[aria-label$="01"]', kind: 'amount' },
      { casilla: '03', label: 'Cuota al 4%', selector: 'input[aria-label$="03"]', kind: 'amount' },
      { casilla: '04', label: 'Base al 10%', selector: 'input[aria-label$="04"]', kind: 'amount' },
      { casilla: '06', label: 'Cuota al 10%', selector: 'input[aria-label$="06"]', kind: 'amount' },
      { casilla: '07', label: 'Base bloqueada', selector: 'input[aria-label$="07"]', kind: 'amount' },
    ],
  }

  const preview = engine.previewAdapter(adapter, context)
  assert.equal(preview.pageReady, true)
  assert.deepEqual(
    preview.changes.map((field) => [field.casilla, field.current, field.next]),
    [
      ['01', '', '1000,00'],
      ['03', '', '40,00'],
      ['04', '', '500,00'],
      ['06', '', '50,00'],
    ],
  )
  assert.equal(document.querySelector('input[aria-label=" 01"]').value, '')
  assert.equal(document.querySelector('input[aria-label="101"]').value, '')
  assert.deepEqual(preview.locked.map((field) => field.casilla), ['07'])

  const applied = await engine.applyAdapter(adapter, context)
  assert.equal(applied.applied, 4)
  assert.equal(document.querySelector('input[aria-label=" 01"]').value, '1000,00')
  assert.equal(document.querySelector('input[aria-label=" 03"]').value, '40,00')
  assert.equal(document.querySelector('input[aria-label=" 07"]').value, '')
  assert.equal(document.querySelector('input[aria-label="101"]').value, '')

  document.querySelector('input[aria-label=" 01"]').value = '1.000,00'
  assert.equal(
    engine.previewAdapter(adapter, context).changes.some((field) => field.casilla === '01'),
    false,
  )
})

test('bloquea la aplicación si falta un campo obligatorio o la página no es la correcta', async () => {
  const missing = {
    code: 'TEST',
    version: '1',
    verified: true,
    page: { contains: ['IVA devengado'] },
    fields: [{ casilla: '99', label: 'Inexistente', selector: 'input[aria-label$="99"]' }],
  }
  const preview = engine.previewAdapter(missing, context)
  assert.equal(preview.missing.length, 1)
  await assert.rejects(() => engine.applyAdapter(missing, context), /No encuentro/)

  const wrongPage = {
    ...missing,
    fields: [],
    page: { contains: ['Otra página'] },
  }
  assert.equal(engine.previewAdapter(wrongPage, context).pageReady, false)
  await assert.rejects(() => engine.applyAdapter(wrongPage, context), /Abre la página correcta/)

  const valid = {
    code: 'TEST',
    version: '1',
    verified: true,
    page: { contains: ['IVA devengado'] },
    fields: [{ casilla: '01', label: 'Base al 4%', selector: 'input[aria-label$="01"]' }],
  }
  for (const status of ['presented', 'not_applicable']) {
    const closed = { ...context, filing: { ...context.filing, status } }
    await assert.rejects(() => engine.applyAdapter(valid, closed), /presentado o no aplica/)
  }
})

test('bloquea el relleno si falta un valor requerido en el ERP', async () => {
  const noData = {
    code: 'TEST',
    version: '1',
    verified: true,
    page: { contains: ['IVA devengado'] },
    fields: [{ casilla: '08', label: 'Dato sin calcular', selector: 'input[aria-label$="08"]' }],
  }
  const preview = engine.previewAdapter(noData, context)
  assert.equal(preview.pageReady, true)
  assert.equal(preview.dataMissing.length, 1)
  assert.equal(preview.dataMissing[0].casilla, '08')
  await assert.rejects(() => engine.applyAdapter(noData, context), /Faltan 1 valor/)
})

test('selecciona el adaptador declarativo por organismo, modelo y ejercicio', () => {
  const aeat303 = engine.getAdapter('303', '2026', 'AEAT')
  assert.equal(aeat303?.entity, 'AEAT')
  assert.equal(aeat303?.verified, true)
  assert.equal(aeat303?.exercise, '2026')
  assert.equal(aeat303?.version, '2026.1')
  assert.equal(aeat303?.updatedAt, '2026-10-30')
  assert.equal(aeat303?.fields.length, 36)
  assert.equal(aeat303?.fields.find((field) => field.casilla === '10')?.required, false)
  assert.equal(
    engine.missingData(aeat303, context).some((field) => field.casilla === '10'),
    false,
  )
  assert.equal(engine.getAdapter('303', '2026', 'TGSS'), undefined)
  assert.equal(engine.getAdapter('303', '2025'), undefined)
  assert.equal(engine.adapterForPage('303', '2026', 'AEAT')?.code, '303')

  const aeat130 = engine.getAdapter('130', '2026', 'AEAT')
  assert.equal(aeat130?.verified, false)
  assert.equal(aeat130?.fields.length, 7)
  assert.equal(aeat130?.fields.every((field) => field.required !== false), true)
})
