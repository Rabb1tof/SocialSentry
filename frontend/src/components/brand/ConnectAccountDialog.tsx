import { useEffect, useState } from "react"
import type { UseFormReturn } from "react-hook-form"
import { z } from "zod"
import { ArrowLeft } from "lucide-react"

import { Modal } from "@/components/brand/Modal"
import { PlatformGlyph } from "@/components/brand/PlatformGlyph"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"

export const vkSchema = z.object({
  group_id: z.string().regex(/^\d+$/, "Только цифры"),
  community_token: z.string().min(10, "Слишком короткий"),
})
export type VKForm = z.infer<typeof vkSchema>

interface ConnectAccountDialogProps {
  open: boolean
  onClose: () => void
  igEnabled: boolean
  vkEnabled: boolean
  onConnectIG: () => void
  connectingIG: boolean
  vkForm: UseFormReturn<VKForm>
  onSubmitVK: (vals: VKForm) => void
  connectingVK: boolean
}

// ConnectAccountDialog — single entry point for adding an account. Step 1 is a
// platform picker (two matching cards); Instagram redirects to OAuth, VK opens
// step 2 (the community_token form). Keeps both platforms visually consistent.
export function ConnectAccountDialog({
  open,
  onClose,
  igEnabled,
  vkEnabled,
  onConnectIG,
  connectingIG,
  vkForm,
  onSubmitVK,
  connectingVK,
}: ConnectAccountDialogProps) {
  const [step, setStep] = useState<"picker" | "vk">("picker")

  // Always start from the picker when (re)opening.
  useEffect(() => {
    if (open) setStep("picker")
  }, [open])

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={step === "vk" ? "Подключить VK сообщество" : "Подключить аккаунт"}
    >
      {step === "picker" ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <PlatformChoice
            kind="instagram"
            name="Instagram"
            desc="OAuth-подключение и приём вебхуков"
            disabled={!igEnabled}
            busy={connectingIG}
            onClick={onConnectIG}
          />
          <PlatformChoice
            kind="vk"
            name="VK"
            desc="Long Poll API сообществ"
            disabled={!vkEnabled}
            onClick={() => setStep("vk")}
          />
        </div>
      ) : (
        <form onSubmit={vkForm.handleSubmit(onSubmitVK)} className="space-y-3.5">
          <div className="space-y-1.5">
            <Label htmlFor="group_id">ID сообщества</Label>
            <Input id="group_id" className="mono" placeholder="123456789" {...vkForm.register("group_id")} />
            {vkForm.formState.errors.group_id && (
              <p className="text-xs text-destructive">{vkForm.formState.errors.group_id.message}</p>
            )}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="community_token">Community Token</Label>
            <Input
              id="community_token"
              type="password"
              className="mono"
              placeholder="vk1.a..."
              {...vkForm.register("community_token")}
            />
            {vkForm.formState.errors.community_token && (
              <p className="text-xs text-destructive">{vkForm.formState.errors.community_token.message}</p>
            )}
          </div>
          <p className="text-xs leading-relaxed text-muted-foreground">
            Создать токен: <span className="mono">Управление сообществом → Работа с API → Ключи доступа</span>.
            Включить Long Poll: <span className="mono">Работа с API → Long Poll API → Включён</span>.
            Нужны права <code className="mono">messages</code>, <code className="mono">wall</code>,{" "}
            <code className="mono">manage</code>.
          </p>
          <div className="flex justify-between gap-2 pt-1">
            <Button type="button" variant="ghost" onClick={() => setStep("picker")}>
              <ArrowLeft className="h-4 w-4" />
              Назад
            </Button>
            <Button type="submit" disabled={connectingVK}>
              {connectingVK ? "Подключаем…" : "Подключить"}
            </Button>
          </div>
        </form>
      )}
    </Modal>
  )
}

interface PlatformChoiceProps {
  kind: "instagram" | "vk"
  name: string
  desc: string
  disabled?: boolean
  busy?: boolean
  onClick: () => void
}

function PlatformChoice({ kind, name, desc, disabled, busy, onClick }: PlatformChoiceProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "flex flex-col items-start gap-2.5 rounded-xl border p-4 text-left transition-colors",
        disabled ? "cursor-not-allowed opacity-50" : "hover:bg-secondary",
      )}
    >
      <PlatformGlyph kind={kind} size={36} />
      <div>
        <div className="font-semibold">{name}</div>
        <div className="text-xs text-muted-foreground">{desc}</div>
      </div>
      <span className="mt-1 text-sm font-medium text-primary">
        {disabled ? "Недоступно" : busy ? "Открываем…" : "Подключить →"}
      </span>
    </button>
  )
}
