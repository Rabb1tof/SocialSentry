import { useEffect, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { toast } from "sonner"
import { Pause, Play, Plus, ScrollText, Trash2, Zap } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"
import { PageHead } from "@/components/brand/PageHead"
import { PlatformGlyph } from "@/components/brand/PlatformGlyph"
import { StatusBadge } from "@/components/brand/StatusBadge"
import { ConnectAccountDialog, vkSchema, type VKForm } from "@/components/brand/ConnectAccountDialog"
import { PlanLimitDialog } from "@/components/PlanLimitDialog"
import { friendlyErrorMessage, friendlyPlanError } from "@/lib/api-errors"
import { PLATFORM_LABEL } from "@/lib/labels"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  useAccounts,
  useConnectInstagram,
  useConnectVK,
  useDeleteAccount,
  useSetAccountStatus,
  type ConnectedAccount,
} from "@/api/accounts"
import { usePlatformAvailability } from "@/api/admin"

export function AccountsListPage() {
  const [params, setParams] = useSearchParams()
  const [connectOpen, setConnectOpen] = useState(false)
  const [planLimit, setPlanLimit] = useState<string | null>(null)
  const accounts = useAccounts()
  const availability = usePlatformAvailability()
  const connectIG = useConnectInstagram()
  const connectVK = useConnectVK()
  const setStatus = useSetAccountStatus()
  const del = useDeleteAccount()

  // Default to enabled until availability loads, so the dialog doesn't flicker.
  const igEnabled = availability.data?.instagram ?? true
  const vkEnabled = availability.data?.vk ?? true

  const vkForm = useForm<VKForm>({
    resolver: zodResolver(vkSchema),
    defaultValues: { group_id: "", community_token: "" },
  })

  // Surface OAuth-callback results returned via query string (IG only — VK has no callback).
  useEffect(() => {
    const ok = params.get("connected")
    const err = params.get("ig_error")
    if (ok) {
      toast.success(`Аккаунт ${ok === "instagram" ? "Instagram" : ok} подключён`)
      params.delete("connected")
      params.delete("account_id")
      setParams(params, { replace: true })
    } else if (err) {
      const msg = params.get("ig_error_message") ?? err
      toast.error(`Подключение Instagram не удалось: ${msg}`)
      params.delete("ig_error")
      params.delete("ig_error_message")
      setParams(params, { replace: true })
    }
  }, [params, setParams])

  const onConnectIG = () => {
    connectIG.mutate(undefined, {
      onSuccess: (data) => {
        window.location.href = data.auth_url
      },
      onError: (e) => {
        const planMsg = friendlyPlanError(e)
        if (planMsg) {
          setConnectOpen(false)
          setPlanLimit(planMsg)
          return
        }
        toast.error(friendlyErrorMessage(e, "Не удалось начать OAuth"))
      },
    })
  }

  const onConnectVK = (vals: VKForm) => {
    connectVK.mutate(vals, {
      onSuccess: () => {
        toast.success("VK сообщество подключено")
        vkForm.reset()
        setConnectOpen(false)
      },
      onError: (e) => {
        const planMsg = friendlyPlanError(e)
        if (planMsg) {
          setConnectOpen(false)
          setPlanLimit(planMsg)
          return
        }
        toast.error(friendlyErrorMessage(e, "VK отклонил токен / group_id"))
      },
    })
  }

  // accountButtons renders the per-account action set, shared by the desktop table
  // row and the mobile card so both stay in sync.
  const accountButtons = (a: ConnectedAccount) => (
    <>
      <Button asChild variant="outline" size="sm">
        <Link to={`/accounts/${a.id}/triggers`}>
          <Zap className="h-3.5 w-3.5" />
          Триггеры
        </Link>
      </Button>
      <Button asChild variant="outline" size="sm">
        <Link to={`/accounts/${a.id}/logs`}>
          <ScrollText className="h-3.5 w-3.5" />
          Логи
        </Link>
      </Button>
      <Button
        variant="outline"
        size="sm"
        onClick={() =>
          setStatus.mutate(
            { id: a.id, active: !a.is_active },
            { onSuccess: () => toast.success(a.is_active ? "Пауза" : "Запущен") },
          )
        }
      >
        {a.is_active ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}
        {a.is_active ? "Пауза" : "Запустить"}
      </Button>
      <Button
        variant="ghost"
        size="sm"
        aria-label="Удалить"
        className="text-destructive hover:bg-destructive/10 hover:text-destructive"
        onClick={() => {
          if (confirm(`Удалить аккаунт ${a.display_name ?? a.platform_id}?`)) {
            del.mutate(a.id, { onSuccess: () => toast.success("Удалён") })
          }
        }}
      >
        <Trash2 className="h-3.5 w-3.5" />
      </Button>
    </>
  )

  const platformDisabled = (a: ConnectedAccount) =>
    (a.platform === "instagram" && !igEnabled) || (a.platform === "vk" && !vkEnabled)

  return (
    <div>
      <PlanLimitDialog open={!!planLimit} message={planLimit ?? ""} onClose={() => setPlanLimit(null)} />
      <ConnectAccountDialog
        open={connectOpen}
        onClose={() => setConnectOpen(false)}
        igEnabled={igEnabled}
        vkEnabled={vkEnabled}
        onConnectIG={onConnectIG}
        connectingIG={connectIG.isPending}
        vkForm={vkForm}
        onSubmitVK={onConnectVK}
        connectingVK={connectVK.isPending}
      />

      <PageHead title="Аккаунты" sub="Подключённые площадки Instagram и VK.">
        <Button variant="grad" onClick={() => setConnectOpen(true)} disabled={!igEnabled && !vkEnabled}>
          <Plus className="h-4 w-4" />
          Подключить аккаунт
        </Button>
      </PageHead>

      {(!igEnabled || !vkEnabled) && (
        <p className="mb-4 text-xs text-muted-foreground">
          {!igEnabled && !vkEnabled
            ? "Подключение Instagram и VK временно отключено администратором."
            : `Подключение ${!igEnabled ? "Instagram" : "VK"} временно отключено администратором.`}
        </p>
      )}

      {accounts.isLoading && <Skeleton className="h-48 w-full" />}
      {accounts.isError && (
        <Card className="py-4 text-center text-sm text-destructive">Не удалось загрузить аккаунты.</Card>
      )}

      {accounts.data && accounts.data.length === 0 && (
        <Card className="px-6 py-6 text-sm text-muted-foreground">
          Ещё нет подключённых аккаунтов. Нажмите «Подключить аккаунт» вверху.
        </Card>
      )}

      {accounts.data && accounts.data.length > 0 && (
        <>
          {/* Desktop: table */}
          <Card className="hidden overflow-hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Платформа</TableHead>
                  <TableHead>Имя / Page ID</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {accounts.data.map((a) => {
                  const platformOff = platformDisabled(a)
                  return (
                    <TableRow
                      key={a.id}
                      className={
                        a.status === "error" ? "bg-destructive/5" : platformOff ? "opacity-60" : undefined
                      }
                    >
                      <TableCell>
                        <div className="flex items-center gap-2.5">
                          <PlatformGlyph kind={a.platform} size={30} />
                          <span className="text-[13px] font-semibold">
                            {PLATFORM_LABEL[a.platform] ?? a.platform}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="font-medium">{a.display_name ?? "—"}</div>
                        {a.page_id && <div className="mono text-xs text-muted-foreground">{a.page_id}</div>}
                        {!a.page_id && a.platform === "vk" && (
                          <div className="mono text-xs text-muted-foreground">group_id={a.platform_id}</div>
                        )}
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={a.status} active={a.is_active} />
                        {platformOff && (
                          <div className="mt-1">
                            <Badge variant="outline">платформа отключена</Badge>
                          </div>
                        )}
                        {a.status_message && (
                          <div className="mt-1 max-w-[220px] text-xs text-muted-foreground">
                            {a.status_message}
                          </div>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap justify-end gap-1.5">{accountButtons(a)}</div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </Card>

          {/* Mobile: card per account */}
          <div className="stagger space-y-3 md:hidden">
            {accounts.data.map((a) => {
              const platformOff = platformDisabled(a)
              return (
                <Card
                  key={a.id}
                  className={cn(
                    "space-y-3 p-4",
                    a.status === "error" && "border-destructive/40",
                    platformOff && "opacity-60",
                  )}
                >
                  <div className="flex items-start gap-3">
                    <PlatformGlyph kind={a.platform} size={36} />
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-semibold">
                        {a.display_name ?? PLATFORM_LABEL[a.platform] ?? a.platform}
                      </div>
                      <div className="mono truncate text-xs text-muted-foreground">
                        {a.page_id ? a.page_id : `group_id=${a.platform_id}`}
                      </div>
                    </div>
                    <StatusBadge status={a.status} active={a.is_active} />
                  </div>
                  {platformOff && <Badge variant="outline">платформа отключена</Badge>}
                  {a.status_message && (
                    <div className="text-xs text-muted-foreground">{a.status_message}</div>
                  )}
                  <div className="flex flex-wrap gap-2">{accountButtons(a)}</div>
                </Card>
              )
            })}
          </div>
        </>
      )}
    </div>
  )
}
