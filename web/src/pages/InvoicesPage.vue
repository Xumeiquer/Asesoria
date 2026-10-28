<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Plus, ReceiptText, Search, Trash2 } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { api, centsFromEuros, dateLabel, euros } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { Invoice } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const invoices = ref<Invoice[]>([])
const loading = ref(true)
const saving = ref(false)
const modalOpen = ref(false)
const direction = ref<'all' | 'issued' | 'received'>('all')
const currentYear = new Date().getFullYear()
const years = Array.from({ length: 5 }, (_, index) => currentYear - 3 + index)
const year = ref(currentYear)
const search = ref('')

const form = ref({
  taxpayer_id: '',
  direction: 'received' as 'issued' | 'received',
  number: '',
  invoice_date: new Date().toISOString().slice(0, 10),
  counterparty_nif: '',
  counterparty_name: '',
  concept: '',
  base: '',
  vat_rate: 21,
  retention_rate: 0,
  category: '',
})

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

const totals = computed(() => {
  const issued = invoices.value.filter((i) => i.direction === 'issued')
  const received = invoices.value.filter((i) => i.direction === 'received')
  return {
    issued: issued.reduce((sum, i) => sum + i.base_cents, 0),
    received: received.reduce((sum, i) => sum + i.base_cents, 0),
    vat: issued.reduce((sum, i) => sum + i.vat_cents, 0) - received.reduce((sum, i) => sum + i.vat_cents, 0),
  }
})

const preview = computed(() => {
  const base = centsFromEuros(form.value.base)
  const vat = Math.round((base * form.value.vat_rate) / 100)
  const retention = Math.round((base * form.value.retention_rate) / 100)
  return { base, vat, retention, total: base + vat - retention }
})

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ year: String(year.value) })
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    invoices.value = await api.get<Invoice[]>(`/invoices?${params}`)
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudieron cargar las facturas')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = {
    taxpayer_id: store.selectedId,
    direction: 'received',
    number: '',
    invoice_date: new Date().toISOString().slice(0, 10),
    counterparty_nif: '',
    counterparty_name: '',
    concept: '',
    base: '',
    vat_rate: 21,
    retention_rate: 0,
    category: '',
  }
  modalOpen.value = true
}

