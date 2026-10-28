<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Download, FileText, FolderOpen, Search, Trash2, Upload } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import { api, bytes, dateLabel, euros } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { DocumentItem } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const documents = ref<DocumentItem[]>([])
const loading = ref(true)
const uploading = ref(false)
const search = ref('')
const kindFilter = ref('')

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const kind = ref('factura_recibida')
const docDate = ref(new Date().toISOString().slice(0, 10))
const amount = ref('')
const counterparty = ref('')

const kinds = [
  { value: 'factura_emitida', label: 'Factura emitida' },
  { value: 'factura_recibida', label: 'Factura recibida' },
  { value: 'nomina', label: 'Nómina' },
  { value: 'recibo', label: 'Recibo' },
  { value: 'justificante', label: 'Justificante' },
  { value: 'modelo', label: 'Modelo presentado' },
  { value: 'other', label: 'Otro' },
]

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  return documents.value.filter((doc) => {
    if (kindFilter.value && doc.kind !== kindFilter.value) return false
    if (!term) return true
    return `${doc.filename} ${doc.counterparty_nif} ${doc.kind}`.toLowerCase().includes(term)
  })
})

const totalSize = computed(() => documents.value.reduce((sum, doc) => sum + doc.size, 0))

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    documents.value = await api.get<DocumentItem[]>(`/documents?${params}`)
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudieron cargar los documentos')
  } finally {
    loading.value = false
  }
}

function pickFile() {
  fileInput.value?.click()
}

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  selectedFile.value = target.files?.[0] ?? null
  if (selectedFile.value && !amount.value) {
    amount.value = ''
  }
}

async function upload() {
  if (!store.selectedId) {
    toasts.error('Selecciona un contribuyente antes de subir documentos')
    return
  }
  if (!selectedFile.value) {
    toasts.error('Selecciona un fichero')
    return
  }
  uploading.value = true
  try {
    const form = new FormData()
    form.set('file', selectedFile.value)
    form.set('taxpayer_id', store.selectedId)
    form.set('kind', kind.value)
    if (docDate.value) form.set('doc_date', docDate.value)
    if (amount.value) form.set('amount', amount.value)
    if (counterparty.value) form.set('counterparty_nif', counterparty.value)
    await api.upload<DocumentItem>('/documents', form)
    toasts.success('Documento subido y archivado')
    selectedFile.value = null
    amount.value = ''
    counterparty.value = ''
    if (fileInput.value) fileInput.value.value = ''
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo subir el documento')
  } finally {
    uploading.value = false
  }
}

async function remove(doc: DocumentItem) {
  if (!window.confirm(`¿Eliminar "${doc.filename}"? Esta acción no se puede deshacer.`)) return
  try {
    await api.del(`/documents/${doc.id}`)
    toasts.success('Documento eliminado')
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo eliminar')
  }
}

function kindLabel(value: string): string {
  return kinds.find((item) => item.value === value)?.label ?? value
}

