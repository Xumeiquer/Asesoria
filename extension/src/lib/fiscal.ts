import type { Filing } from './types'

interface FiscalYearEnd {
  month: number
  day: number
}

/** Descompone el cierre del ejercicio (MM-DD). Por defecto, 31 de diciembre. */
function fiscalYearEnd(value: string | null | undefined): FiscalYearEnd {
  const match = String(value ?? '').match(/^(\d{1,2})-(\d{1,2})$/)
  const month = match ? Number(match[1]) : 12
  const day = match ? Number(match[2]) : 31
  return {
    month: month >= 1 && month <= 12 ? month : 12,
    day: day >= 1 && day <= 31 ? day : 31,
  }
}

function parseISO(value: string): Date {
  return new Date(`${value.slice(0, 10)}T00:00:00`)
}

function fiscalCloseDate(year: number, yearEnd?: string | null): Date {
  const { month, day } = fiscalYearEnd(yearEnd)
  return new Date(year, month - 1, day)
}

/** Ejercicio al que pertenece una fecha, identificado por su año de cierre. */
function exerciseOf(value: string, yearEnd?: string | null): number {
  const date = parseISO(value)
  let year = date.getFullYear()
  while (fiscalCloseDate(year, yearEnd) < date) year++
  return year
}

/**
 * Etiqueta para los modelos que se presentan después del cambio de ejercicio
 * pero cuyo periodo pertenece a un ejercicio anterior.
 */
export function priorExerciseLabel(filing: Filing, yearEnd?: string | null): string {
  const exercise = exerciseOf(filing.period_end || filing.due_date, yearEnd)
  const current = exerciseOf(filing.due_date, yearEnd)
  return exercise < current ? `Ejercicio ${exercise}` : ''
}