async function save() {
  if (!store.selectedId) {
    toasts.error('Selecciona un contribuyente')
    return
  }
  if (!form.value.base) {
    toasts.error('Introduce la base imponible')
    return
  }
  saving.value = true
  try {
    await api.post<Invoice>('/invoices', {
      taxpayer_id: store.selectedId,
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
    })
    toasts.success('Factura registrada')
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
  if (!window.confirm(`¿Eliminar la factura ${invoice.number}?`)) return
  try {
    await api.del(`/invoices/${invoice.id}`)
    toasts.success('Factura eliminada')
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo eliminar')
  }
}

watch([() => store.selectedId, year], load)
onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Facturas"
      subtitle="Libro de facturas emitidas y recibidas. Los importes se guardan en céntimos y alimentan los cálculos de IVA e IRPF."
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
        <p class="mt-1 text-xl font-bold" :class="totals.vat >= 0 ? 'text-slate-900' : 'text-emerald-700'">{{ euros(totals.vat) }}</p>
      </div>
    </div>

    <div class="mb-3 flex flex-col gap-2 sm:flex-row">
      <div class="relative flex-1">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input v-model="search" class="input pl-9" placeholder="Buscar por número, cliente o concepto…" />
      </div>
      <select v-model="direction" class="input sm:w-44">
        <option value="all">Todas</option>
        <option value="issued">Emitidas</option>
        <option value="received">Recibidas</option>
      </select>
      <select v-model.number="year" class="input sm:w-32">
        <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
      </select>
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="h-16 animate-pulse rounded-2xl bg-white" />
    </div>

    <EmptyState
      v-else-if="!filtered.length"
      title="Sin facturas registradas"
      description="Registra las facturas emitidas y recibidas del periodo. El cálculo de 303 y 130 se basa en estos datos."
    >
      <template #icon><ReceiptText class="h-6 w-6" /></template>
      <template #action>
        <button class="btn-accent" @click="openCreate"><Plus class="h-4 w-4" /> Registrar factura</button>
      </template>
    </EmptyState>

    <div v-else class="card overflow-hidden">
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
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="invoice in filtered" :key="invoice.id">
              <td class="whitespace-nowrap text-xs text-slate-500">{{ dateLabel(invoice.invoice_date) }}</td>
              <td class="font-mono text-xs text-slate-700">{{ invoice.number }}</td>
              <td>
                <p class="text-sm font-semibold text-slate-800">{{ invoice.counterparty_name }}</p>
                <p class="text-[11px] text-slate-400">{{ invoice.counterparty_nif }}</p>
              </td>
              <td>
                <span class="chip" :class="invoice.direction === 'issued' ? 'bg-emerald-50 text-emerald-700' : 'bg-sky-50 text-sky-700'">
                  {{ invoice.direction === 'issued' ? 'Emitida' : 'Recibida' }}
                </span>
              </td>
              <td class="text-right font-mono text-xs tabular-nums">{{ euros(invoice.base_cents) }}</td>
              <td class="text-right font-mono text-xs tabular-nums">{{ euros(invoice.vat_cents) }}</td>
              <td class="text-right font-mono text-xs tabular-nums">{{ invoice.retention_cents ? euros(invoice.retention_cents) : '—' }}</td>
              <td class="text-right font-mono text-xs font-semibold tabular-nums">{{ euros(invoice.total_cents) }}</td>
              <td>
                <button class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600" @click="remove(invoice)">
                  <Trash2 class="h-4 w-4" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <Modal v-model="modalOpen" title="Nueva factura" subtitle="Los importes se calculan automáticamente a partir de la base y los tipos." size="lg">
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-2">
          <button
            class="rounded-xl border px-4 py-3 text-sm font-semibold transition"
            :class="form.direction === 'received' ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
            @click="form.direction = 'received'"
          >
            Factura recibida
          </button>
          <button
            class="rounded-xl border px-4 py-3 text-sm font-semibold transition"
            :class="form.direction === 'issued' ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
            @click="form.direction = 'issued'"
          >
            Factura emitida
          </button>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">Número</label>
            <input v-model="form.number" class="input" placeholder="F-2026-001" />
          </div>
          <div>
            <label class="label">Fecha</label>
            <input v-model="form.invoice_date" type="date" class="input" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">NIF contraparte</label>
            <input v-model="form.counterparty_nif" class="input" placeholder="B12345678" />
          </div>
          <div>
            <label class="label">Nombre contraparte</label>
            <input v-model="form.counterparty_name" class="input" placeholder="Empresa o cliente" />
          </div>
        </div>

        <div>
          <label class="label">Concepto</label>
          <input v-model="form.concept" class="input" placeholder="Servicios de consultoría" />
        </div>

        <div class="grid gap-3 sm:grid-cols-3">
          <div>
            <label class="label">Base imponible (€)</label>
            <input v-model="form.base" class="input" placeholder="1.000,00" inputmode="decimal" />
          </div>
          <div>
            <label class="label">IVA</label>
            <select v-model.number="form.vat_rate" class="input">
              <option :value="21">21 %</option>
              <option :value="10">10 %</option>
              <option :value="4">4 %</option>
              <option :value="0">0 %</option>
            </select>
          </div>
          <div>
            <label class="label">Retención IRPF</label>
            <select v-model.number="form.retention_rate" class="input">
              <option :value="0">Sin retención</option>
              <option :value="15">15 %</option>
              <option :value="7">7 %</option>
              <option :value="19">19 %</option>
            </select>
          </div>
        </div>

        <div v-if="form.direction === 'received'">
          <label class="label">Categoría del gasto</label>
          <select v-model="form.category" class="input">
            <option value="">Gasto corriente</option>
            <option value="inversion">Bien de inversión</option>
            <option value="importacion">Importación</option>
          </select>
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
        <button class="btn-ghost" @click="modalOpen = false">Cancelar</button>
        <button class="btn-accent" :disabled="saving" @click="save">
          <Plus class="h-4 w-4" /> {{ saving ? 'Guardando…' : 'Registrar factura' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
