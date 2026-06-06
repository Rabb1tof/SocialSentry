import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { PlanLimitDialog } from "@/components/PlanLimitDialog"
import { PageHead } from "@/components/brand/PageHead"
import { friendlyErrorMessage, friendlyPlanError } from "@/lib/api-errors"
import { useAccount } from "@/api/accounts"
import {
  useCreateTrigger,
  useTestTrigger,
  useTrigger,
  useUpdateTrigger,
  type TestResult,
  type TriggerInput,
} from "@/api/triggers"

const schema = z.object({
  name: z.string().min(1, "Обязательно"),
  is_active: z.boolean(),
  event_type: z.enum(["dm", "comment", "comment_and_dm"]),
  match_mode: z.enum(["keyword", "all", "regex"]),
  keywords_raw: z.string().optional(),
  keywords_mode: z.enum(["contains", "exact", "starts_with"]),
  case_sensitive: z.boolean(),
  reply_to_comment: z.boolean(),
  reply_comment_text: z.string().optional(),
  send_private_reply: z.boolean(),
  private_reply_text: z.string().optional(),
  send_dm: z.boolean(),
  dm_text: z.string().optional(),
  check_subscription: z.boolean(),
  reply_if_unsubscribed: z.string().optional(),
  cooldown_seconds: z.coerce.number().int().min(0),
  reply_delay_seconds: z.coerce.number().int().min(0).max(3600),
  max_replies_per_user: z.coerce.number().int().min(0),
  priority: z.coerce.number().int().min(0).max(1000),
})

type FormVals = z.infer<typeof schema>

function emptyDefaults(): FormVals {
  return {
    name: "",
    is_active: true,
    event_type: "dm",
    match_mode: "keyword",
    keywords_raw: "",
    keywords_mode: "contains",
    case_sensitive: false,
    reply_to_comment: false,
    reply_comment_text: "",
    send_private_reply: false,
    private_reply_text: "",
    send_dm: true,
    dm_text: "",
    check_subscription: false,
    reply_if_unsubscribed: "",
    cooldown_seconds: 0,
    reply_delay_seconds: 0,
    max_replies_per_user: 0,
    priority: 0,
  }
}

function toApi(v: FormVals): TriggerInput {
  const keywords =
    v.match_mode === "all"
      ? []
      : (v.keywords_raw ?? "")
          .split(",")
          .map((s) => s.trim())
          .filter((s) => s.length > 0)
  return {
    name: v.name,
    is_active: v.is_active,
    event_type: v.event_type,
    match_mode: v.match_mode,
    keywords,
    keywords_mode: v.keywords_mode,
    case_sensitive: v.case_sensitive,
    reply_to_comment: v.reply_to_comment,
    reply_comment_text: v.reply_comment_text || null,
    send_private_reply: v.send_private_reply,
    private_reply_text: v.private_reply_text || null,
    send_dm: v.send_dm,
    dm_text: v.dm_text || null,
    check_subscription: v.check_subscription,
    reply_if_subscribed: null, // deprecated: subscription is now a gate, not a text picker
    reply_if_unsubscribed: v.reply_if_unsubscribed || null,
    cooldown_seconds: Number(v.cooldown_seconds),
    reply_delay_seconds: Number(v.reply_delay_seconds),
    max_replies_per_user: Number(v.max_replies_per_user),
    priority: Number(v.priority),
  }
}

