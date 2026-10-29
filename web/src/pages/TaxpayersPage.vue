<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Building2, Check, Pencil, Plus, Trash2, UserRound, X } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { api, centsFromEuros, dateLabel, euros } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type {
  RetaBaseChange,
  RetaBracket,
  RetaBracketsResponse,
  Taxpayer,
  TaxpayerMutationResult,
} from '@/types'

const store = useAppStore()
const toasts = useToasts()
const route = useRoute()
const router = useRouter()
const modalOpen = ref(false)
const saving = ref(false)
const editingId = ref<string | null>(null)
const step = ref(1)
const errors = ref<Record<string, string>>({})
const confirmTaxpayer = ref<Taxpayer | null>(null)
const deleting = ref(false)
const brackets = ref<RetaBracket[]>([])
const bracketsYear = ref(new Date().getFullYear())
const history = ref<RetaBaseChange[]>([])
const historyLoading = ref(false)

function todayISO(): string {
  const now = new Date()
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 10)
}

function centsToInput(cents: number): string {
  if (!cents) return ''
  return (cents / 100).toFixed(2).replace('.', ',')
}

const nifPattern = /^(?:\d{8}[A-Z]|[XYZ]\d{7}[A-Z]|[A-Z]\d{7}[A-Z0-9])$/

function normalizeNIF(value: string): string {
  return value.toUpperCase().replace(/[\s.-]/g, '')
}

function validIBAN(value: string): boolean {
  const iban = value.toUpperCase().replace(/\s/g, '')
  if (!/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(iban)) return false
  const rearranged = iban.slice(4) + iban.slice(0, 4)
  let remainder = 0
  for (const char of rearranged) {
    if (char >= '0' && char <= '9') {
      remainder = (remainder * 10 + Number(char)) % 97
    } else {
      remainder = (remainder * 100 + char.charCodeAt(0) - 55) % 97
    }
  }
  return remainder === 1
}

function validateIdentity(): boolean {
  const next: Record<string, string> = {}
  if (!form.value.name.trim()) next.name = 'Indica el nombre o la razón social.'
  if (!form.value.nif.trim()) next.nif = 'El NIF es obligatorio.'
  else if (!nifPattern.test(normalizeNIF(form.value.nif))) next.nif = 'El NIF no tiene un formato válido.'
  errors.value = { ...errors.value, ...next }
  return Object.keys(next).length === 0
}

function validateActivity(): boolean {
  const cnaesValid = commitCnaeDraft()
  const next: Record<string, string> = {}
  if (form.value.iban.trim() && !validIBAN(form.value.iban)) {
    next.iban = 'El IBAN no es válido.'
  }
  if (!form.value.activity_start) {
    next.activity_start = 'Indica cuándo empezó la actividad.'
  } else if (Number.isNaN(Date.parse(`${form.value.activity_start}T00:00:00`))) {
    next.activity_start = 'La fecha de inicio no es válida.'
  }
  if (!/^(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])$/.test(form.value.fiscal_year_end)) {
    next.fiscal_year_end = 'El cierre fiscal debe tener formato MM-DD.'
  }
  errors.value = { ...errors.value, ...next }
  return cnaesValid && Object.keys(next).length === 0
}

function nextStep() {
  if (step.value === 1 && !validateIdentity()) return
  if (step.value === 2 && !validateActivity()) return
  step.value = Math.min(3, step.value + 1)
}

function prevStep() {
  step.value = Math.max(1, step.value - 1)
}

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
  cnaes: [] as string[],
  fiscal_year_end: '12-31',
  iban: '',
  activity_start: todayISO(),
  reta_base: '',
  reta_base_from: todayISO(),
  reta_net_income: '',
  reta_net_income_year: new Date().getFullYear(),
})

const form = ref(emptyForm())
const cnaeDraft = ref('')
const isCompany = computed(() => form.value.legal_form === 'sl')

