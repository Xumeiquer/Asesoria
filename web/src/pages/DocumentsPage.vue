<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  AlertTriangle,
  Download,
  FileText,
  FolderOpen,
  FolderTree,
  Loader2,
  Pencil,
  ScanLine,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import Modal from '@/components/Modal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { api, bytes, dateLabel, euros } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { DocumentItem, LibraryConfig, ScanResult } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const documents = ref<DocumentItem[]>([])
const loading = ref(true)
const uploading = ref(false)
const search = ref('')
const kindFilter = ref('')
const reviewFilter = ref(false)
const library = ref<LibraryConfig | null>(null)
const scanning = ref(false)
const dragOver = ref(false)

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFiles = ref<File[]>([])
const kind = ref('factura_recibida')
const docDate = ref(localISODate())
const amount = ref('')
const counterparty = ref('')

const reviewOpen = ref(false)
const reviewDoc = ref<DocumentItem | null>(null)
const reviewSaving = ref(false)
const reprocessing = ref(false)
const reviewForm = ref({
  taxpayer_id: '',
  kind: '',
  doc_date: '',
  amount: '',
  counterparty_nif: '',
})

const confirmDoc = ref<DocumentItem | null>(null)
const deleting = ref(false)

const kinds = [
  { value: 'factura_emitida', label: 'Factura emitida' },
  { value: 'factura_recibida', label: 'Factura recibida' },
  { value: 'factura', label: 'Factura (sin determinar)' },
  { value: 'nomina', label: 'Nómina' },
  { value: 'recibo', label: 'Recibo' },
  { value: 'justificante', label: 'Justificante' },
  { value: 'modelo', label: 'Modelo presentado' },
  { value: 'contrato', label: 'Contrato' },
  { value: 'seguro', label: 'Seguro' },
  { value: 'banco', label: 'Banco' },
  { value: 'other', label: 'Otro' },
]

function localISODate(date = new Date()): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  return documents.value.filter((doc) => {
    if (kindFilter.value && doc.kind !== kindFilter.value) return false
    if (reviewFilter.value && doc.review_status !== 'review') return false
    if (!term) return true
    return `${doc.filename} ${doc.counterparty_nif} ${doc.kind} ${doc.text_excerpt ?? ''}`
      .toLowerCase()
      .includes(term)
  })
})

const reviewCount = computed(() => documents.value.filter((doc) => doc.review_status === 'review').length)
const totalSize = computed(() => documents.value.reduce((sum, doc) => sum + doc.size, 0))
const hasFilters = computed(() => Boolean(search.value.trim() || kindFilter.value || reviewFilter.value))
const reviewReasons = computed(() => {
  const raw = reviewDoc.value?.metadata?.review_reasons ?? reviewDoc.value?.metadata?.reasons
  return Array.isArray(raw) ? raw.filter((item): item is string => typeof item === 'string') : []
})

function reasonLabel(reason: string): string {
  const labels: Record<string, string> = {
    fecha: 'Fecha detectada automáticamente',
    importe: 'Importe detectado automáticamente',
    nif: 'NIF de la contraparte detectado automáticamente',
    tipo: 'Tipo de documento inferido automáticamente',
    date: 'Fecha detectada automáticamente',
    amount: 'Importe detectado automáticamente',
    counterparty_nif: 'NIF de la contraparte detectado automáticamente',
    kind: 'Tipo de documento inferido automáticamente',
  }
  return labels[reason] ?? reason
}

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (reviewFilter.value) {
      // La cola de revisión es global para poder rescatar documentos sin
      // contribuyente asignado.
      params.set('review', 'review')
    } else if (store.selectedId) {
      params.set('taxpayer_id', store.selectedId)
    }
    params.set('limit', '500')
    documents.value = await api.get<DocumentItem[]>(`/documents?${params}`)
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudieron cargar los documentos')
  } finally {
    loading.value = false
  }
}

async function loadLibrary() {
  try {
    library.value = await api.get<LibraryConfig>('/library')
  } catch {
    library.value = null
  }
}

