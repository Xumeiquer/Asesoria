import { defineStore } from 'pinia'
import { api, ensureSession } from '@/api'
import type { AppMeta, Taxpayer } from '@/types'

const STORAGE_KEY = 'asesoria.taxpayer'

export const useAppStore = defineStore('app', {
  state: () => ({
    taxpayers: [] as Taxpayer[],
    selectedId: localStorage.getItem(STORAGE_KEY) ?? '',
    meta: null as AppMeta | null,
    ready: false,
    loading: false,
  }),
  getters: {
    selectedTaxpayer(state): Taxpayer | null {
      return state.taxpayers.find((t) => t.id === state.selectedId) ?? null
    },
    hasTaxpayers(state): boolean {
      return state.taxpayers.length > 0
    },
  },
  actions: {
    async init() {
      if (this.ready) return
      this.loading = true
      try {
        await ensureSession()
        await Promise.all([this.loadTaxpayers(), this.loadMeta()])
        this.ready = true
      } finally {
        this.loading = false
      }
    },
    async loadTaxpayers() {
      this.taxpayers = await api.get<Taxpayer[]>('/taxpayers')
      if (!this.taxpayers.some((t) => t.id === this.selectedId)) {
        this.selectedId = this.taxpayers[0]?.id ?? ''
        localStorage.setItem(STORAGE_KEY, this.selectedId)
      }
    },
    async loadMeta() {
      this.meta = await api.get<AppMeta>('/meta')
    },
    select(id: string) {
      this.selectedId = id
      localStorage.setItem(STORAGE_KEY, id)
    },
  },
})