function commitCnaeDraft(): boolean {
  const parts = cnaeDraft.value
    .split(/[\s,;]+/)
    .map((value) => value.trim())
    .filter(Boolean)
  if (!parts.length) {
    delete errors.value.cnaes
    return true
  }
  const next = [...form.value.cnaes]
  for (const code of parts) {
    if (!/^\d{4}$/.test(code)) {
      errors.value.cnaes = 'Cada CNAE debe tener 4 dígitos.'
      return false
    }
    if (!next.includes(code)) next.push(code)
  }
  form.value.cnaes = next
  cnaeDraft.value = ''
  delete errors.value.cnaes
  return true
}

function removeCnae(code: string) {
  form.value.cnaes = form.value.cnaes.filter((value) => value !== code)
  delete errors.value.cnaes
}

const retaPreview = computed(() => {
  if (isCompany.value || !brackets.value.length) return null
  const annual = centsFromEuros(form.value.reta_net_income)
  const base = centsFromEuros(form.value.reta_base)
  const hasIncome = form.value.reta_net_income.trim() !== ''
  const hasBase = form.value.reta_base.trim() !== ''
  if (!hasIncome && !hasBase) return null
  const year = Number(form.value.reta_net_income_year) || new Date().getFullYear()
  if (year !== bracketsYear.value) return null

  const monthly = annual > 0 ? Math.round(annual / 12) : 0
  const bracket = brackets.value.find(
    (item) => monthly >= item.min_income_cents && (item.max_income_cents === 0 || monthly <= item.max_income_cents),
  )
  if (!bracket) return null
  return {
    bracket,
    monthly,
    hasBase,
    baseValid: hasBase && base >= bracket.min_base_cents && base <= bracket.max_base_cents,
  }
})

function openCreate() {
  editingId.value = null
  step.value = 1
  errors.value = {}
  form.value = emptyForm()
  cnaeDraft.value = ''
  history.value = []
  historyLoading.value = false
  modalOpen.value = true
}

function openEdit(taxpayer: Taxpayer) {
  editingId.value = taxpayer.id
  step.value = 1
  errors.value = {}
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
    cnaes: [...(taxpayer.cnaes ?? [])],
    fiscal_year_end: taxpayer.fiscal_year_end,
    iban: taxpayer.iban,
    activity_start: taxpayer.activity_start || taxpayer.created_at.slice(0, 10) || todayISO(),
    reta_base: centsToInput(taxpayer.reta_base_cents),
    reta_base_from: taxpayer.reta_base_from || todayISO(),
    reta_net_income: centsToInput(taxpayer.reta_net_income_cents),
    reta_net_income_year: taxpayer.reta_net_income_year || new Date().getFullYear(),
  }
  cnaeDraft.value = ''
  history.value = []
  void loadHistory(taxpayer.id)
  modalOpen.value = true
}

async function loadBrackets() {
  const year = new Date().getFullYear()
  const response = await api.get<RetaBracketsResponse>(`/reta/brackets?year=${year}`)
  bracketsYear.value = response.year
  brackets.value = response.brackets
}