async function scanLibrary() {
  if (!library.value?.configured) {
    toasts.error('Configura la biblioteca documental en Ajustes')
    return
  }
  scanning.value = true
  try {
    const res = await api.post<ScanResult>('/library/scan', { taxpayer_id: store.selectedId || '' })
    const extra = res.truncated ? ' Se alcanzó el límite de 1000 ficheros; vuelve a escanear.' : ''
    toasts.success(
      `Escaneo terminado: ${res.imported} importados, ${res.duplicates} duplicados, ${res.needs_review} por revisar.${extra}`,
    )
    await Promise.all([load(), loadLibrary()])
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo escanear la biblioteca')
  } finally {
    scanning.value = false
  }
}

function openReview(doc: DocumentItem) {
  reviewDoc.value = doc
  reviewForm.value = {
    taxpayer_id: doc.taxpayer_id ?? '',
    kind: doc.kind,
    doc_date: doc.doc_date,
    amount: doc.amount_cents !== null ? (doc.amount_cents / 100).toFixed(2).replace('.', ',') : '',
    counterparty_nif: doc.counterparty_nif,
  }
  reviewOpen.value = true
}

async function saveReview() {
  if (!reviewDoc.value) return
  reviewSaving.value = true
  try {
    await api.patch<DocumentItem>(`/documents/${reviewDoc.value.id}`, {
      ...reviewForm.value,
      review_status: 'ok',
    })
    toasts.success('Documento revisado')
    reviewOpen.value = false
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo guardar la revisión')
  } finally {
    reviewSaving.value = false
  }
}

async function reprocess() {
  if (!reviewDoc.value) return
  reprocessing.value = true
  try {
    await api.post<DocumentItem>(`/documents/${reviewDoc.value.id}/reprocess`)
    toasts.success('Texto reprocesado')
    reviewOpen.value = false
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo reprocesar el documento')
  } finally {
    reprocessing.value = false
  }
}

function ocrLabel(value: string): string {
  const labels: Record<string, string> = {
    text: 'Texto digital',
    ocr: 'Texto por OCR',
    none: 'Sin texto',
    unavailable: 'OCR no disponible',
    unsupported: 'Formato no soportado',
    error: 'Error de extracción',
  }
  return labels[value] ?? value
}

function pickFile() {
  fileInput.value?.click()
}

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  selectedFiles.value = Array.from(target.files ?? [])
}

function onDrop(event: DragEvent) {
  dragOver.value = false
  selectedFiles.value = Array.from(event.dataTransfer?.files ?? [])
}

function removeSelectedFile(index: number) {
  selectedFiles.value = selectedFiles.value.filter((_, i) => i !== index)
  if (!selectedFiles.value.length && fileInput.value) fileInput.value.value = ''
}

async function upload() {
  if (!store.selectedId) {
    toasts.error('Selecciona un contribuyente antes de subir documentos')
    return
  }
  if (!selectedFiles.value.length) {
    toasts.error('Selecciona al menos un fichero')
    return
  }
  uploading.value = true
  let uploaded = 0
  let reviewTarget: DocumentItem | null = null
  let failed = 0
  const failedFiles: File[] = []
  try {
    for (const file of selectedFiles.value) {
      const form = new FormData()
      form.set('file', file)
      form.set('taxpayer_id', store.selectedId)
      form.set('kind', kind.value)
      if (docDate.value) form.set('doc_date', docDate.value)
      if (amount.value) form.set('amount', amount.value)
      if (counterparty.value) form.set('counterparty_nif', counterparty.value)
      try {
        const doc = await api.upload<DocumentItem>('/documents', form)
        uploaded++
        if (!reviewTarget && doc.review_status === 'review') reviewTarget = doc
      } catch (error) {
        failed++
        failedFiles.push(file)
        if (failed === 1) {
          toasts.error(error instanceof Error ? error.message : `No se pudo subir ${file.name}`)
        }
      }
    }
    if (uploaded) {
      toasts.success(`${uploaded} documento${uploaded === 1 ? '' : 's'} subido${uploaded === 1 ? '' : 's'}`)
    }
    // Conserva solo los que fallaron para que el usuario pueda reintentarlos.
    selectedFiles.value = failedFiles
    amount.value = ''
    counterparty.value = ''
    if (fileInput.value) fileInput.value.value = ''
    await load()
    if (reviewTarget) openReview(reviewTarget)
  } finally {
    uploading.value = false
  }
}

