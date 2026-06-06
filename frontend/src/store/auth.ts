import { create } from "zustand"
import { persist, createJSONStorage } from "zustand/middleware"

interface AuthState {
  accessToken: string | null
  setAccessToken: (token: string | null) => void
  clear: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      setAccessToken: (token) => set({ accessToken: token }),
      clear: () => set({ accessToken: null }),
    }),
    {
      name: "socialsentry-auth",
      storage: createJSONStorage(() => localStorage),
    },
  ),
)
