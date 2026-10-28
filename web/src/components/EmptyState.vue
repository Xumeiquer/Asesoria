<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <div class="card flex flex-col items-center justify-center px-6 py-14 text-center">
    <div class="grid h-12 w-12 place-items-center rounded-2xl bg-slate-100 text-slate-400">
      <slot name="icon" />
    </div>
    <h3 class="mt-4 text-base font-bold text-slate-900">{{ title }}</h3>
    <p v-if="description" class="mt-1.5 max-w-md text-sm leading-relaxed text-slate-500">
      {{ description }}
    </p>
    <div v-if="$slots.action" class="mt-5">
      <slot name="action" />
    </div>
  </div>
</template>
