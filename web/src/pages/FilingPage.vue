<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  AlertTriangle,
  ArrowLeft,
  Calculator,
  CheckCircle2,
  ClipboardCheck,
  Download,
  ExternalLink,
  FileCheck2,
  FileText,
  Save,
  Upload,
} from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { api, bytes, dateLabel, dateTimeLabel, daysUntil } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { DocumentItem, Filing, FilingStatus, Model, Taxpayer } from '@/types'

const props = defineProps<{ id: string }>()
const store = useAppStore()
const toasts = useToasts()

const filing = ref<Filing | null>(null)
const taxpayer = ref<Taxpayer | null>(null)
const model = ref<Model | null>(null)
const documents = ref<DocumentItem[]>([])
const loading = ref(true)
const busy = ref('')
const tab = ref<'summary' | 'docs' | 'present'>('summary')
const notes = ref('')
const checklist = ref<Record<string, boolean>>({})
const receiptInput = ref<HTMLInputElement | null>(null)
const receiptFile = ref<File | null>(null)

const tabs = [
  { id: 'summary', label: 'Resumen y cálculo' },
  { id: 'docs', label: 'Documentación' },
  { id: 'present', label: 'Presentación' },
] as const

function setTab(id: 'summary' | 'docs' | 'present') {
  tab.value = id
}

const lines = computed(() => filing.value?.computed?.lines ?? [])
const warnings = computed(() => filing.value?.computed?.warnings ?? [])
const dueDays = computed(() => (filing.value ? daysUntil(filing.value.due_date) : 0))
const isOpen = computed(() => filing.value && !['presented', 'not_applicable'].includes(filing.value.status))

async function load() {
  loading.value = true
  try {
    const f = await api.get<Filing>(`/filings/${props.id}`)
    filing.value = f
    if (store.selectedId && store.selectedId !== f.taxpayer_id) store.select(f.taxpayer_id)
    notes.value = f.notes
    const stored = (f.data as { checklist?: Record<string, boolean> }).checklist
    checklist.value = stored && typeof stored === 'object' ? { ...stored } : {}

    const [t, docs, catalog] = await Promise.all([
      api.get<Taxpayer>(`/taxpayers/${f.taxpayer_id}`),
      api.get<DocumentItem[]>(`/documents?taxpayer_id=${f.taxpayer_id}`),
      api.get<{ models: Model[] }>(
        `/catalog${f.taxpayer_id ? `?taxpayer_id=${f.taxpayer_id}` : ''}`,
      ),
    ])
    taxpayer.value = t
    documents.value = docs
    model.value = catalog.models.find((m) => m.code === f.model_code) ?? null
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo cargar el expediente')
  } finally {
    loading.value = false
  }
}

async function calculate() {
  busy.value = 'calculate'
  try {
    const data = await api.post<{ filing: Filing; invoices: number }>(`/filings/${props.id}/calculate`)
    filing.value = data.filing
    toasts.success(`Cálculo completado con ${data.invoices} facturas`)
    tab.value = 'summary'
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo calcular el modelo')
  } finally {
    busy.value = ''
  }
}

async function saveNotes() {
  busy.value = 'notes'
  try {
    filing.value = await api.patch<Filing>(`/filings/${props.id}`, { notes: notes.value })
    toasts.success('Notas guardadas')
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudieron guardar las notas')
  } finally {
    busy.value = ''
  }
}

async function toggleCheck(item: string) {
  if (!filing.value) return
  checklist.value = { ...checklist.value, [item]: !checklist.value[item] }
  try {
    filing.value = await api.patch<Filing>(`/filings/${props.id}`, {
      data: { ...filing.value.data, checklist: checklist.value },
    })
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo guardar la checklist')
  }
}

function onStatusChange(event: Event) {
  changeStatus((event.target as HTMLSelectElement).value as FilingStatus)
}

async function changeStatus(status: FilingStatus) {
  try {
    filing.value = await api.patch<Filing>(`/filings/${props.id}`, { status })
    toasts.success('Estado actualizado')
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo actualizar el estado')
  }
}

