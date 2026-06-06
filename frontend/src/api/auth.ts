import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/api/client"
import { useAuthStore } from "@/store/auth"

export interface User {
  id: string
  email: string
  role: string
}

interface DataEnvelope<T> {
  data: T
}

interface LoginResponse {
  access_token: string
  user: User
}

export function useMe() {
  return useQuery({
    queryKey: ["auth", "me"],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<User>>("/me")
      return resp.data.data
    },
    retry: false,
    staleTime: 60_000,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { email: string; password: string }) => {
      const resp = await api.post<DataEnvelope<LoginResponse>>("/auth/login", vars)
      return resp.data.data
    },
    onSuccess: (data) => {
      useAuthStore.getState().setAccessToken(data.access_token)
      qc.setQueryData(["auth", "me"], data.user)
    },
  })
}

export function useRegister() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { email: string; password: string }) => {
      await api.post<DataEnvelope<User>>("/auth/register", vars)
      // Auto-login after register so the caller lands on /dashboard immediately.
      const resp = await api.post<DataEnvelope<LoginResponse>>("/auth/login", vars)
      return resp.data.data
    },
    onSuccess: (data) => {
      useAuthStore.getState().setAccessToken(data.access_token)
      qc.setQueryData(["auth", "me"], data.user)
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      try {
        await api.post("/auth/logout")
      } catch {
        // Swallow — we clear local state regardless.
      }
    },
    onSettled: () => {
      useAuthStore.getState().clear()
      qc.removeQueries({ queryKey: ["auth", "me"] })
    },
  })
}
