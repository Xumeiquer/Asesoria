import type { Adapter, AdapterField } from './types'

function amount(casilla: string, label: string): AdapterField {
  return {
    casilla,
    label,
    selector: `input[aria-label$="${casilla}"]`,
    kind: 'amount',
  }
}

/**
 * Adaptador preparado para el formulario 130/2026. Todavía no se activa:
 * falta validar los selectores contra el HTML saneado del formulario
 * autenticado. Mientras `verified` sea false, el panel solo ofrece copiar
 * casillas y avisa de que el relleno está en preparación.
 */
export const aeat130: Adapter = {
  code: '130',
  exercise: '2026',
  version: '2026.1',
  verified: false,
  page: {
    contains: ['Rendimiento neto', 'Resultado'],
  },
  pageHint: 'Abre el formulario del modelo 130 y vuelve a intentarlo.',
  notes:
    'Pendiente de verificar con HTML saneado del formulario autenticado. Los selectores actuales son una hipótesis basada en etiquetas de casilla.',
  fields: [
    amount('01', 'Ingresos íntegros acumulados'),
    amount('02', 'Gastos deducibles acumulados'),
    amount('03', 'Rendimiento neto acumulado'),
    amount('04', '20% del rendimiento neto acumulado'),
    amount('05', 'Retenciones e ingresos a cuenta'),
    amount('06', 'Pagos fraccionados anteriores'),
    amount('07', 'Resultado a ingresar'),
  ],
}
