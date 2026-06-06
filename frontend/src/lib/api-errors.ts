// Helpers for mapping the backend's standard error envelope into friendly UI messages.
// The envelope shape is { error: string, message: string, ... }.

import type { AxiosError } from "axios"

export interface ApiErrorEnvelope {
  error: string
  message: string
  subscription_status?: string
  details?: Record<string, unknown>
}

export function apiError(e: unknown): ApiErrorEnvelope | null {
  const ax = e as AxiosError<ApiErrorEnvelope> | undefined
  return ax?.response?.data ?? null
}

export function isErrorCode(e: unknown, code: string): boolean {
  return apiError(e)?.error === code
}

const planLimitMessages: Record<string, string> = {
  limit_exceeded:
    "Лимит для текущего тарифа достигнут. Перейдите на более высокий тариф или удалите неиспользуемые ресурсы.",
  platform_not_allowed:
    "Ваш тариф не разрешает одновременно подключать аккаунты разных платформ. Откройте подписку Pro или Enterprise, чтобы соединить и Instagram, и VK.",
  conflict: "Этот ресурс уже подключён.",
  subscription_required:
    "Нужна активная подписка, чтобы выполнить это действие. Обратитесь к администратору для выдачи доступа.",
}

// friendlyPlanError returns a UI-ready message for plan / subscription related
// error codes, or null when the error is not one of those. Callers typically pair
// this with a follow-up to render a dialog rather than a toast.
export function friendlyPlanError(e: unknown): string | null {
  const env = apiError(e)
  if (!env) return null
  const known = planLimitMessages[env.error]
  if (!known) return null
  return known
}

export function friendlyErrorMessage(e: unknown, fallback = "Что-то пошло не так"): string {
  const env = apiError(e)
  if (!env) return fallback
  return planLimitMessages[env.error] ?? env.message ?? fallback
}