async function loadHistory(taxpayerID: string) {
  historyLoading.value = true
  try {
    history.value = await api.get<RetaBaseChange[]>(`/taxpayers/${taxpayerID}/reta-history`)
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

async function save() {
  if (!validateIdentity()) {
    step.value = 1
    return
  }
  if (!validateActivity()) {
    step.value = 2
    return
  }
  saving.value = true
  try {
    const payload = {
      name: form.value.name,
      nif: form.value.nif,
      legal_form: form.value.legal_form,
      email: form.value.email,
      phone: form.value.phone,
      vat_regime: form.value.vat_regime,
      irpf_method: form.value.irpf_method,
      has_employees: form.value.has_employees,
      has_rentals: form.value.has_rentals,
      economic_activity: form.value.economic_activity,
      cnaes: form.value.cnaes,
      fiscal_year_end: form.value.fiscal_year_end,
      iban: form.value.iban,
      activity_start: form.value.activity_start,
      reta_base_cents: isCompany.value ? 0 : centsFromEuros(form.value.reta_base),
      reta_base_from: isCompany.value ? '' : form.value.reta_base_from,
      reta_net_income_cents: isCompany.value ? 0 : centsFromEuros(form.value.reta_net_income),
      reta_net_income_year: isCompany.value ? 0 : Number(form.value.reta_net_income_year) || 0,
    }
    if (editingId.value) {
      const result = await api.put<TaxpayerMutationResult>(`/taxpayers/${editingId.value}`, payload)
      toasts.success(
        result.generated_filings
          ? `Contribuyente actualizado · ${result.generated_filings} nuevos vencimientos`
          : 'Contribuyente actualizado',
      )
    } else {
      const created = await api.post<TaxpayerMutationResult>('/taxpayers', payload)
      toasts.success(
        `Contribuyente creado con ${created.generated_filings} vencimientos. Revisa el calendario.`,
      )
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

function remove(taxpayer: Taxpayer) {
  confirmTaxpayer.value = taxpayer
}

async function confirmDelete() {
  if (!confirmTaxpayer.value) return
  deleting.value = true
  try {
    await api.del(`/taxpayers/${confirmTaxpayer.value.id}`)
    toasts.success('Contribuyente eliminado')
    confirmTaxpayer.value = null
    await store.loadTaxpayers()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo eliminar')
  } finally {
    deleting.value = false
  }
}

function onDeleteDialogUpdate(open: boolean) {
  if (!open && !deleting.value) confirmTaxpayer.value = null
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
  try {
    await loadBrackets()
  } catch {
    // The backend still validates the base when saving; the preview is optional.
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
          <div v-if="taxpayer.legal_form === 'autonomo' && taxpayer.reta_base_cents" class="flex justify-between gap-3">
            <dt class="text-slate-400">Base RETA</dt>
            <dd class="font-medium text-slate-700">{{ euros(taxpayer.reta_base_cents) }}/mes</dd>
          </div>
          <div v-if="taxpayer.legal_form === 'autonomo' && taxpayer.reta_base_cents" class="flex justify-between gap-3">
            <dt class="text-slate-400">Previsión anual</dt>
            <dd class="font-medium text-slate-700">{{ euros(taxpayer.reta_net_income_cents) }}</dd>
          </div>
          <div v-if="taxpayer.legal_form === 'autonomo' && taxpayer.reta_base_cents" class="flex justify-between gap-3">
            <dt class="text-slate-400">Vigente desde</dt>
            <dd class="font-medium text-slate-700">{{ dateLabel(taxpayer.reta_base_from) }}</dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="text-slate-400">Inicio de actividad</dt>
            <dd class="font-medium text-slate-700">
              {{ dateLabel(taxpayer.activity_start || taxpayer.created_at) }}
            </dd>
          </div>
          <div class="flex justify-between gap-3">
            <dt class="text-slate-400">En Asesoría desde</dt>
            <dd class="font-medium text-slate-700">{{ dateLabel(taxpayer.created_at) }}</dd>
          </div>
        </dl>

        <div class="mt-4 flex flex-wrap gap-1.5">
          <span v-if="taxpayer.has_employees" class="chip bg-sky-50 text-sky-700">Empleados</span>
          <span v-if="taxpayer.has_rentals" class="chip bg-amber-50 text-amber-700">Alquileres</span>
          <span v-if="!taxpayer.economic_activity" class="chip bg-slate-100 text-slate-500">Sin actividad</span>
          <span
            v-for="code in taxpayer.cnaes"
            :key="code"
            class="chip bg-indigo-50 text-indigo-700"
          >
            CNAE {{ code }}
          </span>
        </div>

        <div class="mt-5 flex items-center gap-2 border-t border-slate-100 pt-4">
          <button class="btn-ghost btn-sm flex-1" @click="store.select(taxpayer.id)">
            {{ taxpayer.id === store.selectedId ? 'Seleccionado' : 'Activar' }}
          </button>
          <button class="btn-ghost btn-sm" @click="openEdit(taxpayer)"><Pencil class="h-3.5 w-3.5" /> Editar</button>
          <button
            class="rounded-lg p-1.5 text-slate-500 hover:bg-rose-50 hover:text-rose-600"
            :aria-label="`Eliminar contribuyente ${taxpayer.name}`"
            title="Eliminar contribuyente"
            @click="remove(taxpayer)"
          >
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
      persistent
    >
      <div class="space-y-5">
        <ol class="grid grid-cols-3 gap-2 text-xs" aria-label="Pasos del alta">
          <li>
            <button
              type="button"
              class="flex w-full items-center gap-2 rounded-xl border px-3 py-2 text-left font-semibold transition"
              :class="step === 1 ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
              @click="step = 1"
            >
              <span class="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-slate-100 text-slate-700">1</span>
              Identidad
            </button>
          </li>
          <li>
            <button
              type="button"
              class="flex w-full items-center gap-2 rounded-xl border px-3 py-2 text-left font-semibold transition"
              :class="step === 2 ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
              @click="step = 2"
            >
              <span class="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-slate-100 text-slate-700">2</span>
              Actividad
            </button>
          </li>
          <li>
            <button
              type="button"
              class="flex w-full items-center gap-2 rounded-xl border px-3 py-2 text-left font-semibold transition"
              :class="step === 3 ? 'border-slate-900 bg-slate-900 text-white' : 'border-slate-200 bg-white text-slate-600'"
              @click="step = 3"
            >
              <span class="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-slate-100 text-slate-700">3</span>
              {{ isCompany ? 'Resumen' : 'RETA' }}
            </button>
          </li>
        </ol>

        <p v-if="editingId" class="rounded-xl bg-amber-50 px-3 py-2 text-xs leading-relaxed text-amber-800">
          Cambiar el perfil fiscal puede añadir nuevos vencimientos. Al guardar se actualizará el calendario de este
          ejercicio y del siguiente; revisa después los modelos aplicables.
        </p>

        <template v-if="step === 1">
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
            <label class="label" for="taxpayer-name">Nombre o razón social</label>
            <input
              id="taxpayer-name"
              v-model="form.name"
              class="input"
              :class="errors.name ? 'border-rose-300' : ''"
              placeholder="Lucía Pérez / Estudio SL"
            />
            <p v-if="errors.name" class="mt-1 text-xs text-rose-600">{{ errors.name }}</p>
          </div>
          <div>
            <label class="label" for="taxpayer-nif">NIF</label>
            <input
              id="taxpayer-nif"
              v-model="form.nif"
              class="input"
              :class="errors.nif ? 'border-rose-300' : ''"
              placeholder="12345678Z / B12345678"
            />
            <p v-if="errors.nif" class="mt-1 text-xs text-rose-600">{{ errors.nif }}</p>
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
        </template>

        <template v-else-if="step === 2">
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
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label class="label" for="taxpayer-cnae">CNAE</label>
            <div class="flex gap-2">
              <input
                id="taxpayer-cnae"
                v-model="cnaeDraft"
                class="input"
                :class="errors.cnaes ? 'border-rose-300' : ''"
                inputmode="numeric"
                autocomplete="off"
                placeholder="7022, 6201"
                @keydown.enter.prevent="commitCnaeDraft"
              />
              <button type="button" class="btn-ghost shrink-0" @click="commitCnaeDraft">
                <Plus class="h-4 w-4" />
                Añadir
              </button>
            </div>
            <div v-if="form.cnaes.length" class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="code in form.cnaes"
                :key="code"
                class="chip bg-indigo-50 text-indigo-700"
              >
                {{ code }}
                <button
                  type="button"
                  class="rounded-full p-0.5 hover:bg-indigo-100"
                  :aria-label="`Quitar CNAE ${code}`"
                  @click="removeCnae(code)"
                >
                  <X class="h-3 w-3" />
                </button>
              </span>
            </div>
            <p v-if="errors.cnaes" class="mt-1 text-xs text-rose-600">{{ errors.cnaes }}</p>
            <p v-else class="mt-1 text-[11px] text-slate-500">
              Puedes indicar varios códigos de 4 dígitos, separados por coma o añadidos uno a uno.
            </p>
          </div>
          <div>
            <label class="label" for="taxpayer-iban">IBAN</label>
            <input
              id="taxpayer-iban"
              v-model="form.iban"
              class="input"
              :class="errors.iban ? 'border-rose-300' : ''"
              placeholder="ES00 0000 0000 0000 0000 0000"
            />
            <p v-if="errors.iban" class="mt-1 text-xs text-rose-600">{{ errors.iban }}</p>
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <label class="label" for="taxpayer-activity-start">Inicio de actividad</label>
            <input
              id="taxpayer-activity-start"
              v-model="form.activity_start"
              type="date"
              class="input"
              :class="errors.activity_start ? 'border-rose-300' : ''"
            />
            <p v-if="errors.activity_start" class="mt-1 text-xs text-rose-600">{{ errors.activity_start }}</p>
            <p v-else class="mt-1 text-[11px] text-slate-500">
              Fecha real desde la que se generan los vencimientos.
            </p>
          </div>
          <div>
            <label class="label" for="taxpayer-year-end">Cierre del ejercicio</label>
            <input
              id="taxpayer-year-end"
              v-model="form.fiscal_year_end"
              class="input"
              :class="errors.fiscal_year_end ? 'border-rose-300' : ''"
              placeholder="12-31"
            />
            <p v-if="errors.fiscal_year_end" class="mt-1 text-xs text-rose-600">{{ errors.fiscal_year_end }}</p>
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
        </template>

        <template v-else>
        <div class="rounded-xl bg-slate-50 p-4">
          <h3 class="text-sm font-bold text-slate-900">Resumen del perfil</h3>
          <dl class="mt-3 grid gap-2 text-sm sm:grid-cols-2">
            <div>
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Nombre</dt>
              <dd class="mt-0.5 font-medium text-slate-700">{{ form.name || '—' }}</dd>
            </div>
            <div>
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">NIF</dt>
              <dd class="mt-0.5 font-medium text-slate-700">{{ form.nif || '—' }}</dd>
            </div>
            <div class="sm:col-span-2">
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">CNAE</dt>
              <dd class="mt-0.5 font-medium text-slate-700">
                {{ form.cnaes.length ? form.cnaes.join(', ') : '—' }}
              </dd>
            </div>
            <div>
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Forma jurídica</dt>
              <dd class="mt-0.5 font-medium text-slate-700">{{ isCompany ? 'Sociedad Limitada' : 'Autónomo' }}</dd>
            </div>
            <div>
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Inicio de actividad</dt>
              <dd class="mt-0.5 font-medium text-slate-700">{{ dateLabel(form.activity_start) }}</dd>
            </div>
            <div>
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">Régimen de IVA</dt>
              <dd class="mt-0.5 font-medium capitalize text-slate-700">{{ form.vat_regime }}</dd>
            </div>
            <div>
              <dt class="text-[11px] font-bold uppercase tracking-wider text-slate-500">
                {{ isCompany ? 'Cierre del ejercicio' : 'Estimación IRPF' }}
              </dt>
              <dd class="mt-0.5 font-medium capitalize text-slate-700">
                {{ isCompany ? form.fiscal_year_end : form.irpf_method }}
              </dd>
            </div>
          </dl>
        </div>

        <div v-if="!isCompany" class="rounded-xl border border-slate-200 p-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs font-bold uppercase tracking-wider text-slate-500">Cotización RETA</p>
            <span class="chip bg-slate-100 text-slate-500">Tramos {{ bracketsYear }}</span>
          </div>
          <p class="mt-1 text-xs text-slate-500">
            La previsión anual determina el tramo. La base elegida debe estar entre la mínima y la máxima de ese tramo y
            puede cambiar hasta seis veces al año.
          </p>
          <p class="mt-1 text-[11px] text-slate-400">
            Validación con la tabla general y reducida de 2026. No cubre pluriactividad, actividades artísticas ni otras
            particularidades.
          </p>
          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <div>
              <label class="label">Previsión de rendimientos netos (anual)</label>
              <input v-model="form.reta_net_income" class="input" inputmode="decimal" placeholder="18.000,00" />
            </div>
            <div>
              <label class="label">Año de la previsión</label>
              <input v-model.number="form.reta_net_income_year" type="number" min="2023" max="2100" class="input" />
            </div>
            <div>
              <label class="label">Base de cotización mensual</label>
              <input v-model="form.reta_base" class="input" inputmode="decimal" placeholder="1.200,00" />
            </div>
            <div>
              <label class="label">Vigente desde</label>
              <input v-model="form.reta_base_from" type="date" class="input" />
            </div>
          </div>

          <div v-if="retaPreview" class="mt-3 rounded-lg bg-slate-50 p-3 text-xs">
            <p class="font-semibold text-slate-700">
              Tramo {{ retaPreview.bracket.number }} · tabla {{ retaPreview.bracket.table }}
            </p>
            <p class="mt-1 text-slate-500">
              Rendimiento mensual: {{ euros(retaPreview.monthly) }} · Base permitida:
              {{ euros(retaPreview.bracket.min_base_cents) }} – {{ euros(retaPreview.bracket.max_base_cents) }}
            </p>
            <p v-if="!retaPreview.hasBase" class="mt-1 text-amber-600">Indica la base de cotización para validarla.</p>
            <p v-else-if="retaPreview.baseValid" class="mt-1 text-emerald-600">Base válida para este tramo.</p>
            <p v-else class="mt-1 text-rose-600">La base está fuera del rango permitido para este tramo.</p>
          </div>
          <p v-else class="mt-3 text-xs text-slate-400">
            Introduce la previsión anual para ver el tramo y el rango de base.
          </p>

          <div v-if="editingId" class="mt-4 border-t border-slate-100 pt-3">
            <p class="text-xs font-semibold text-slate-600">Historial de base</p>
            <p v-if="historyLoading" class="mt-1 text-xs text-slate-400">Cargando…</p>
            <ul v-else-if="history.length" class="mt-2 space-y-1 text-xs text-slate-500">
              <li v-for="change in history" :key="change.id" class="flex flex-wrap justify-between gap-2">
                <span>{{ dateLabel(change.base_from) }}</span>
                <span>{{ euros(change.base_cents) }}/mes · previsión {{ euros(change.net_income_cents) }}/año</span>
              </li>
            </ul>
            <p v-else class="mt-1 text-xs text-slate-400">Sin cambios registrados.</p>
          </div>
        </div>
        <p v-else class="rounded-xl bg-slate-50 p-4 text-sm leading-relaxed text-slate-600">
          Las sociedades limitadas no tienen perfil RETA. Sus obligaciones se generan a partir del régimen de IVA,
          empleados, alquileres y cierre del ejercicio.
        </p>
        </template>
      </div>

      <template #footer>
        <button class="btn-ghost" :disabled="saving" @click="modalOpen = false">Cancelar</button>
        <button v-if="step > 1" class="btn-ghost" :disabled="saving" @click="prevStep">Anterior</button>
        <button v-if="step < 3" class="btn-primary" @click="nextStep">
          Siguiente
          <Check class="h-4 w-4" />
        </button>
        <button v-else class="btn-accent" :disabled="saving" @click="save">
          <Plus class="h-4 w-4" />
          {{ saving ? 'Guardando…' : editingId ? 'Guardar cambios' : 'Crear contribuyente' }}
        </button>
      </template>
    </Modal>

    <ConfirmDialog
      :model-value="!!confirmTaxpayer"
      title="Eliminar contribuyente"
      :message="`Se eliminará a ${confirmTaxpayer?.name ?? ''} y sus expedientes, facturas y relaciones locales. Los documentos de la biblioteca no se borran del disco. Esta acción no se puede deshacer.`"
      confirm-label="Eliminar contribuyente"
      tone="danger"
      :busy="deleting"
      @update:model-value="onDeleteDialogUpdate"
      @confirm="confirmDelete"
    />
  </div>
</template>
