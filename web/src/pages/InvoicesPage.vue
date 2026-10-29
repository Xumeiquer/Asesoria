<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Copy, Paperclip, Pencil, Plus, ReceiptText, Search, Trash2 } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { api, centsFromEuros, dateLabel, euros } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { DocumentItem, Invoice } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const route = useRoute()
const router = useRouter()

const invoices = ref<Invoice[]>([])
const documents = ref<DocumentItem[]>([])
const loading = ref(true)
const saving = ref(false)
const deleting = ref(false)
const modalOpen = ref(false)
const editingId = ref<string | null>(null)
const confirmInvoice = ref<Invoice | null>(null)
const direction = ref<'all' | 'issued' | 'received'>('all')
const sort = ref<'date_desc' | 'date_asc' | 'amount_desc' | 'amount_asc'>('date_desc')
const errors = ref<Record<string, string>>({})
const currentYear = new Date().getFullYear()
const years = Array.from({ length: 5 }, (_, index) => currentYear - 3 + index)
const year = ref(currentYear)
const search = ref('')

const form = ref({
  taxpayer_id: '',
  direction: 'received' as 'issued' | 'received',
  number: '',
  invoice_date: localISODate(),
  counterparty_nif: '',
  counterparty_name: '',
  concept: '',
  base: '',
  vat_rate: 21,
  retention_rate: 0,
  category: '',
  document_id: '',
})

function localISODate(date = new Date()): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

function inputEuros(cents: number): string {
  return (cents / 100).toFixed(2).replace('.', ',')
}

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  return invoices.value.filter((invoice) => {
    if (direction.value !== 'all' && invoice.direction !== direction.value) return false
    if (!term) return true
    return `${invoice.number} ${invoice.counterparty_name} ${invoice.counterparty_nif} ${invoice.concept}`
      .toLowerCase()
      .includes(term)
  })
})

const sortedFiltered = computed(() => {
  const list = [...filtered.value]
  switch (sort.value) {
    case 'date_asc':
      return list.sort((a, b) => a.invoice_date.localeCompare(b.invoice_date))
    case 'amount_desc':
      return list.sort((a, b) => b.total_cents - a.total_cents)
    case 'amount_asc':
      return list.sort((a, b) => a.total_cents - b.total_cents)
    default:
      return list.sort((a, b) => b.invoice_date.localeCompare(a.invoice_date))
  }
})

const totals = computed(() => {
  const issued = invoices.value.filter((invoice) => invoice.direction === 'issued')
  const received = invoices.value.filter((invoice) => invoice.direction === 'received')
  return {
    issued: issued.reduce((sum, invoice) => sum + invoice.base_cents, 0),
    received: received.reduce((sum, invoice) => sum + invoice.base_cents, 0),
    vat:
      issued.reduce((sum, invoice) => sum + invoice.vat_cents, 0) -
      received.reduce((sum, invoice) => sum + invoice.vat_cents, 0),
  }
})

const preview = computed(() => {
  const base = centsFromEuros(form.value.base)
  const vat = Math.round((base * form.value.vat_rate) / 100)
  const retention = Math.round((base * form.value.retention_rate) / 100)
  return { base, vat, retention, total: base + vat - retention }
})

function clearFilters() {
  search.value = ''
  direction.value = 'all'
}

function documentLabel(id: string | null): string {
  if (!id) return ''
  return documents.value.find((document) => document.id === id)?.filename ?? 'Documento adjunto'
}

async function loadDocuments() {
  if (!store.selectedId) {
    documents.value = []
    return
  }
  try {
    documents.value = await api.get<DocumentItem[]>(
      `/documents?taxpayer_id=${encodeURIComponent(store.selectedId)}&limit=200`,
    )
  } catch {
    documents.value = []
  }
}

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ year: String(year.value) })
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    invoices.value = await api.get<Invoice[]>(`/invoices?${params}`)
    await loadDocuments()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudieron cargar las facturas')
  } finally {
    loading.value = false
  }
}

