import { Instagram } from "lucide-react"

import { cn } from "@/lib/utils"

interface PlatformGlyphProps {
  kind: "instagram" | "vk"
  size?: number
  className?: string
}

// PlatformGlyph — the brand mark for a connected platform: Instagram's multicolour
// gradient square, or VK's blue square with "VK". Mirrors the UI-kit `Platform` glyph.
export function PlatformGlyph({ kind, size = 32, className }: PlatformGlyphProps) {
  const box = { width: size, height: size, borderRadius: Math.round(size * 0.28) }
  if (kind === "instagram") {
    return (
      <span
        className={cn("inline-flex shrink-0 items-center justify-center bg-ig-gradient text-white", className)}
        style={box}
      >
        <Instagram size={Math.round(size * 0.56)} strokeWidth={2.2} />
      </span>
    )
  }
  return (
    <span
      className={cn("inline-flex shrink-0 items-center justify-center bg-vk font-extrabold text-white", className)}
      style={{ ...box, fontSize: Math.round(size * 0.42) }}
    >
      VK
    </span>
  )
}
