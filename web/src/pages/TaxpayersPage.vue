<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Building2, Pencil, Plus, Trash2, UserRound } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { api, dateLabel } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { Taxpayer } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const route = useRoute()
const router = useRouter()
const modalOpen = ref(false)
const saving = ref(false)
const editingId = ref<string | null>(null)

const emptyForm = () => ({
  name: '',
  nif: '',
  legal_form: 'autonomo' as 'autonomo' | 'sl',
  email: '',
  phone: '',
  vat_regime: 'general',
  irpf_method: 'directa',
  has_employees: false,
  has_rentals: false,
  economic_activity: true,
  cnae: '',
  fiscal_year_end: '12-31',
  iban: '',
})

const form = ref(emptyForm())
const isCompany = computed(() => form.value.legal_form === 'sl')

function openCreate() {
  editingId.value = null
  form.value = emptyForm()
  modalOpen.value = true
}

function openEdit(taxpayer: Taxpayer) {
  editingId.value = taxpayer.id
  form.value = {
    name: taxpayer.name,
    nif: taxpayer.nif,
    legal_form: taxpayer.legal_form,
    email: taxpayer.email,
    phone: taxpayer.phone,
    vat_regime: taxpayer.vat_regime,
    irpf_method: taxpayer.irpf_method,
    has_employees: taxpayer.has_employees,
    has_rentals: taxpayer.has_rentals,
    economic_activity: taxpayer.economic_activity,
    cnae: taxpayer.cnae,
    fiscal_year_end: taxpayer.fiscal_year_end,
    iban: taxpayer.iban,
  }
  modalOpen.value = true
}

async function save() {
  if (!form.value.name.trim() || !form.value.nif.trim()) {
    toasts.error('Nombre y NIF son obligatorios')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await api.put<Taxpayer>(`/taxpayers/${editingId.value}`, form.value)
      toasts.success('Contribuyente actualizado')
    } else {
      const created = await api.post<{ taxpayer: Taxpayer; generated_filings: number }>('/taxpayers', form.value)
      toasts.success(`Contribuyente creado con ${created.generated_filings} vencimientos`)
      store.select(created.taxpayer.id)
    }
    await store.loadTaxpayers()
    modalOpen.value = false
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo guardar el contribuyente')
  } finally {
    saving.value = false
  }
}

async function remove(taxpayer: Taxpayer) {
  if (
    !window.confirm(
      `¿Eliminar a ${taxpayer.name}? Se borrarán sus expedientes y facturas. Esta acción no se puede deshacer.`,
    )
  )
    return
  try {
    await api.del(`/taxpayers/${taxpayer.id}`)
    toasts.success('Contribuyente eliminado')
    await store.loadTaxpayers()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo eliminar')
  }
}

watch(
  () => route.query.nuevo,
  (value) => {
    if (value !== '1') return
    openCreate()
    const query = { ...route.query }
    delete query.nuevo
    void router.replace({ query })
  },
  { immediate: true },
)

onMounted(async () => {
  try {
    await store.loadTaxpayers()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudieron cargar los contribuyentes')
  }
})
</script>

