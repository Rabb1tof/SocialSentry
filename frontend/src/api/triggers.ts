import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/api/client"

export type EventType = "dm" | "comment" | "comment_and_dm"
export type MatchMode = "keyword" | "all" | "regex"
export type KeywordsMode = "contains" | "exact" | "starts_with"

export interface Trigger {
  id: string
  account_id: string
  name: string
  is_active: boolean
  event_type: EventType
  match_mode: MatchMode
  keywords: string[]
  keywords_mode: KeywordsMode
  case_sensitive: boolean
  reply_to_comment: boolean
  reply_comment_text?: string | null
  send_private_reply: boolean
  private_reply_text?: string | null
  send_dm: boolean
  dm_text?: string | null
  check_subscription: boolean
  reply_if_subscribed?: string | null
  reply_if_unsubscribed?: string | null
  cooldown_seconds: number
  max_replies_per_user: number
  priority: number
  reply_delay_seconds: number
  created_at?: string
  updated_at?: string
}

export type TriggerInput = Omit<Trigger, "id" | "account_id" | "created_at" | "updated_at">

interface ListEnvelope<T> {
  data: T[]
}
interface DataEnvelope<T> {
  data: T
}

export function useTriggers(accountId: string | undefined) {
  return useQuery({
    queryKey: ["triggers", accountId],
    queryFn: async () => {
      const resp = await api.get<ListEnvelope<Trigger>>(`/accounts/${accountId}/triggers`)
      return resp.data.data
    },
    enabled: !!accountId,
  })
}

export function useTrigger(accountId: string | undefined, triggerId: string | undefined) {
  return useQuery({
    queryKey: ["triggers", accountId, triggerId],
    queryFn: async () => {
      const resp = await api.get<DataEnvelope<Trigger>>(
        `/accounts/${accountId}/triggers/${triggerId}`,
      )
      return resp.data.data
    },
    enabled: !!accountId && !!triggerId,
  })
}

export function useCreateTrigger(accountId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: TriggerInput) => {
      const resp = await api.post<DataEnvelope<Trigger>>(
        `/accounts/${accountId}/triggers`,
        input,
      )
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["triggers", accountId] })
    },
  })
}

export function useUpdateTrigger(accountId: string, triggerId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: TriggerInput) => {
      const resp = await api.put<DataEnvelope<Trigger>>(
        `/accounts/${accountId}/triggers/${triggerId}`,
        input,
      )
      return resp.data.data
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["triggers", accountId] })
      qc.invalidateQueries({ queryKey: ["triggers", accountId, triggerId] })
    },
  })
}

export function useDeleteTrigger(accountId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (triggerId: string) => {
      await api.delete(`/accounts/${accountId}/triggers/${triggerId}`)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["triggers", accountId] })
    },
  })
}

export function useToggleTrigger(accountId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (vars: { triggerId: string; is_active: boolean }) => {
      await api.patch(`/accounts/${accountId}/triggers/${vars.triggerId}/toggle`, {
        is_active: vars.is_active,
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["triggers", accountId] })
    },
  })
}

// triggerToInput strips the server-managed fields from a Trigger so we can PUT it back.
// Used by useReorderTriggers and any caller that wants to update just one or two fields
// without re-deriving the whole TriggerInput shape.
function triggerToInput(t: Trigger, overrides?: Partial<TriggerInput>): TriggerInput {
  return {
    name: t.name,
    is_active: t.is_active,
    event_type: t.event_type,
    match_mode: t.match_mode,
    keywords: t.keywords,
    keywords_mode: t.keywords_mode,
    case_sensitive: t.case_sensitive,
    reply_to_comment: t.reply_to_comment,
    reply_comment_text: t.reply_comment_text ?? null,
    send_private_reply: t.send_private_reply,
    private_reply_text: t.private_reply_text ?? null,
    send_dm: t.send_dm,
    dm_text: t.dm_text ?? null,
    check_subscription: t.check_subscription,
    reply_if_subscribed: t.reply_if_subscribed ?? null,
    reply_if_unsubscribed: t.reply_if_unsubscribed ?? null,
    cooldown_seconds: t.cooldown_seconds,
    max_replies_per_user: t.max_replies_per_user,
    priority: t.priority,
    reply_delay_seconds: t.reply_delay_seconds,
    ...overrides,
  }
}

// useReorderTriggers persists a new priority order by PUTting each changed trigger.
// Pass the post-drag array of {trigger, priority} pairs — only rows whose priority
// actually changed should be included to avoid unnecessary writes.
export function useReorderTriggers(accountId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (
      updates: { trigger: Trigger; priority: number }[],
    ): Promise<void> => {
      if (updates.length === 0) return
      await Promise.all(
        updates.map(({ trigger, priority }) =>
          api.put<DataEnvelope<Trigger>>(
            `/accounts/${accountId}/triggers/${trigger.id}`,
            triggerToInput(trigger, { priority }),
          ),
        ),
      )
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["triggers", accountId] })
    },
  })
}

export interface TestReply {
  channel: "comment_reply" | "private_reply" | "dm"
  text: string
}

export interface TestResult {
  event_type_matched: boolean
  text_matched: boolean
  matched_keyword?: string
  would_fire: boolean
  replies: TestReply[]
}

export interface TestInput {
  text: string
  sender_id?: string
  sender_name?: string
  sender_username?: string
  kind: "dm" | "comment"
}

export function useTestTrigger(accountId: string, triggerId: string) {
  return useMutation({
    mutationFn: async (input: TestInput) => {
      const resp = await api.post<DataEnvelope<TestResult>>(
        `/accounts/${accountId}/triggers/${triggerId}/test`,
        input,
      )
      return resp.data.data
    },
  })
}

export interface TriggerLog {
  id: string
  trigger_id: string
  account_id: string
  event_type: string
  platform_event_id?: string | null
  sender_id: string
  sender_username?: string | null
  incoming_text?: string | null
  matched_keyword?: string | null
  action_taken: string
  error_message?: string | null
  created_at: string
}

interface PaginatedListEnvelope<T> {
  data: T[]
  meta: { limit: number; offset: number; total: number }
}

export function useAccountLogs(accountId: string | undefined, limit = 50, offset = 0) {
  return useQuery({
    queryKey: ["logs", "account", accountId, { limit, offset }],
    queryFn: async () => {
      const resp = await api.get<PaginatedListEnvelope<TriggerLog>>(
        `/accounts/${accountId}/logs`,
        { params: { limit, offset } },
      )
      return resp.data
    },
    enabled: !!accountId,
  })
}

// useRecentLogs — recent activity across all of the user's accounts (dashboard feed).
// Backed by GET /logs/recent (auth-only). Live-invalidated by useRealtime on the
// ["logs","recent"] key.
export function useRecentLogs(limit = 20) {
  return useQuery({
    queryKey: ["logs", "recent"],
    queryFn: async () => {
      const resp = await api.get<ListEnvelope<TriggerLog>>("/logs/recent", {
        params: { limit },
      })
      return resp.data.data
    },
  })
}