function resetForm() {
  form.value = {
    taxpayer_id: store.selectedId,
    direction: 'received',
    number: '',
    invoice_date: localISODate(),
    counterparty_nif: '',
    counterparty_name: '',
    concept: '',
    base: '',
    vat_rate: 21,
    retention_rate: 0,
    category: '',
    document_id: '',
  }
  errors.value = {}
}

function openCreate() {
  editingId.value = null
  resetForm()
  modalOpen.value = true
}

function openEdit(invoice: Invoice) {
  editingId.value = invoice.id
  form.value = {
    taxpayer_id: invoice.taxpayer_id,
    direction: invoice.direction,
    number: invoice.number,
    invoice_date: invoice.invoice_date,
    counterparty_nif: invoice.counterparty_nif,
    counterparty_name: invoice.counterparty_name,
    concept: invoice.concept,
    base: inputEuros(invoice.base_cents),
    vat_rate: invoice.vat_rate / 100,
    retention_rate: invoice.retention_rate / 100,
    category: invoice.category,
    document_id: invoice.document_id ?? '',
  }
  errors.value = {}
  modalOpen.value = true
}

function duplicate(invoice: Invoice) {
  openEdit(invoice)
  editingId.value = null
  form.value.number = ''
  form.value.invoice_date = localISODate()
}

function validateForm(): boolean {
  const next: Record<string, string> = {}
  if (!form.value.taxpayer_id && !store.selectedId) {
    next.taxpayer = 'Selecciona un contribuyente antes de guardar.'
  }
  if (!form.value.invoice_date) {
    next.invoice_date = 'Indica la fecha de la factura.'
  }
  if (!form.value.base.trim()) {
    next.base = 'Introduce la base imponible.'
  } else if (centsFromEuros(form.value.base) < 0) {
    next.base = 'La base no puede ser negativa.'
  }
  const nif = form.value.counterparty_nif.replace(/[\s.-]/g, '').toUpperCase()
  if (nif && !/^(?:\d{8}[A-Z]|[XYZ]\d{7}[A-Z]|[A-Z]\d{7}[A-Z0-9])$/.test(nif)) {
    next.counterparty_nif = 'El NIF no tiene un formato válido.'
  }
  errors.value = next
  return Object.keys(next).length === 0
}

async function save() {
  if (!validateForm()) return
  saving.value = true
  try {
    const payload = {
      taxpayer_id: form.value.taxpayer_id || store.selectedId,
      direction: form.value.direction,
      number: form.value.number,
      invoice_date: form.value.invoice_date,
      counterparty_nif: form.value.counterparty_nif,
      counterparty_name: form.value.counterparty_name,
      concept: form.value.concept,
      base_cents: centsFromEuros(form.value.base),
      vat_rate: form.value.vat_rate * 100,
      retention_rate: form.value.retention_rate * 100,
      category: form.value.category,
      document_id: form.value.document_id || null,
    }
    if (editingId.value) {
      await api.put<Invoice>(`/invoices/${editingId.value}`, payload)
      toasts.success('Factura actualizada')
    } else {
      await api.post<Invoice>('/invoices', payload)
      toasts.success('Factura registrada')
    }
    modalOpen.value = false
    const invoiceYear = Number(form.value.invoice_date.slice(0, 4))
    if (invoiceYear && years.includes(invoiceYear)) year.value = invoiceYear
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo guardar la factura')
  } finally {
    saving.value = false
  }
}

async function remove(invoice: Invoice) {
  confirmInvoice.value = invoice
}

async function confirmDelete() {
  if (!confirmInvoice.value) return
  deleting.value = true
  try {
    await api.del(`/invoices/${confirmInvoice.value.id}`)
    toasts.success('Factura eliminada')
    confirmInvoice.value = null
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo eliminar')
  } finally {
    deleting.value = false
  }
}

function onDeleteDialogUpdate(open: boolean) {
  if (!open && !deleting.value) confirmInvoice.value = null
}

function handleNewQuery() {
  if (route.query.nueva !== '1') return
  openCreate()
  const nextQuery = { ...route.query }
  delete nextQuery.nueva
  router.replace({ path: route.path, query: nextQuery })
}

