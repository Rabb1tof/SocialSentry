import { cn } from "@/lib/utils"

// Skeleton — a shimmer sweep (falls back to a static block under reduced motion;
// the `.skeleton-shimmer` sweep is defined in index.css and gated by the media query).
export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("skeleton-shimmer rounded-md bg-muted", className)} {...props} />
  )
}
