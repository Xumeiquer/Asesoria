const FILING_PATH = /^\/expedientes\/([^/?#]+)\/?$/
const SEDE_HOSTS = [
  /(^|\.)agenciatributaria\.gob\.es$/,
  /(^|\.)seg-social\.gob\.es$/,
  /(^|\.)registradores\.org$/,
]

let lastID = ''

function isSedeURL(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && SEDE_HOSTS.some((pattern) => pattern.test(url.hostname))
  } catch {
    return false
  }
}

function currentFilingID(): string {
  const match = window.location.pathname.match(FILING_PATH)
  if (!match) return ''
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}

/**
 * Recuerda el expediente que el usuario está viendo en el ERP. Así el panel
 * puede aparecer al abrir la sede desde la ficha, no solo desde el popup.
 */
function sync(force = false): void {
  const id = currentFilingID()
  if (!id) {
    lastID = ''
    return
  }
  if (id === lastID && !force) return
  lastID = id
  chrome.runtime.sendMessage({ type: 'SET_ACTIVE', id }, () => {
    // Evita el aviso de "message port closed" al recargar la página.
    void chrome.runtime.lastError
  })
}

void sync()
window.addEventListener('popstate', () => sync())
window.addEventListener('focus', () => sync(true))
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) sync(true)
})
// Al abrir la sede desde una ficha, fijamos el expediente de esta pestaña.
document.addEventListener(
  'click',
  (event) => {
    const link = (event.target as HTMLElement | null)?.closest<HTMLAnchorElement>('a[href]')
    if (link && isSedeURL(link.href)) sync(true)
  },
  true,
)
// La navegación del ERP es de cliente: la URL cambia sin recargar la página.
window.setInterval(() => sync(), 1_000)
