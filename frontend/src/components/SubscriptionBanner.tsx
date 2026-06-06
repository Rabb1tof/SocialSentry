import { Link } from "react-router-dom"
import { TriangleAlert } from "lucide-react"

import { useMySubscription } from "@/api/subscription"

export function SubscriptionBanner() {
  const { data, isLoading } = useMySubscription()
  if (isLoading || !data) return null
  if (data.status === "active") return null

  return (
    <div className="mb-4 flex items-start gap-2.5 rounded-xl border border-warning/40 bg-warning/10 px-4 py-3 text-sm">
      <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" />
      <div>
        <span className="font-semibold">Подписка не активна.</span>{" "}
        Без активной подписки вы не можете подключать аккаунты или редактировать триггеры.{" "}
        <Link to="/subscription" className="font-medium text-primary hover:underline">
          Подробнее
        </Link>
      </div>
    </div>
  )
}
