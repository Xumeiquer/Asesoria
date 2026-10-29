<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  AlertTriangle,
  Check,
  Copy,
  Database,
  Download,
  FolderTree,
  HardDrive,
  KeyRound,
  Link2,
  Loader2,
  Lock,
  RefreshCw,
  ScanLine,
  ShieldCheck,
  Trash2,
} from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { api, dateLabel, dateTimeLabel } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { ApiToken, LibraryConfig, PairingCode, ScanResult } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const codes = ref<PairingCode | null>(null)
const tokens = ref<ApiToken[]>([])
const loading = ref(false)
const copied = ref(false)
const dataDirCopied = ref(false)
const confirmToken = ref<ApiToken | null>(null)
const revoking = ref(false)

const library = ref<LibraryConfig | null>(null)
const libraryRoot = ref('')
const ocrLang = ref('spa')
const librarySaving = ref(false)
const treeBusy = ref(false)
const scanBusy = ref(false)
const treeMessage = ref('')
const scanResult = ref<ScanResult | null>(null)

function applyLibrary(cfg: LibraryConfig) {
  library.value = cfg
  libraryRoot.value = cfg.root
  ocrLang.value = cfg.ocr_lang || 'spa'
  if (cfg.last_scan) scanResult.value = cfg.last_scan
}

async function load() {
  try {
    await store.loadMeta()
    const [t] = await Promise.all([api.get<ApiToken[]>('/pairing/tokens')])
    tokens.value = t
    applyLibrary(await api.get<LibraryConfig>('/library'))
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo cargar la configuración')
  }
}

async function saveLibrary() {
  librarySaving.value = true
  treeMessage.value = ''
  try {
    const cfg = await api.put<LibraryConfig>('/library', {
      root: libraryRoot.value,
      ocr_lang: ocrLang.value,
    })
    applyLibrary(cfg)
    treeMessage.value = cfg.configured
      ? 'Biblioteca guardada y árbol de carpetas generado.'
      : 'Biblioteca desactivada. No se ha borrado ningún fichero.'
    toasts.success(cfg.configured ? 'Biblioteca configurada' : 'Biblioteca desactivada')
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo guardar la biblioteca')
  } finally {
    librarySaving.value = false
  }
}

async function generateTree() {
  if (!library.value?.configured) {
    toasts.error('Configura y guarda primero una carpeta de biblioteca')
    return
  }
  treeBusy.value = true
  treeMessage.value = ''
  try {
    const res = await api.post<{ folders: number; taxpayers: number }>('/library/tree')
    treeMessage.value = `Árbol actualizado: ${res.taxpayers} contribuyente(s), ${res.folders} carpeta(s) nueva(s).`
    toasts.success('Árbol de carpetas actualizado')
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo generar el árbol')
  } finally {
    treeBusy.value = false
  }
}

async function scanLibrary() {
  if (!library.value?.configured) {
    toasts.error('Configura y guarda primero una carpeta de biblioteca')
    return
  }
  scanBusy.value = true
  try {
    const res = await api.post<ScanResult>('/library/scan', { taxpayer_id: store.selectedId || '' })
    scanResult.value = res
    const extra = res.truncated
      ? ' Se alcanzó el límite de 1000 ficheros; vuelve a escanear para continuar.'
      : ''
    toasts.success(
      `Escaneo terminado: ${res.imported} importados, ${res.duplicates} duplicados, ${res.needs_review} por revisar.${extra}`,
    )
    applyLibrary(await api.get<LibraryConfig>('/library'))
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo escanear la biblioteca')
  } finally {
    scanBusy.value = false
  }
}

async function generate() {
  loading.value = true
  try {
    codes.value = await api.post<PairingCode>('/pairing/codes')
    copied.value = false
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo generar el código')
  } finally {
    loading.value = false
  }
}

async function copyCode() {
  if (!codes.value) return
  try {
    await navigator.clipboard.writeText(codes.value.code)
    copied.value = true
    toasts.success('Código copiado')
    window.setTimeout(() => (copied.value = false), 2500)
  } catch {
    toasts.error('No se pudo copiar el código')
  }
}

function requestRevoke(token: ApiToken) {
  confirmToken.value = token
}

async function confirmRevoke() {
  if (!confirmToken.value) return
  revoking.value = true
  try {
    await api.del(`/pairing/tokens/${confirmToken.value.id}`)
    toasts.success('Acceso revocado')
    confirmToken.value = null
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo revocar el acceso')
  } finally {
    revoking.value = false
  }
}

