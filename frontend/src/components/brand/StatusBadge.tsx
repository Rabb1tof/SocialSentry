import { Badge } from "@/components/ui/badge"

// StatusBadge — connected-account state: paused / error / running (with a live dot).
export function StatusBadge({ status, active }: { status: string; active: boolean }) {
  if (!active) return <Badge variant="soft">пауза</Badge>
  if (status === "error") return <Badge variant="destructive">ошибка</Badge>
  return (
    <Badge variant="success" className="gap-1.5">
      <span className="h-1.5 w-1.5 rounded-full bg-white" />
      работает
    </Badge>
  )
}
