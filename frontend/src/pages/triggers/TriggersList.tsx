import { useEffect, useMemo, useState } from "react"
import { Link, useParams } from "react-router-dom"
import { toast } from "sonner"
import { Plus, ScrollText } from "lucide-react"
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core"
import {
  SortableContext,
  arrayMove,
  rectSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { PageHead } from "@/components/brand/PageHead"
import { TriggerCard } from "@/components/brand/TriggerCard"
import { useAccount } from "@/api/accounts"
import { usePlatformAvailability } from "@/api/admin"
import {
  useDeleteTrigger,
  useReorderTriggers,
  useToggleTrigger,
  useTriggers,
  type Trigger,
} from "@/api/triggers"

// computePriorities assigns descending priority values to the new order so that the
// card at index 0 has the highest priority. Steps of 10 leave room for future inserts.
function computePriorities(rows: Trigger[]): number[] {
  return rows.map((_, i) => (rows.length - i) * 10)
}

export function TriggersListPage() {
  const { id: accountID } = useParams()
  const account = useAccount(accountID)
  const triggers = useTriggers(accountID)
  const availability = usePlatformAvailability()
  const platformOff = account.data ? availability.data?.[account.data.platform] === false : false
  const toggle = useToggleTrigger(accountID!)
  const del = useDeleteTrigger(accountID!)
  const reorder = useReorderTriggers(accountID!)

  // Local mirror of the server list so we can render an optimistic order during drag.
  const [localOrder, setLocalOrder] = useState<Trigger[]>([])
  useEffect(() => {
    if (triggers.data) setLocalOrder(triggers.data)
  }, [triggers.data])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const ids = useMemo(() => localOrder.map((t) => t.id), [localOrder])

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const oldIndex = localOrder.findIndex((t) => t.id === active.id)
    const newIndex = localOrder.findIndex((t) => t.id === over.id)
    if (oldIndex === -1 || newIndex === -1) return

    const next = arrayMove(localOrder, oldIndex, newIndex)
    const priorities = computePriorities(next)
    const changes = next
      .map((t, i) => ({ trigger: t, priority: priorities[i] }))
      .filter(({ trigger, priority }) => trigger.priority !== priority)

    // Optimistic UI: apply new priorities locally while the PUTs are in flight.
    setLocalOrder(next.map((t, i) => ({ ...t, priority: priorities[i] })))

    reorder.mutate(changes, {
      onSuccess: () => {
        if (changes.length > 0) {
          toast.success(
            `Порядок обновлён (${changes.length} триггер${changes.length === 1 ? "" : "ов"})`,
          )
        }
      },
      onError: () => {
        toast.error("Не удалось сохранить порядок")
        if (triggers.data) setLocalOrder(triggers.data)
      },
    })
  }

  return (
    <div>
      <PageHead
        title="Триггеры"
        sub={
          account.data
            ? `${account.data.display_name ?? account.data.platform_id} (${account.data.platform})`
            : "Правила автоответов по ключевым словам и regex."
        }
      >
        <Button asChild variant="outline">
          <Link to={`/accounts/${accountID}/logs`}>
            <ScrollText className="h-4 w-4" />
            Логи
          </Link>
        </Button>
        <Button asChild variant="grad">
          <Link to={`/accounts/${accountID}/triggers/new`}>
            <Plus className="h-4 w-4" />
            Новый триггер
          </Link>
        </Button>
      </PageHead>

      {platformOff && (
        <Card className="mb-4 px-4 py-3 text-sm text-warning">
          Платформа отключена администратором — триггеры этого аккаунта не срабатывают.
        </Card>
      )}

      {triggers.isLoading && <Skeleton className="h-48 w-full" />}
      {triggers.isError && (
        <Card className="py-4 text-center text-sm text-destructive">Не удалось загрузить триггеры.</Card>
      )}

      {triggers.data && triggers.data.length === 0 && (
        <Card className="px-6 py-6 text-sm text-muted-foreground">
          Триггеров пока нет. Создайте первый, нажав кнопку выше.
        </Card>
      )}

      {localOrder.length > 0 && (
        <>
          <div className="mb-3 text-xs text-muted-foreground">
            Перетащите карточки за иконку <span className="mono">⋮⋮</span> слева, чтобы изменить
            приоритет. Сверху — выше приоритет.
          </div>
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
            <SortableContext items={ids} strategy={rectSortingStrategy}>
              <div className="grid gap-3.5 sm:grid-cols-2 xl:grid-cols-3">
                {localOrder.map((t) => (
                  <SortableTriggerCard
                    key={t.id}
                    t={t}
                    accountID={accountID!}
                    toggling={toggle.isPending}
                    onToggle={() =>
                      toggle.mutate(
                        { triggerId: t.id, is_active: !t.is_active },
                        { onSuccess: () => toast.success(t.is_active ? "Выключен" : "Включён") },
                      )
                    }
                    onDelete={() => {
                      if (confirm(`Удалить триггер «${t.name}»?`)) {
                        del.mutate(t.id, { onSuccess: () => toast.success("Удалён") })
                      }
                    }}
                  />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        </>
      )}
    </div>
  )
}

interface SortableTriggerCardProps {
  t: Trigger
  accountID: string
  onToggle: () => void
  onDelete: () => void
  toggling?: boolean
}

function SortableTriggerCard({ t, accountID, onToggle, onDelete, toggling }: SortableTriggerCardProps) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } =
    useSortable({ id: t.id })

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    zIndex: isDragging ? 10 : undefined,
  }

  return (
    <div ref={setNodeRef} style={style}>
      <TriggerCard
        trigger={t}
        editHref={`/accounts/${accountID}/triggers/${t.id}`}
        onToggle={onToggle}
        onDelete={onDelete}
        toggling={toggling}
        dragHandleRef={setActivatorNodeRef}
        dragHandleProps={{ ...attributes, ...listeners }}
      />
    </div>
  )
}
