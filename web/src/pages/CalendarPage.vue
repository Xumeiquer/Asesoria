<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { CalendarDays, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import FilingItem from '@/components/FilingItem.vue'
import EmptyState from '@/components/EmptyState.vue'
import { api, dateLabel, daysUntil } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { Filing, FilingStatus } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const year = ref(new Date().getFullYear())
const status = ref<'' | FilingStatus>('')
const filings = ref<Filing[]>([])
const loading = ref(true)
const generating = ref(false)

const statusOptions: Array<{ value: '' | FilingStatus; label: string }> = [
  { value: '', label: 'Todos los estados' },
  { value: 'pending', label: 'Pendientes' },
  { value: 'in_progress', label: 'En curso' },
  { value: 'ready', label: 'Listos para presentar' },
  { value: 'presented', label: 'Presentados' },
  { value: 'late', label: 'Fuera de plazo' },
]

const months = computed(() => {
  const sorted = [...filings.value].sort((a, b) => a.due_date.localeCompare(b.due_date))
  const groups = new Map<string, Filing[]>()
  for (const filing of sorted) {
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
      short: new Intl.DateTimeFormat('es-ES', { month: 'short' }).format(new Date(y, m - 1, 1)),
      items,
    }
  })
})

const stats = computed(() => {
  const pending = filings.value.filter((f) => ['pending', 'in_progress', 'ready'].includes(f.status)).length
  const overdue = filings.value.filter(
    (f) => ['pending', 'in_progress', 'ready'].includes(f.status) && daysUntil(f.due_date) < 0,
  ).length
  const presented = filings.value.filter((f) => f.status === 'presented').length
  return { pending, overdue, presented }
})

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ year: String(year.value) })
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    if (status.value) params.set('status', status.value)
    const data = await api.get<{ year: number; filings: Filing[] }>(`/calendar?${params}`)
    filings.value = data.filings
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo cargar el calendario')
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
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo generar el calendario')
  } finally {
    generating.value = false
  }
}

watch([() => store.selectedId, year, status], load)
onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Calendario fiscal"
      subtitle="Todos los modelos y trámites ordenados por fecha de vencimiento. Los plazos que caen en fin de semana se trasladan al lunes."
    >
      <template #actions>
        <div class="flex items-center rounded-xl border border-slate-200 bg-white">
          <button class="rounded-l-xl p-2 text-slate-500 hover:bg-slate-50" @click="year--">
            <ChevronLeft class="h-4 w-4" />
          </button>
          <span class="min-w-[4.5rem] text-center text-sm font-bold text-slate-800">{{ year }}</span>
          <button class="rounded-r-xl p-2 text-slate-500 hover:bg-slate-50" @click="year++">
            <ChevronRight class="h-4 w-4" />
          </button>
        </div>
        <button class="btn-accent" :disabled="generating" @click="generate">
          <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': generating }" />
          Generar calendario
        </button>
      </template>
    </PageHeader>

    <div class="mb-5 grid gap-3 sm:grid-cols-3">
      <div class="card flex items-center justify-between px-4 py-3">
        <span class="text-xs font-bold uppercase tracking-wider text-slate-500">Pendientes</span>
        <span class="text-xl font-bold text-slate-900">{{ stats.pending }}</span>
      </div>
      <div class="card flex items-center justify-between px-4 py-3">
        <span class="text-xs font-bold uppercase tracking-wider text-slate-500">Vencidos</span>
        <span class="text-xl font-bold" :class="stats.overdue ? 'text-rose-600' : 'text-slate-900'">{{ stats.overdue }}</span>
      </div>
      <div class="card flex items-center justify-between px-4 py-3">
        <span class="text-xs font-bold uppercase tracking-wider text-slate-500">Presentados</span>
        <span class="text-xl font-bold text-emerald-600">{{ stats.presented }}</span>
      </div>
    </div>

    <div class="mb-4 flex flex-wrap items-center gap-3">
      <select v-model="status" class="input w-auto min-w-[13rem]">
        <option v-for="option in statusOptions" :key="option.value" :value="option.value">
          {{ option.label }}
        </option>
      </select>
      <p class="text-xs text-slate-500">
        {{ filings.length }} vencimientos ·
        {{ store.selectedTaxpayer?.name ?? 'todos los contribuyentes' }}
      </p>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 6" :key="i" class="h-[74px] animate-pulse rounded-2xl bg-white" />
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

    <div v-else class="space-y-8">
      <section v-for="month in months" :key="month.key" class="grid gap-3 md:grid-cols-[7rem_1fr]">
        <div class="pt-2">
          <p class="text-sm font-bold capitalize text-slate-900">{{ month.label }}</p>
          <p class="text-xs text-slate-400">{{ month.items.length }} trámites</p>
          <p class="mt-2 hidden text-[11px] font-medium uppercase tracking-wider text-slate-400 md:block">
            {{ dateLabel(month.key + '-01') }}
          </p>
        </div>
        <div class="space-y-2">
          <FilingItem v-for="filing in month.items" :key="filing.id" :filing="filing" show-entity />
        </div>
      </section>
    </div>
  </div>
</template>
