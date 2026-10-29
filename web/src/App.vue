<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import {
  CalendarDays,
  ChevronDown,
  FileText,
  FolderOpen,
  LayoutDashboard,
  Menu,
  Plus,
  ReceiptText,
  Settings,
  Users,
  X,
} from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import ToastStack from '@/components/ToastStack.vue'

const store = useAppStore()
const toasts = useToasts()
const route = useRoute()
const mobileOpen = ref(false)

function isActive(to: string): boolean {
  if (to === '/') return route.path === '/'
  if (to === '/calendario') {
    return route.path.startsWith('/calendario') || route.path.startsWith('/expedientes')
  }
  return route.path.startsWith(to)
}

const nav = [
  { to: '/', label: 'Inicio', icon: LayoutDashboard },
  { to: '/calendario', label: 'Calendario', icon: CalendarDays },
  { to: '/modelos', label: 'Modelos y trámites', icon: FileText },
  { to: '/documentos', label: 'Documentos', icon: FolderOpen },
  { to: '/facturas', label: 'Facturas', icon: ReceiptText },
  { to: '/contribuyentes', label: 'Contribuyentes', icon: Users },
  { to: '/ajustes', label: 'Ajustes', icon: Settings },
]

const selectedName = computed(() => store.selectedTaxpayer?.name ?? 'Sin contribuyente')

onMounted(async () => {
  try {
    await store.init()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo conectar con el backend local')
  }
})
</script>

<template>
  <div class="min-h-screen bg-slate-100">
    <!-- Mobile overlay -->
    <div
      v-if="mobileOpen"
      class="fixed inset-0 z-30 bg-slate-950/50 backdrop-blur-sm lg:hidden"
      @click="mobileOpen = false"
    />

    <!-- Sidebar -->
    <aside
      class="fixed inset-y-0 left-0 z-40 flex w-72 flex-col bg-slate-950 text-white transition-transform duration-200 lg:translate-x-0"
      :class="mobileOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex items-center justify-between px-5 pt-6">
        <RouterLink to="/" class="flex items-center gap-3" @click="mobileOpen = false">
          <span class="grid h-10 w-10 place-items-center rounded-2xl bg-emerald-500 text-slate-950">
            <span class="text-lg font-black">A</span>
          </span>
          <span>
            <span class="block text-base font-bold tracking-tight">Asesoría</span>
            <span class="block text-[11px] font-medium uppercase tracking-[0.18em] text-slate-400">
              ERP fiscal local
            </span>
          </span>
        </RouterLink>
        <button
          class="rounded-lg p-1.5 text-slate-300 hover:bg-white/10 lg:hidden"
          aria-label="Cerrar menú"
          @click="mobileOpen = false"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <nav class="mt-8 flex-1 space-y-1 overflow-y-auto px-3">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="nav-item"
          :class="{ 'nav-item-active': isActive(item.to) }"
          @click="mobileOpen = false"
        >
          <component :is="item.icon" class="h-[18px] w-[18px] shrink-0" />
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="m-3 rounded-2xl bg-white/5 p-4">
        <div class="flex items-center gap-2">
          <span
            class="h-2 w-2 rounded-full"
            :class="store.meta?.paired_tokens ? 'bg-emerald-400' : 'bg-amber-400'"
          />
          <p class="text-xs font-semibold text-slate-200">
            {{ store.meta?.paired_tokens ? 'Extensión conectada' : 'Extensión sin conectar' }}
          </p>
        </div>
        <p class="mt-1.5 text-xs leading-relaxed text-slate-400">
          {{
            store.meta?.paired_tokens
              ? 'Puedes lanzar el relleno desde el navegador.'
              : 'Empareja la extensión en Ajustes para rellenar modelos.'
          }}
        </p>
        <RouterLink
          to="/ajustes"
          class="mt-3 inline-flex text-xs font-semibold text-emerald-400 hover:text-emerald-300"
          @click="mobileOpen = false"
        >
          Gestionar conexión →
        </RouterLink>
      </div>

      <div class="px-5 pb-5 text-[11px] text-slate-400">
        v{{ store.meta?.version ?? '0.1.0' }} · catálogo {{ store.meta?.catalog_version ?? '—' }}
      </div>
    </aside>

    <!-- Main -->
    <div class="lg:pl-72">
      <header
        class="sticky top-0 z-20 border-b border-slate-200/80 bg-slate-100/80 backdrop-blur-md"
      >
        <div class="flex items-center gap-3 px-4 py-3 sm:px-6 lg:px-8">
          <button
            class="rounded-xl border border-slate-200 bg-white p-2 text-slate-600 lg:hidden"
            aria-label="Abrir menú"
            @click="mobileOpen = true"
          >
            <Menu class="h-5 w-5" />
          </button>

          <div class="relative min-w-0 flex-1 sm:max-w-md">
            <label class="sr-only" for="taxpayer-select">Contribuyente activo</label>
            <div class="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-3 py-2">
              <Users class="h-4 w-4 shrink-0 text-slate-400" />
              <select
                id="taxpayer-select"
                class="min-w-0 flex-1 appearance-none bg-transparent pr-6 text-sm font-semibold text-slate-800 outline-none"
                :value="store.selectedId"
                @change="store.select(($event.target as HTMLSelectElement).value)"
              >
                <option v-if="!store.hasTaxpayers" value="">Sin contribuyente</option>
                <option v-for="t in store.taxpayers" :key="t.id" :value="t.id">
                  {{ t.name }} · {{ t.nif }}
                </option>
              </select>
              <ChevronDown class="pointer-events-none h-4 w-4 shrink-0 text-slate-400" />
            </div>
          </div>

          <div class="ml-auto flex items-center gap-2">
            <RouterLink to="/facturas?nueva=1" class="btn-ghost hidden sm:inline-flex">
              <ReceiptText class="h-4 w-4" />
              Nueva factura
            </RouterLink>
            <RouterLink v-if="!store.hasTaxpayers" to="/contribuyentes?nuevo=1" class="btn-primary">
              <Plus class="h-4 w-4" />
              Empezar
            </RouterLink>
            <span v-else class="hidden text-right text-xs text-slate-500 md:block">
              <span class="block font-semibold text-slate-700">{{ selectedName }}</span>
              <span>Datos guardados en este equipo</span>
            </span>
          </div>
        </div>
      </header>

      <main class="mx-auto min-w-0 w-full max-w-7xl px-4 pb-16 pt-6 sm:px-6 lg:px-8">
        <RouterView v-slot="{ Component }">
          <component :is="Component" class="animate-in" />
        </RouterView>
      </main>
    </div>

    <ToastStack />
  </div>
</template>
