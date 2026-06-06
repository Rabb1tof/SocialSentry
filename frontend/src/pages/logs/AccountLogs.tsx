import { useMemo, useState } from "react"
import { useParams } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { PageHead } from "@/components/brand/PageHead"
import { FilterChips } from "@/components/brand/FilterChips"
import { ActionBadge } from "@/components/brand/ActionBadge"
import { PlatformGlyph } from "@/components/brand/PlatformGlyph"
import { DELIVERED_ACTIONS, SKIP_REASON, fmtDate } from "@/lib/labels"
import { useAccount } from "@/api/accounts"
import { useAccountLogs, type TriggerLog } from "@/api/triggers"

const PAGE = 50

type ActionFilter = "all" | "delivered" | "skipped" | "error"

const FILTER_ITEMS: { id: ActionFilter; label: string }[] = [
  { id: "all", label: "Все" },
  { id: "delivered", label: "Отправлено" },
  { id: "skipped", label: "Пропущено" },
  { id: "error", label: "Ошибки" },
]

// renderActionDetail produces the small descriptive line beneath the action badge.
// For 'skipped' rows the error_message column carries the matcher's skip reason
// (cooldown / max_replies_reached / no_action_text) — surface it as a friendlier sentence.
function renderActionDetail(l: TriggerLog): React.ReactNode {
  if (l.action_taken === "skipped" && l.error_message) {
    const label = SKIP_REASON[l.error_message] ?? l.error_message
    return <div className="mt-1 max-w-xs text-xs text-muted-foreground">{label}</div>
  }
  if (l.error_message) {
    return (
      <div className="mt-1 max-w-xs truncate text-xs text-destructive" title={l.error_message}>
        {l.error_message}
      </div>
    )
  }
  return null
}

function passesFilter(l: TriggerLog, f: ActionFilter): boolean {
  if (f === "all") return true
  if (f === "skipped") return l.action_taken === "skipped"
  if (f === "error") return l.action_taken === "error"
  // delivered = anything that actually sent something
  return (DELIVERED_ACTIONS as readonly string[]).includes(l.action_taken)
}

export function AccountLogsPage() {
  const { id: accountID } = useParams()
  const [offset, setOffset] = useState(0)
  const [filter, setFilter] = useState<ActionFilter>("all")
  const account = useAccount(accountID)
  const logs = useAccountLogs(accountID, PAGE, offset)

  // Client-side filter — the backend doesn't yet support an action_taken query param.
  const visible = useMemo(() => {
    if (!logs.data) return []
    return logs.data.data.filter((l) => passesFilter(l, filter))
  }, [logs.data, filter])

  const counts = useMemo<Record<ActionFilter, number>>(() => {
    if (!logs.data) return { all: 0, delivered: 0, skipped: 0, error: 0 }
    return logs.data.data.reduce(
      (acc, l) => {
        acc.all++
        if (passesFilter(l, "delivered")) acc.delivered++
        if (l.action_taken === "skipped") acc.skipped++
        if (l.action_taken === "error") acc.error++
        return acc
      },
      { all: 0, delivered: 0, skipped: 0, error: 0 },
    )
  }, [logs.data])

  return (
    <div>
      <PageHead
        title="Логи"
        sub={
          account.data ? (
            <span className="inline-flex items-center gap-2">
              <PlatformGlyph kind={account.data.platform} size={18} />
              {account.data.display_name ?? account.data.platform_id}
            </span>
          ) : (
            "Журнал срабатываний по аккаунту."
          )
        }
      >
        <FilterChips current={filter} items={FILTER_ITEMS} counts={counts} onChange={setFilter} />
      </PageHead>

      {logs.isLoading && <Skeleton className="h-64 w-full" />}
      {logs.isError && (
        <Card className="py-4 text-center text-sm text-destructive">Не удалось загрузить логи.</Card>
      )}

      {logs.data && (
        <Card className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Время</TableHead>
                <TableHead>Тип</TableHead>
                <TableHead>Отправитель</TableHead>
                <TableHead>Текст</TableHead>
                <TableHead>Ключ</TableHead>
                <TableHead>Действие</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visible.map((l) => (
                <TableRow key={l.id}>
                  <TableCell className="mono whitespace-nowrap text-xs text-muted-foreground">
                    {fmtDate(l.created_at)}
                  </TableCell>
                  <TableCell>
                    <Badge variant="soft">{l.event_type === "comment" ? "коммент" : "ЛС"}</Badge>
                  </TableCell>
                  <TableCell className="text-[13px]">
                    {l.sender_username ?? <span className="mono text-xs">{l.sender_id}</span>}
                  </TableCell>
                  <TableCell className="max-w-[180px] truncate text-xs text-muted-foreground">
                    {l.incoming_text}
                  </TableCell>
                  <TableCell className="mono text-xs">{l.matched_keyword ?? "—"}</TableCell>
                  <TableCell>
                    <ActionBadge value={l.action_taken} />
                    {renderActionDetail(l)}
                  </TableCell>
                </TableRow>
              ))}
              {visible.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="py-10 text-center text-sm text-muted-foreground">
                    {filter === "all"
                      ? "Пока ничего не залогировано."
                      : "В текущем фильтре нет записей. Попробуйте сменить фильтр."}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </Card>
      )}

      {logs.data && logs.data.meta.total > PAGE && (
        <div className="mt-4 flex justify-between text-sm">
          <Button
            variant="outline"
            size="sm"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE))}
          >
            ← Назад
          </Button>
          <span className="mono text-muted-foreground">
            {offset + 1}–{Math.min(offset + PAGE, logs.data.meta.total)} из {logs.data.meta.total}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={offset + PAGE >= logs.data.meta.total}
            onClick={() => setOffset(offset + PAGE)}
          >
            Вперёд →
          </Button>
        </div>
      )}
    </div>
  )
}
