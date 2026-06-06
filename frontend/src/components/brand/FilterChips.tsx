import { cn } from "@/lib/utils"

interface FilterChipsProps<T extends string> {
  current: T
  items: { id: T; label: string }[]
  counts: Record<T, number>
  onChange: (id: T) => void
}

// FilterChips — segmented filter pills with per-segment counts (logs screen).
export function FilterChips<T extends string>({ current, items, counts, onChange }: FilterChipsProps<T>) {
  return (
    <div className="flex flex-wrap gap-2">
      {items.map((item) => {
        const active = item.id === current
        return (
          <button
            key={item.id}
            type="button"
            onClick={() => onChange(item.id)}
            className={cn(
              "inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-[13px] font-medium transition-colors",
              active
                ? "border-transparent bg-primary text-primary-foreground"
                : "border-input bg-card text-muted-foreground hover:bg-secondary hover:text-foreground",
            )}
          >
            {item.label}
            <span
              className={cn(
                "mono rounded px-1.5 py-px text-[11px]",
                active ? "bg-white/20 text-primary-foreground" : "bg-secondary text-muted-foreground",
              )}
            >
              {counts[item.id]}
            </span>
          </button>
        )
      })}
    </div>
  )
}
