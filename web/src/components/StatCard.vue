<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    value: string | number
    hint?: string
    tone?: 'default' | 'emerald' | 'amber' | 'rose' | 'sky'
  }>(),
  { tone: 'default' },
)

const tones = {
  default: 'bg-slate-100 text-slate-600',
  emerald: 'bg-emerald-50 text-emerald-600',
  amber: 'bg-amber-50 text-amber-600',
  rose: 'bg-rose-50 text-rose-600',
  sky: 'bg-sky-50 text-sky-600',
}
</script>

<template>
  <div class="card card-pad">
    <div class="flex items-start justify-between gap-3">
      <div>
        <p class="text-[11px] font-bold uppercase tracking-wider text-slate-500">{{ label }}</p>
        <p class="mt-2 text-3xl font-bold tracking-tight text-slate-900">{{ value }}</p>
        <p v-if="hint" class="mt-1 text-xs text-slate-500">{{ hint }}</p>
      </div>
      <span class="grid h-10 w-10 place-items-center rounded-xl" :class="tones[tone]">
        <slot name="icon" />
      </span>
    </div>
  </div>
</template>
