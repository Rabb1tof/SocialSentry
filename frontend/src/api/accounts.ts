import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/api/client"

export interface ConnectedAccount {
  id: string
  user_id: string
  platform: "instagram" | "vk"
  platform_id: string
  display_name?: string | null
  avatar_url?: string | null
  page_id?: string | null
  extra?: Record<string, unknown> | null
  is_active: boolean
  status: "running" | "paused" | "error"
  status_message?: string | null
  created_at?: string
  updated_at?: string
}

interface ListEnvelope<T> {
  data: T[]
}
interface DataEnvelope<T> {
  data: T
}

export function useAccounts() {
  return useQuery({
    queryKey: ["accounts"],
    queryFn: async () => {
      const resp = await api.get<ListEnvelope<ConnectedAccount>>("/accounts")
      return resp.data.data
    },
    staleTime: 30_000,
  })
}

export function useAccount(id: string | undefined) {
  return useQuery({
    queryKey: ["accounts", id],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<ConnectedAccount>>(`/accounts/${id}`)
      return resp.data.data
    },
    enabled: !!id,
  })
}

export function useDeleteAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      await api.delete(`/accounts/${id}`)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["accounts"] })
    },
  })
}

export function useSetAccountStatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { id: string; active: boolean }) => {
      await api.patch(`/accounts/${vars.id}/status`, { active: vars.active })
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["accounts"] })
      qc.invalidateQueries({ queryKey: ["accounts", vars.id] })
    },
  })
}

export function useConnectInstagram() {
  return useMutation({
    mutationFn: async () => {
      const resp = await api.post<DataEnvelope<{ auth_url: string; state: string }>>(
        "/accounts/instagram/connect",
      )
      return resp.data.data
    },
  })
}

export function useConnectVK() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { group_id: string; community_token: string }) => {
      const resp = await api.post<DataEnvelope<ConnectedAccount>>(
        "/accounts/vk/connect",
        vars,
      )
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["accounts"] })
    },
  })
}
