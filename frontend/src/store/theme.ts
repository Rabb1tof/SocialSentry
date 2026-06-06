import { create } from "zustand"

type Theme = "light" | "dark"

// The initial `dark` class is set synchronously in index.html (before paint) to
// avoid a flash; the store stays in sync with that DOM state and the same
// localStorage key ("theme" → "light" | "dark", stored as a raw string).
function getInitialTheme(): Theme {
  if (typeof document === "undefined") return "light"
  return document.documentElement.classList.contains("dark") ? "dark" : "light"
}

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark")
  try {
    localStorage.setItem("theme", theme)
  } catch {
    // ignore (e.g. storage disabled)
  }
}

interface ThemeState {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggle: () => void
}

export const useThemeStore = create<ThemeState>()((set, get) => ({
  theme: getInitialTheme(),
  setTheme: (theme) => {
    applyTheme(theme)
    set({ theme })
  },
  toggle: () => {
    const next: Theme = get().theme === "dark" ? "light" : "dark"
    applyTheme(next)
    set({ theme: next })
  },
}))
