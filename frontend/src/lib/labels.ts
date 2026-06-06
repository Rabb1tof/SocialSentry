// Centralised Russian labels + formatters shared across screens.
// Previously these maps were duplicated in Dashboard/Accounts/Logs/Subscription —
// keep a single source of truth here.

export const PLATFORM_LABEL: Record<string, string> = {
  instagram: "Instagram",
  vk: "VK",
}

export const PLAN_LABEL: Record<string, string> = {
  basic: "Basic",
  pro: "Pro",
  enterprise: "Enterprise",
}

// Trigger event types (DM / comment / both).
export const EVENT_LABEL: Record<string, string> = {
  dm: "ЛС",
  comment: "Коммент",
  comment_and_dm: "Коммент + ЛС",
}

export const MATCH_LABEL: Record<string, string> = {
  keyword: "ключи",
  all: "все",
  regex: "regex",
}

// Log "action_taken" values that mean a reply was actually delivered.
export const DELIVERED_ACTIONS = ["sent_dm", "replied_comment", "both"] as const

export const ACTION_LABEL: Record<string, string> = {
  sent_dm: "ЛС отправлено",
  replied_comment: "ответ в комменте",
  both: "коммент + ЛС",
}

// Friendly text for matcher skip reasons stored in trigger_logs.error_message.
export const SKIP_REASON: Record<string, string> = {
  cooldown: "Кулдаун ещё активен для этого пользователя",
  max_replies_reached: "Достигнут лимит ответов на одного пользователя",
  no_action_text: "Триггер сработал, но текст ответа пуст",
}

// fmtDate — ru-RU date-time. Pass `dateOnly` for the short dd.mm.yy form.
export function fmtDate(iso?: string | null, dateOnly = false): string {
  if (!iso) return "—"
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return "—"
  return dateOnly
    ? d.toLocaleDateString("ru-RU", { day: "2-digit", month: "2-digit", year: "2-digit" })
    : d.toLocaleString("ru-RU")
}
