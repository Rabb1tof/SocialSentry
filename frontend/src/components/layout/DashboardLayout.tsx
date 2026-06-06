import { useEffect, useState } from "react"
import { NavLink, Outlet, useLocation } from "react-router-dom"
import {
  AtSign,
  BadgeCheck,
  CreditCard,
  LayoutDashboard,
  Menu,
  Moon,
  SlidersHorizontal,
  Sun,
  Users,
  X,
  type LucideIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { useLogout, useMe } from "@/api/auth"
import { useRealtime } from "@/lib/realtime"
import { useThemeStore } from "@/store/theme"
import { Wordmark } from "@/components/brand/Wordmark"
import { SubscriptionBanner } from "@/components/SubscriptionBanner"

interface NavItem {
  to: string
  label: string
  icon: LucideIcon
}

const NAV: NavItem[] = [
  { to: "/dashboard", label: "Дашборд", icon: LayoutDashboard },
  { to: "/accounts", label: "Аккаунты", icon: AtSign },
  { to: "/subscription", label: "Подписка", icon: CreditCard },
]

const ADMIN_NAV: NavItem[] = [
  { to: "/admin/users", label: "Пользователи", icon: Users },
  { to: "/admin/subscriptions", label: "Подписки", icon: BadgeCheck },
  { to: "/admin/platform-settings", label: "Платформы", icon: SlidersHorizontal },
]

// Title shown in the sticky topbar, derived from the current path.
function pageTitle(pathname: string): string {
  if (pathname.includes("/triggers")) return "Триггеры"
  if (pathname.includes("/logs")) return "Логи"
  if (pathname.startsWith("/accounts")) return "Аккаунты"
  if (pathname.startsWith("/subscription")) return "Подписка"
  if (pathname.startsWith("/admin/users")) return "Пользователи"
  if (pathname.startsWith("/admin/subscriptions")) return "Подписки"
  if (pathname.startsWith("/admin/platform-settings")) return "Платформы"
  return "Дашборд"
}

function NavRail({ items, onNavigate }: { items: NavItem[]; onNavigate?: () => void }) {
  return (
    <div className="space-y-0.5">
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              "flex items-center gap-3 rounded-[10px] px-3 py-2.5 text-sm font-medium transition-colors",
              isActive
                ? "ss-grad"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground",
            )
          }
        >
          <item.icon className="h-[18px] w-[18px] shrink-0" />
          {item.label}
        </NavLink>
      ))}
    </div>
  )
}

function NavSection({ label }: { label: string }) {
  return (
    <div className="px-3 pb-1 pt-4 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
      {label}
    </div>
  )
}

// SidebarNav is the menu body, shared between the desktop rail and the mobile drawer.
function SidebarNav({ isAdmin, onNavigate }: { isAdmin: boolean; onNavigate?: () => void }) {
  return (
    <nav className="p-2.5">
      <NavSection label="Меню" />
      <NavRail items={NAV} onNavigate={onNavigate} />
      {isAdmin && (
        <>
          <NavSection label="Администрирование" />
          <NavRail items={ADMIN_NAV} onNavigate={onNavigate} />
        </>
      )}
    </nav>
  )
}

export function DashboardLayout() {
  const { data: user } = useMe()
  const logout = useLogout()
  const isAdmin = user?.role === "admin"
  const theme = useThemeStore((s) => s.theme)
  const toggleTheme = useThemeStore((s) => s.toggle)
  const location = useLocation()
  const [drawerOpen, setDrawerOpen] = useState(false)

  // Live updates: refresh logs + toast on trigger fires while authenticated.
  useRealtime()

  // Close the mobile drawer on navigation and on Escape.
  useEffect(() => {
    setDrawerOpen(false)
  }, [location.pathname])
  useEffect(() => {
    if (!drawerOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setDrawerOpen(false)
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [drawerOpen])

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      {/* Desktop sidebar (md and up) */}
      <aside className="hidden w-64 flex-col border-r bg-card md:flex">
        <div className="border-b px-5 py-[18px]">
          <Wordmark size={26} />
        </div>
        <SidebarNav isAdmin={isAdmin} />
      </aside>

      {/* Mobile drawer (below md) */}
      <div
        className={cn("fixed inset-0 z-[60] md:hidden", !drawerOpen && "pointer-events-none")}
        aria-hidden={!drawerOpen}
      >
        <div
          onClick={() => setDrawerOpen(false)}
          className={cn(
            "absolute inset-0 bg-black/50 backdrop-blur-sm transition-opacity duration-200",
            drawerOpen ? "opacity-100" : "opacity-0",
          )}
        />
        <aside
          className={cn(
            "absolute inset-y-0 left-0 flex w-72 max-w-[82%] flex-col border-r bg-card shadow-xl transition-transform duration-200 ease-out",
            drawerOpen ? "translate-x-0" : "-translate-x-full",
          )}
        >
          <div className="flex items-center justify-between border-b px-5 py-[18px]">
            <Wordmark size={26} />
            <button
              type="button"
              onClick={() => setDrawerOpen(false)}
              aria-label="Закрыть меню"
              className="text-muted-foreground hover:text-foreground"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
          <SidebarNav isAdmin={isAdmin} onNavigate={() => setDrawerOpen(false)} />
        </aside>
      </div>

      {/* min-w-0 lets wide tables scroll inside their card instead of widening the page. */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-50 flex items-center gap-2 border-b bg-card/80 px-4 py-3 backdrop-blur sm:px-6">
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            onClick={() => setDrawerOpen(true)}
            aria-label="Меню"
          >
            <Menu className="h-5 w-5" />
          </Button>
          <div className="truncate text-base font-bold tracking-tight">
            {pageTitle(location.pathname)}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <span className="hidden max-w-[40vw] truncate text-sm text-muted-foreground sm:inline">
              {user?.email}
            </span>
            {isAdmin && (
              <span className="hidden rounded bg-primary px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-primary-foreground sm:inline">
                admin
              </span>
            )}
            <Button
              variant="ghost"
              size="icon"
              onClick={toggleTheme}
              aria-label="Переключить тему"
              title={theme === "dark" ? "Светлая тема" : "Тёмная тема"}
            >
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => logout.mutate()}
              disabled={logout.isPending}
            >
              Выйти
            </Button>
          </div>
        </header>
        <main className="flex-1 p-4 sm:p-6">
          <div className="mx-auto max-w-[1080px]">
            {!isAdmin && <SubscriptionBanner />}
            {/* Replay a gentle entry on each route change (keyed by path). */}
            <div key={location.pathname} className="animate-fade-in-up">
              <Outlet />
            </div>
          </div>
        </main>
      </div>
    </div>
  )
}
