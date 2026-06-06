import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { toast } from "sonner"
import { Pencil, UserPlus, X } from "lucide-react"

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
import {
  useAdminCreateUser,
  useAdminPatchUser,
  useAdminUsers,
  type AdminUser,
} from "@/api/admin"
import { friendlyErrorMessage } from "@/lib/api-errors"

const createUserSchema = z.object({
  email: z.string().email("Неверный email"),
  password: z.string().min(8, "Минимум 8 символов"),
  role: z.enum(["user", "admin"]),
})
type CreateUserForm = z.infer<typeof createUserSchema>

const PAGE_SIZE = 20

const selectClass =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"

export function AdminUsersPage() {
  const [offset, setOffset] = useState(0)
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<AdminUser | null>(null)
  const { data, isLoading, isError } = useAdminUsers(PAGE_SIZE, offset)
  const patch = useAdminPatchUser()
  const create = useAdminCreateUser()

  const createForm = useForm<CreateUserForm>({
    resolver: zodResolver(createUserSchema),
    defaultValues: { email: "", password: "", role: "user" },
  })

  const onCreate = (values: CreateUserForm) => {
    create.mutate(values, {
      onSuccess: (u) => {
        toast.success(`Создан ${u.email} (${u.role})`)
        createForm.reset({ email: "", password: "", role: "user" })
        setCreateOpen(false)
      },
      onError: (e) => {
        toast.error(friendlyErrorMessage(e, "Не удалось создать пользователя"))
      },
    })
  }

  // onSaveEdit sends only the changed fields (password only when provided).
  const onSaveEdit = (patchBody: { role?: string; is_blocked?: boolean; email?: string; password?: string }) => {
    if (!editing) return
    if (Object.keys(patchBody).length === 0) {
      setEditing(null)
      return
    }
    patch.mutate(
      { id: editing.id, ...patchBody },
      {
        onSuccess: () => {
          toast.success("Сохранено")
          setEditing(null)
        },
        onError: (e) => toast.error(friendlyErrorMessage(e, "Не удалось сохранить")),
      },
    )
  }

  return (
    <div>
      {editing && (
        <UserEditModal
          key={editing.id}
          user={editing}
          saving={patch.isPending}
          onClose={() => setEditing(null)}
          onSave={onSaveEdit}
        />
      )}

      <PageHead title="Пользователи" sub={`Всего: ${data?.meta.total ?? "—"}`}>
        <Button variant="outline" onClick={() => setCreateOpen((v) => !v)}>
          {createOpen ? <X className="h-4 w-4" /> : <UserPlus className="h-4 w-4" />}
          {createOpen ? "Скрыть форму" : "Создать пользователя"}
        </Button>
      </PageHead>

      {createOpen && (
        <Card className="mb-4 p-5">
          <div className="mb-3.5 font-semibold">Новый пользователь</div>
          <form
            onSubmit={createForm.handleSubmit(onCreate)}
            className="grid grid-cols-1 gap-3.5 md:grid-cols-3"
          >
            <div className="space-y-1.5">
              <Label htmlFor="new_email">Email</Label>
              <Input
                id="new_email"
                type="email"
                placeholder="user@example.com"
                autoComplete="off"
                {...createForm.register("email")}
              />
              {createForm.formState.errors.email && (
                <p className="text-xs text-destructive">{createForm.formState.errors.email.message}</p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new_password">Пароль (мин. 8)</Label>
              <Input
                id="new_password"
                type="password"
                autoComplete="new-password"
                {...createForm.register("password")}
              />
              {createForm.formState.errors.password && (
                <p className="text-xs text-destructive">
                  {createForm.formState.errors.password.message}
                </p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new_role">Роль</Label>
              <select id="new_role" className={selectClass} {...createForm.register("role")}>
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </div>
            <div className="flex items-center justify-between gap-3 md:col-span-3">
              <p className="max-w-[460px] text-xs text-muted-foreground">
                Учётка создаётся напрямую — без подтверждения email и без подписки. Пароль
                пользователь сможет сменить позже.
              </p>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? "Создаём…" : "Создать"}
              </Button>
            </div>
          </form>
        </Card>
      )}

      {isLoading && <Skeleton className="h-64 w-full" />}
      {isError && (
        <Card className="py-4 text-center text-sm text-destructive">
          Не удалось загрузить пользователей.
        </Card>
      )}

      {data && (
        <Card className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Email</TableHead>
                <TableHead>ID</TableHead>
                <TableHead>Роль</TableHead>
                <TableHead>Статус</TableHead>
                <TableHead className="text-right">Действия</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.data.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.email}</TableCell>
                  <TableCell>
                    <button
                      type="button"
                      title="Скопировать UUID"
                      onClick={() => {
                        void navigator.clipboard.writeText(u.id)
                        toast.success("ID скопирован")
                      }}
                      className="mono text-xs text-muted-foreground hover:text-foreground"
                    >
                      {u.id.slice(0, 8)}… ⧉
                    </button>
                  </TableCell>
                  <TableCell>
                    {u.role === "admin" ? <Badge>admin</Badge> : <Badge variant="soft">user</Badge>}
                  </TableCell>
                  <TableCell>
                    {u.is_blocked ? (
                      <Badge variant="destructive">Заблокирован</Badge>
                    ) : (
                      <Badge variant="outline">Активен</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="outline" size="sm" onClick={() => setEditing(u)}>
                      <Pencil className="h-3.5 w-3.5" />
                      Изменить
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {data && (
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
            {offset + 1}–{Math.min(offset + PAGE_SIZE, data.meta.total)} из {data.meta.total}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={offset + PAGE_SIZE >= data.meta.total}
          >
            Вперёд →
          </Button>
        </div>
      )}
    </div>
  )
}

interface UserEditModalProps {
  user: AdminUser
  saving: boolean
  onClose: () => void
  onSave: (body: { role?: string; is_blocked?: boolean; email?: string; password?: string }) => void
}

function UserEditModal({ user, saving, onClose, onSave }: UserEditModalProps) {
  const [email, setEmail] = useState(user.email)
  const [role, setRole] = useState(user.role)
  const [blocked, setBlocked] = useState(user.is_blocked)
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)

  const submit = () => {
    const trimmedEmail = email.trim()
    if (!trimmedEmail.includes("@")) {
      setError("Введите корректный email")
      return
    }
    if (password && password.length < 8) {
      setError("Пароль должен быть не короче 8 символов")
      return
    }
    setError(null)
    const body: { role?: string; is_blocked?: boolean; email?: string; password?: string } = {}
    if (trimmedEmail.toLowerCase() !== user.email.toLowerCase()) body.email = trimmedEmail
    if (role !== user.role) body.role = role
    if (blocked !== user.is_blocked) body.is_blocked = blocked
    if (password) body.password = password
    onSave(body)
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Редактировать пользователя"
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
          <Label htmlFor="edit_email">Email</Label>
          <Input id="edit_email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="edit_role">Роль</Label>
          <select
            id="edit_role"
            className={selectClass}
            value={role}
            onChange={(e) => setRole(e.target.value as "user" | "admin")}
          >
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <div className="flex items-center justify-between">
          <Label htmlFor="edit_blocked">Заблокирован</Label>
          <Switch checked={blocked} onChange={setBlocked} tone="azure" ariaLabel="Заблокирован" />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="edit_password">Новый пароль</Label>
          <Input
            id="edit_password"
            type="password"
            autoComplete="new-password"
            placeholder="Оставьте пустым, чтобы не менять"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        {error && <p className="text-xs text-destructive">{error}</p>}
      </div>
    </Modal>
  )
}
