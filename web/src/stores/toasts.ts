import { reactive } from 'vue'

export type ToastTone = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  message: string
  tone: ToastTone
}

const state = reactive<{ items: Toast[] }>({ items: [] })
let counter = 0

export function useToasts() {
  function push(message: string, tone: ToastTone = 'info', ttl = 4200) {
    const id = ++counter
    state.items.push({ id, message, tone })
    window.setTimeout(() => dismiss(id), ttl)
  }
  function dismiss(id: number) {
    const index = state.items.findIndex((t) => t.id === id)
    if (index >= 0) state.items.splice(index, 1)
  }
  return {
    toasts: state.items,
    push,
    dismiss,
    success: (message: string) => push(message, 'success'),
    error: (message: string) => push(message, 'error', 6500),
  }
}
