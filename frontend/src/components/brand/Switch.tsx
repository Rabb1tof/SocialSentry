import { cn } from "@/lib/utils"

interface SwitchProps {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  /** On-colour: success (triggers) or azure (admin platform toggles). */
  tone?: "success" | "azure"
  ariaLabel?: string
  className?: string
}

// Switch — a small sliding toggle (no extra deps), matching the UI-kit toggles.
export function Switch({ checked, onChange, disabled, tone = "success", ariaLabel, className }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-6 w-11 shrink-0 items-center rounded-full p-0.5 transition-colors disabled:opacity-50",
        checked ? (tone === "azure" ? "bg-azure" : "bg-success") : "bg-secondary",
        className,
      )}
    >
      <span
        className={cn(
          "h-5 w-5 rounded-full bg-white shadow transition-transform",
          checked ? "translate-x-5" : "translate-x-0",
        )}
      />
    </button>
  )
}
