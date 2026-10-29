const dateFormatter = new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: 'short' })
const longDateFormatter = new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: 'long', year: 'numeric' })

export function formatDate(value: string): string {
  if (!value) return '—'
  const date = new Date(`${value}T00:00:00`)
  if (Number.isNaN(date.getTime())) return value
  return dateFormatter.format(date).replace('.', '')
}

export function formatLongDate(value: string): string {
  if (!value) return '—'
  const date = new Date(`${value}T00:00:00`)
  if (Number.isNaN(date.getTime())) return value
  return longDateFormatter.format(date)
}

/** Fecha local en formato ISO (YYYY-MM-DD), sin pasar por UTC. */
export function isoDate(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

export function todayISO(): string {
  return isoDate(new Date())
}

/** Días completos hasta una fecha; negativo si ya ha vencido. */
export function daysUntil(value: string): number {
  const target = new Date(`${value.slice(0, 10)}T00:00:00`)
  if (Number.isNaN(target.getTime())) return Number.NaN
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return Math.round((target.getTime() - today.getTime()) / 86_400_000)
}

export type DueTone = 'normal' | 'soon' | 'late' | 'done'

export function dueTone(value: string, status: string): DueTone {
  if (status === 'presented' || status === 'not_applicable') return 'done'
  const days = daysUntil(value)
  if (!Number.isFinite(days)) return 'normal'
  if (days < 0) return 'late'
  if (days <= 7) return 'soon'
  return 'normal'
}

/** Texto relativo equivalente al bloque «Próximos vencimientos» de la webapp. */
export function dueLabel(value: string, status: string): string {
  if (!value) return '—'
  if (status === 'presented' || status === 'not_applicable') return formatLongDate(value)
  const days = daysUntil(value)
  if (!Number.isFinite(days)) return formatLongDate(value)
  if (days < 0) {
    const count = Math.abs(days)
    return `Vencido hace ${count} ${count === 1 ? 'día' : 'días'}`
  }
  if (days === 0) return 'Vence hoy'
  return `Vence en ${days} ${days === 1 ? 'día' : 'días'}`
}

export const statusLabels: Record<string, string> = {
  pending: 'Pendiente',
  in_progress: 'En curso',
  ready: 'Listo',
  presented: 'Presentado',
  late: 'Fuera de plazo',
  not_applicable: 'No aplica',
}

export function statusLabel(status: string): string {
  return statusLabels[status] ?? status
}

export type StatusTone = 'ready' | 'pending' | 'progress' | 'presented' | 'late' | 'muted'

export function statusTone(status: string): StatusTone {
  switch (status) {
    case 'ready':
      return 'ready'
    case 'in_progress':
      return 'progress'
    case 'presented':
      return 'presented'
    case 'late':
      return 'late'
    case 'not_applicable':
      return 'muted'
    default:
      return 'pending'
  }
}

export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export function defaultExtensionName(): string {
  const ua = navigator.userAgent
  const browser = ua.includes('Firefox') ? 'Firefox' : ua.includes('Edg/') ? 'Edge' : 'Chrome'
  const platform = /Mac/i.test(navigator.platform)
    ? 'macOS'
    : /Win/i.test(navigator.platform)
      ? 'Windows'
      : /Linux/i.test(navigator.platform)
        ? 'Linux'
        : 'este equipo'
  return `${browser} en ${platform}`
}
