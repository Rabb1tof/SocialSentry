import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { toast } from "sonner"
import { Pencil } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
import { Modal } from "@/components/brand/Modal"
import { Switch } from "@/components/brand/Switch"
import { PLAN_LABEL, fmtDate } from "@/lib/labels"
import type { Subscription } from "@/api/subscription"
import {
  useAdminSubscriptions,
  useAdminUsers,
  useGrantSubscription,
  useRevokeSubscription,
  useUpdateSubscription,
} from "@/api/admin"

const PAGE_SIZE = 20

const selectClass =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"

const planEnum = z.enum(["basic", "pro", "enterprise"])

const grantSchema = z.object({
  user_id: z.string().uuid("Введите валидный UUID"),
  plan: planEnum,
  expires_at: z
    .string()
    .optional()
    .refine((s) => !s || !Number.isNaN(Date.parse(s)), "ISO-формат (yyyy-mm-ddThh:mm)"),
  note: z.string().optional(),
})

type GrantForm = z.infer<typeof grantSchema>

export function AdminSubscriptionsPage() {
  const [offset, setOffset] = useState(0)
  const [editing, setEditing] = useState<Subscription | null>(null)
  const subs = useAdminSubscriptions(PAGE_SIZE, offset)
  const users = useAdminUsers(100, 0)
  const grant = useGrantSubscription()
  const revoke = useRevokeSubscription()
  const update = useUpdateSubscription()

  // User picker for the grant form: search by email/UUID instead of pasting a raw UUID.
  const [userQuery, setUserQuery] = useState("")
  const [pickerOpen, setPickerOpen] = useState(false)

  const userList = users.data?.data ?? []
  const filteredUsers =
    userQuery.trim() === ""
      ? userList
      : userList.filter(
          (u) =>
            u.email.toLowerCase().includes(userQuery.toLowerCase()) ||
            u.id.toLowerCase().includes(userQuery.toLowerCase()),
        )
  const emailById = new Map(userList.map((u) => [u.id, u.email]))

  const form = useForm<GrantForm>({
    resolver: zodResolver(grantSchema),
    defaultValues: { plan: "pro" },
  })
  const selectedUserId = form.watch("user_id")

  const onGrant = (values: GrantForm) => {
    grant.mutate(
      {
        user_id: values.user_id,
        plan: values.plan,
        expires_at: values.expires_at ? new Date(values.expires_at).toISOString() : null,
        note: values.note || null,
      },
      {
        onSuccess: () => {
          toast.success("Подписка выдана")
          form.reset({ plan: "pro" })
          setUserQuery("")
        },
        onError: (err: any) => {
          const msg = err?.response?.data?.message ?? "Не удалось выдать подписку"
          toast.error(msg)
        },
      },
    )
  }

  const onRevoke = (id: string) => {
    revoke.mutate(id, {
      onSuccess: () => toast.success("Подписка отозвана"),
      onError: () => toast.error("Не удалось отозвать"),
    })
  }

  const onSaveEdit = (body: {
    plan: "basic" | "pro" | "enterprise"
    is_active: boolean
    expires_at: string | null
    note: string | null
  }) => {
    if (!editing) return
    update.mutate(
      { id: editing.id, ...body },
      {
        onSuccess: () => {
          toast.success("Сохранено")
          setEditing(null)
        },
        onError: (err: any) =>
          toast.error(err?.response?.data?.message ?? "Не удалось сохранить"),
      },
    )
  }

  return (
    <div>
      {editing && (
        <SubEditModal key={editing.id} sub={editing} saving={update.isPending} onClose={() => setEditing(null)} onSave={onSaveEdit} />
      )}

      <PageHead title="Подписки" sub={`Всего: ${subs.data?.meta.total ?? "—"}`} />

      <Card className="mb-6 p-5">
        <div className="mb-3.5 font-semibold">Выдать подписку</div>
        <form onSubmit={form.handleSubmit(onGrant)} className="grid grid-cols-1 gap-3.5 md:grid-cols-2">
          <div className="relative space-y-1.5">
            <Label htmlFor="user_search">Пользователь</Label>
            <input type="hidden" {...form.register("user_id")} />
            <Input
              id="user_search"
              placeholder="Поиск по email или UUID…"
              autoComplete="off"
              value={userQuery}
              onChange={(e) => {
                setUserQuery(e.target.value)
                setPickerOpen(true)
                // Typing invalidates a previous selection until a row is picked.
                form.setValue("user_id", "", { shouldValidate: false })
              }}
              onFocus={() => setPickerOpen(true)}
              onBlur={() => setTimeout(() => setPickerOpen(false), 150)}
            />
            {pickerOpen && filteredUsers.length > 0 && (
              <ul className="absolute z-10 mt-1 max-h-56 w-full overflow-auto rounded-md border bg-popover shadow-md">
                {filteredUsers.slice(0, 20).map((u) => (
                  <li key={u.id}>
                    <button
                      type="button"
                      className="flex w-full flex-col items-start px-3 py-2 text-left text-sm hover:bg-secondary"
                      onMouseDown={(e) => {
                        // onMouseDown fires before input blur, so the selection sticks.
                        e.preventDefault()
                        form.setValue("user_id", u.id, { shouldValidate: true })
                        setUserQuery(u.email)
                        setPickerOpen(false)
                      }}
                    >
                      <span className="font-medium">{u.email}</span>
                      <span className="mono text-xs text-muted-foreground">{u.id}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {selectedUserId && (
              <p className="text-xs text-muted-foreground">
                Выбран: <span className="mono">{selectedUserId}</span>
              </p>
            )}
            {form.formState.errors.user_id && (
              <p className="text-xs text-destructive">Выберите пользователя из списка</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="plan">План</Label>
            <select id="plan" className={selectClass} {...form.register("plan")}>
              <option value="basic">basic</option>
              <option value="pro">pro</option>
              <option value="enterprise">enterprise</option>
            </select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="expires_at">Истекает (необяз.)</Label>
            <Input id="expires_at" type="datetime-local" {...form.register("expires_at")} />
            {form.formState.errors.expires_at && (
              <p className="text-xs text-destructive">{form.formState.errors.expires_at.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="note">Заметка (необяз.)</Label>
            <Input id="note" placeholder="Например: оплата за квартал" {...form.register("note")} />
          </div>
          <div className="flex justify-end md:col-span-2">
            <Button variant="grad" type="submit" disabled={grant.isPending}>
              {grant.isPending ? "Выдаём…" : "Выдать подписку"}
            </Button>
          </div>
        </form>
      </Card>

      {subs.isLoading && <Skeleton className="h-64 w-full" />}
      {subs.isError && (
        <Card className="py-4 text-center text-sm text-destructive">Не удалось загрузить подписки.</Card>
      )}

      {subs.data && (
        <Card className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Пользователь</TableHead>
                <TableHead>План</TableHead>
                <TableHead>Статус</TableHead>
                <TableHead>Начало</TableHead>
                <TableHead>Истекает</TableHead>
                <TableHead className="text-right">Действия</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {subs.data.data.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="text-xs">
                    {emailById.has(s.user_id) && (
                      <div className="text-[13px] font-medium">{emailById.get(s.user_id)}</div>
                    )}
                    <div className="mono text-muted-foreground">{s.user_id}</div>
                    {s.note && (
                      <div className="mt-1 max-w-[260px] truncate text-muted-foreground" title={s.note}>
                        Заметка: {s.note}
                      </div>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={s.plan === "enterprise" ? "default" : "soft"}>
                      {PLAN_LABEL[s.plan] ?? s.plan}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {s.is_active ? (
                      <Badge variant="success">активна</Badge>
                    ) : (
                      <Badge variant="outline">отозвана</Badge>
                    )}
                  </TableCell>
                  <TableCell className="mono text-xs">{fmtDate(s.starts_at, true)}</TableCell>
                  <TableCell className="mono text-xs">
                    {s.expires_at ? fmtDate(s.expires_at, true) : "бессрочно"}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1.5">
                      <Button variant="outline" size="sm" onClick={() => setEditing(s)}>
                        <Pencil className="h-3.5 w-3.5" />
                        Изменить
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={!s.is_active || revoke.isPending}
                        onClick={() => onRevoke(s.id)}
                      >
                        Отозвать
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {subs.data.data.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="py-10 text-center text-sm text-muted-foreground">
                    Подписок пока нет.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </Card>
      )}

      {subs.data && subs.data.data.length > 0 && (
        <div className="mt-4 flex justify-between text-sm">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            disabled={offset === 0}
          >
            ← Назад
          </Button>
          <span className="mono text-muted-foreground">
            {offset + 1}–{Math.min(offset + PAGE_SIZE, subs.data.meta.total)} из {subs.data.meta.total}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={offset + PAGE_SIZE >= subs.data.meta.total}
          >
            Вперёд →
          </Button>
        </div>
      )}
    </div>
  )
}

// toLocalInput converts an ISO timestamp to the `yyyy-MM-ddTHH:mm` shape that a
// datetime-local input expects, in the browser's local time zone.
function toLocalInput(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

interface SubEditModalProps {
  sub: Subscription
  saving: boolean
  onClose: () => void
  onSave: (body: {
    plan: "basic" | "pro" | "enterprise"
    is_active: boolean
    expires_at: string | null
    note: string | null
  }) => void
}

function SubEditModal({ sub, saving, onClose, onSave }: SubEditModalProps) {
  const [plan, setPlan] = useState<"basic" | "pro" | "enterprise">(sub.plan)
  const [isActive, setIsActive] = useState(sub.is_active)
  const [expiresLocal, setExpiresLocal] = useState(sub.expires_at ? toLocalInput(sub.expires_at) : "")
  const [note, setNote] = useState(sub.note ?? "")

  const submit = () => {
    onSave({
      plan,
      is_active: isActive,
      expires_at: expiresLocal ? new Date(expiresLocal).toISOString() : null,
      note: note.trim() || null,
    })
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Редактировать подписку"
      footer={
        <>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Отмена
          </Button>
          <Button size="sm" onClick={submit} disabled={saving}>
            {saving ? "Сохраняем…" : "Сохранить"}
          </Button>
        </>
      }
    >
      <div className="space-y-3.5">
        <div className="space-y-1.5">
          <Label htmlFor="edit_plan">План</Label>
          <select
            id="edit_plan"
            className={selectClass}
            value={plan}
            onChange={(e) => setPlan(e.target.value as "basic" | "pro" | "enterprise")}
          >
            <option value="basic">basic</option>
            <option value="pro">pro</option>
            <option value="enterprise">enterprise</option>
          </select>
        </div>
        <div className="flex items-center justify-between">
          <Label htmlFor="edit_active">Активна</Label>
          <Switch checked={isActive} onChange={setIsActive} ariaLabel="Активна" />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="edit_expires">Истекает (пусто = бессрочно)</Label>
          <Input
            id="edit_expires"
            type="datetime-local"
            value={expiresLocal}
            onChange={(e) => setExpiresLocal(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="edit_note">Заметка администратора</Label>
          <Input
            id="edit_note"
            placeholder="Видна только администраторам"
            value={note}
            onChange={(e) => setNote(e.target.value)}
          />
        </div>
      </div>
    </Modal>
  )
}
