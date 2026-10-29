<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  AlertTriangle,
  ArrowRight,
  CalendarDays,
  CalendarClock,
  CheckCircle2,
  FileClock,
  FileText,
  FolderOpen,
  Plus,
  ReceiptText,
  Sparkles,
  Users,
} from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import StatCard from '@/components/StatCard.vue'
import FilingItem from '@/components/FilingItem.vue'
import EmptyState from '@/components/EmptyState.vue'
import { api } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { Dashboard, Filing } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const currentYear = new Date().getFullYear()
const years = Array.from({ length: 5 }, (_, index) => currentYear - 3 + index)
const year = ref(currentYear)
const data = ref<Dashboard | null>(null)
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({ year: String(year.value) })
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    data.value = await api.get<Dashboard>(`/dashboard?${params}`)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'No se pudo cargar el panel'
    toasts.error(error.value)
  } finally {
    loading.value = false
  }
}

watch([() => store.selectedId, year], load)
onMounted(load)

const upcomingMonths = computed(() => {
  const groups = new Map<string, Filing[]>()
  for (const filing of data.value?.upcoming ?? []) {
    if (!filing.due_date) continue
    const key = filing.due_date.slice(0, 7)
    const list = groups.get(key) ?? []
    list.push(filing)
    groups.set(key, list)
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, items]) => {
      const [y, m] = key.split('-').map(Number)
      const month = new Intl.DateTimeFormat('es-ES', { month: 'long' }).format(new Date(y, m - 1, 1))
      return { key, label: `${month} ${y}`, items }
    })
})
</script>

