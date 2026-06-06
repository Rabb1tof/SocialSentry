import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"

import { Card } from "@/components/ui/card"
import { cn } from "@/lib/utils"

interface StatCardProps {
  icon: LucideIcon
  label: string
  sub?: string
  /** Colour of the icon chip. Omit for a neutral chip. */
  accent?: "grad" | "success" | "violet"
  children?: ReactNode
}

const accentClass: Record<NonNullable<StatCardProps["accent"]>, string> = {
  grad: "bg-azure text-white",
  success: "bg-success text-white",
  violet: "bg-accent text-accent-foreground",
}

// StatCard — a dashboard summary card with an icon chip header and free-form body.
export function StatCard({ icon: Icon, label, sub, accent, children }: StatCardProps) {
  return (
    <Card className="p-[18px]">
      <div className="flex items-center gap-2.5">
        <span
          className={cn(
            "inline-flex h-9 w-9 items-center justify-center rounded-lg",
            accent ? accentClass[accent] : "bg-secondary text-muted-foreground",
          )}
        >
          <Icon className="h-[18px] w-[18px]" />
        </span>
        <div className="leading-tight">
          <div className="text-sm font-semibold">{label}</div>
          {sub && <div className="text-xs text-muted-foreground">{sub}</div>}
        </div>
      </div>
      {children && <div className="mt-3.5">{children}</div>}
    </Card>
  )
}
