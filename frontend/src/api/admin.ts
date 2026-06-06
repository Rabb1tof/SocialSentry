import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/api/client"
import type { Subscription } from "@/api/subscription"

export interface AdminUser {
  id: string
  email: string
  role: "user" | "admin"
  is_blocked: boolean
  created_at?: string
  updated_at?: string
}

export interface PaginatedMeta {
  total: number
  limit: number
  offset: number
}

export interface AdminStats {
  total_users: number
  active_subscriptions: number
  active_accounts: number
}

interface ListEnvelope<T> {
  data: T[]
  meta: PaginatedMeta
}
interface DataEnvelope<T> {
  data: T
}

export function useAdminUsers(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["admin", "users", { limit, offset }],
    queryFn: async () => {
      const resp = await api.get<ListEnvelope<AdminUser>>("/admin/users", {
        params: { limit, offset },
      })
      return resp.data
    },
  })
}

export interface AdminCreateUserInput {
  email: string
  password: string
  role: "user" | "admin"
}

export function useAdminCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: AdminCreateUserInput) => {
      const resp = await api.post<DataEnvelope<AdminUser>>("/admin/users", vars)
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin", "users"] })
      qc.invalidateQueries({ queryKey: ["admin", "stats"] })
    },
  })
}

export function useAdminPatchUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      id: string
      role?: string
      is_blocked?: boolean
      email?: string
      password?: string
    }) => {
      const { id, ...body } = vars
      await api.patch(`/admin/users/${id}`, body)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin", "users"] })
      qc.invalidateQueries({ queryKey: ["admin", "stats"] })
    },
  })
}

export function useAdminSubscriptions(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["admin", "subscriptions", { limit, offset }],
    queryFn: async () => {
      const resp = await api.get<ListEnvelope<Subscription>>("/admin/subscriptions", {
        params: { limit, offset },
      })
      return resp.data
    },
  })
}

export function useGrantSubscription() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      user_id: string
      plan: "basic" | "pro" | "enterprise"
      expires_at?: string | null
      note?: string | null
    }) => {
      const resp = await api.post<DataEnvelope<Subscription>>("/admin/subscriptions", vars)
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin", "subscriptions"] })
      qc.invalidateQueries({ queryKey: ["admin", "stats"] })
      qc.invalidateQueries({ queryKey: ["subscription", "me"] })
    },
  })
}

export function useUpdateSubscription() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: {
      id: string
      plan: "basic" | "pro" | "enterprise"
      is_active: boolean
      expires_at?: string | null
      note?: string | null
    }) => {
      const { id, ...body } = vars
      const resp = await api.patch<DataEnvelope<Subscription>>(`/admin/subscriptions/${id}`, body)
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin", "subscriptions"] })
      qc.invalidateQueries({ queryKey: ["subscription", "me"] })
    },
  })
}

export function useRevokeSubscription() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      await api.delete(`/admin/subscriptions/${id}`)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin", "subscriptions"] })
      qc.invalidateQueries({ queryKey: ["admin", "stats"] })
      qc.invalidateQueries({ queryKey: ["subscription", "me"] })
    },
  })
}

export function useAdminStats(enabled = true) {
  return useQuery({
    queryKey: ["admin", "stats"],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<AdminStats>>("/admin/stats")
      return resp.data.data
    },
    enabled,
  })
}

export type Platform = "instagram" | "vk"

export interface PlatformSetting {
  platform: Platform
  enabled: boolean
  updated_at?: string
}

// usePlatformSettings — admin view of every platform's on/off state.
export function usePlatformSettings() {
  return useQuery({
    queryKey: ["admin", "platform-settings"],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<PlatformSetting[]>>("/admin/platform-settings")
      return resp.data.data
    },
  })
}

export function useSetPlatformEnabled() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { platform: Platform; enabled: boolean }) => {
      const resp = await api.patch<DataEnvelope<PlatformSetting>>(
        `/admin/platform-settings/${vars.platform}`,
        { enabled: vars.enabled },
      )
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin", "platform-settings"] })
      qc.invalidateQueries({ queryKey: ["platform", "availability"] })
    },
  })
}

export interface PlatformAvailability {
  instagram: boolean
  vk: boolean
}

// usePlatformAvailability — auth-only read used by the connect UI to hide disabled platforms.
export function usePlatformAvailability() {
  return useQuery({
    queryKey: ["platform", "availability"],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<PlatformAvailability>>("/platform-settings")
      return resp.data.data
    },
  })
}
