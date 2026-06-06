import { cn } from "@/lib/utils"

interface WordmarkProps {
  /** Logo-mark height in px (wordmark scales with it). */
  size?: number
  className?: string
}

// Wordmark — logo mark + "Social" (theme foreground) / "Sentry" (brand gradient).
export function Wordmark({ size = 28, className }: WordmarkProps) {
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <img src="/logo-mark.png" alt="" style={{ height: size, width: size }} className="shrink-0" />
      <span className="font-extrabold tracking-tight" style={{ fontSize: size * 0.82 }}>
        <span className="text-foreground">Social</span>
        <span className="ss-grad-text">Sentry</span>
      </span>
    </span>
  )
}
