import type { Adapter, AdapterField } from './types'

function amount(casilla: string, label: string, optional = false): AdapterField {
  return {
    casilla,
    label,
    selector: `input[aria-label$="${casilla}"]`,
    kind: 'amount',
    optional,
  }
}

/**
 * Casillas editables de la liquidación del régimen general del 303/2026.
 * No se incluyen las casillas calculadas por la AEAT (tipos, totales y
 * resultado): el formulario las actualiza al escribir las bases y cuotas.
 */
export const aeat303: Adapter = {
  code: '303',
  exercise: '2026',
  version: '2026.1',
  verified: true,
  verifiedAgainst:
    'https://www2.agenciatributaria.gob.es/wlpl/A303-FWME/E2026/OPEN/index.zul',
  page: {
    contains: ['IVA devengado', 'Total cuota devengada'],
  },
  pageHint:
    'Abre la página «Liquidación: régimen general» (página 2 del formulario) y vuelve a intentarlo.',
  notes:
    'Selectores probados contra el simulador oficial 303/2026. Revisa siempre los importes antes de firmar.',
  fields: [
    amount('150', 'Base imponible al 0%', true),
    amount('01', 'Base imponible al 4%'),
    amount('03', 'Cuota devengada al 4%'),
    amount('04', 'Base imponible al 10%'),
    amount('06', 'Cuota devengada al 10%'),
    amount('07', 'Base imponible al 21%'),
    amount('09', 'Cuota devengada al 21%'),
    amount('10', 'Adquisiciones intracomunitarias de bienes y servicios', true),
    amount('11', 'Cuota devengada por adquisiciones intracomunitarias', true),
    amount('12', 'Otras operaciones con inversión del sujeto pasivo', true),
    amount('13', 'Cuota devengada por otras operaciones con inversión del sujeto pasivo', true),
    amount('14', 'Modificación de bases', true),
    amount('15', 'Modificación de cuotas', true),
    amount('19', 'Base del recargo de equivalencia al 1,4%', true),
    amount('21', 'Cuota del recargo de equivalencia al 1,4%', true),
    amount('22', 'Base del recargo de equivalencia al 5,2%', true),
    amount('24', 'Cuota del recargo de equivalencia al 5,2%', true),
    amount('25', 'Modificación de bases del recargo', true),
    amount('26', 'Modificación de cuotas del recargo', true),
    amount('28', 'Base deducible en operaciones interiores corrientes'),
    amount('29', 'Cuota deducible en operaciones interiores corrientes'),
    amount('30', 'Base deducible en bienes de inversión'),
    amount('31', 'Cuota deducible en bienes de inversión'),
    amount('32', 'Base deducible en importaciones de bienes corrientes'),
    amount('33', 'Cuota deducible en importaciones de bienes corrientes'),
    amount('34', 'Base deducible en importaciones de bienes de inversión'),
    amount('35', 'Cuota deducible en importaciones de bienes de inversión'),
    amount('36', 'Base deducible en adquisiciones intracomunitarias corrientes'),
    amount('37', 'Cuota deducible en adquisiciones intracomunitarias corrientes'),
    amount('38', 'Base deducible en adquisiciones intracomunitarias de bienes de inversión'),
    amount('39', 'Cuota deducible en adquisiciones intracomunitarias de bienes de inversión'),
    amount('40', 'Rectificación de deducciones (base)'),
    amount('41', 'Rectificación de deducciones (cuota)'),
    amount('42', 'Compensaciones del régimen especial agrario'),
    amount('43', 'Regularización de bienes de inversión'),
    amount('44', 'Regularización por porcentaje definitivo de prorrata'),
  ],
}
