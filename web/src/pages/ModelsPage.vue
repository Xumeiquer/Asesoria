<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowUpRight, Calculator, CheckCircle2, FileText, Search, XCircle } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { api, dateLabel } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { Model } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const models = ref<Model[]>([])
const catalogVersion = ref('')
const catalogUpdatedAt = ref('')
const loading = ref(true)
const error = ref('')
const search = ref('')
const category = ref('')
const selected = ref<Model | null>(null)
const detailOpen = ref(false)

const categories = computed(() => [...new Set(models.value.map((m) => m.category))].sort())
const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  return models.value.filter((m) => {
    if (category.value && m.category !== category.value) return false
    if (!term) return true
    return `${m.code} ${m.name} ${m.description} ${m.entity}`.toLowerCase().includes(term)
  })
})
const applicableCount = computed(() => models.value.filter((m) => m.applicable && m.auto).length)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (store.selectedId) params.set('taxpayer_id', store.selectedId)
    const data = await api.get<{ version: string; updated_at: string; models: Model[] }>(`/catalog?${params}`)
    catalogVersion.value = data.version
    catalogUpdatedAt.value = data.updated_at
    models.value = data.models
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'No se pudo cargar el catálogo'
    toasts.error(error.value)
  } finally {
    loading.value = false
  }
}

function openDetail(model: Model) {
  selected.value = model
  detailOpen.value = true
}

function entityTone(entity: string): string {
  if (entity === 'AEAT') return 'bg-indigo-50 text-indigo-700'
  if (entity === 'TGSS') return 'bg-sky-50 text-sky-700'
  return 'bg-amber-50 text-amber-700'
}

