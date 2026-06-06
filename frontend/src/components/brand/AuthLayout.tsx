import type { ReactNode } from "react"
import { MessagesSquare, Moon, ShieldCheck, Sun, Zap, type LucideIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { useThemeStore } from "@/store/theme"
import { Wordmark } from "@/components/brand/Wordmark"

interface AuthLayoutProps {
  title: string
  subtitle: string
  children: ReactNode // the form
  footer: ReactNode // switch-mode link line
}

const FEATURES: [LucideIcon, string][] = [
  [MessagesSquare, "Комментарии + ЛС"],
  [Zap, "Real-time движок"],
  [ShieldCheck, "Кулдаун и лимиты"],
]

// AuthLayout — split brand/form layout for login & register, with a theme toggle.
// The brand panel adapts to the theme (light gradient / dark navy) and collapses
// on small screens, where the form panel takes over and shows the wordmark inline.
export function AuthLayout({ title, subtitle, children, footer }: AuthLayoutProps) {
  const theme = useThemeStore((s) => s.theme)
  const toggle = useThemeStore((s) => s.toggle)

  return (
    <div className="flex min-h-screen overflow-hidden">
      {/* Brand panel */}
      <div className="relative hidden flex-[1.1] flex-col justify-between overflow-hidden border-r p-10 lg:flex bg-[linear-gradient(157deg,#EAF0FF_0%,#F6F8FC_70%)] dark:border-transparent dark:bg-[linear-gradient(157deg,#0B1120_0%,#070A12_70%)]">
        {/* Restrained ambient: one soft azure wash + a faint, theme-aware dot grid.
            (Replaces the old azure+magenta blur-blob mesh.) */}
        <div
          aria-hidden
          className="pointer-events-none absolute -top-40 left-1/2 h-[480px] w-[680px] -translate-x-1/2 rounded-full opacity-[0.14] blur-3xl"
          style={{ background: "radial-gradient(circle, hsl(var(--azure)), transparent 70%)" }}
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 [background-image:radial-gradient(hsl(var(--foreground)/0.05)_1px,transparent_1px)] [background-size:22px_22px] [mask-image:radial-gradient(ellipse_at_center,black,transparent_72%)]"
        />
        <div className="animate-fade-in-up relative">
          <Wordmark size={30} />
        </div>
        <div className="stagger relative max-w-[460px]">
          <h1 className="text-[2.6rem] font-extrabold leading-[1.08] tracking-tight text-foreground">
            Автоответы, которые <span className="text-primary">не спят</span>.
          </h1>
          <p className="mt-[18px] text-[1.03rem] leading-relaxed text-muted-foreground">
            Подключайте Instagram и VK, собирайте триггеры по ключевым словам и regex —
            SocialSentry отвечает на комментарии и сообщения за вас, в реальном времени.
          </p>
          <div className="mt-7 flex flex-wrap gap-5">
            {FEATURES.map(([Icon, label]) => (
              <div key={label} className="flex items-center gap-2 text-[0.84rem] text-muted-foreground">
                <Icon className="h-[17px] w-[17px] text-azure dark:text-cyan" />
                {label}
              </div>
            ))}
          </div>
        </div>
        <div className="animate-fade-in relative text-xs text-muted-foreground">
          © 2026 SocialSentry · Мультиаккаунтность для Instagram и VK
        </div>
      </div>

      {/* Form panel */}
      <div className="relative flex flex-1 flex-col bg-background">
        <div className="absolute right-4 top-4">
          <Button variant="ghost" size="icon" onClick={toggle} aria-label="Переключить тему">
            {theme === "dark" ? <Sun className="h-[18px] w-[18px]" /> : <Moon className="h-[18px] w-[18px]" />}
          </Button>
        </div>
        <div className="animate-fade-in-up m-auto w-full max-w-[380px] p-7">
          <div className="mb-6 lg:hidden">
            <Wordmark size={28} />
          </div>
          <h2 className="text-[1.6rem] font-bold tracking-tight">{title}</h2>
          <p className="mb-6 mt-1.5 text-[0.9rem] text-muted-foreground">{subtitle}</p>
          {children}
          <div className="mt-6 text-center text-sm text-muted-foreground">{footer}</div>
        </div>
      </div>
    </div>
  )
}
