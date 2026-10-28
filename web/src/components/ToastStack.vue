<script setup lang="ts">
import { AlertTriangle, CheckCircle2, Info, X } from 'lucide-vue-next'
import { useToasts } from '@/stores/toasts'

const { toasts, dismiss } = useToasts()
const icons = { success: CheckCircle2, error: AlertTriangle, info: Info }
const tones = {
  success: 'border-emerald-200 bg-white text-emerald-700',
  error: 'border-rose-200 bg-white text-rose-700',
  info: 'border-slate-200 bg-white text-slate-700',
}
</script>

<template>
  <div class="pointer-events-none fixed bottom-4 right-4 z-50 flex w-[min(92vw,26rem)] flex-col gap-2">
    <TransitionGroup
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="translate-y-2 opacity-0"
      leave-active-class="transition duration-150 ease-in"
      leave-to-class="translate-y-1 opacity-0"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="pointer-events-auto flex items-start gap-3 rounded-2xl border p-4 shadow-lg shadow-slate-900/5"
        :class="tones[toast.tone]"
      >
        <component :is="icons[toast.tone]" class="mt-0.5 h-5 w-5 shrink-0" />
        <p class="flex-1 text-sm font-medium leading-relaxed">{{ toast.message }}</p>
        <button class="rounded-lg p-1 text-slate-400 hover:bg-slate-100" @click="dismiss(toast.id)">
          <X class="h-4 w-4" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