function remove(doc: DocumentItem) {
  confirmDoc.value = doc
}

async function confirmDelete() {
  if (!confirmDoc.value) return
  deleting.value = true
  try {
    await api.del(`/documents/${confirmDoc.value.id}`)
    toasts.success('Documento eliminado')
    confirmDoc.value = null
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo eliminar')
  } finally {
    deleting.value = false
  }
}

function onDeleteDialogUpdate(open: boolean) {
  if (!open && !deleting.value) confirmDoc.value = null
}

function clearFilters() {
  search.value = ''
  kindFilter.value = ''
  reviewFilter.value = false
}

function kindLabel(value: string): string {
  return kinds.find((item) => item.value === value)?.label ?? value
}

function isImage(doc: DocumentItem): boolean {
  return doc.mime.startsWith('image/')
}

function isPDF(doc: DocumentItem): boolean {
  return doc.mime === 'application/pdf'
}

function previewURL(doc: DocumentItem): string {
  return `/api/documents/${doc.id}/download?inline=1`
}

function downloadURL(doc: DocumentItem): string {
  return `/api/documents/${doc.id}/download`
}

watch(() => store.selectedId, load)
watch(reviewFilter, load)
onMounted(() => {
  load()
  loadLibrary()
})
</script>

<template>
  <div>
    <PageHeader
      title="Documentos"
      subtitle="Facturas, nóminas, recibos y justificantes del contribuyente activo. La biblioteca organiza los originales en tu equipo y extrae el texto en local."
    >
      <template #actions>
        <button v-if="library?.configured" class="btn-ghost" :disabled="scanning" @click="scanLibrary">
          <Loader2 v-if="scanning" class="h-4 w-4 animate-spin" />
          <ScanLine v-else class="h-4 w-4" />
          {{ scanning ? 'Escaneando…' : 'Escanear biblioteca' }}
        </button>
        <span class="chip bg-slate-100 text-slate-600">
          {{ documents.length }} fichero{{ documents.length === 1 ? '' : 's' }} · {{ bytes(totalSize) }}
        </span>
      </template>
    </PageHeader>

    <div
      v-if="library?.configured"
      class="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl border border-slate-200 bg-white px-4 py-3 text-xs text-slate-500"
    >
      <span class="flex min-w-0 items-center gap-2">
        <FolderTree class="h-4 w-4 shrink-0 text-slate-400" />
        <code class="truncate font-mono text-[11px] text-slate-600">{{ library.root }}</code>
      </span>
      <span v-if="reviewCount" class="chip bg-amber-50 text-amber-700">
        <AlertTriangle class="mr-1 h-3 w-3" /> {{ reviewCount }} por revisar
      </span>
      <span v-if="!library.tesseract" class="chip bg-slate-100 text-slate-500">OCR no disponible</span>
      <span class="ml-auto text-[11px] text-slate-400">
        Las copias se guardan por contribuyente / año / trimestre / tipo.
      </span>
    </div>
    <div v-else class="mb-4 rounded-2xl border border-dashed border-slate-200 px-4 py-3 text-xs text-slate-500">
      <FolderTree class="mr-1 inline h-4 w-4 text-slate-400" />
      La biblioteca documental está desactivada.
      <RouterLink to="/ajustes" class="font-semibold text-slate-700 underline decoration-slate-300 underline-offset-2">
        Configúrala en Ajustes
      </RouterLink>
      para organizar los ficheros automáticamente.
    </div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
      <section class="card card-pad h-fit min-w-0">
        <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Subir documentos</h2>
        <p class="mt-1 text-xs text-slate-500">
          Para {{ store.selectedTaxpayer?.name ?? 'el contribuyente activo' }}. Puedes arrastrar varios ficheros.
        </p>

        <button
          type="button"
          class="mt-4 flex w-full flex-col items-center justify-center rounded-2xl border-2 border-dashed px-4 py-7 text-center transition"
          :class="dragOver ? 'border-slate-900 bg-slate-100' : 'border-slate-200 bg-slate-50/60 hover:border-slate-300 hover:bg-slate-50'"
          @click="pickFile"
          @dragover.prevent="dragOver = true"
          @dragleave="dragOver = false"
          @drop.prevent="onDrop"
        >
          <Upload class="h-6 w-6 text-slate-400" />
          <span class="mt-2 text-sm font-semibold text-slate-700">
            {{
              selectedFiles.length
                ? `${selectedFiles.length} fichero${selectedFiles.length === 1 ? '' : 's'} seleccionado${selectedFiles.length === 1 ? '' : 's'}`
                : 'Seleccionar o arrastrar ficheros'
            }}
          </span>
          <span class="mt-0.5 text-xs text-slate-500">
            PDF, imágenes o cualquier documento · hasta 64 MB por fichero
          </span>
        </button>
        <input ref="fileInput" type="file" class="hidden" multiple @change="onFileChange" />

        <div v-if="selectedFiles.length" class="mt-2 space-y-1">
          <div
            v-for="(file, index) in selectedFiles"
            :key="`${file.name}-${index}`"
            class="flex items-center gap-2 rounded-lg bg-slate-50 px-2.5 py-1.5 text-xs text-slate-600"
          >
            <FileText class="h-3.5 w-3.5 shrink-0 text-slate-400" />
            <span class="min-w-0 flex-1 truncate">{{ file.name }}</span>
            <span class="shrink-0 text-slate-400">{{ bytes(file.size) }}</span>
            <button
              type="button"
              class="rounded p-0.5 text-slate-400 hover:bg-white hover:text-slate-700"
              :aria-label="`Quitar ${file.name}`"
              @click="removeSelectedFile(index)"
            >
              <X class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        <p class="mt-3 text-[11px] text-slate-500">
          Los datos de tipo, fecha, importe y NIF se aplican a todos los ficheros seleccionados.
        </p>

        <div class="mt-4 space-y-3">
          <div>
            <label class="label" for="upload-kind">Tipo</label>
            <select id="upload-kind" v-model="kind" class="input">
              <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="label" for="upload-date">Fecha</label>
              <input id="upload-date" v-model="docDate" type="date" class="input" />
            </div>
            <div>
              <label class="label" for="upload-amount">Importe</label>
              <input id="upload-amount" v-model="amount" class="input" placeholder="0,00" inputmode="decimal" />
            </div>
          </div>
          <div>
            <label class="label" for="upload-nif">NIF contraparte</label>
            <input id="upload-nif" v-model="counterparty" class="input" placeholder="Opcional" />
          </div>
        </div>

        <button class="btn-accent mt-4 w-full" :disabled="uploading || !selectedFiles.length" @click="upload">
          <Loader2 v-if="uploading" class="h-4 w-4 animate-spin" />
          <Upload v-else class="h-4 w-4" />
          {{ uploading ? 'Subiendo…' : `Subir ${selectedFiles.length || ''} y archivar` }}
        </button>
        <p class="mt-3 text-[11px] leading-relaxed text-slate-500">
          El cálculo de modelos usa las facturas registradas, no los PDF. Sube el fichero y registra la factura
          en la sección Facturas.
        </p>
      </section>

      <section class="min-w-0 lg:col-span-2">
        <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center">
          <div class="relative flex-1">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              v-model="search"
              class="input pl-9"
              type="search"
              placeholder="Buscar por nombre, NIF o texto…"
              aria-label="Buscar documentos"
            />
          </div>
          <select v-model="kindFilter" class="input sm:w-52" aria-label="Filtrar por tipo">
            <option value="">Todos los tipos</option>
            <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
          <label class="flex shrink-0 items-center gap-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-medium text-slate-600">
            <input v-model="reviewFilter" type="checkbox" class="h-4 w-4 rounded border-slate-300" />
            Solo por revisar
            <span v-if="reviewCount" class="chip bg-amber-50 text-amber-700">{{ reviewCount }}</span>
          </label>
        </div>

        <div v-if="loading" class="space-y-2">
          <div v-for="i in 5" :key="i" class="h-16 animate-pulse rounded-2xl bg-white" />
        </div>

        <EmptyState
          v-else-if="!documents.length"
          title="Sin documentos todavía"
          description="Sube facturas, nóminas o justificantes para tener toda la documentación del expediente en un único sitio."
        >
          <template #icon><FolderOpen class="h-6 w-6" /></template>
          <template #action>
            <button class="btn-ghost" @click="pickFile"><Upload class="h-4 w-4" /> Subir el primero</button>
          </template>
        </EmptyState>

        <EmptyState
          v-else-if="!filtered.length"
          title="Sin resultados con estos filtros"
          description="Prueba otra búsqueda o quita el filtro de revisión para ver todos los documentos."
        >
          <template #icon><Search class="h-6 w-6" /></template>
          <template #action>
            <button v-if="hasFilters" class="btn-ghost" @click="clearFilters">Limpiar filtros</button>
          </template>
        </EmptyState>

        <div v-else class="card overflow-hidden">
          <p class="px-4 pt-3 text-[11px] text-slate-500 sm:hidden">Desliza lateralmente para ver todas las columnas.</p>
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Fichero</th>
                  <th>Tipo</th>
                  <th>Fecha</th>
                  <th class="text-right">Importe</th>
                  <th class="text-right">Tamaño</th>
                  <th><span class="sr-only">Acciones</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="doc in filtered" :key="doc.id">
                  <td>
                    <button
                      type="button"
                      class="flex min-w-0 items-center gap-2 text-left"
                      title="Ver y revisar"
                      @click="openReview(doc)"
                    >
                      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-slate-100 text-slate-500">
                        <FileText class="h-4 w-4" />
                      </span>
                      <div class="min-w-0">
                        <p class="max-w-[18rem] truncate text-sm font-semibold text-slate-800">{{ doc.filename }}</p>
                        <p class="max-w-[18rem] truncate text-[11px] text-slate-400">
                          {{ doc.text_excerpt || doc.counterparty_nif || doc.mime }}
                        </p>
                      </div>
                    </button>
                  </td>
                  <td>
                    <div class="flex flex-wrap items-center gap-1">
                      <span class="chip bg-slate-100 text-slate-600">{{ kindLabel(doc.kind) }}</span>
                      <span v-if="doc.review_status === 'review'" class="chip bg-amber-50 text-amber-700">
                        <AlertTriangle class="mr-1 h-3 w-3" /> Revisar
                      </span>
                    </div>
                  </td>
                  <td class="text-xs text-slate-500">
                    {{ doc.doc_date ? dateLabel(doc.doc_date) : dateLabel(doc.created_at) }}
                  </td>
                  <td class="text-right font-mono text-xs tabular-nums text-slate-600">
                    {{ doc.amount_cents !== null ? euros(doc.amount_cents) : '—' }}
                  </td>
                  <td class="text-right text-xs text-slate-500">{{ bytes(doc.size) }}</td>
                  <td>
                    <div class="flex justify-end gap-1">
                      <button
                        class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 hover:text-slate-700"
                        aria-label="Revisar documento"
                        title="Revisar"
                        @click="openReview(doc)"
                      >
                        <Pencil class="h-4 w-4" />
                      </button>
                      <a
                        :href="downloadURL(doc)"
                        class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 hover:text-slate-700"
                        aria-label="Descargar documento"
                        title="Descargar"
                      >
                        <Download class="h-4 w-4" />
                      </a>
                      <button
                        class="rounded-lg p-1.5 text-slate-500 hover:bg-rose-50 hover:text-rose-600"
                        aria-label="Eliminar documento"
                        title="Eliminar"
                        @click="remove(doc)"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>

    <Modal
      v-model="reviewOpen"
      title="Revisar documento"
      subtitle="Confirma el contribuyente, el tipo y los datos detectados. La revisión no modifica el fichero original."
      size="lg"
      persistent
    >
      <div v-if="reviewDoc" class="space-y-4">
        <div class="rounded-2xl border border-slate-200 bg-slate-50/60 p-4">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-slate-800">{{ reviewDoc.filename }}</p>
              <p v-if="reviewDoc.library_path" class="mt-0.5 break-all font-mono text-[11px] text-slate-400">
                {{ reviewDoc.library_path }}
              </p>
            </div>
            <span class="chip bg-white text-slate-500">{{ ocrLabel(reviewDoc.ocr_status) }}</span>
          </div>

          <div v-if="isImage(reviewDoc) || isPDF(reviewDoc)" class="mt-3 overflow-hidden rounded-xl border border-slate-200 bg-white">
            <img
              v-if="isImage(reviewDoc)"
              :src="previewURL(reviewDoc)"
              :alt="`Vista previa de ${reviewDoc.filename}`"
              class="max-h-80 w-full object-contain"
            />
            <iframe
              v-else
              :src="previewURL(reviewDoc)"
              :title="`Vista previa de ${reviewDoc.filename}`"
              class="h-80 w-full"
            />
          </div>

          <div class="mt-3 max-h-56 overflow-auto rounded-xl bg-white p-3">
            <pre
              v-if="reviewDoc.text_excerpt"
              class="whitespace-pre-wrap break-words font-sans text-xs leading-relaxed text-slate-600"
            >{{ reviewDoc.text_excerpt }}</pre>
            <p v-else class="text-xs text-slate-500">
              No hay texto extraído. Si el documento es una imagen o un PDF escaneado, instala Tesseract y usa
              «Reprocesar texto».
            </p>
          </div>
          <ul v-if="reviewDoc.review_status === 'review' && reviewReasons.length" class="mt-3 space-y-1 text-[11px] text-amber-700">
            <li v-for="reason in reviewReasons" :key="reason" class="flex gap-1.5">
              <AlertTriangle class="mt-0.5 h-3 w-3 shrink-0" /> {{ reasonLabel(reason) }}
            </li>
          </ul>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label" for="review-taxpayer">Contribuyente</label>
            <select id="review-taxpayer" v-model="reviewForm.taxpayer_id" class="input">
              <option value="">Sin asignar</option>
              <option v-for="t in store.taxpayers" :key="t.id" :value="t.id">{{ t.name }} · {{ t.nif }}</option>
            </select>
          </div>
          <div>
            <label class="label" for="review-kind">Tipo</label>
            <select id="review-kind" v-model="reviewForm.kind" class="input">
              <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </div>
          <div>
            <label class="label" for="review-date">Fecha</label>
            <input id="review-date" v-model="reviewForm.doc_date" type="date" class="input" />
          </div>
          <div>
            <label class="label" for="review-amount">Importe</label>
            <input id="review-amount" v-model="reviewForm.amount" class="input" placeholder="0,00" inputmode="decimal" />
          </div>
          <div class="sm:col-span-2">
            <label class="label" for="review-nif">NIF contraparte</label>
            <input id="review-nif" v-model="reviewForm.counterparty_nif" class="input" placeholder="Opcional" />
          </div>
        </div>

        <p class="text-[11px] leading-relaxed text-slate-500">
          Al guardar, el documento deja de estar pendiente de revisión. Si cambias la fecha o el tipo, el fichero
          permanece en su carpeta actual; puedes reorganizar la biblioteca desde Ajustes.
        </p>
      </div>

      <template #footer>
        <button class="btn-ghost mr-auto" :disabled="reprocessing" @click="reprocess">
          <Loader2 v-if="reprocessing" class="h-4 w-4 animate-spin" />
          <ScanLine v-else class="h-4 w-4" />
          {{ reprocessing ? 'Procesando…' : 'Reprocesar texto' }}
        </button>
        <a
          v-if="reviewDoc"
          :href="downloadURL(reviewDoc)"
          class="btn-ghost"
        >
          <Download class="h-4 w-4" /> Descargar
        </a>
        <button class="btn-ghost" @click="reviewOpen = false">Cancelar</button>
        <button class="btn-accent" :disabled="reviewSaving" @click="saveReview">
          <Loader2 v-if="reviewSaving" class="h-4 w-4 animate-spin" />
          {{ reviewSaving ? 'Guardando…' : 'Guardar revisión' }}
        </button>
      </template>
    </Modal>

    <ConfirmDialog
      :model-value="!!confirmDoc"
      title="Eliminar documento"
      :message="`Se eliminará «${confirmDoc?.filename ?? ''}» de la aplicación y del almacenamiento local. Esta acción no se puede deshacer.`"
      confirm-label="Eliminar documento"
      tone="danger"
      :busy="deleting"
      @update:model-value="onDeleteDialogUpdate"
      @confirm="confirmDelete"
    />
  </div>
</template>
