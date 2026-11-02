import { create } from 'zustand'
import { api, type User } from '../../lib/api'

interface AuthState {
  token: string | null
  user: User | null
  setSession: (token: string, user: User) => void
  setUser: (user: User | null) => void
  clearSession: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: api.getToken(),
  user: null,
  setSession: (token, user) => {
    api.setToken(token)
    set({ token, user })
  },
  setUser: (user) => set({ user }),
  clearSession: () => {
    api.setToken(null)
    set({ token: null, user: null })
  },
  isAuthenticated: () => Boolean(get().token || api.getToken()),
}))
