<script setup lang="ts">
import { computed } from 'vue'
import type { FilingStatus } from '@/types'

const props = defineProps<{ status: FilingStatus }>()

const map: Record<FilingStatus, { label: string; classes: string; dot: string }> = {
  pending: { label: 'Pendiente', classes: 'bg-slate-100 text-slate-600', dot: 'bg-slate-400' },
  in_progress: { label: 'En curso', classes: 'bg-sky-50 text-sky-700', dot: 'bg-sky-500' },
  ready: { label: 'Listo', classes: 'bg-emerald-50 text-emerald-700', dot: 'bg-emerald-500' },
  presented: { label: 'Presentado', classes: 'bg-emerald-100 text-emerald-800', dot: 'bg-emerald-600' },
  late: { label: 'Fuera de plazo', classes: 'bg-rose-50 text-rose-700', dot: 'bg-rose-500' },
  not_applicable: { label: 'No aplica', classes: 'bg-slate-100 text-slate-400', dot: 'bg-slate-300' },
}

const view = computed(() => map[props.status] ?? map.pending)
</script>

<template>
  <span class="chip" :class="view.classes">
    <span class="h-1.5 w-1.5 rounded-full" :class="view.dot" />
    {{ view.label }}
  </span>
</template>