<template>
  <div>
    <PageHeader
      title="Tu situación fiscal, de un vistazo"
      subtitle="Plazos, modelos y documentación del contribuyente activo. Todo se calcula y guarda en este equipo."
    >
      <template #actions>
        <select v-model.number="year" class="input w-auto" aria-label="Ejercicio">
          <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
        </select>
        <RouterLink to="/calendario" class="btn-ghost">
          <CalendarDays class="h-4 w-4" />
          Ver calendario
        </RouterLink>
      </template>
    </PageHeader>

    <div v-if="error" class="mb-5 flex flex-wrap items-center gap-3 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800">
      <AlertTriangle class="h-4 w-4 shrink-0" />
      <span class="min-w-0 flex-1">{{ error }}</span>
      <button class="btn-ghost" @click="load">Reintentar</button>
    </div>

    <div v-if="!store.loading && !store.hasTaxpayers" class="card card-pad">
      <div class="flex flex-col items-start gap-6 sm:flex-row sm:items-center">
        <div class="grid h-14 w-14 shrink-0 place-items-center rounded-2xl bg-emerald-50 text-emerald-600">
          <Sparkles class="h-7 w-7" />
        </div>
        <div class="flex-1">
          <h2 class="text-lg font-bold text-slate-900">Empieza creando tu primer contribuyente</h2>
          <p class="mt-1 max-w-2xl text-sm leading-relaxed text-slate-500">
            Indica si es autónomo o sociedad y su perfil fiscal. La aplicación generará automáticamente
            el calendario de modelos de AEAT, TGSS y Registro Mercantil que te corresponden.
          </p>
        </div>
        <RouterLink to="/contribuyentes?nuevo=1" class="btn-accent">
          <Plus class="h-4 w-4" />
          Crear contribuyente
        </RouterLink>
      </div>
    </div>

    <template v-else>
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label="Vencen en 45 días"
          :value="data?.due_soon ?? '—'"
          tone="amber"
          hint="Revisa la documentación con tiempo"
          to="/calendario?range=next45"
        >
          <template #icon><CalendarClock class="h-5 w-5" /></template>
        </StatCard>
        <StatCard
          label="Vencidos sin presentar"
          :value="data?.overdue ?? '—'"
          tone="rose"
          hint="Fuera de plazo en el ejercicio"
          to="/calendario?range=overdue"
        >
          <template #icon><AlertTriangle class="h-5 w-5" /></template>
        </StatCard>
        <StatCard
          label="Pendientes en plazo"
          :value="data?.pending ?? '—'"
          tone="sky"
          hint="Aún puedes presentarlos"
          to="/calendario?range=open"
        >
          <template #icon><FileClock class="h-5 w-5" /></template>
        </StatCard>
        <StatCard
          label="Presentados"
          :value="data?.presented ?? '—'"
          tone="emerald"
          :hint="
            data
              ? `${data.presented_with_receipt} con justificante · ${data.presented_without_receipt} sin justificante`
              : 'Con o sin justificante archivado'
          "
          to="/calendario?range=all&status=presented"
        >
          <template #icon><CheckCircle2 class="h-5 w-5" /></template>
        </StatCard>
      </div>

      <div class="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
        <section class="min-w-0 lg:col-span-2">
          <div class="mb-3 flex items-center justify-between">
            <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Próximos vencimientos</h2>
            <RouterLink to="/calendario" class="inline-flex items-center gap-1 text-xs font-semibold text-slate-600 hover:text-slate-900">
              Todo el calendario <ArrowRight class="h-3.5 w-3.5" />
            </RouterLink>
          </div>
          <div v-if="loading" class="space-y-2">
            <div v-for="i in 4" :key="i" class="h-[74px] animate-pulse rounded-2xl bg-white" />
          </div>
          <div v-else-if="data?.upcoming.length" class="space-y-5">
            <section v-for="month in upcomingMonths" :key="month.key" class="space-y-2">
              <div class="flex items-center gap-2">
                <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500">
                  {{ month.label }}
                </h3>
                <span class="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-semibold text-slate-500">
                  {{ month.items.length }}
                </span>
                <span class="h-px flex-1 bg-slate-200" aria-hidden="true" />
              </div>
              <FilingItem
                v-for="filing in month.items"
                :key="filing.id"
                :filing="filing"
                show-entity
              />
            </section>
          </div>
          <EmptyState
            v-else
            title="Sin vencimientos próximos"
            description="No hay modelos pendientes en el rango consultado. Puedes generar el calendario del ejercicio si aún no lo has hecho."
          >
            <template #icon><CalendarDays class="h-6 w-6" /></template>
            <template #action>
              <RouterLink to="/calendario" class="btn-ghost">Ir al calendario</RouterLink>
            </template>
          </EmptyState>
        </section>

        <aside class="min-w-0 space-y-4">
          <div class="card card-pad">
            <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Carga por entidad</h2>
            <div class="mt-4 space-y-3">
              <div
                v-for="(count, entity) in data?.by_entity ?? {}"
                :key="entity"
                class="flex items-center justify-between gap-3"
              >
                <span class="text-sm font-medium text-slate-700">{{ entity }}</span>
                <span class="chip bg-slate-100 text-slate-600">{{ count }} pendientes</span>
              </div>
              <p v-if="!Object.keys(data?.by_entity ?? {}).length" class="text-sm text-slate-400">
                Sin obligaciones pendientes.
              </p>
            </div>
          </div>

          <div class="card card-pad">
            <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Accesos rápidos</h2>
            <div class="mt-3 grid gap-2">
              <RouterLink to="/facturas?nueva=1" class="flex items-center gap-3 rounded-xl border border-slate-200 px-3 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50">
                <ReceiptText class="h-4 w-4 text-slate-400" /> Registrar factura
              </RouterLink>
              <RouterLink to="/documentos" class="flex items-center gap-3 rounded-xl border border-slate-200 px-3 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50">
                <FolderOpen class="h-4 w-4 text-slate-400" /> Subir documento
              </RouterLink>
              <RouterLink to="/modelos" class="flex items-center gap-3 rounded-xl border border-slate-200 px-3 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50">
                <FileText class="h-4 w-4 text-slate-400" /> Ver modelos y trámites
              </RouterLink>
              <RouterLink to="/contribuyentes" class="flex items-center gap-3 rounded-xl border border-slate-200 px-3 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50">
                <Users class="h-4 w-4 text-slate-400" /> Gestionar contribuyentes
              </RouterLink>
            </div>
          </div>

          <div class="rounded-2xl bg-slate-900 p-5 text-white">
            <p class="text-xs font-bold uppercase tracking-wider text-slate-400">Datos bajo control</p>
            <p class="mt-2 text-sm leading-relaxed text-slate-300">
              {{ data?.documents ?? 0 }} documentos y {{ data?.invoices ?? 0 }} facturas registradas.
              Las copias de seguridad se hacen sobre la carpeta de datos local.
            </p>
          </div>
        </aside>
      </div>
    </template>
  </div>
</template>
