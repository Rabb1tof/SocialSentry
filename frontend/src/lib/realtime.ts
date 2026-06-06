import { useEffect, useRef } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { useAuthStore } from "@/store/auth"

// Mirrors the backend queue.TriggerFiredEvent payload.
interface TriggerFiredEvent {
  account_id: string
  event_type: string
  action_taken: string
  sender_username?: string
  created_at: string
}

// Actions that actually delivered something — only these surface a toast.
const DELIVERED = new Set(["sent_dm", "replied_comment", "both"])

const MAX_BACKOFF_MS = 30_000

// useRealtime opens a WebSocket to the API for the duration the user is authenticated.
// On every trigger-log event for one of the user's accounts it invalidates that account's
// logs query (so the table refetches live) and toasts on a successful reply. Reconnects
// with exponential backoff; tears down on logout/unmount.
export function useRealtime() {
  const token = useAuthStore((s) => s.accessToken)
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!token) return

    let closedByUs = false
    let attempts = 0

    const connect = () => {
      const proto = window.location.protocol === "https:" ? "wss" : "ws"
      const url = `${proto}://${window.location.host}/ws?token=${encodeURIComponent(token)}`
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => {
        attempts = 0
      }

      ws.onmessage = (e) => {
        let evt: TriggerFiredEvent
        try {
          evt = JSON.parse(e.data as string)
        } catch {
          return
        }
        if (!evt.account_id) return

        queryClient.invalidateQueries({ queryKey: ["logs", "account", evt.account_id] })
        queryClient.invalidateQueries({ queryKey: ["logs", "recent"] })

        if (DELIVERED.has(evt.action_taken)) {
          toast.success("Новое срабатывание", {
            description: evt.sender_username
              ? `Ответ отправлен @${evt.sender_username}`
              : `Тип события: ${evt.event_type}`,
          })
        }
      }

      ws.onclose = () => {
        if (closedByUs) return
        attempts += 1
        const delay = Math.min(MAX_BACKOFF_MS, 1000 * 2 ** Math.min(attempts, 5))
        reconnectRef.current = setTimeout(connect, delay)
      }

      ws.onerror = () => {
        // Let onclose drive the reconnect.
        ws.close()
      }
    }

    connect()

    return () => {
      closedByUs = true
      if (reconnectRef.current) clearTimeout(reconnectRef.current)
      wsRef.current?.close()
    }
  }, [token, queryClient])
}