function onRevokeDialogUpdate(open: boolean) {
  if (!open && !revoking.value) confirmToken.value = null
}

async function copyDataDir() {
  const value = store.meta?.data_dir
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    dataDirCopied.value = true
    toasts.success('Ruta de datos copiada')
    window.setTimeout(() => (dataDirCopied.value = false), 2500)
  } catch {
    toasts.error('No se pudo copiar la ruta')
  }
}

function downloadBackup() {
  const link = document.createElement('a')
  link.href = '/api/backup'
  link.download = ''
  document.body.appendChild(link)
  link.click()
  link.remove()
  toasts.success('Preparando la copia de seguridad…')
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Ajustes y conexión"
      subtitle="La aplicación escucha solo en 127.0.0.1 y guarda todo en tu equipo. La extensión se conecta mediante un token revocable."
    />

    <div class="grid gap-6 lg:grid-cols-3">
      <section class="space-y-5 lg:col-span-2">
        <div class="card card-pad">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
                <FolderTree class="h-4 w-4" /> Biblioteca documental
              </h2>
              <p class="mt-1.5 text-sm text-slate-500">
                Una carpeta de tu equipo donde la aplicación organiza los documentos por contribuyente, año, trimestre y tipo.
                El texto se extrae en local; nada sale de este ordenador.
              </p>
            </div>
            <span
              class="chip"
              :class="library?.configured ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'"
            >
              {{ library?.configured ? 'Activa' : 'Sin configurar' }}
            </span>
          </div>

          <div class="mt-4 grid gap-3 sm:grid-cols-[1fr_11rem]">
            <div>
              <label class="label">Carpeta raíz</label>
              <input
                v-model="libraryRoot"
                class="input font-mono text-xs"
                placeholder="/home/tu-usuario/Documentos/Asesoría"
              />
              <p class="mt-1 text-[11px] text-slate-400">
                Déjalo vacío para desactivar la biblioteca. Los ficheros existentes no se borran.
              </p>
            </div>
            <div>
              <label class="label">Idioma OCR</label>
              <select v-model="ocrLang" class="input">
                <option value="spa">Español (spa)</option>
                <option value="eng">Inglés (eng)</option>
                <option value="spa+eng">Español + inglés</option>
              </select>
            </div>
          </div>

          <div class="mt-3 flex flex-wrap items-center gap-2 text-[11px]">
            <span class="chip" :class="library?.pdf_text ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'">
              <HardDrive class="mr-1 h-3 w-3" /> PDF con texto {{ library?.pdf_text ? 'disponible' : 'no disponible' }}
            </span>
            <span class="chip" :class="library?.tesseract ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'">
              <ScanLine class="mr-1 h-3 w-3" /> OCR de imágenes {{ library?.tesseract ? 'disponible' : 'no disponible' }}
            </span>
            <span class="chip" :class="library?.pdf_images ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'">
              <ScanLine class="mr-1 h-3 w-3" /> OCR de PDF {{ library?.pdf_images ? 'disponible' : 'no disponible' }}
            </span>
          </div>

          <div v-if="library?.configured && (!library.tesseract || !library.pdf_text)" class="mt-3 flex gap-2 rounded-xl border border-amber-200 bg-amber-50/70 p-3 text-xs text-amber-800">
            <AlertTriangle class="mt-0.5 h-4 w-4 shrink-0" />
            <span>
              Para leer PDF escaneados e imágenes instala Poppler y Tesseract en el sistema. Sin ellos, los documentos se archivan
              igualmente y podrás reprocesarlos después.
            </span>
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <button class="btn-primary" :disabled="librarySaving" @click="saveLibrary">
              <Loader2 v-if="librarySaving" class="h-4 w-4 animate-spin" />
              <FolderTree v-else class="h-4 w-4" />
              {{ librarySaving ? 'Guardando…' : 'Guardar y generar árbol' }}
            </button>
            <button class="btn-ghost" :disabled="treeBusy || !library?.configured" @click="generateTree">
              <Loader2 v-if="treeBusy" class="h-4 w-4 animate-spin" />
              <RefreshCw v-else class="h-4 w-4" />
              Regenerar árbol
            </button>
            <button class="btn-ghost" :disabled="scanBusy || !library?.configured" @click="scanLibrary">
              <Loader2 v-if="scanBusy" class="h-4 w-4 animate-spin" />
              <ScanLine v-else class="h-4 w-4" />
              {{ scanBusy ? 'Escaneando…' : 'Escanear ahora' }}
            </button>
          </div>

          <p v-if="treeMessage" class="mt-3 text-xs text-emerald-700">{{ treeMessage }}</p>

          <div v-if="library?.configured" class="mt-3 rounded-xl bg-slate-50 p-3 text-xs text-slate-500">
            <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span>Árbol: {{ library.years.join(', ') }}</span>
              <span v-if="scanResult">Último escaneo: {{ dateTimeLabel(scanResult.finished_at) }}</span>
              <span v-else>Sin escaneos todavía</span>
            </div>
            <div v-if="scanResult" class="mt-1 flex flex-wrap gap-x-4 gap-y-1">
              <span>{{ scanResult.imported }} importados</span>
              <span>{{ scanResult.duplicates }} duplicados</span>
              <span>{{ scanResult.needs_review }} por revisar</span>
              <span v-if="scanResult.errors" class="text-rose-600">{{ scanResult.errors }} errores</span>
            </div>
          </div>
        </div>

        <div class="card card-pad">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
                <Link2 class="h-4 w-4" /> Conectar la extensión
              </h2>
              <p class="mt-1.5 text-sm text-slate-500">
                Genera un código de un solo uso y pégalo en el popup de la extensión. Caduca en 10 minutos.
              </p>
            </div>
            <button class="btn-primary shrink-0" :disabled="loading" @click="generate">
              <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loading }" />
              Generar código
            </button>
          </div>

          <div v-if="codes" class="mt-5 rounded-2xl border border-emerald-200 bg-emerald-50/60 p-5">
            <p class="text-xs font-bold uppercase tracking-wider text-emerald-700">Código de emparejamiento</p>
            <div class="mt-2 flex flex-wrap items-center gap-3">
              <span class="font-mono text-4xl font-bold tracking-[0.25em] text-emerald-900">{{ codes.code }}</span>
              <button class="btn-ghost" @click="copyCode">
                <Check v-if="copied" class="h-4 w-4 text-emerald-600" />
                <Copy v-else class="h-4 w-4" />
                {{ copied ? 'Copiado' : 'Copiar' }}
              </button>
            </div>
            <p class="mt-2 text-xs text-emerald-700">Caduca el {{ dateTimeLabel(codes.expires_at) }}.</p>
          </div>

          <ol class="mt-5 grid gap-3 text-sm text-slate-600">
            <li class="flex gap-3">
              <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">1</span>
              <span>
                Abre <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">chrome://extensions</code>, activa el modo
                desarrollador y elige «Cargar descomprimida». Selecciona la carpeta
                <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">extension/dist</code> de este proyecto.
              </span>
            </li>
            <li class="flex gap-3">
              <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">2</span>
              <span>Abre el popup, pega el código de arriba y pulsa Conectar. El token aparece abajo y puedes revocarlo cuando quieras.</span>
            </li>
          </ol>
          <details class="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-xs text-slate-600">
            <summary class="cursor-pointer font-semibold text-slate-700">¿No existe la carpeta extension/dist?</summary>
            <p class="mt-2 leading-relaxed">
              Compílala una vez desde la carpeta del proyecto:
              <code class="rounded bg-white px-1.5 py-0.5 font-mono text-[11px]">cd extension &amp;&amp; npm install &amp;&amp; npm run build</code>.
              Después vuelve a cargar <code class="rounded bg-white px-1.5 py-0.5 font-mono text-[11px]">extension/dist</code>.
            </p>
          </details>
          <p class="mt-3 text-xs text-slate-500">
            La extensión es opcional: solo sirve para importar datos desde el navegador. La aplicación funciona sin ella.
          </p>
        </div>

        <div class="card">
          <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
            <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
              <KeyRound class="h-4 w-4" /> Accesos de extensiones
            </h2>
            <span class="chip bg-slate-100 text-slate-600">{{ tokens.length }} {{ tokens.length === 1 ? 'activo' : 'activos' }}</span>
          </div>
          <div v-if="tokens.length">
            <p class="px-5 pt-3 text-[11px] text-slate-500 sm:hidden">Desliza lateralmente para ver todas las columnas.</p>
            <div class="table-wrap">
              <table class="table">
                <thead>
                  <tr>
                    <th>Nombre</th>
                    <th>Creado</th>
                    <th>Último uso</th>
                    <th><span class="sr-only">Acciones</span></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="token in tokens" :key="token.id">
                    <td class="font-medium text-slate-800">{{ token.name }}</td>
                    <td class="text-xs text-slate-500">{{ dateTimeLabel(token.created_at) }}</td>
                    <td class="text-xs text-slate-500">{{ token.last_used_at ? dateTimeLabel(token.last_used_at) : 'Nunca' }}</td>
                    <td class="text-right">
                      <button
                        class="rounded-lg p-1.5 text-slate-500 hover:bg-rose-50 hover:text-rose-600"
                        :aria-label="`Revocar acceso de ${token.name}`"
                        title="Revocar acceso"
                        @click="requestRevoke(token)"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <p v-else class="px-5 py-8 text-center text-sm text-slate-500">
            Todavía no hay ninguna extensión conectada.
          </p>
        </div>

        <div class="card card-pad">
          <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
            <ShieldCheck class="h-4 w-4" /> Cómo protegemos tus datos
          </h2>
          <ul class="mt-3 space-y-2 text-sm text-slate-600">
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> La API solo acepta peticiones de localhost y de orígenes de extensión autorizados.</li>
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> Los tokens se guardan hasheados; el token original solo se muestra una vez a la extensión.</li>
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> La aplicación nunca accede al certificado ni a la clave privada: la firma ocurre siempre en el navegador.</li>
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> Puedes revocar el acceso de la extensión en cualquier momento.</li>
          </ul>
        </div>
      </section>

      <aside class="space-y-4">
        <div class="card card-pad">
          <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
            <Database class="h-4 w-4" /> Copia de seguridad
          </h2>
          <p class="mt-2 text-sm leading-relaxed text-slate-600">
            Descarga un ZIP con una copia consistente de la base SQLite y los datos locales. Puedes guardarlo en un
            disco externo o en tu carpeta de copias.
          </p>
          <button class="btn-primary mt-4 w-full" @click="downloadBackup">
            <Download class="h-4 w-4" /> Descargar copia de seguridad
          </button>
          <p class="mt-3 text-xs leading-relaxed text-slate-500">
            Los documentos de la biblioteca viven en su propia carpeta. Si la usas, inclúyela también en tus copias.
          </p>
        </div>

        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Entorno local</h2>
          <dl class="mt-4 space-y-3 text-sm">
            <div>
              <dt class="text-xs text-slate-400">Versión</dt>
              <dd class="font-medium text-slate-800">{{ store.meta?.version ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Catálogo</dt>
              <dd class="font-medium text-slate-800">{{ store.meta?.catalog_version ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Catálogo actualizado</dt>
              <dd class="font-medium text-slate-800">{{ dateLabel(store.meta?.catalog_updated_at ?? '') }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Calculadoras</dt>
              <dd class="font-medium text-slate-800">{{ store.meta?.calculators.join(', ') ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Directorio de datos</dt>
              <dd class="mt-1 flex items-start gap-2 rounded-lg bg-slate-50 p-2">
                <code class="min-w-0 flex-1 break-all font-mono text-xs text-slate-600">{{ store.meta?.data_dir ?? '—' }}</code>
                <button
                  class="shrink-0 rounded p-1 text-slate-400 hover:bg-white hover:text-slate-700"
                  :aria-label="dataDirCopied ? 'Ruta copiada' : 'Copiar ruta de datos'"
                  :title="dataDirCopied ? 'Copiado' : 'Copiar ruta'"
                  @click="copyDataDir"
                >
                  <Check v-if="dataDirCopied" class="h-3.5 w-3.5 text-emerald-600" />
                  <Copy v-else class="h-3.5 w-3.5" />
                </button>
              </dd>
            </div>
          </dl>
          <p class="mt-4 text-xs leading-relaxed text-slate-500">
            Copia esta carpeta para hacer una copia de seguridad. Contiene la base SQLite y los documentos subidos. Si usas la
            biblioteca documental, copia también su carpeta raíz.
          </p>
        </div>

        <div class="rounded-2xl bg-slate-900 p-5 text-white">
          <p class="text-xs font-bold uppercase tracking-wider text-slate-400">Recordatorio legal</p>
          <p class="mt-2 text-sm leading-relaxed text-slate-300">
            Esta herramienta te ayuda a organizar y calcular, pero no sustituye el asesoramiento profesional.
            Verifica siempre las casillas y los plazos antes de firmar.
          </p>
        </div>
      </aside>
    </div>

    <ConfirmDialog
      :model-value="!!confirmToken"
      title="Revocar acceso de la extensión"
      :message="`La extensión «${confirmToken?.name ?? ''}» dejará de poder leer datos de esta aplicación. Puedes volver a conectarla generando un código nuevo.`"
      confirm-label="Revocar acceso"
      tone="danger"
      :busy="revoking"
      @update:model-value="onRevokeDialogUpdate"
      @confirm="confirmRevoke"
    />
  </div>
</template>
