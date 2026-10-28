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
