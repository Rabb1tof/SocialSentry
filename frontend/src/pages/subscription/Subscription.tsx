import { Crown, Info } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { PageHead } from "@/components/brand/PageHead"
import { PLAN_LABEL, fmtDate } from "@/lib/labels"
import { useMySubscription } from "@/api/subscription"
import { useMe } from "@/api/auth"

export function SubscriptionPage() {
  const { data, isLoading, isError } = useMySubscription()
  const { data: me } = useMe()
  const isAdmin = me?.role === "admin"

  return (
    <div className="max-w-[560px]">
      <PageHead title="Подписка" sub="Доступ к подключению аккаунтов и автоответам." />

      {isLoading && <Skeleton className="h-40 w-full" />}
      {isError && (
        <Card className="py-4 text-center text-sm text-destructive">Не удалось загрузить подписку.</Card>
      )}

      {data && data.status !== "active" && (
        <Card className="p-[22px]">
          <div className="mb-2 flex items-center gap-2">
            <span className="text-lg font-bold">Нет активной подписки</span>
            <Badge variant="warning">неактивна</Badge>
          </div>
          <p className="text-sm leading-relaxed text-muted-foreground">
            Подписки выдаются только администратором платформы. Свяжитесь с ним, чтобы получить
            доступ к подключению аккаунтов, созданию триггеров и отправке автоответов.
          </p>
        </Card>
      )}

      {data && data.status === "active" && data.subscription && (
        <>
          <Card className="p-[22px]">
            <div className="mb-1 flex items-center gap-3">
              <span className="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-azure text-white">
                <Crown className="h-[22px] w-[22px]" />
              </span>
              <div>
                <div className="text-lg font-bold">
                  {PLAN_LABEL[data.subscription.plan] ?? data.subscription.plan}
                </div>
                <Badge variant="success">активна</Badge>
              </div>
            </div>
            <div className="mt-3.5">
              <Row label="План" value={PLAN_LABEL[data.subscription.plan] ?? data.subscription.plan} />
              <Row label="Начало" value={fmtDate(data.subscription.starts_at, true)} />
              <Row
                label="Истекает"
                value={data.subscription.expires_at ? fmtDate(data.subscription.expires_at, true) : "бессрочно"}
              />
              {isAdmin && data.subscription.note && (
                <div className="flex justify-between gap-4 py-2.5">
                  <span className="text-muted-foreground">Заметка администратора</span>
                  <span className="max-w-[280px] text-right font-medium text-muted-foreground">
                    {data.subscription.note}
                  </span>
                </div>
              )}
            </div>
          </Card>
          <p className="mt-3.5 flex items-start gap-1.5 text-[13px] leading-relaxed text-muted-foreground">
            <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            Подписки выдаёт администратор платформы. Чтобы изменить план — свяжитесь с ним.
          </p>
        </>
      )}
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4 border-b py-2.5 last:border-b-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  )
}