watch(() => store.selectedId, load)
onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Modelos y trámites"
      subtitle="Catálogo declarativo de obligaciones. La aplicación marca las que corresponden al perfil del contribuyente y las que tienen cálculo automático."
    >
      <template #actions>
        <span class="chip bg-slate-100 text-slate-600">
          Catálogo {{ catalogVersion }} · {{ dateLabel(catalogUpdatedAt) }}
        </span>
        <span class="chip bg-emerald-50 text-emerald-700">
          {{ applicableCount }} obligaciones periódicas aplicables
        </span>
      </template>
    </PageHeader>

    <div v-if="error" class="mb-5 flex flex-wrap items-center gap-3 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800">
      <XCircle class="h-4 w-4 shrink-0" />
      <span class="min-w-0 flex-1">{{ error }}</span>
      <button class="btn-ghost" @click="load">Reintentar</button>
    </div>

    <div class="mb-5 flex flex-col gap-3 sm:flex-row">
      <div class="relative flex-1">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          v-model="search"
          class="input pl-9"
          placeholder="Buscar por modelo, nombre o entidad…"
          aria-label="Buscar modelos"
        />
      </div>
      <select v-model="category" class="input sm:w-56" aria-label="Filtrar por categoría">
        <option value="">Todas las categorías</option>
        <option v-for="c in categories" :key="c" :value="c">{{ c }}</option>
      </select>
    </div>

    <div v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <div v-for="i in 9" :key="i" class="h-48 animate-pulse rounded-2xl bg-white" />
    </div>

    <EmptyState
      v-else-if="!filtered.length"
      title="Sin resultados"
      description="Prueba con otro término o cambia la categoría."
    >
      <template #icon><FileText class="h-6 w-6" /></template>
    </EmptyState>

    <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
      <button
        v-for="model in filtered"
        :key="model.code"
        class="card group flex h-full min-w-0 flex-col p-5 text-left transition hover:border-slate-300 hover:shadow-md"
        :class="{ 'opacity-60': !model.applicable }"
        @click="openDetail(model)"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex min-w-0 items-center gap-3">
            <span class="grid h-11 min-w-11 place-items-center rounded-xl bg-slate-900 px-2 text-sm font-bold text-white">
              {{ model.code }}
            </span>
            <div>
              <p class="text-sm font-bold leading-tight text-slate-900">{{ model.name }}</p>
              <p class="mt-0.5 text-xs text-slate-500">
                {{ model.periodicity === 'Puntual' ? 'Puntual' : `Periodicidad: ${model.periodicity}` }}
              </p>
            </div>
          </div>
          <span class="chip shrink-0" :class="entityTone(model.entity)">{{ model.entity }}</span>
        </div>

        <p class="mt-4 line-clamp-3 flex-1 text-sm leading-relaxed text-slate-500">{{ model.description }}</p>

        <div class="mt-4 flex flex-wrap items-center gap-2">
          <span v-if="model.calculator" class="chip bg-emerald-50 text-emerald-700">
            <Calculator class="h-3.5 w-3.5" /> Cálculo automático
          </span>
          <span v-if="model.obsolete" class="chip bg-slate-200 text-slate-600">Suprimido</span>
          <span v-else-if="model.applicable && model.auto" class="chip bg-slate-100 text-slate-600">
            <CheckCircle2 class="h-3.5 w-3.5" /> En tu calendario
          </span>
          <span v-else-if="!model.applicable" class="chip bg-rose-50 text-rose-600">
            <XCircle class="h-3.5 w-3.5" /> No aplica
          </span>
          <span v-else class="chip bg-amber-50 text-amber-700">Trámite puntual</span>
        </div>
      </button>
    </div>

    <Modal
      v-model="detailOpen"
      :title="selected ? `${selected.code} · ${selected.name}` : ''"
      :subtitle="selected ? `${selected.entity} · ${selected.periodicity}` : ''"
      size="lg"
    >
      <div v-if="selected" class="space-y-6">
        <div>
          <h3 class="mb-2 text-xs font-bold uppercase tracking-wider text-slate-500">Para qué sirve</h3>
          <p class="text-sm leading-relaxed text-slate-600">{{ selected.description }}</p>
        </div>

        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Entidad</p>
            <p class="mt-1 text-sm font-semibold text-slate-800">{{ selected.entity }}</p>
          </div>
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Periodicidad</p>
            <p class="mt-1 text-sm font-semibold text-slate-800">{{ selected.periodicity }}</p>
          </div>
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Calendario</p>
            <p class="mt-1 text-sm font-semibold text-slate-800">
              {{ selected.auto ? 'Automático' : 'Manual / puntual' }}
            </p>
          </div>
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Actualizado</p>
            <p class="mt-1 text-sm font-semibold text-slate-800">{{ dateLabel(selected.updated_at ?? '') }}</p>
          </div>
        </div>

        <div>
          <h3 class="mb-2 text-xs font-bold uppercase tracking-wider text-slate-500">Documentación necesaria</h3>
          <ul class="space-y-1.5">
            <li v-for="doc in selected.documents" :key="doc" class="flex items-start gap-2 text-sm text-slate-600">
              <CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
              {{ doc }}
            </li>
          </ul>
          <p v-if="!selected.documents.length" class="text-sm text-slate-400">Sin checklist específica.</p>
        </div>

        <div v-if="selected.notes" class="rounded-xl border border-amber-200 bg-amber-50 p-4">
          <p class="text-xs font-bold uppercase tracking-wider text-amber-700">A tener en cuenta</p>
          <p class="mt-1 text-sm leading-relaxed text-amber-800">{{ selected.notes }}</p>
        </div>

        <div class="flex flex-wrap gap-2">
          <a :href="selected.url" target="_blank" rel="noreferrer" class="btn-ghost">
            Abrir en {{ selected.entity }} <ArrowUpRight class="h-4 w-4" />
          </a>
          <span v-if="!selected.applicable" class="chip bg-rose-50 text-rose-600">
            <XCircle class="h-3.5 w-3.5" /> No corresponde al perfil actual
          </span>
        </div>
      </div>

      <template #footer>
        <button class="btn-ghost" @click="detailOpen = false">Cerrar</button>
      </template>
    </Modal>
  </div>
</template>
