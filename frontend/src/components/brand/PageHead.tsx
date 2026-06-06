import type { ReactNode } from "react"

interface PageHeadProps {
  title: string
  sub?: ReactNode
  children?: ReactNode
}

// PageHead — the standard screen header: big title + optional subtitle on the left,
// action buttons on the right. Mirrors the UI-kit `PageHead`.
export function PageHead({ title, sub, children }: PageHeadProps) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-[1.7rem] font-bold leading-tight tracking-tight">{title}</h1>
        {sub && <p className="mt-1 text-sm text-muted-foreground">{sub}</p>}
      </div>
      {children && <div className="flex flex-wrap items-center gap-2.5">{children}</div>}
    </div>
  )
}
