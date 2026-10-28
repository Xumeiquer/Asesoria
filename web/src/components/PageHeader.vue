<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
    <div>
      <h1 class="text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl">{{ title }}</h1>
      <p v-if="subtitle" class="mt-1.5 max-w-2xl text-sm leading-relaxed text-slate-500">
        {{ subtitle }}
      </p>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
