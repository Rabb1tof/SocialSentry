import { useEffect, useState } from "react"

export interface ViewportRect {
  /** Height of the visible area, i.e. excluding the on-screen keyboard. */
  height: number
  /** Top offset of the visual viewport within the layout viewport. */
  offsetTop: number
  /** Heuristic: the keyboard (or similar UI) covers a meaningful slice of the screen. */
  keyboardOpen: boolean
}

// useVisualViewport — tracks the visual viewport so a modal can stay inside the
// area NOT covered by the mobile keyboard. Returns null when the API is missing
// (SSR / old browsers); callers then fall back to plain CSS centering.
export function useVisualViewport(active: boolean): ViewportRect | null {
  const [rect, setRect] = useState<ViewportRect | null>(null)

  useEffect(() => {
    if (!active || typeof window === "undefined") return
    const vv = window.visualViewport
    if (!vv) return

    const update = () => {
      setRect({
        height: vv.height,
        offsetTop: vv.offsetTop,
        // >120px gap between layout and visual viewport ≈ keyboard is up
        // (small gaps from browser chrome don't count).
        keyboardOpen: window.innerHeight - vv.height > 120,
      })
    }

    update()
    vv.addEventListener("resize", update)
    vv.addEventListener("scroll", update)
    return () => {
      vv.removeEventListener("resize", update)
      vv.removeEventListener("scroll", update)
    }
  }, [active])

  return rect
}
