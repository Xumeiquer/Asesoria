<script setup lang="ts">
import { CalendarDays } from 'lucide-vue-next'

defineProps<{ label: string }>()
</script>

<template>
  <div
    class="flex items-center gap-3 py-1"
    role="separator"
    :aria-label="`Cambio de ejercicio el ${label}`"
  >
    <span class="h-px flex-1 bg-slate-300" aria-hidden="true" />
    <span
      class="inline-flex items-center gap-1.5 rounded-full bg-slate-900 px-3 py-1 text-[11px] font-bold uppercase tracking-wider text-white"
    >
      <CalendarDays class="h-3.5 w-3.5" />
      Cambio de ejercicio · {{ label }}
    </span>
    <span class="h-px flex-1 bg-slate-300" aria-hidden="true" />
  </div>
</template>
