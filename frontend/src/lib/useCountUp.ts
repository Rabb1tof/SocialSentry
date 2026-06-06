import { useEffect, useRef, useState } from "react"

const prefersReducedMotion = () =>
  typeof window !== "undefined" &&
  window.matchMedia("(prefers-reduced-motion: reduce)").matches

// useCountUp — eases a number up to `value` with requestAnimationFrame.
// Returns the final value immediately under prefers-reduced-motion, and
// re-animates from the previous value whenever `value` changes.
export function useCountUp(value: number, durationMs = 900): number {
  const [display, setDisplay] = useState(() =>
    prefersReducedMotion() || !Number.isFinite(value) ? value : 0,
  )
  const fromRef = useRef(0)

  useEffect(() => {
    if (prefersReducedMotion() || !Number.isFinite(value)) {
      setDisplay(value)
      fromRef.current = value
      return
    }

    const from = fromRef.current
    const start = performance.now()
    let raf = 0

    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / durationMs)
      const eased = 1 - Math.pow(1 - t, 3) // easeOutCubic
      setDisplay(Math.round(from + (value - from) * eased))
      if (t < 1) {
        raf = requestAnimationFrame(tick)
      } else {
        fromRef.current = value
      }
    }

    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [value, durationMs])

  return display
}
