import { useMemo } from "react"
import { Link } from "react-router-dom"
import { AtSign, CreditCard, Shield } from "lucide-react"

import { useMe } from "@/api/auth"
import { useAccounts, type ConnectedAccount } from "@/api/accounts"
import { useMySubscription } from "@/api/subscription"
import { useRecentLogs, type TriggerLog } from "@/api/triggers"
import { useAdminStats } from "@/api/admin"
import { Badge } from "@/components/ui/badge"
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
import { StatCard } from "@/components/brand/StatCard"
import { PlatformGlyph } from "@/components/brand/PlatformGlyph"
import { ActionBadge } from "@/components/brand/ActionBadge"
import { PLAN_LABEL, fmtDate } from "@/lib/labels"
import { useCountUp } from "@/lib/useCountUp"

const subStatusLabel: Record<string, string> = {
  active: "активна",
  expired: "истекла",
  none: "нет",
}

function accountCounts(accounts: ConnectedAccount[]) {
  return accounts.reduce(
    (acc, a) => {
      acc.total++
      if (a.status === "error") acc.error++
      else if (!a.is_active) acc.paused++
      else acc.running++
      return acc
    },
    { total: 0, running: 0, paused: 0, error: 0 },
  )
}

export function DashboardPage() {
  const { data: user } = useMe()
  const isAdmin = user?.role === "admin"
  const accounts = useAccounts()
  const subscription = useMySubscription()
  const logs = useRecentLogs(20)
  const adminStats = useAdminStats(isAdmin) // admin-only endpoint; skip the request for normal users

  const counts = useMemo(() => accountCounts(accounts.data ?? []), [accounts.data])
  const totalDisplay = useCountUp(counts.total)

  // Map account_id -> account for enriching the activity feed with platform/name.
  const accountById = useMemo(() => {
    const m = new Map<string, ConnectedAccount>()
    for (const a of accounts.data ?? []) m.set(a.id, a)
    return m
  }, [accounts.data])

  return (
    <div>
      <PageHead title="Дашборд" sub="Обзор аккаунтов, подписки и последних срабатываний." />

      <div className="stagger grid gap-3.5 sm:grid-cols-2 lg:grid-cols-3">
        {/* Accounts summary */}
        <StatCard icon={AtSign} label="Аккаунты" sub="Подключённые площадки" accent="grad">
          {accounts.isLoading ? (
            <Skeleton className="h-12 w-40" />
          ) : (
            <div className="space-y-2">
              <div className="mono text-[2rem] font-bold leading-none tracking-tight">{totalDisplay}</div>
              <div className="flex flex-wrap gap-1.5">
                <Badge variant="success">работает: {counts.running}</Badge>
                <Badge variant="soft">пауза: {counts.paused}</Badge>
                <Badge variant="destructive">ошибка: {counts.error}</Badge>
              </div>
              <Link
                to="/accounts"
                className="inline-block text-xs text-muted-foreground hover:text-foreground"
              >
                Управлять аккаунтами →
              </Link>
            </div>
          )}
        </StatCard>

        {/* Subscription */}
        <StatCard icon={CreditCard} label="Подписка" sub="Текущий статус">
          {subscription.isLoading ? (
            <Skeleton className="h-12 w-40" />
          ) : (
            <div className="space-y-2 text-sm">
              <div>
                {subscription.data?.status === "active" ? (
                  <Badge variant="success">{subStatusLabel.active}</Badge>
                ) : subscription.data?.status === "expired" ? (
                  <Badge variant="warning">{subStatusLabel.expired}</Badge>
                ) : (
                  <Badge variant="outline">{subStatusLabel.none}</Badge>
                )}
              </div>
              {subscription.data?.subscription && (
                <>
                  <div>
                    <span className="text-muted-foreground">План:</span>{" "}
                    <span className="font-medium">
                      {PLAN_LABEL[subscription.data.subscription.plan] ?? subscription.data.subscription.plan}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Истекает:</span>{" "}
                    <span className="font-medium">
                      {subscription.data.subscription.expires_at
                        ? fmtDate(subscription.data.subscription.expires_at, true)
                        : "бессрочно"}
                    </span>
                  </div>
                </>
              )}
            </div>
          )}
        </StatCard>

        {/* Platform-wide stats (admins only) */}
        {isAdmin && (
          <StatCard icon={Shield} label="Платформа" sub="Сводка по системе" accent="violet">
            {adminStats.isLoading ? (
              <Skeleton className="h-12 w-40" />
            ) : (
              <div className="space-y-1 text-sm leading-relaxed">
                <div>
                  <span className="text-muted-foreground">Пользователей:</span>{" "}
                  <span className="mono font-medium">{adminStats.data?.total_users ?? "—"}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Активных подписок:</span>{" "}
                  <span className="mono font-medium">{adminStats.data?.active_subscriptions ?? "—"}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Активных аккаунтов:</span>{" "}
                  <span className="mono font-medium">{adminStats.data?.active_accounts ?? "—"}</span>
                </div>
              </div>
            )}
          </StatCard>
        )}
      </div>

      {/* Recent activity */}
      <Card className="mt-5 overflow-hidden">
        <div className="border-b px-[18px] py-4">
          <div className="font-semibold">Последняя активность</div>
          <div className="text-[13px] text-muted-foreground">
            Свежие срабатывания по всем вашим аккаунтам
          </div>
        </div>
        <div className="overflow-x-auto">
          {logs.isLoading && <Skeleton className="m-4 h-40 w-[calc(100%-2rem)]" />}
          {logs.isError && (
            <div className="px-[18px] py-4 text-sm text-destructive">Не удалось загрузить активность.</div>
          )}
          {logs.data && logs.data.length === 0 && (
            <div className="px-[18px] py-6 text-sm text-muted-foreground">Пока нет срабатываний.</div>
          )}
          {logs.data && logs.data.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Время</TableHead>
                  <TableHead>Аккаунт</TableHead>
                  <TableHead>Тип</TableHead>
                  <TableHead>Отправитель</TableHead>
                  <TableHead>Действие</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.data.slice(0, 8).map((l: TriggerLog) => {
                  const acc = accountById.get(l.account_id)
                  return (
                    <TableRow key={l.id}>
                      <TableCell className="mono whitespace-nowrap text-xs text-muted-foreground">
                        {fmtDate(l.created_at)}
                      </TableCell>
                      <TableCell>
                        {acc ? (
                          <div className="flex items-center gap-2">
                            <PlatformGlyph kind={acc.platform} size={22} />
                            <span className="text-[13px]">{acc.display_name ?? acc.platform_id}</span>
                          </div>
                        ) : (
                          <span className="mono text-xs">{l.account_id.slice(0, 8)}</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant="soft">{l.event_type === "comment" ? "коммент" : "ЛС"}</Badge>
                      </TableCell>
                      <TableCell className="text-[13px]">
                        {l.sender_username ?? <span className="mono text-xs">{l.sender_id}</span>}
                      </TableCell>
                      <TableCell>
                        <ActionBadge value={l.action_taken} />
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </div>
      </Card>
    </div>
  )
}
