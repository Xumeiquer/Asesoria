<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle } from 'lucide-vue-next'
import Modal from '@/components/Modal.vue'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title: string
    message: string
    confirmLabel?: string
    cancelLabel?: string
    tone?: 'danger' | 'warning' | 'primary'
    busy?: boolean
  }>(),
  {
    confirmLabel: 'Confirmar',
    cancelLabel: 'Cancelar',
    tone: 'warning',
    busy: false,
  },
)
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; confirm: [] }>()

const toneClasses = computed(() => {
  switch (props.tone) {
    case 'danger':
      return 'bg-rose-50 text-rose-600'
    case 'primary':
      return 'bg-sky-50 text-sky-600'
    default:
      return 'bg-amber-50 text-amber-600'
  }
})
const confirmClasses = computed(() => {
  switch (props.tone) {
    case 'danger':
      return 'btn-danger'
    case 'primary':
      return 'btn-primary'
    default:
      return 'btn-accent'
  }
})
</script>

<template>
  <Modal
    :model-value="modelValue"
    :title="title"
    persistent
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="flex items-start gap-3">
      <span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl" :class="toneClasses">
        <AlertTriangle class="h-5 w-5" />
      </span>
      <p class="pt-1 text-sm leading-relaxed text-slate-600">{{ message }}</p>
    </div>
    <template #footer>
      <button class="btn-ghost" :disabled="busy" @click="emit('update:modelValue', false)">
        {{ cancelLabel }}
      </button>
      <button class="btn" :class="confirmClasses" :disabled="busy" @click="emit('confirm')">
        {{ busy ? 'Procesando…' : confirmLabel }}
      </button>
    </template>
  </Modal>
</template>
