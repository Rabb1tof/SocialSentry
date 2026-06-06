import { Badge } from "@/components/ui/badge"
import { ACTION_LABEL, DELIVERED_ACTIONS } from "@/lib/labels"

// ActionBadge — trigger-log "action_taken": delivered / skipped / error / other.
export function ActionBadge({ value }: { value: string }) {
  if ((DELIVERED_ACTIONS as readonly string[]).includes(value)) {
    return <Badge variant="success">{ACTION_LABEL[value] ?? value}</Badge>
  }
  if (value === "skipped") return <Badge variant="warning">пропущено</Badge>
  if (value === "error") return <Badge variant="destructive">ошибка</Badge>
  return <Badge variant="soft">{value}</Badge>
}
