<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowUpRight, CalendarClock } from 'lucide-vue-next'
import StatusBadge from '@/components/StatusBadge.vue'
import type { Filing } from '@/types'
import { dateLabel, daysUntil } from '@/api'

const props = withDefaults(defineProps<{ filing: Filing; showEntity?: boolean }>(), {
  showEntity: false,
})

const days = computed(() => daysUntil(props.filing.due_date))
const isDone = computed(() => props.filing.status === 'presented' || props.filing.status === 'not_applicable')
const dueTone = computed(() => {
  if (isDone.value) return 'text-slate-400'
  if (days.value < 0) return 'text-rose-600'
  if (days.value <= 7) return 'text-amber-600'
  return 'text-slate-500'
})
const dueText = computed(() => {
  if (isDone.value) return dateLabel(props.filing.due_date)
  if (days.value < 0) return `Vencido hace ${Math.abs(days.value)} días`
  if (days.value === 0) return 'Vence hoy'
  return `Vence en ${days.value} días`
})
</script>

<template>
  <RouterLink
    :to="`/expedientes/${filing.id}`"
    class="group flex items-center gap-3 rounded-2xl border border-slate-200/80 bg-white p-3.5 transition hover:border-slate-300 hover:shadow-sm sm:gap-4 sm:p-4"
  >
    <div
      class="grid h-12 w-14 shrink-0 place-items-center rounded-xl text-white"
      :class="filing.status === 'presented' ? 'bg-emerald-600' : 'bg-slate-900'"
    >
      <span class="text-sm font-bold tracking-tight">{{ filing.model_code }}</span>
    </div>

    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <p class="truncate text-sm font-semibold text-slate-900">{{ filing.model_name }}</p>
        <span
          v-if="showEntity"
          class="chip bg-slate-100 text-slate-500"
        >{{ filing.entity }}</span>
      </div>
      <p class="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-slate-500">
        <span class="font-medium text-slate-600">{{ filing.period_label }}</span>
        <span class="text-slate-300">·</span>
        <span class="inline-flex items-center gap-1">
          <CalendarClock class="h-3.5 w-3.5" />
          {{ dateLabel(filing.due_date) }}
        </span>
        <span class="text-slate-300">·</span>
        <span :class="dueTone">{{ dueText }}</span>
      </p>
    </div>

    <div class="hidden sm:block">
      <StatusBadge :status="filing.status" />
    </div>
    <ArrowUpRight class="h-4 w-4 shrink-0 text-slate-300 transition group-hover:text-slate-600" />
  </RouterLink>
</template>
