import { useEffect, type ReactNode } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"

import { cn } from "@/lib/utils"
import { useVisualViewport } from "@/lib/useVisualViewport"

interface ModalProps {
  open: boolean
  onClose: () => void
  title: ReactNode
  children: ReactNode
  footer?: ReactNode
  className?: string
}

// Modal — a lightweight dialog (no extra deps). Rendered through a portal into
// document.body so `position: fixed` is always relative to the viewport, even
// when an ancestor has a transform (e.g. the page-transition wrapper). Backdrop
// is a SOLID scrim (no backdrop-filter — that one breaks on iOS when the keyboard
// resizes the viewport). Closes on backdrop click and Escape.
export function Modal({ open, onClose, title, children, footer, className }: ModalProps) {
  const vp = useVisualViewport(open)

  useEffect(() => {
    if (!open) return
    // Lock background scroll while the modal is open (no bleed on mobile).
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = "hidden"
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", onKey)
    return () => {
      document.body.style.overflow = prevOverflow
      document.removeEventListener("keydown", onKey)
    }
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div
      className="animate-fade-in fixed inset-0 z-50 overflow-y-auto bg-black/60"
      role="dialog"
      aria-modal="true"
      onClick={onClose}
    >
      {/* min-h-full centers the card when it fits and lets the overlay scroll when
          it doesn't (top stays reachable). minHeight follows the visible viewport
          so the card sits in the area above the on-screen keyboard. */}
      <div
        className="flex min-h-full items-center justify-center p-4"
        style={vp ? { minHeight: vp.height } : undefined}
      >
        <div
          className={cn(
            "animate-pop-in w-full max-w-md rounded-xl border bg-card text-card-foreground shadow-lg",
            className,
          )}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center justify-between gap-3 border-b px-5 py-3.5">
            <h2 className="text-lg font-semibold">{title}</h2>
            <button
              type="button"
              onClick={onClose}
              aria-label="Закрыть"
              className="text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <div className="p-5">{children}</div>
          {footer && <div className="flex justify-end gap-2 border-t px-5 py-3">{footer}</div>}
        </div>
      </div>
    </div>,
    document.body,
  )
}
