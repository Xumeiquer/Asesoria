<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'
import { X } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title: string
    subtitle?: string
    size?: 'md' | 'lg' | 'xl'
  }>(),
  { size: 'md' },
)
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

function close() {
  emit('update:modelValue', false)
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      document.addEventListener('keydown', onKey)
      document.body.style.overflow = 'hidden'
    } else {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  },
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})

const widths = { md: 'max-w-xl', lg: 'max-w-3xl', xl: 'max-w-5xl' }
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0"
      leave-active-class="transition duration-100 ease-in"
      leave-to-class="opacity-0"
    >
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-6">
        <div class="absolute inset-0 bg-slate-950/45 backdrop-blur-sm" @click="close" />
        <div
          class="relative flex max-h-[92vh] w-full flex-col overflow-hidden rounded-t-3xl bg-white shadow-2xl sm:rounded-3xl"
          :class="widths[size]"
        >
          <div class="flex items-start justify-between gap-4 border-b border-slate-100 px-6 py-5">
            <div>
              <h2 class="text-lg font-bold tracking-tight text-slate-900">{{ title }}</h2>
              <p v-if="subtitle" class="mt-1 text-sm text-slate-500">{{ subtitle }}</p>
            </div>
            <button class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600" @click="close">
              <X class="h-5 w-5" />
            </button>
          </div>
          <div class="overflow-y-auto px-6 py-5">
            <slot />
          </div>
          <div v-if="$slots.footer" class="flex items-center justify-end gap-2 border-t border-slate-100 bg-slate-50/60 px-6 py-4">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
