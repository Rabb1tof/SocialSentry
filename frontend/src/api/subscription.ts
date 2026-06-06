import { useQuery } from "@tanstack/react-query"

import { api } from "@/api/client"

export interface Subscription {
  id: string
  user_id: string
  plan: "basic" | "pro" | "enterprise"
  is_active: boolean
  starts_at: string
  expires_at?: string | null
  note?: string | null
  granted_by?: string | null
  created_at: string
}

export interface MySubscriptionResponse {
  status: "active" | "expired" | "none"
  subscription: Subscription | null
}

interface DataEnvelope<T> {
  data: T
}

export function useMySubscription() {
  return useQuery({
    queryKey: ["subscription", "me"],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<MySubscriptionResponse>>("/subscription")
      return resp.data.data
    },
    staleTime: 60_000,
  })
}