watch(() => store.selectedId, load)
onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Documentos"
      subtitle="Facturas, nóminas, recibos y justificantes del contribuyente activo. Los ficheros se guardan en local y se referencian por hash."
    >
      <template #actions>
        <span class="chip bg-slate-100 text-slate-600">{{ documents.length }} ficheros · {{ bytes(totalSize) }}</span>
      </template>
    </PageHeader>

    <div class="grid gap-6 lg:grid-cols-3">
      <section class="card card-pad h-fit">
        <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Subir documento</h2>
        <p class="mt-1 text-xs text-slate-500">
          Para {{ store.selectedTaxpayer?.name ?? 'el contribuyente activo' }}.
        </p>

        <button
          class="mt-4 flex w-full flex-col items-center justify-center rounded-2xl border-2 border-dashed border-slate-200 bg-slate-50/60 px-4 py-7 text-center transition hover:border-slate-300 hover:bg-slate-50"
          @click="pickFile"
        >
          <Upload class="h-6 w-6 text-slate-400" />
          <span class="mt-2 text-sm font-semibold text-slate-700">
            {{ selectedFile?.name ?? 'Seleccionar fichero' }}
          </span>
          <span class="mt-0.5 text-xs text-slate-400">
            {{ selectedFile ? bytes(selectedFile.size) : 'PDF, imagen o cualquier documento' }}
          </span>
        </button>
        <input ref="fileInput" type="file" class="hidden" @change="onFileChange" />

        <div class="mt-4 space-y-3">
          <div>
            <label class="label">Tipo</label>
            <select v-model="kind" class="input">
              <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="label">Fecha</label>
              <input v-model="docDate" type="date" class="input" />
            </div>
            <div>
              <label class="label">Importe</label>
              <input v-model="amount" class="input" placeholder="0,00" inputmode="decimal" />
            </div>
          </div>
          <div>
            <label class="label">NIF contraparte</label>
            <input v-model="counterparty" class="input" placeholder="Opcional" />
          </div>
        </div>

        <button class="btn-accent mt-4 w-full" :disabled="uploading" @click="upload">
          <Upload class="h-4 w-4" /> {{ uploading ? 'Subiendo…' : 'Subir y archivar' }}
        </button>
        <p class="mt-3 text-[11px] leading-relaxed text-slate-400">
          El cálculo de modelos usa las facturas registradas, no los PDF. Sube el fichero y registra la factura en la sección Facturas.
        </p>
      </section>

      <section class="lg:col-span-2">
        <div class="mb-3 flex flex-col gap-2 sm:flex-row">
          <div class="relative flex-1">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input v-model="search" class="input pl-9" placeholder="Buscar documento…" />
          </div>
          <select v-model="kindFilter" class="input sm:w-52">
            <option value="">Todos los tipos</option>
            <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>

        <div v-if="loading" class="space-y-2">
          <div v-for="i in 5" :key="i" class="h-16 animate-pulse rounded-2xl bg-white" />
        </div>

        <EmptyState
          v-else-if="!filtered.length"
          title="Sin documentos"
          description="Sube facturas, nóminas o justificantes para tener toda la documentación del expediente en un único sitio."
        >
          <template #icon><FolderOpen class="h-6 w-6" /></template>
          <template #action>
            <button class="btn-ghost" @click="pickFile"><Upload class="h-4 w-4" /> Subir el primero</button>
          </template>
        </EmptyState>

        <div v-else class="card overflow-hidden">
          <div class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Fichero</th>
                  <th>Tipo</th>
                  <th>Fecha</th>
                  <th class="text-right">Importe</th>
                  <th class="text-right">Tamaño</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="doc in filtered" :key="doc.id">
                  <td>
                    <div class="flex items-center gap-2">
                      <span class="grid h-8 w-8 place-items-center rounded-lg bg-slate-100 text-slate-500">
                        <FileText class="h-4 w-4" />
                      </span>
                      <div class="min-w-0">
                        <p class="max-w-[16rem] truncate text-sm font-semibold text-slate-800">{{ doc.filename }}</p>
                        <p class="max-w-[16rem] truncate text-[11px] text-slate-400">{{ doc.counterparty_nif || doc.mime }}</p>
                      </div>
                    </div>
                  </td>
                  <td><span class="chip bg-slate-100 text-slate-600">{{ kindLabel(doc.kind) }}</span></td>
                  <td class="text-xs text-slate-500">{{ doc.doc_date ? dateLabel(doc.doc_date) : dateLabel(doc.created_at) }}</td>
                  <td class="text-right font-mono text-xs tabular-nums text-slate-600">
                    {{ doc.amount_cents !== null ? euros(doc.amount_cents) : '—' }}
                  </td>
                  <td class="text-right text-xs text-slate-500">{{ bytes(doc.size) }}</td>
                  <td>
                    <div class="flex justify-end gap-1">
                      <a :href="`/api/documents/${doc.id}/download`" class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-700" title="Descargar">
                        <Download class="h-4 w-4" />
                      </a>
                      <button class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600" title="Eliminar" @click="remove(doc)">
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
  </div>
</template>
