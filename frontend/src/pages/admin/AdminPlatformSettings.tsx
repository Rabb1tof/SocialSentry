import { toast } from "sonner"

import { Card } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { PageHead } from "@/components/brand/PageHead"
import { PlatformGlyph } from "@/components/brand/PlatformGlyph"
import { Switch } from "@/components/brand/Switch"
import { friendlyErrorMessage } from "@/lib/api-errors"
import { PLATFORM_LABEL } from "@/lib/labels"
import {
  usePlatformSettings,
  useSetPlatformEnabled,
  type Platform,
  type PlatformSetting,
} from "@/api/admin"

const PLATFORMS: Platform[] = ["instagram", "vk"]
const PLATFORM_SUB: Record<Platform, string> = {
  instagram: "OAuth-подключение и приём вебхуков",
  vk: "Long Poll API сообществ",
}

export function AdminPlatformSettingsPage() {
  const settings = usePlatformSettings()
  const setEnabled = useSetPlatformEnabled()

  const byPlatform = new Map<string, PlatformSetting>()
  for (const s of settings.data ?? []) byPlatform.set(s.platform, s)

  const onToggle = (platform: Platform, current: boolean) => {
    setEnabled.mutate(
      { platform, enabled: !current },
      {
        onSuccess: (ps) =>
          toast.success(`${PLATFORM_LABEL[platform]}: ${ps.enabled ? "включена" : "выключена"}`),
        onError: (e) => toast.error(friendlyErrorMessage(e, "Не удалось изменить настройку")),
      },
    )
  }

  return (
    <div>
      <PageHead title="Платформы" sub="Глобальное включение подключения площадок." />

      <p className="mb-4 max-w-[620px] text-sm leading-relaxed text-muted-foreground">
        При выключении воркеры останавливаются, входящие события игнорируются, а подключение новых
        аккаунтов блокируется.
      </p>

      {settings.isLoading && <Skeleton className="h-40 w-full" />}
      {settings.isError && (
        <Card className="py-4 text-center text-sm text-destructive">
          Не удалось загрузить настройки платформ.
        </Card>
      )}

      {settings.data && (
        <div className="grid max-w-[620px] gap-3.5">
          {PLATFORMS.map((p) => {
            const enabled = byPlatform.get(p)?.enabled ?? true
            return (
              <Card key={p} className="flex items-center gap-3.5 p-[18px]">
                <PlatformGlyph kind={p} size={40} />
                <div className="flex-1">
                  <div className="font-semibold">{PLATFORM_LABEL[p]}</div>
                  <div className="text-sm text-muted-foreground">{PLATFORM_SUB[p]}</div>
                </div>
                <Switch
                  checked={enabled}
                  tone="azure"
                  disabled={setEnabled.isPending}
                  ariaLabel={PLATFORM_LABEL[p]}
                  onChange={() => onToggle(p, enabled)}
                />
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
