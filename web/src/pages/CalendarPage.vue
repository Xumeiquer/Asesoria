<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  AlertTriangle,
  Ban,
  CalendarDays,
  Check,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  RotateCcw,
  Search,
  X,
} from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import FilingItem from '@/components/FilingItem.vue'
import EmptyState from '@/components/EmptyState.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { api, dateLabel } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { Filing, FilingStatus } from '@/types'

type Range = 'open' | 'overdue' | 'next30' | 'next45' | 'next90' | 'all'

const store = useAppStore()
const toasts = useToasts()
const route = useRoute()

const year = ref(new Date().getFullYear())
const status = ref<'' | FilingStatus>('')
const range = ref<Range>('open')
const entity = ref('')
const modelCode = ref('')
const query = ref('')
const filings = ref<Filing[]>([])
const loading = ref(true)
const error = ref('')
const generating = ref(false)
const selectedIds = ref<Set<string>>(new Set())
const bulkBusy = ref(false)
const bulkConfirm = ref<{
  status: FilingStatus
  title: string
  message: string
  confirmLabel: string
  tone: 'danger' | 'warning' | 'primary'
} | null>(null)

const statusOptions: Array<{ value: '' | FilingStatus; label: string }> = [
  { value: '', label: 'Todos los estados' },
  { value: 'pending', label: 'Pendientes' },
  { value: 'in_progress', label: 'En curso' },
  { value: 'ready', label: 'Listos para presentar' },
  { value: 'presented', label: 'Presentados' },
  { value: 'not_applicable', label: 'No aplica' },
]

const rangeOptions: Array<{ value: Range; label: string }> = [
  { value: 'open', label: 'Cola abierta' },
  { value: 'overdue', label: 'Solo vencidos' },
  { value: 'next30', label: 'Próximos 30 días' },
  { value: 'next45', label: 'Próximos 45 días' },
  { value: 'next90', label: 'Próximos 90 días' },
  { value: 'all', label: 'Todo el ejercicio' },
]

const openStatuses: FilingStatus[] = ['pending', 'in_progress', 'ready']

function localISO(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

function isoToday(): string {
  return localISO(new Date())
}

function shiftDays(days: number): string {
  const date = new Date()
  date.setDate(date.getDate() + days)
  return localISO(date)
}

function isOpenStatus(filing: Filing): boolean {
  return openStatuses.includes(filing.status)
}

function isOverdue(filing: Filing): boolean {
  return isOpenStatus(filing) && filing.due_date < isoToday()
}

function inRange(filing: Filing): boolean {
  const today = isoToday()
  switch (range.value) {
    case 'open':
      return isOpenStatus(filing)
    case 'overdue':
      return isOverdue(filing)
    case 'next30':
      return isOpenStatus(filing) && filing.due_date >= today && filing.due_date <= shiftDays(30)
    case 'next45':
      return isOpenStatus(filing) && filing.due_date >= today && filing.due_date <= shiftDays(45)
    case 'next90':
      return isOpenStatus(filing) && filing.due_date >= today && filing.due_date <= shiftDays(90)
    case 'all':
      return true
  }
}

function initialRange(): Range {
  const raw = String(route.query.range ?? '')
  return rangeOptions.some((option) => option.value === raw) ? (raw as Range) : 'open'
}

function initialStatus(): '' | FilingStatus {
  const raw = String(route.query.status ?? '')
  return statusOptions.some((option) => option.value === raw) ? (raw as FilingStatus) : ''
}

range.value = initialRange()
status.value = initialStatus()

const stats = computed(() => {
  const today = isoToday()
  const open = filings.value.filter(isOpenStatus)
  return {
    pending: open.filter((filing) => filing.due_date >= today).length,
    overdue: open.filter((filing) => filing.due_date < today).length,
    presented: filings.value.filter((filing) => filing.status === 'presented').length,
    presentedWithoutReceipt: filings.value.filter(
      (filing) => filing.status === 'presented' && !filing.receipt_document_id,
    ).length,
  }
})

const entityOptions = computed(() =>
  [...new Set(filings.value.map((filing) => filing.entity))].filter(Boolean).sort(),
)

const modelOptions = computed(() => {
  const seen = new Map<string, string>()
  for (const filing of filings.value) {
    if (!seen.has(filing.model_code)) seen.set(filing.model_code, filing.model_name)
  }
  return [...seen.entries()].sort(([a], [b]) => a.localeCompare(b, 'es', { numeric: true }))
})

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return filings.value
    .filter((filing) => {
      if (!inRange(filing)) return false
      if (status.value && filing.status !== status.value) return false
      if (entity.value && filing.entity !== entity.value) return false
      if (modelCode.value && filing.model_code !== modelCode.value) return false
      if (needle) {
        const haystack = `${filing.model_code} ${filing.model_name} ${filing.period_label} ${filing.entity}`.toLowerCase()
        if (!haystack.includes(needle)) return false
      }
      return true
    })
    .sort((a, b) => b.due_date.localeCompare(a.due_date) || a.model_code.localeCompare(b.model_code))
})