watch([() => store.selectedId, year], load)
watch(() => route.query.nueva, handleNewQuery)
onMounted(async () => {
  await load()
  handleNewQuery()
})
</script>

<template>
  <div>
    <PageHeader
      title="Facturas"
      subtitle="Libro de facturas emitidas y recibidas. Puedes editar, duplicar y adjuntar el documento original."
    >
      <template #actions>
        <button class="btn-accent" @click="openCreate"><Plus class="h-4 w-4" /> Nueva factura</button>
      </template>
    </PageHeader>

    <div class="mb-5 grid gap-3 sm:grid-cols-3">
      <div class="card px-4 py-3">
        <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Base emitida</p>
        <p class="mt-1 text-xl font-bold text-slate-900">{{ euros(totals.issued) }}</p>
      </div>
      <div class="card px-4 py-3">
        <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Base recibida</p>
        <p class="mt-1 text-xl font-bold text-slate-900">{{ euros(totals.received) }}</p>
      </div>
      <div class="card px-4 py-3">
        <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">IVA neto del ejercicio</p>
        <p class="mt-1 text-xl font-bold" :class="totals.vat >= 0 ? 'text-slate-900' : 'text-emerald-700'">
          {{ euros(totals.vat) }}
        </p>
      </div>
    </div>

    <div class="mb-3 flex flex-col gap-2 lg:flex-row">
      <div class="relative flex-1">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          v-model="search"
          class="input pl-9"
          type="search"
          placeholder="Buscar por número, cliente o concepto…"
          aria-label="Buscar facturas"
        />
      </div>
      <select v-model="direction" class="input lg:w-44" aria-label="Tipo de factura">
        <option value="all">Todas</option>
        <option value="issued">Emitidas</option>
        <option value="received">Recibidas</option>
      </select>
      <select v-model="sort" class="input lg:w-52" aria-label="Ordenar facturas">
        <option value="date_desc">Fecha más reciente</option>
        <option value="date_asc">Fecha más antigua</option>
        <option value="amount_desc">Importe mayor</option>
        <option value="amount_asc">Importe menor</option>
      </select>
      <select v-model.number="year" class="input lg:w-32" aria-label="Ejercicio">
        <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
      </select>
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="h-16 animate-pulse rounded-2xl bg-white" />
    </div>

    <EmptyState
      v-else-if="!invoices.length"
      title="Sin facturas registradas"
      description="Registra las facturas emitidas y recibidas del periodo. El cálculo de 303 y 130 se basa en estos datos."
    >
      <template #icon><ReceiptText class="h-6 w-6" /></template>
      <template #action>
        <button class="btn-accent" @click="openCreate"><Plus class="h-4 w-4" /> Registrar factura</button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="!filtered.length"
      title="Sin resultados con estos filtros"
      description="Cambia la búsqueda, el tipo de factura o el ejercicio para localizar los registros."
    >
      <template #icon><Search class="h-6 w-6" /></template>
      <template #action>
        <button class="btn-ghost" @click="clearFilters">Limpiar filtros</button>
      </template>
    </EmptyState>

    <div v-else class="card overflow-hidden">
      <p class="px-4 pt-3 text-[11px] text-slate-500 sm:hidden">Desliza lateralmente para ver todas las columnas.</p>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>Fecha</th>
              <th>Número</th>
              <th>Contraparte</th>
              <th>Tipo</th>
              <th class="text-right">Base</th>
              <th class="text-right">IVA</th>
              <th class="text-right">Retención</th>
              <th class="text-right">Total</th>
              <th><span class="sr-only">Acciones</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="invoice in sortedFiltered" :key="invoice.id">
              <td class="whitespace-nowrap text-xs text-slate-500">{{ dateLabel(invoice.invoice_date) }}</td>
              <td class="font-mono text-xs text-slate-700">
                <span class="inline-flex items-center gap-1">
                  {{ invoice.number }}
                  <Paperclip
                    v-if="invoice.document_id"
                    class="h-3.5 w-3.5 text-slate-400"
                    :title="documentLabel(invoice.document_id)"
                  />
                </span>
              </td>
              <td>
                <p class="text-sm font-semibold text-slate-800">{{ invoice.counterparty_name || '—' }}</p>
                <p class="text-[11px] text-slate-400">{{ invoice.counterparty_nif || 'Sin NIF' }}</p>
              </td>
              <td>
                <span
                  class="chip"
                  :class="
                    invoice.direction === 'issued'
                      ? 'bg-emerald-50 text-emerald-700'
                      : 'bg-sky-50 text-sky-700'
                  "
                >
                  {{ invoice.direction === 'issued' ? 'Emitida' : 'Recibida' }}
                </span>
              </td>
              <td class="text-right font-mono text-xs tabular-nums">{{ euros(invoice.base_cents) }}</td>
              <td class="text-right font-mono text-xs tabular-nums">{{ euros(invoice.vat_cents) }}</td>
              <td class="text-right font-mono text-xs tabular-nums">
                {{ invoice.retention_cents ? euros(invoice.retention_cents) : '—' }}
              </td>
              <td class="text-right font-mono text-xs font-semibold tabular-nums">{{ euros(invoice.total_cents) }}</td>
              <td>
                <div class="flex justify-end gap-1">
                  <button
                    class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 hover:text-slate-800"
                    :aria-label="`Duplicar factura ${invoice.number}`"
                    title="Duplicar"
                    @click="duplicate(invoice)"
                  >
                    <Copy class="h-4 w-4" />
                  </button>
                  <button
                    class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 hover:text-slate-800"
                    :aria-label="`Editar factura ${invoice.number}`"
                    title="Editar"
                    @click="openEdit(invoice)"
                  >
                    <Pencil class="h-4 w-4" />
                  </button>
                  <button
                    class="rounded-lg p-1.5 text-slate-500 hover:bg-rose-50 hover:text-rose-600"
                    :aria-label="`Eliminar factura ${invoice.number}`"
                    title="Eliminar"
                    @click="remove(invoice)"
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

    <Modal
      v-model="modalOpen"
      :title="editingId ? 'Editar factura' : 'Nueva factura'"
      subtitle="Los importes se calculan automáticamente a partir de la base y los tipos."
      size="lg"
      persistent
    >
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-2">
          <button
            type="button"
            class="rounded-xl border px-4 py-3 text-sm font-semibold transition"
            :class="
              form.direction === 'received'
                ? 'border-slate-900 bg-slate-900 text-white'
                : 'border-slate-200 bg-white text-slate-600'
            "
            @click="form.direction = 'received'"
          >
            Factura recibida
          </button>
          <button
            type="button"
            class="rounded-xl border px-4 py-3 text-sm font-semibold transition"
            :class="
              form.direction === 'issued'
                ? 'border-slate-900 bg-slate-900 text-white'
                : 'border-slate-200 bg-white text-slate-600'
            "
            @click="form.direction = 'issued'"
          >
            Factura emitida
          </button>
        </div>

        <p v-if="errors.taxpayer" class="text-sm font-medium text-rose-600">{{ errors.taxpayer }}</p>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label" for="invoice-number">Número</label>
            <input id="invoice-number" v-model="form.number" class="input" placeholder="F-2026-001" />
          </div>
          <div>
            <label class="label" for="invoice-date">
              Fecha <span class="text-rose-500">*</span>
            </label>
            <input
              id="invoice-date"
              v-model="form.invoice_date"
              type="date"
              class="input"
              :class="errors.invoice_date ? 'border-rose-300' : ''"
            />
            <p v-if="errors.invoice_date" class="mt-1 text-xs text-rose-600">{{ errors.invoice_date }}</p>
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label" for="invoice-nif">NIF contraparte</label>
            <input
              id="invoice-nif"
              v-model="form.counterparty_nif"
              class="input"
              :class="errors.counterparty_nif ? 'border-rose-300' : ''"
              placeholder="B12345678"
            />
            <p v-if="errors.counterparty_nif" class="mt-1 text-xs text-rose-600">{{ errors.counterparty_nif }}</p>
          </div>
          <div>
            <label class="label" for="invoice-name">Nombre contraparte</label>
            <input id="invoice-name" v-model="form.counterparty_name" class="input" placeholder="Empresa o cliente" />
          </div>
        </div>

        <div>
          <label class="label" for="invoice-concept">Concepto</label>
          <input id="invoice-concept" v-model="form.concept" class="input" placeholder="Servicios de consultoría" />
        </div>

        <div class="grid gap-3 sm:grid-cols-3">
          <div>
            <label class="label" for="invoice-base">
              Base imponible (€) <span class="text-rose-500">*</span>
            </label>
            <input
              id="invoice-base"
              v-model="form.base"
              class="input"
              :class="errors.base ? 'border-rose-300' : ''"
              placeholder="1.000,00"
              inputmode="decimal"
            />
            <p v-if="errors.base" class="mt-1 text-xs text-rose-600">{{ errors.base }}</p>
          </div>
          <div>
            <label class="label" for="invoice-vat">IVA</label>
            <select id="invoice-vat" v-model.number="form.vat_rate" class="input">
              <option :value="21">21 %</option>
              <option :value="10">10 %</option>
              <option :value="4">4 %</option>
              <option :value="0">0 %</option>
            </select>
          </div>
          <div>
            <label class="label" for="invoice-retention">Retención IRPF</label>
            <select id="invoice-retention" v-model.number="form.retention_rate" class="input">
              <option :value="0">Sin retención</option>
              <option :value="15">15 %</option>
              <option :value="7">7 %</option>
              <option :value="19">19 %</option>
            </select>
          </div>
        </div>

        <div v-if="form.direction === 'received'">
          <label class="label" for="invoice-category">Categoría del gasto</label>
          <select id="invoice-category" v-model="form.category" class="input">
            <option value="">Gasto corriente</option>
            <option value="inversion">Bien de inversión</option>
            <option value="importacion">Importación</option>
          </select>
        </div>

        <div>
          <label class="label" for="invoice-document">Documento adjunto</label>
          <select id="invoice-document" v-model="form.document_id" class="input">
            <option value="">Sin documento</option>
            <option v-for="document in documents" :key="document.id" :value="document.id">
              {{ document.filename }}
            </option>
          </select>
          <p class="mt-1 text-xs text-slate-500">
            Si aún no lo has subido, puedes hacerlo desde
            <RouterLink to="/documentos" class="font-semibold text-slate-700 underline-offset-2 hover:underline">
              Documentos
            </RouterLink>
            y volver a editarla.
          </p>
        </div>

        <div class="rounded-xl bg-slate-50 p-4">
          <div class="grid grid-cols-2 gap-2 text-sm sm:grid-cols-4">
            <div>
              <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Base</p>
              <p class="mt-1 font-mono font-semibold">{{ euros(preview.base) }}</p>
            </div>
            <div>
              <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">IVA</p>
              <p class="mt-1 font-mono font-semibold">{{ euros(preview.vat) }}</p>
            </div>
            <div>
              <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Retención</p>
              <p class="mt-1 font-mono font-semibold">{{ euros(preview.retention) }}</p>
            </div>
            <div>
              <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Total</p>
              <p class="mt-1 font-mono font-bold">{{ euros(preview.total) }}</p>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <button class="btn-ghost" :disabled="saving" @click="modalOpen = false">Cancelar</button>
        <button class="btn-accent" :disabled="saving" @click="save">
          <Plus class="h-4 w-4" />
          {{ saving ? 'Guardando…' : editingId ? 'Guardar cambios' : 'Registrar factura' }}
        </button>
      </template>
    </Modal>

    <ConfirmDialog
      :model-value="!!confirmInvoice"
      title="Eliminar factura"
      :message="`Se eliminará la factura ${confirmInvoice?.number ?? ''}. El documento adjunto seguirá en la biblioteca.`"
      confirm-label="Eliminar factura"
      tone="danger"
      :busy="deleting"
      @update:model-value="onDeleteDialogUpdate"
      @confirm="confirmDelete"
    />
  </div>
</template>