async function markPresented() {
  if (!window.confirm('¿Marcar este expediente como presentado? La aplicación no presenta nada en AEAT: solo registra el estado y archiva el justificante.')) return
  busy.value = 'present'
  try {
    filing.value = await api.post<Filing>(`/filings/${props.id}/present`, {})
    toasts.success('Expediente marcado como presentado')
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo marcar como presentado')
  } finally {
    busy.value = ''
  }
}

function pickReceipt() {
  receiptInput.value?.click()
}

function onReceiptChange(event: Event) {
  const target = event.target as HTMLInputElement
  receiptFile.value = target.files?.[0] ?? null
}

async function uploadReceipt() {
  if (!receiptFile.value || !filing.value || !taxpayer.value) {
    toasts.error('Selecciona primero el justificante')
    return
  }
  busy.value = 'receipt'
  try {
    const form = new FormData()
    form.set('file', receiptFile.value)
    form.set('taxpayer_id', taxpayer.value.id)
    form.set('filing_id', filing.value.id)
    form.set('kind', 'justificante')
    const doc = await api.upload<DocumentItem>('/documents', form)
    filing.value = await api.post<Filing>(`/filings/${props.id}/present`, { receipt_document_id: doc.id })
    documents.value = await api.get<DocumentItem[]>(`/documents?taxpayer_id=${taxpayer.value.id}`)
    receiptFile.value = null
    toasts.success('Justificante archivado y expediente marcado como presentado')
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo subir el justificante')
  } finally {
    busy.value = ''
  }
}

function kindLabel(kind: string): string {
  const map: Record<string, string> = {
    factura_emitida: 'Factura emitida',
    factura_recibida: 'Factura recibida',
    nomina: 'Nómina',
    recibo: 'Recibo',
    justificante: 'Justificante',
    modelo: 'Modelo presentado',
    other: 'Otro',
  }
  return map[kind] ?? kind
}

