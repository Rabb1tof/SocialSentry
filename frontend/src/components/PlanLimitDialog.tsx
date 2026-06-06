import { createPortal } from "react-dom"
import { Link } from "react-router-dom"
import { CircleAlert } from "lucide-react"

import { Button } from "@/components/ui/button"

interface PlanLimitDialogProps {
  open: boolean
  message: string
  onClose: () => void
}

/**
 * PlanLimitDialog is a small modal shown when the backend rejects a mutation with
 * `limit_exceeded` or `platform_not_allowed`. It points the user to the subscription
 * page for upgrade context. Render at the page root and toggle `open` from mutation
 * `onError` callbacks (typically via the `friendlyPlanError` helper from `@/lib/api-errors`).
 */
export function PlanLimitDialog({ open, message, onClose }: PlanLimitDialogProps) {
  if (!open) return null
  return createPortal(
    <div
      className="animate-fade-in fixed inset-0 z-50 overflow-y-auto bg-black/60"
      role="dialog"
      aria-modal="true"
      onClick={onClose}
    >
      <div className="flex min-h-full items-center justify-center p-4">
        <div
          className="animate-pop-in w-full max-w-md rounded-xl border bg-card text-card-foreground shadow-lg"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center gap-2.5 border-b px-5 py-3.5">
            <span className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-warning/15 text-warning">
              <CircleAlert className="h-[18px] w-[18px]" />
            </span>
            <h2 className="text-lg font-semibold">Ограничения тарифа</h2>
          </div>
          <div className="space-y-3 p-5 text-sm">
            <p>{message}</p>
            <p className="text-muted-foreground">
              Посмотреть текущий статус подписки можно на странице{" "}
              <Link
                to="/subscription"
                className="font-medium text-primary hover:underline"
                onClick={onClose}
              >
                «Подписка»
              </Link>
              .
            </p>
          </div>
          <div className="flex justify-end gap-2 border-t px-5 py-3">
            <Button asChild variant="outline" size="sm">
              <Link to="/subscription" onClick={onClose}>
                Открыть подписку
              </Link>
            </Button>
            <Button size="sm" onClick={onClose}>
              Закрыть
            </Button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  )
}