export function TriggerFormPage() {
  const { id: accountID, tid } = useParams()
  const navigate = useNavigate()
  const isNew = !tid || tid === "new"

  const existing = useTrigger(accountID, isNew ? undefined : tid)
  const account = useAccount(accountID)
  const platform = account.data?.platform
  const create = useCreateTrigger(accountID!)
  const update = useUpdateTrigger(accountID!, tid ?? "")

  const [planLimit, setPlanLimit] = useState<string | null>(null)

  const form = useForm<FormVals>({
    resolver: zodResolver(schema),
    defaultValues: emptyDefaults(),
  })

  // Hydrate form when editing.
  useEffect(() => {
    if (existing.data) {
      form.reset({
        name: existing.data.name,
        is_active: existing.data.is_active,
        event_type: existing.data.event_type,
        match_mode: existing.data.match_mode,
        keywords_raw: existing.data.keywords.join(", "),
        keywords_mode: existing.data.keywords_mode,
        case_sensitive: existing.data.case_sensitive,
        reply_to_comment: existing.data.reply_to_comment,
        reply_comment_text: existing.data.reply_comment_text ?? "",
        send_private_reply: existing.data.send_private_reply,
        private_reply_text: existing.data.private_reply_text ?? "",
        send_dm: existing.data.send_dm,
        dm_text: existing.data.dm_text ?? "",
        check_subscription: existing.data.check_subscription,
        reply_if_unsubscribed: existing.data.reply_if_unsubscribed ?? "",
        cooldown_seconds: existing.data.cooldown_seconds,
        reply_delay_seconds: existing.data.reply_delay_seconds,
        max_replies_per_user: existing.data.max_replies_per_user,
        priority: existing.data.priority,
      })
    }
  }, [existing.data, form])

  const eventType = form.watch("event_type")
  const matchMode = form.watch("match_mode")
  const wantsCommentActions = eventType === "comment" || eventType === "comment_and_dm"
  const wantsDMActions = eventType === "dm" || eventType === "comment_and_dm"
  const checkSub = form.watch("check_subscription")

  const onSubmit = (vals: FormVals) => {
    const body = toApi(vals)
    const handleError = (e: unknown) => {
      const planMsg = friendlyPlanError(e)
      if (planMsg) {
        setPlanLimit(planMsg)
        return
      }
      toast.error(friendlyErrorMessage(e, "Не удалось сохранить"))
    }
    if (isNew) {
      create.mutate(body, {
        onSuccess: () => {
          toast.success("Триггер создан")
          navigate(`/accounts/${accountID}/triggers`)
        },
        onError: handleError,
      })
    } else {
      update.mutate(body, {
        onSuccess: () => {
          toast.success("Сохранено")
          navigate(`/accounts/${accountID}/triggers`)
        },
        onError: handleError,
      })
    }
  }

  if (!isNew && existing.isLoading) {
    return <Skeleton className="h-96 w-full" />
  }
  if (!isNew && existing.isError) {
    return (
      <Card>
        <CardContent className="py-4 text-sm text-destructive">
          Триггер не найден.
        </CardContent>
      </Card>
    )
  }

  return (
    <form className="space-y-4 max-w-3xl" onSubmit={form.handleSubmit(onSubmit)}>
      <PlanLimitDialog
        open={!!planLimit}
        message={planLimit ?? ""}
        onClose={() => setPlanLimit(null)}
      />
      <PageHead
        title={isNew ? "Новый триггер" : "Редактировать триггер"}
        sub={account.data ? (account.data.display_name ?? account.data.platform_id) : undefined}
      />

      <Card>
        <CardHeader>
          <CardTitle>Основное</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div className="space-y-1 md:col-span-2">
            <Label htmlFor="name">Название</Label>
            <Input id="name" {...form.register("name")} />
            {form.formState.errors.name && (
              <p className="text-xs text-destructive">{form.formState.errors.name.message}</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="is_active"
              {...form.register("is_active")}
              className="size-4"
            />
            <Label htmlFor="is_active">Активен</Label>
          </div>
          <div className="space-y-1">
            <Label htmlFor="priority">Приоритет (больше → выше)</Label>
            <Input id="priority" type="number" min={0} max={1000} {...form.register("priority")} />
          </div>
          <div className="space-y-1">
            <Label htmlFor="event_type">Тип события</Label>
            <select
              id="event_type"
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
              {...form.register("event_type")}
            >
              <option value="dm">Личное сообщение</option>
              <option value="comment">Комментарий</option>
              <option value="comment_and_dm">Оба</option>
            </select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="match_mode">Режим матчинга</Label>
            <select
              id="match_mode"
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
              {...form.register("match_mode")}
            >
              <option value="keyword">По ключевым словам</option>
              <option value="all">На любое сообщение</option>
              <option value="regex">Регулярное выражение</option>
            </select>
          </div>
        </CardContent>
      </Card>

      {matchMode !== "all" && (
        <Card>
          <CardHeader>
            <CardTitle>
              {matchMode === "regex" ? "Регулярное выражение" : "Ключевые слова"}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="keywords_raw">
                {matchMode === "regex"
                  ? "Регулярное выражение (первое значение через запятую используется)"
                  : "Через запятую"}
              </Label>
              <Input
                id="keywords_raw"
                className={matchMode === "regex" ? "mono" : undefined}
                placeholder={
                  matchMode === "regex"
                    ? `^цена\\s+\\d+$`
                    : "привет, hello, hi"
                }
                {...form.register("keywords_raw")}
              />
            </div>
            {matchMode === "keyword" && (
              <>
                <div className="space-y-1">
                  <Label htmlFor="keywords_mode">Тип сравнения</Label>
                  <select
                    id="keywords_mode"
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                    {...form.register("keywords_mode")}
                  >
                    <option value="contains">Содержит</option>
                    <option value="exact">Точное совпадение</option>
                    <option value="starts_with">Начинается с</option>
                  </select>
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="case_sensitive"
                    {...form.register("case_sensitive")}
                    className="size-4"
                  />
                  <Label htmlFor="case_sensitive">Учитывать регистр</Label>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {wantsCommentActions && (
        <Card>
          <CardHeader>
            <CardTitle>Действия для комментариев</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="reply_to_comment"
                {...form.register("reply_to_comment")}
                className="size-4"
              />
              <Label htmlFor="reply_to_comment">Ответить публично в треде комментария</Label>
            </div>
            <Input
              placeholder="Текст публичного ответа"
              {...form.register("reply_comment_text")}
            />
            <div className="flex items-center gap-2 pt-2 border-t">
              <input
                type="checkbox"
                id="send_private_reply"
                {...form.register("send_private_reply")}
                className="size-4"
              />
              <Label htmlFor="send_private_reply">
                Отправить личное сообщение комментатору (окно: 7 дней)
              </Label>
            </div>
            <Input
              placeholder="Текст приватного ответа"
              {...form.register("private_reply_text")}
            />
          </CardContent>
        </Card>
      )}

      {wantsDMActions && (
        <Card>
          <CardHeader>
            <CardTitle>Действие для личного сообщения</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="send_dm"
                {...form.register("send_dm")}
                className="size-4"
              />
              <Label htmlFor="send_dm">
                Ответить в диалог (окно: 24 часа после входящего сообщения)
              </Label>
            </div>
            <Input placeholder="Текст ответа" {...form.register("dm_text")} />
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Проверка подписки</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="check_subscription"
              {...form.register("check_subscription")}
              className="size-4"
            />
            <Label htmlFor="check_subscription">
              Отвечать только подписчикам {platform === "vk" ? "сообщества" : "аккаунта"}
            </Label>
          </div>
          <p className="text-xs text-muted-foreground">
            Подписчики получают обычный ответ триггера. Неподписчикам вместо ответа уходит
            текст-призыв подписаться (по тем же каналам, что включены в триггере).
            {platform === "instagram" ? (
              <>
                {" "}Для Instagram работает только в триггерах на <b>личные сообщения</b> и только
                если пользователь уже писал вам (ограничение Messaging API); для комментариев
                статус подписки недоступен.
              </>
            ) : (
              <> Для VK проверяется членство в сообществе — и для ЛС, и для комментариев.</>
            )}
          </p>
          {checkSub && (
            <Input
              placeholder="Текст для неподписчиков (например: «Подпишись, и я пришлю ответ»)"
              {...form.register("reply_if_unsubscribed")}
            />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Лимиты</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-1">
            <Label htmlFor="cooldown_seconds">Кулдаун (сек)</Label>
            <Input
              id="cooldown_seconds"
              type="number"
              min={0}
              {...form.register("cooldown_seconds")}
            />
            <p className="text-xs text-muted-foreground">
              Пауза между повторными ответами <b>одному и тому же</b> пользователю по этому
              триггеру. 0 = без паузы. Напр., 60: если человек снова напишет ключевое слово в
              течение минуты — бот промолчит.
            </p>
          </div>
          <div className="space-y-1">
            <Label htmlFor="reply_delay_seconds">Задержка ответа (сек)</Label>
            <Input
              id="reply_delay_seconds"
              type="number"
              min={0}
              max={3600}
              {...form.register("reply_delay_seconds")}
            />
            <p className="text-xs text-muted-foreground">
              Бот ответит не сразу, а через указанное время после сообщения — выглядит
              естественнее, «по-человечески». 0 = моментально. Макс. 3600 (1 час).
            </p>
          </div>
          <div className="space-y-1">
            <Label htmlFor="max_replies_per_user">Макс. ответов одному юзеру (0 = ∞)</Label>
            <Input
              id="max_replies_per_user"
              type="number"
              min={0}
              {...form.register("max_replies_per_user")}
            />
            <p className="text-xs text-muted-foreground">
              Сколько всего раз этот триггер может ответить одному пользователю. 0 = без
              ограничения.
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="py-4 text-xs text-muted-foreground space-y-1">
          <div className="font-medium text-foreground">Переменные для текстов ответов:</div>
          <div>
            <code>{"{{name}}"}</code> · <code>{"{{username}}"}</code> · <code>{"{{keyword}}"}</code> ·{" "}
            <code>{"{{time}}"}</code> · <code>{"{{date}}"}</code>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button
          type="button"
          variant="outline"
          onClick={() => navigate(`/accounts/${accountID}/triggers`)}
        >
          Отмена
        </Button>
        <Button variant="grad" type="submit" disabled={create.isPending || update.isPending}>
          {isNew ? "Создать" : "Сохранить"}
        </Button>
      </div>

      {!isNew && tid && accountID && (
        <TestPanel accountID={accountID} triggerID={tid} />
      )}
    </form>
  )
}

interface TestPanelProps {
  accountID: string
  triggerID: string
}

function TestPanel({ accountID, triggerID }: TestPanelProps) {
  const [text, setText] = useState("hello there")
  const [senderName, setSenderName] = useState("Тест Юзер")
  const [senderID, setSenderID] = useState("123456789")
  const [kind, setKind] = useState<"dm" | "comment">("dm")
  const test = useTestTrigger(accountID, triggerID)
  const [result, setResult] = useState<TestResult | null>(null)

  const onTest = () => {
    test.mutate(
      {
        text,
        sender_id: senderID || undefined,
        sender_name: senderName || undefined,
        kind,
      },
      {
        onSuccess: (data) => setResult(data),
        onError: (e: any) => {
          const msg = e?.response?.data?.message ?? "Не удалось выполнить тест"
          toast.error(msg)
          setResult(null)
        },
      },
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Тест триггера</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div className="space-y-1 md:col-span-2">
            <Label htmlFor="test_text">Входящий текст</Label>
            <Input
              id="test_text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="Сообщение, как будто бы пришло от пользователя"
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="test_kind">Тип</Label>
            <select
              id="test_kind"
              value={kind}
              onChange={(e) => setKind(e.target.value as "dm" | "comment")}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
            >
              <option value="dm">Личное сообщение</option>
              <option value="comment">Комментарий</option>
            </select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="test_name">Имя отправителя (для {`{{name}}`})</Label>
            <Input
              id="test_name"
              value={senderName}
              onChange={(e) => setSenderName(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="test_id">Sender ID</Label>
            <Input
              id="test_id"
              value={senderID}
              onChange={(e) => setSenderID(e.target.value)}
            />
          </div>
          <div className="flex items-end">
            <Button
              type="button"
              onClick={onTest}
              disabled={test.isPending}
              className="w-full"
            >
              {test.isPending ? "Проверяем…" : "Прогнать тест"}
            </Button>
          </div>
        </div>

        {result && (
          <div className="space-y-2 rounded-md border bg-muted/30 p-3 text-sm">
            <div className="flex flex-wrap gap-2 items-center">
              <span className="text-muted-foreground">Тип события:</span>
              {result.event_type_matched ? (
                <Badge variant="success">подходит</Badge>
              ) : (
                <Badge variant="outline">не подходит</Badge>
              )}
              <span className="text-muted-foreground ml-2">Совпадение по тексту:</span>
              {result.text_matched ? (
                <Badge variant="success">да</Badge>
              ) : (
                <Badge variant="outline">нет</Badge>
              )}
              {result.matched_keyword && (
                <>
                  <span className="text-muted-foreground ml-2">Ключ:</span>
                  <code className="text-xs">{result.matched_keyword}</code>
                </>
              )}
            </div>

            <div className="flex flex-wrap gap-2 items-center">
              <span className="text-muted-foreground">Триггер сработает:</span>
              {result.would_fire ? (
                <Badge variant="success">да</Badge>
              ) : (
                <Badge variant="destructive">нет</Badge>
              )}
            </div>

            {result.replies.length > 0 && (
              <div className="space-y-2 pt-2 border-t">
                <div className="text-muted-foreground text-xs">Сообщения, которые будут отправлены:</div>
                {result.replies.map((r, i) => (
                  <div key={i} className="rounded border bg-background p-2">
                    <div className="text-xs text-muted-foreground mb-1">
                      <Badge variant="secondary">{channelLabel(r.channel)}</Badge>
                    </div>
                    <div className="whitespace-pre-wrap">{r.text}</div>
                  </div>
                ))}
              </div>
            )}

            {result.would_fire && result.replies.length === 0 && (
              <div className="text-xs text-muted-foreground pt-2 border-t">
                Триггер сработал, но ни одно действие не настроено для этого типа события.
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function channelLabel(c: string): string {
  switch (c) {
    case "dm":
      return "Личное сообщение"
    case "comment_reply":
      return "Публичный ответ"
    case "private_reply":
      return "Private Reply (DM)"
    default:
      return c
  }
}