onMounted(load)
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <div class="h-24 animate-pulse rounded-2xl bg-white" />
    <div class="h-64 animate-pulse rounded-2xl bg-white" />
  </div>

  <div v-else-if="!filing" class="card card-pad text-sm text-slate-500">
    Expediente no encontrado.
  </div>

  <div v-else>
    <div class="mb-4">
      <RouterLink to="/calendario" class="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 hover:text-slate-900">
        <ArrowLeft class="h-3.5 w-3.5" /> Volver al calendario
      </RouterLink>
    </div>

    <PageHeader :title="`${filing.model_code} · ${filing.model_name}`" :subtitle="`${taxpayer?.name ?? ''} · ${taxpayer?.nif ?? ''}`">
      <template #actions>
        <button v-if="model?.calculator" class="btn-primary" :disabled="busy === 'calculate'" @click="calculate">
          <Calculator class="h-4 w-4" :class="{ 'animate-pulse': busy === 'calculate' }" />
          {{ busy === 'calculate' ? 'Calculando…' : 'Calcular' }}
        </button>
        <a v-if="model?.url" :href="model.url" target="_blank" rel="noreferrer" class="btn-accent">
          <ExternalLink class="h-4 w-4" /> Abrir en AEAT
        </a>
      </template>
    </PageHeader>

    <div class="mb-5 flex flex-wrap items-center gap-3">
      <StatusBadge :status="filing.status" />
      <span class="chip bg-slate-100 text-slate-600">{{ filing.entity }}</span>
      <span class="chip bg-slate-100 text-slate-600">{{ filing.period_label }}</span>
      <span class="chip bg-slate-100 text-slate-600">Vence {{ dateLabel(filing.due_date) }}</span>
      <span v-if="filing.presented_at" class="chip bg-emerald-50 text-emerald-700">
        Presentado {{ dateTimeLabel(filing.presented_at) }}
      </span>
    </div>

    <div
      v-if="isOpen && dueDays < 0"
      class="mb-5 flex items-start gap-3 rounded-2xl border border-rose-200 bg-rose-50 p-4"
    >
      <AlertTriangle class="mt-0.5 h-5 w-5 shrink-0 text-rose-500" />
      <div>
        <p class="text-sm font-semibold text-rose-800">Este vencimiento pasó hace {{ Math.abs(dueDays) }} días</p>
        <p class="mt-0.5 text-sm text-rose-700">
          Si aún no lo has presentado, revisa el plazo y posible recargo antes de continuar.
        </p>
      </div>
    </div>

    <div class="mb-5 flex gap-1 overflow-x-auto rounded-2xl border border-slate-200 bg-white p-1">
      <button
        v-for="item in tabs"
        :key="item.id"
        class="flex-1 whitespace-nowrap rounded-xl px-4 py-2 text-sm font-semibold transition"
        :class="tab === item.id ? 'bg-slate-900 text-white' : 'text-slate-500 hover:bg-slate-50'"
        @click="setTab(item.id)"
      >
        {{ item.label }}
      </button>
    </div>

    <!-- Resumen -->
    <div v-if="tab === 'summary'" class="grid gap-5 lg:grid-cols-3">
      <section class="space-y-4 lg:col-span-2">
        <div class="card">
          <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
            <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Casillas calculadas</h2>
            <span v-if="filing.computed?.version" class="text-xs text-slate-400">
              v{{ filing.computed.version }}
            </span>
          </div>

          <div v-if="lines.length" class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th class="w-20">Casilla</th>
                  <th>Concepto</th>
                  <th class="text-right">Importe</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="line in lines" :key="line.id" :class="{ 'bg-slate-50/70 font-semibold': line.kind === 'resultado' }">
                  <td class="font-mono text-xs text-slate-500">{{ line.id }}</td>
                  <td>{{ line.label }}</td>
                  <td class="text-right font-mono tabular-nums">
                    {{ line.kind === 'tipo' ? `${line.value} %` : `${line.value} €` }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="px-5 py-10 text-center">
            <Calculator class="mx-auto h-8 w-8 text-slate-300" />
            <p class="mt-3 text-sm font-semibold text-slate-700">Sin cálculo todavía</p>
            <p class="mx-auto mt-1 max-w-md text-sm text-slate-500">
              {{
                model?.calculator
                  ? model.code === '130'
                    ? 'El modelo 130 se calcula de forma acumulada desde el 1 de enero. Revisa que las facturas del ejercicio estén registradas y pulsa Calcular.'
                    : 'La aplicación puede calcular este modelo a partir de las facturas del periodo. Revisa que estén registradas y pulsa Calcular.'
                  : 'Este modelo se completa manualmente o con la extensión. Usa la documentación de la pestaña siguiente como checklist.'
              }}
            </p>
            <button v-if="model?.calculator" class="btn-primary mt-4" :disabled="busy === 'calculate'" @click="calculate">
              <Calculator class="h-4 w-4" /> Calcular ahora
            </button>
          </div>
        </div>

        <div v-if="warnings.length" class="card border-amber-200 bg-amber-50/60 p-5">
          <h2 class="text-xs font-bold uppercase tracking-wider text-amber-700">Revisa antes de presentar</h2>
          <ul class="mt-3 space-y-2">
            <li v-for="warning in warnings" :key="warning" class="flex items-start gap-2 text-sm text-amber-800">
              <AlertTriangle class="mt-0.5 h-4 w-4 shrink-0" />
              {{ warning }}
            </li>
          </ul>
        </div>
      </section>

      <aside class="space-y-4">
        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Datos del expediente</h2>
          <dl class="mt-4 space-y-3 text-sm">
            <div class="flex items-start justify-between gap-3">
              <dt class="text-slate-500">Periodo</dt>
              <dd class="text-right font-medium text-slate-800">{{ dateLabel(filing.period_start) }} — {{ dateLabel(filing.period_end) }}</dd>
            </div>
            <div class="flex items-start justify-between gap-3">
              <dt class="text-slate-500">Vencimiento</dt>
              <dd class="text-right font-medium text-slate-800">{{ dateLabel(filing.due_date) }}</dd>
            </div>
            <div class="flex items-start justify-between gap-3">
              <dt class="text-slate-500">Presentado</dt>
              <dd class="text-right font-medium text-slate-800">{{ dateTimeLabel(filing.presented_at) }}</dd>
            </div>
            <div class="flex items-start justify-between gap-3">
              <dt class="text-slate-500">Justificante</dt>
              <dd class="text-right font-medium text-slate-800">
                <a
                  v-if="filing.receipt_document_id"
                  :href="`/api/documents/${filing.receipt_document_id}/download`"
                  class="text-emerald-700 hover:underline"
                >Descargar</a>
                <span v-else class="text-slate-400">Sin archivar</span>
              </dd>
            </div>
          </dl>

          <div class="mt-5">
            <label class="label">Estado del expediente</label>
            <select class="input" :value="filing.status" @change="onStatusChange">
              <option value="pending">Pendiente</option>
              <option value="in_progress">En curso</option>
              <option value="ready">Listo para presentar</option>
              <option value="presented">Presentado</option>
              <option value="late">Fuera de plazo</option>
              <option value="not_applicable">No aplica</option>
            </select>
          </div>
        </div>

        <div v-if="model?.documents.length" class="card card-pad">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Checklist</h2>
            <span class="text-xs text-slate-400">
              {{ model.documents.filter((d) => checklist[d]).length }}/{{ model.documents.length }}
            </span>
          </div>
          <ul class="mt-3 space-y-2">
            <li v-for="item in model.documents" :key="item">
              <label class="flex cursor-pointer items-start gap-2.5 rounded-lg p-1.5 text-sm hover:bg-slate-50">
                <input
                  type="checkbox"
                  class="mt-0.5 h-4 w-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                  :checked="Boolean(checklist[item])"
                  @change="toggleCheck(item)"
                />
                <span :class="checklist[item] ? 'text-slate-400 line-through' : 'text-slate-600'">{{ item }}</span>
              </label>
            </li>
          </ul>
        </div>

        <div class="rounded-2xl bg-slate-900 p-5 text-white">
          <p class="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-slate-400">
            <ClipboardCheck class="h-3.5 w-3.5" /> Flujo recomendado
          </p>
          <ol class="mt-3 space-y-2 text-sm text-slate-300">
            <li>1. Revisa facturas y documentos.</li>
            <li>2. Calcula el modelo.</li>
            <li>3. Abre AEAT y firma con tu certificado.</li>
            <li>4. Archiva el justificante.</li>
          </ol>
        </div>
      </aside>
    </div>

    <!-- Documentación -->
    <div v-else-if="tab === 'docs'" class="grid gap-5 lg:grid-cols-3">
      <section class="card lg:col-span-2">
        <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Documentos del contribuyente</h2>
          <RouterLink to="/documentos" class="btn-ghost btn-sm">
            <Upload class="h-3.5 w-3.5" /> Subir
          </RouterLink>
        </div>
        <div v-if="documents.length" class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Fichero</th>
                <th>Tipo</th>
                <th>Fecha</th>
                <th class="text-right">Tamaño</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="doc in documents" :key="doc.id">
                <td>
                  <div class="flex items-center gap-2">
                    <FileText class="h-4 w-4 shrink-0 text-slate-400" />
                    <span class="max-w-[16rem] truncate font-medium text-slate-800">{{ doc.filename }}</span>
                  </div>
                </td>
                <td class="text-xs text-slate-500">{{ kindLabel(doc.kind) }}</td>
                <td class="text-xs text-slate-500">{{ doc.doc_date ? dateLabel(doc.doc_date) : dateLabel(doc.created_at) }}</td>
                <td class="text-right text-xs text-slate-500">{{ bytes(doc.size) }}</td>
                <td class="text-right">
                  <a :href="`/api/documents/${doc.id}/download`" class="btn-ghost btn-sm" title="Descargar">
                    <Download class="h-3.5 w-3.5" />
                  </a>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="px-5 py-12 text-center text-sm text-slate-500">
          No hay documentos asociados. Sube facturas, nóminas o justificantes para tenerlos a mano.
        </div>
      </section>

      <aside class="space-y-4">
        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Documentación necesaria</h2>
          <ul class="mt-3 space-y-2">
            <li v-for="item in model?.documents ?? []" :key="item" class="flex items-start gap-2 text-sm text-slate-600">
              <CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
              {{ item }}
            </li>
            <li v-if="!model?.documents.length" class="text-sm text-slate-400">Sin checklist específica.</li>
          </ul>
        </div>
        <div v-if="model?.notes" class="rounded-2xl border border-amber-200 bg-amber-50 p-4">
          <p class="text-xs font-bold uppercase tracking-wider text-amber-700">A tener en cuenta</p>
          <p class="mt-1 text-sm leading-relaxed text-amber-800">{{ model.notes }}</p>
        </div>
      </aside>
    </div>

    <!-- Presentación -->
    <div v-else class="grid gap-5 lg:grid-cols-3">
      <section class="space-y-4 lg:col-span-2">
        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Presentar en la sede electrónica</h2>
          <div class="mt-4 space-y-4">
            <div class="flex gap-3">
              <span class="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">1</span>
              <div>
                <p class="text-sm font-semibold text-slate-800">Revisa el cálculo y la documentación</p>
                <p class="text-sm text-slate-500">Comprueba las casillas y las advertencias antes de abrir la sede.</p>
              </div>
            </div>
            <div class="flex gap-3">
              <span class="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">2</span>
              <div>
                <p class="text-sm font-semibold text-slate-800">Rellena el modelo</p>
                <p class="text-sm text-slate-500">
                  Con la extensión emparejada, abre el expediente desde el popup y los datos se mostrarán sobre la web de AEAT.
                  También puedes abrir la sede y copiarlos manualmente.
                </p>
              </div>
            </div>
            <div class="flex gap-3">
              <span class="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">3</span>
              <div>
                <p class="text-sm font-semibold text-slate-800">Firma con tu certificado</p>
                <p class="text-sm text-slate-500">
                  La firma se realiza siempre en el navegador con tu certificado. La aplicación nunca accede a tus claves.
                </p>
              </div>
            </div>
            <div class="flex gap-3">
              <span class="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">4</span>
              <div>
                <p class="text-sm font-semibold text-slate-800">Archiva el justificante</p>
                <p class="text-sm text-slate-500">Sube el PDF de acuse de recibo para cerrar el expediente.</p>
              </div>
            </div>
          </div>

          <div class="mt-5 flex flex-wrap gap-2">
            <a v-if="model?.url" :href="model.url" target="_blank" rel="noreferrer" class="btn-accent">
              <ExternalLink class="h-4 w-4" /> Abrir {{ filing.entity }}
            </a>
            <button class="btn-ghost" :disabled="busy === 'present'" @click="markPresented">
              <FileCheck2 class="h-4 w-4" /> Marcar como presentado
            </button>
          </div>
        </div>

        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Justificante de presentación</h2>
          <p class="mt-2 text-sm text-slate-500">
            Sube el PDF de acuse. Al hacerlo, el expediente se marca como presentado y el fichero queda archivado.
          </p>
          <div class="mt-4 flex flex-wrap items-center gap-3">
            <input ref="receiptInput" type="file" class="hidden" accept="application/pdf,image/*" @change="onReceiptChange" />
            <button class="btn-ghost" @click="pickReceipt">
              <Upload class="h-4 w-4" /> Seleccionar fichero
            </button>
            <span class="text-sm text-slate-500">{{ receiptFile?.name ?? 'Ningún fichero seleccionado' }}</span>
            <button class="btn-primary" :disabled="!receiptFile || busy === 'receipt'" @click="uploadReceipt">
              <Save class="h-4 w-4" /> {{ busy === 'receipt' ? 'Subiendo…' : 'Archivar y marcar' }}
            </button>
          </div>
          <p v-if="filing.receipt_document_id" class="mt-4 text-sm text-emerald-700">
            <CheckCircle2 class="mr-1 inline h-4 w-4" />
            Justificante archivado.
            <a :href="`/api/documents/${filing.receipt_document_id}/download`" class="font-semibold underline">Descargar</a>
          </p>
        </div>
      </section>

      <aside class="space-y-4">
        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Notas internas</h2>
          <textarea v-model="notes" rows="6" class="input mt-3 resize-y" placeholder="Incidencias, número de justificante, observaciones…" />
          <button class="btn-primary mt-3 w-full" :disabled="busy === 'notes'" @click="saveNotes">
            <Save class="h-4 w-4" /> Guardar notas
          </button>
        </div>

        <div v-if="filing.computed?.calculated_at" class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Último cálculo</h2>
          <p class="mt-2 text-sm text-slate-600">{{ dateTimeLabel(filing.computed.calculated_at) }}</p>
          <p class="mt-1 text-xs text-slate-400">
            Los importes se basan en {{ documents.length }} documentos y las facturas registradas en el periodo de cálculo.
          </p>
        </div>
      </aside>
    </div>
  </div>
</template>