const months = computed(() => {
  const groups = new Map<string, Filing[]>()
  for (const filing of filtered.value) {
    const key = filing.due_date.slice(0, 7)
    const list = groups.get(key) ?? []
    list.push(filing)
    groups.set(key, list)
  }
  return [...groups.entries()].map(([key, items]) => {
    const [y, m] = key.split('-').map(Number)
    return {
      key,
      label: new Intl.DateTimeFormat('es-ES', { month: 'long' }).format(new Date(y, m - 1, 1)),
      items,
      overdue: items.filter(isOverdue).length,
    }
  })
})

const hasFilters = computed(
  () =>
    Boolean(status.value || entity.value || modelCode.value || query.value) || range.value !== 'open',
)

const selectedCount = computed(() => selectedIds.value.size)
const allVisibleSelected = computed(
  () => filtered.value.length > 0 && filtered.value.every((filing) => selectedIds.value.has(filing.id)),
)

function resetFilters() {
  status.value = ''
  range.value = 'open'
  entity.value = ''
  modelCode.value = ''
  query.value = ''
}

function showOpen() {
  status.value = ''
  range.value = 'open'
}

function showOverdue() {
  status.value = ''
  range.value = 'overdue'
}

function showPresented() {
  status.value = 'presented'
  range.value = 'all'
}

function toggleSelection(id: string) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

function clearSelection() {
  selectedIds.value = new Set()
}

function toggleVisible() {
  if (allVisibleSelected.value) {
    const visible = new Set(filtered.value.map((filing) => filing.id))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => !visible.has(id)))
    return
  }
  const next = new Set(selectedIds.value)
  for (const filing of filtered.value) next.add(filing.id)
  selectedIds.value = next
}

async function load() {
  loading.value = true
  error.value = ''
  selectedIds.value = new Set()
  try {
    const params = new URLSearchParams({ year: String(year.value) })
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    const data = await api.get<{ year: number; filings: Filing[] }>(`/calendar?${params}`)
    filings.value = data.filings
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'No se pudo cargar el calendario'
    toasts.error(error.value)
  } finally {
    loading.value = false
  }
}

async function generate() {
  generating.value = true
  try {
    const data = await api.post<{ inserted: number; taxpayers: number }>('/calendar/generate', {
      year: year.value,
      taxpayer_id: store.selectedId || undefined,
    })
    if (data.inserted > 0) {
      toasts.success(`Calendario actualizado: ${data.inserted} nuevos vencimientos`)
    } else {
      toasts.push('No había vencimientos nuevos que añadir')
    }
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'No se pudo generar el calendario')
  } finally {
    generating.value = false
  }
}

function requestBulk(nextStatus: FilingStatus) {
  const count = selectedCount.value
  if (!count) return
  if (nextStatus === 'presented') {
    bulkConfirm.value = {
      status: nextStatus,
      title: 'Marcar como presentados',
      message: `Se marcarán ${count} vencimientos como presentados sin justificante. Podrás adjuntar el justificante después desde cada expediente.`,
      confirmLabel: 'Marcar presentados',
      tone: 'warning',
    }
    return
  }
  if (nextStatus === 'not_applicable') {
    bulkConfirm.value = {
      status: nextStatus,
      title: 'Marcar como no aplicable',
      message: `Se retirarán ${count} vencimientos de la cola de trabajo. Seguirán disponibles en el calendario con el estado «No aplica».`,
      confirmLabel: 'Marcar no aplica',
      tone: 'warning',
    }
    return
  }
  bulkConfirm.value = {
    status: nextStatus,
    title: 'Reabrir vencimientos',
    message:
      count === 1
        ? 'El vencimiento seleccionado volverá a la cola como pendiente.'
        : `Los ${count} vencimientos seleccionados volverán a la cola como pendientes.`,
    confirmLabel: 'Reabrir',
    tone: 'primary',
  }
}

function closeBulkDialog() {
  if (!bulkBusy.value) bulkConfirm.value = null
}

function onBulkDialogUpdate(open: boolean) {
  if (!open) closeBulkDialog()
}

async function applyBulk() {
  if (!bulkConfirm.value) return
  bulkBusy.value = true
  const nextStatus = bulkConfirm.value.status
  try {
    const result = await api.post<{ updated: number }>('/filings/bulk-status', {
      ids: [...selectedIds.value],
      status: nextStatus,
    })
    toasts.success(`${result.updated} vencimientos actualizados`)
    bulkConfirm.value = null
    await load()
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'No se pudo actualizar la selección')
  } finally {
    bulkBusy.value = false
  }
}