<template>
  <div>
    <PageHeader
      title="Contribuyentes"
      subtitle="Autónomos y sociedades que gestionas. El perfil fiscal determina qué modelos se generan y qué documentación se exige."
    >
      <template #actions>
        <button class="btn-accent" @click="openCreate"><Plus class="h-4 w-4" /> Nuevo contribuyente</button>
      </template>
    </PageHeader>

    <EmptyState
      v-if="!store.taxpayers.length"
      title="Aún no hay contribuyentes"
      description="Crea el primero indicando su forma jurídica y régimen fiscal. Podrás cambiarlo después."
    >
      <template #icon><UserRound class="h-6 w-6" /></template>
      <template #action>
        <button class="btn-accent" @click="openCreate"><Plus class="h-4 w-4" /> Crear contribuyente</button>
      </template>
    </EmptyState>

    <div v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article
        v-for="taxpayer in store.taxpayers"
        :key="taxpayer.id"
        class="card flex flex-col p-5"
        :class="{ 'ring-2 ring-emerald-500/30': taxpayer.id === store.selectedId }"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-3">
            <span
              class="grid h-11 w-11 place-items-center rounded-xl"
              :class="taxpayer.legal_form === 'sl' ? 'bg-indigo-50 text-indigo-600' : 'bg-emerald-50 text-emerald-600'"
            >
              <Building2 v-if="taxpayer.legal_form === 'sl'" class="h-5 w-5" />
              <UserRound v-else class="h-5 w-5" />
            </span>
            <div>
              <h3 class="text-sm font-bold text-slate-900">{{ taxpayer.name }}</h3>
              <p class="text-xs text-slate-500">{{ taxpayer.nif }}</p>
            </div>
          </div>
          <span v-if="taxpayer.id === store.selectedId" class="chip bg-emerald-50 text-emerald-700">Activo</span>
        </div>

        <dl class="mt-4 space-y-1.5 text-xs">
          <div class="flex justify-between gap-3">
            <dt class="text-slate-400">Forma jurídica</dt>
            <dd class="font-medium text-slate-700">{{ taxpayer.legal_form === 'sl' ? 'Sociedad Limitada' : 'Autónomo' }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="text-slate-400">IVA</dt>
            <dd class="font-medium capitalize text-slate-700">{{ taxpayer.vat_regime }}</dd>
          </div>
          <div v-if="taxpayer.legal_form === 'autonomo'" class="flex justify-between gap-3">
            <dt class="text-slate-400">IRPF</dt>
            <dd class="font-medium capitalize text-slate-700">{{ taxpayer.irpf_method }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="text-slate-400">Alta</dt>
            <dd class="font-medium text-slate-700">{{ dateLabel(taxpayer.created_at) }}</dd>
          </div>
        </dl>

        <div class="mt-4 flex flex-wrap gap-1.5">
          <span v-if="taxpayer.has_employees" class="chip bg-sky-50 text-sky-700">Empleados</span>
          <span v-if="taxpayer.has_rentals" class="chip bg-amber-50 text-amber-700">Alquileres</span>
          <span v-if="!taxpayer.economic_activity" class="chip bg-slate-100 text-slate-500">Sin actividad</span>
        </div>

        <div class="mt-5 flex items-center gap-2 border-t border-slate-100 pt-4">
          <button class="btn-ghost btn-sm flex-1" @click="store.select(taxpayer.id)">
            {{ taxpayer.id === store.selectedId ? 'Seleccionado' : 'Activar' }}
          </button>
          <button class="btn-ghost btn-sm" @click="openEdit(taxpayer)"><Pencil class="h-3.5 w-3.5" /> Editar</button>
          <button class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600" @click="remove(taxpayer)">
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
      </article>
    </div>

    <Modal
      v-model="modalOpen"
      :title="editingId ? 'Editar contribuyente' : 'Nuevo contribuyente'"
      subtitle="El perfil determina los modelos y plazos que se generan."
      size="lg"
    >
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-2">
          <button
            class="rounded-xl border px-4 py-3 text-sm font-semibold transition"
            :class="form.legal_form === 'autonomo' ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
            @click="form.legal_form = 'autonomo'"
          >
            Autónomo
          </button>
          <button
            class="rounded-xl border px-4 py-3 text-sm font-semibold transition"
            :class="form.legal_form === 'sl' ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
            @click="form.legal_form = 'sl'"
          >
            Sociedad Limitada
          </button>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">Nombre o razón social</label>
            <input v-model="form.name" class="input" placeholder="Lucía Pérez / Estudio SL" />
          </div>
          <div>
            <label class="label">NIF</label>
            <input v-model="form.nif" class="input" placeholder="12345678Z / B12345678" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">Email</label>
            <input v-model="form.email" type="email" class="input" placeholder="opcional" />
          </div>
          <div>
            <label class="label">Teléfono</label>
            <input v-model="form.phone" class="input" placeholder="opcional" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">Régimen de IVA</label>
            <select v-model="form.vat_regime" class="input">
              <option value="general">General</option>
              <option value="simplificado">Simplificado</option>
              <option value="recargo">Recargo de equivalencia</option>
              <option value="exento">Exento</option>
            </select>
          </div>
          <div v-if="!isCompany">
            <label class="label">Estimación IRPF</label>
            <select v-model="form.irpf_method" class="input">
              <option value="directa">Directa</option>
              <option value="modulos">Módulos</option>
            </select>
          </div>
          <div v-else>
            <label class="label">Cierre del ejercicio</label>
            <input v-model="form.fiscal_year_end" class="input" placeholder="12-31" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label">CNAE</label>
            <input v-model="form.cnae" class="input" placeholder="7022" />
          </div>
          <div>
            <label class="label">IBAN</label>
            <input v-model="form.iban" class="input" placeholder="ES00 0000 0000 0000 0000 0000" />
          </div>
        </div>

        <div class="rounded-xl border border-slate-200 p-4">
          <p class="text-xs font-bold uppercase tracking-wider text-slate-500">Perfil de obligaciones</p>
          <div class="mt-3 space-y-2.5">
            <label class="flex items-center gap-2.5 text-sm text-slate-700">
              <input v-model="form.economic_activity" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-emerald-600" />
              Tiene actividad económica
            </label>
            <label class="flex items-center gap-2.5 text-sm text-slate-700">
              <input v-model="form.has_employees" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-emerald-600" />
              Tiene empleados (111, 190, TC1/TC2)
            </label>
            <label class="flex items-center gap-2.5 text-sm text-slate-700">
              <input v-model="form.has_rentals" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-emerald-600" />
              Paga alquileres sujetos a retención (115, 180)
            </label>
          </div>
        </div>
      </div>

      <template #footer>
        <button class="btn-ghost" @click="modalOpen = false">Cancelar</button>
        <button class="btn-accent" :disabled="saving" @click="save">
          <Plus class="h-4 w-4" /> {{ saving ? 'Guardando…' : editingId ? 'Guardar cambios' : 'Crear contribuyente' }}
        </button>
      </template>
    </Modal>
  </div>
</template>
