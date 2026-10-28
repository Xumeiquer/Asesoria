<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Check, Copy, KeyRound, Link2, Lock, RefreshCw, ShieldCheck, Trash2 } from 'lucide-vue-next'
import PageHeader from '@/components/PageHeader.vue'
import { api, dateTimeLabel } from '@/api'
import { useAppStore } from '@/stores/app'
import { useToasts } from '@/stores/toasts'
import type { ApiToken, PairingCode } from '@/types'

const store = useAppStore()
const toasts = useToasts()
const codes = ref<PairingCode | null>(null)
const tokens = ref<ApiToken[]>([])
const loading = ref(false)
const copied = ref(false)

async function load() {
  try {
    const [t] = await Promise.all([api.get<ApiToken[]>('/pairing/tokens'), store.loadMeta()])
    tokens.value = t
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo cargar la configuración')
  }
}

async function generate() {
  loading.value = true
  try {
    codes.value = await api.post<PairingCode>('/pairing/codes')
    copied.value = false
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo generar el código')
  } finally {
    loading.value = false
  }
}

async function copyCode() {
  if (!codes.value) return
  try {
    await navigator.clipboard.writeText(codes.value.code)
    copied.value = true
    toasts.success('Código copiado')
    window.setTimeout(() => (copied.value = false), 2500)
  } catch {
    toasts.error('No se pudo copiar el código')
  }
}

async function revoke(token: ApiToken) {
  if (!window.confirm(`¿Revocar el acceso de "${token.name}"? La extensión dejará de poder leer datos.`)) return
  try {
    await api.del(`/pairing/tokens/${token.id}`)
    toasts.success('Acceso revocado')
    await load()
  } catch (error) {
    toasts.error(error instanceof Error ? error.message : 'No se pudo revocar el acceso')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Ajustes y conexión"
      subtitle="La aplicación escucha solo en 127.0.0.1 y guarda todo en tu equipo. La extensión se conecta mediante un token revocable."
    />

    <div class="grid gap-6 lg:grid-cols-3">
      <section class="space-y-5 lg:col-span-2">
        <div class="card card-pad">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
                <Link2 class="h-4 w-4" /> Conectar la extensión
              </h2>
              <p class="mt-1.5 text-sm text-slate-500">
                Genera un código de un solo uso y pégalo en el popup de la extensión. Caduca en 10 minutos.
              </p>
            </div>
            <button class="btn-primary shrink-0" :disabled="loading" @click="generate">
              <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loading }" />
              Generar código
            </button>
          </div>

          <div v-if="codes" class="mt-5 rounded-2xl border border-emerald-200 bg-emerald-50/60 p-5">
            <p class="text-xs font-bold uppercase tracking-wider text-emerald-700">Código de emparejamiento</p>
            <div class="mt-2 flex flex-wrap items-center gap-3">
              <span class="font-mono text-4xl font-bold tracking-[0.25em] text-emerald-900">{{ codes.code }}</span>
              <button class="btn-ghost" @click="copyCode">
                <Check v-if="copied" class="h-4 w-4 text-emerald-600" />
                <Copy v-else class="h-4 w-4" />
                {{ copied ? 'Copiado' : 'Copiar' }}
              </button>
            </div>
            <p class="mt-2 text-xs text-emerald-700">Caduca el {{ dateTimeLabel(codes.expires_at) }}.</p>
          </div>

          <ol class="mt-5 grid gap-3 text-sm text-slate-600">
            <li class="flex gap-3">
              <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">1</span>
              <span>Compila la extensión: <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">cd extension && npm install && npm run build</code>.</span>
            </li>
            <li class="flex gap-3">
              <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">2</span>
              <span>Abre <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">chrome://extensions</code>, activa el modo desarrollador y carga la carpeta <code class="rounded bg-slate-100 px-1.5 py-0.5 text-xs">extension/dist</code>.</span>
            </li>
            <li class="flex gap-3">
              <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-slate-900 text-xs font-bold text-white">3</span>
              <span>Abre el popup, pega el código y pulsa Conectar. El token aparece abajo y puedes revocarlo cuando quieras.</span>
            </li>
          </ol>
        </div>

        <div class="card">
          <div class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
            <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
              <KeyRound class="h-4 w-4" /> Accesos de extensiones
            </h2>
            <span class="chip bg-slate-100 text-slate-600">{{ tokens.length }} {{ tokens.length === 1 ? 'activo' : 'activos' }}</span>
          </div>
          <div v-if="tokens.length" class="table-wrap">
            <table class="table">
              <thead>
                <tr>
                  <th>Nombre</th>
                  <th>Creado</th>
                  <th>Último uso</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="token in tokens" :key="token.id">
                  <td class="font-medium text-slate-800">{{ token.name }}</td>
                  <td class="text-xs text-slate-500">{{ dateTimeLabel(token.created_at) }}</td>
                  <td class="text-xs text-slate-500">{{ token.last_used_at ? dateTimeLabel(token.last_used_at) : 'Nunca' }}</td>
                  <td class="text-right">
                    <button class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600" @click="revoke(token)">
                      <Trash2 class="h-4 w-4" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="px-5 py-8 text-center text-sm text-slate-500">
            Todavía no hay ninguna extensión conectada.
          </p>
        </div>

        <div class="card card-pad">
          <h2 class="flex items-center gap-2 text-sm font-bold uppercase tracking-wider text-slate-500">
            <ShieldCheck class="h-4 w-4" /> Cómo protegemos tus datos
          </h2>
          <ul class="mt-3 space-y-2 text-sm text-slate-600">
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> La API solo acepta peticiones de localhost y de orígenes de extensión autorizados.</li>
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> Los tokens se guardan hasheados; el token original solo se muestra una vez a la extensión.</li>
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> La aplicación nunca accede al certificado ni a la clave privada: la firma ocurre siempre en el navegador.</li>
            <li class="flex gap-2"><Lock class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /> Puedes revocar el acceso de la extensión en cualquier momento.</li>
          </ul>
        </div>
      </section>

      <aside class="space-y-4">
        <div class="card card-pad">
          <h2 class="text-sm font-bold uppercase tracking-wider text-slate-500">Entorno local</h2>
          <dl class="mt-4 space-y-3 text-sm">
            <div>
              <dt class="text-xs text-slate-400">Versión</dt>
              <dd class="font-medium text-slate-800">{{ store.meta?.version ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Catálogo</dt>
              <dd class="font-medium text-slate-800">{{ store.meta?.catalog_version ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Calculadoras</dt>
              <dd class="font-medium text-slate-800">{{ store.meta?.calculators.join(', ') ?? '—' }}</dd>
            </div>
            <div>
              <dt class="text-xs text-slate-400">Directorio de datos</dt>
              <dd class="break-all rounded-lg bg-slate-50 p-2 font-mono text-xs text-slate-600">{{ store.meta?.data_dir ?? '—' }}</dd>
            </div>
          </dl>
          <p class="mt-4 text-xs leading-relaxed text-slate-500">
            Copia esta carpeta para hacer una copia de seguridad. Contiene la base SQLite y los documentos subidos.
          </p>
        </div>

        <div class="rounded-2xl bg-slate-900 p-5 text-white">
          <p class="text-xs font-bold uppercase tracking-wider text-slate-400">Recordatorio legal</p>
          <p class="mt-2 text-sm leading-relaxed text-slate-300">
            Esta herramienta te ayuda a organizar y calcular, pero no sustituye el asesoramiento profesional.
            Verifica siempre las casillas y los plazos antes de firmar.
          </p>
        </div>
      </aside>
    </div>
  </div>
</template>