watch([() => store.selectedId, year], load)
watch(
  () => [route.query.range, route.query.status],
  () => {
    range.value = initialRange()
    status.value = initialStatus()
  },
)
onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Calendario fiscal"
      subtitle="Una bandeja de trabajo para cerrar los modelos y trámites del ejercicio. Los plazos que caen en fin de semana se trasladan al lunes."
    >
      <template #actions>
        <div class="flex items-center rounded-xl border border-slate-200 bg-white">
          <button class="rounded-l-xl p-2 text-slate-500 hover:bg-slate-50" aria-label="Año anterior" @click="year--">
            <ChevronLeft class="h-4 w-4" />
          </button>
          <span class="min-w-[4.5rem] text-center text-sm font-bold text-slate-800">{{ year }}</span>
          <button class="rounded-r-xl p-2 text-slate-500 hover:bg-slate-50" aria-label="Año siguiente" @click="year++">
            <ChevronRight class="h-4 w-4" />
          </button>
        </div>
        <button class="btn-accent" :disabled="generating" @click="generate">
          <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': generating }" />
          {{ filings.length ? 'Actualizar calendario' : 'Generar calendario' }}
        </button>
      </template>
    </PageHeader>

    <div class="mb-5 grid gap-3 sm:grid-cols-3">
      <button
        type="button"
        class="card flex items-center justify-between px-4 py-3 text-left transition hover:border-slate-300 hover:shadow-sm"
        :class="range === 'open' && !status ? 'ring-2 ring-sky-500/30' : ''"
        @click="showOpen"
      >
        <span>
          <span class="block text-xs font-bold uppercase tracking-wider text-slate-500">En plazo</span>
          <span class="mt-0.5 block text-[11px] text-slate-400">Pendientes de presentar</span>
        </span>
        <span class="text-xl font-bold text-slate-900">{{ stats.pending }}</span>
      </button>
      <button
        type="button"
        class="card flex items-center justify-between px-4 py-3 text-left transition hover:border-slate-300 hover:shadow-sm"
        :class="range === 'overdue' ? 'ring-2 ring-rose-500/30' : ''"
        @click="showOverdue"
      >
        <span>
          <span class="block text-xs font-bold uppercase tracking-wider text-slate-500">Vencidos</span>
          <span class="mt-0.5 block text-[11px] text-slate-400">Fuera de plazo</span>
        </span>
        <span class="text-xl font-bold" :class="stats.overdue ? 'text-rose-600' : 'text-slate-900'">
          {{ stats.overdue }}
        </span>
      </button>
      <button
        type="button"
        class="card flex items-center justify-between px-4 py-3 text-left transition hover:border-slate-300 hover:shadow-sm"
        :class="status === 'presented' ? 'ring-2 ring-emerald-500/30' : ''"
        @click="showPresented"
      >
        <span>
          <span class="block text-xs font-bold uppercase tracking-wider text-slate-500">Presentados</span>
          <span class="mt-0.5 block text-[11px] text-slate-400">
            {{ stats.presentedWithoutReceipt }} sin justificante
          </span>
        </span>
        <span class="text-xl font-bold text-emerald-600">{{ stats.presented }}</span>
      </button>
    </div>

    <div class="card card-pad mb-4">
      <div class="grid gap-3 lg:grid-cols-[minmax(14rem,1.4fr)_repeat(4,minmax(9rem,1fr))_auto]">
        <label class="relative block">
          <span class="sr-only">Buscar vencimiento</span>
          <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input v-model="query" class="input pl-9" type="search" placeholder="Buscar modelo, periodo…" />
        </label>
        <label>
          <span class="sr-only">Rango de vencimiento</span>
          <select v-model="range" class="input w-full">
            <option v-for="option in rangeOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <label>
          <span class="sr-only">Estado</span>
          <select v-model="status" class="input w-full">
            <option v-for="option in statusOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <label>
          <span class="sr-only">Entidad</span>
          <select v-model="entity" class="input w-full">
            <option value="">Todas las entidades</option>
            <option v-for="item in entityOptions" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>
        <label>
          <span class="sr-only">Modelo</span>
          <select v-model="modelCode" class="input w-full">
            <option value="">Todos los modelos</option>
            <option v-for="[code, name] in modelOptions" :key="code" :value="code">
              {{ code }} · {{ name }}
            </option>
          </select>
        </label>
        <button v-if="hasFilters" class="btn-ghost" @click="resetFilters">
          <X class="h-4 w-4" />
          Limpiar
        </button>
      </div>
      <div class="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-3">
        <p class="text-xs text-slate-500">
          {{ filtered.length }} de {{ filings.length }} vencimientos ·
          {{ store.selectedTaxpayer?.name ?? 'todos los contribuyentes' }}
        </p>
        <label class="inline-flex items-center gap-2 text-xs font-semibold text-slate-600">
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-slate-500"
            :checked="allVisibleSelected"
            :disabled="!filtered.length"
            @change="toggleVisible"
          />
          Seleccionar visibles
        </label>
      </div>
    </div>

    <div
      v-if="selectedCount"
      class="mb-4 flex flex-wrap items-center gap-2 rounded-2xl bg-slate-900 px-4 py-3 text-white shadow-lg"
    >
      <p class="mr-auto text-sm font-semibold">
        {{ selectedCount }} seleccionado{{ selectedCount === 1 ? '' : 's' }}
      </p>
      <button class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500 px-3 py-1.5 text-xs font-bold text-slate-950 hover:bg-emerald-400" @click="requestBulk('presented')">
        <Check class="h-3.5 w-3.5" /> Marcar presentado
      </button>
      <button class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-1.5 text-xs font-bold text-white hover:bg-white/20" @click="requestBulk('not_applicable')">
        <Ban class="h-3.5 w-3.5" /> No aplica
      </button>
      <button class="inline-flex items-center gap-1.5 rounded-lg bg-white/10 px-3 py-1.5 text-xs font-bold text-white hover:bg-white/20" @click="requestBulk('pending')">
        <RotateCcw class="h-3.5 w-3.5" /> Reabrir
      </button>
      <button class="rounded-lg p-1.5 text-slate-300 hover:bg-white/10 hover:text-white" aria-label="Limpiar selección" @click="clearSelection">
        <X class="h-4 w-4" />
      </button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 6" :key="i" class="h-[74px] animate-pulse rounded-2xl bg-white" />
    </div>

    <div v-else-if="error" class="card card-pad text-center">
      <p class="text-sm font-semibold text-slate-900">No se pudo cargar el calendario</p>
      <p class="mt-1 text-sm text-slate-500">{{ error }}</p>
      <button class="btn-ghost mt-4" @click="load">Reintentar</button>
    </div>

    <EmptyState
      v-else-if="!filings.length"
      title="No hay vencimientos en este ejercicio"
      description="Genera el calendario para crear automáticamente los modelos que corresponden al perfil fiscal del contribuyente."
    >
      <template #icon><CalendarDays class="h-6 w-6" /></template>
      <template #action>
        <button class="btn-accent" :disabled="generating" @click="generate">
          <RefreshCw class="h-4 w-4" /> Generar calendario {{ year }}
        </button>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="!filtered.length"
      title="Sin resultados con estos filtros"
      description="Prueba con otro rango de fechas, entidad o estado. También puedes limpiar los filtros para ver toda la cola."
    >
      <template #icon><AlertTriangle class="h-6 w-6" /></template>
      <template #action>
        <button class="btn-ghost" @click="resetFilters">Limpiar filtros</button>
      </template>
    </EmptyState>

    <div v-else class="space-y-8">
      <section v-for="month in months" :key="month.key" class="grid grid-cols-1 gap-3 md:grid-cols-[7rem_1fr]">
        <div class="pt-2">
          <div class="flex flex-wrap items-center gap-1.5">
            <p class="text-sm font-bold capitalize text-slate-900">{{ month.label }}</p>
            <span v-if="month.overdue" class="chip bg-rose-50 text-rose-700">{{ month.overdue }} vencidos</span>
          </div>
          <p class="text-xs text-slate-400">{{ month.items.length }} trámites</p>
          <p class="mt-2 hidden text-[11px] font-medium uppercase tracking-wider text-slate-400 md:block">
            {{ dateLabel(month.key + '-01') }}
          </p>
        </div>
        <div class="min-w-0 space-y-2">
          <div v-for="filing in month.items" :key="filing.id" class="flex items-start gap-2">
            <input
              type="checkbox"
              class="mt-4 h-4 w-4 shrink-0 rounded border-slate-300 text-slate-900 focus:ring-slate-500"
              :checked="selectedIds.has(filing.id)"
              :aria-label="`Seleccionar ${filing.model_code} ${filing.period_label}`"
              @change="toggleSelection(filing.id)"
            />
            <FilingItem :filing="filing" show-entity class="min-w-0 flex-1" />
          </div>
        </div>
      </section>
    </div>

    <ConfirmDialog
      :model-value="!!bulkConfirm"
      :title="bulkConfirm?.title ?? ''"
      :message="bulkConfirm?.message ?? ''"
      :confirm-label="bulkConfirm?.confirmLabel"
      :tone="bulkConfirm?.tone"
      :busy="bulkBusy"
      @update:model-value="onBulkDialogUpdate"
      @confirm="applyBulk"
    />
  </div>
</template>
