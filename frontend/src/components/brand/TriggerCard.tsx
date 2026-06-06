import type { HTMLAttributes } from "react"
import { Link } from "react-router-dom"
import { CornerDownRight, GripVertical, Pencil, Trash2, Zap } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Switch } from "@/components/brand/Switch"
import { cn } from "@/lib/utils"
import { EVENT_LABEL, MATCH_LABEL } from "@/lib/labels"
import type { Trigger } from "@/api/triggers"

interface TriggerCardProps {
  trigger: Trigger
  editHref: string
  onToggle: () => void
  onDelete: () => void
  toggling?: boolean
  // dnd-kit wiring (optional): applied to the grip handle so cards stay sortable.
  dragHandleRef?: (el: HTMLElement | null) => void
  dragHandleProps?: HTMLAttributes<HTMLButtonElement>
}

function patternText(t: Trigger): string {
  if (t.match_mode === "all") return "(любое сообщение)"
  if (t.match_mode === "regex") return t.keywords[0] ?? "(regex не задан)"
  return t.keywords.join(", ") || "(нет ключевых слов)"
}

function replyText(t: Trigger): string {
  return t.dm_text || t.reply_comment_text || t.private_reply_text || "(текст ответа не задан)"
}

// TriggerCard — a single trigger as a card: status chip, scope/match badges, on/off
// switch, the match pattern (mono, cyan), a reply preview and footer actions.
export function TriggerCard({
  trigger,
  editHref,
  onToggle,
  onDelete,
  toggling,
  dragHandleRef,
  dragHandleProps,
}: TriggerCardProps) {
  return (
    <Card className="flex flex-col gap-2.5 p-4 transition-shadow duration-150 hover:shadow-md motion-reduce:transition-none">
      <div className="flex items-start gap-2.5">
        <button
          ref={dragHandleRef}
          type="button"
          aria-label="Перетащить"
          className="mt-0.5 cursor-grab text-muted-foreground active:cursor-grabbing"
          {...dragHandleProps}
        >
          <GripVertical className="h-4 w-4" />
        </button>
        <span
          className={cn(
            "inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg",
            trigger.is_active ? "bg-azure text-white" : "bg-secondary text-muted-foreground",
          )}
        >
          <Zap className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="truncate font-semibold">{trigger.name}</div>
          <div className="mt-1 flex flex-wrap gap-1.5">
            <Badge variant="soft">{EVENT_LABEL[trigger.event_type] ?? trigger.event_type}</Badge>
            <Badge variant="outline" className="mono">
              {MATCH_LABEL[trigger.match_mode] ?? trigger.match_mode}
            </Badge>
          </div>
        </div>
        <Switch
          checked={trigger.is_active}
          onChange={onToggle}
          disabled={toggling}
          ariaLabel="Включить/выключить триггер"
        />
      </div>

      <div className="mono overflow-x-auto whitespace-nowrap rounded-lg border bg-background px-2.5 py-2 text-xs ss-live">
        {patternText(trigger)}
      </div>

      <div className="flex gap-1.5 text-[0.84rem] leading-snug text-muted-foreground">
        <CornerDownRight className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <span className="line-clamp-2">{replyText(trigger)}</span>
      </div>

      <div className="mt-0.5 flex items-center justify-between">
        <span className="mono text-xs text-muted-foreground">приоритет {trigger.priority}</span>
        <div className="flex items-center gap-1">
          <Button asChild variant="ghost" size="sm">
            <Link to={editHref}>
              <Pencil className="h-3.5 w-3.5" />
              Изменить
            </Link>
          </Button>
          <Button variant="ghost" size="sm" onClick={onDelete} aria-label="Удалить">
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </Card>
  )
}
