import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useNavigate, Link } from "react-router-dom"
import { toast } from "sonner"
import { Lock, Mail } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { AuthLayout } from "@/components/brand/AuthLayout"
import { useRegister } from "@/api/auth"

const schema = z.object({
  email: z.string().email("Введите корректный email"),
  password: z.string().min(8, "Пароль должен быть не короче 8 символов"),
})

type FormValues = z.infer<typeof schema>

export function RegisterPage() {
  const navigate = useNavigate()
  const register_ = useRegister()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const onSubmit = handleSubmit(async (values) => {
    try {
      await register_.mutateAsync(values)
      toast.success("Аккаунт создан")
      navigate("/dashboard", { replace: true })
    } catch (err: unknown) {
      const msg = extractMessage(err) ?? "Не удалось создать аккаунт."
      toast.error(msg)
    }
  })

  return (
    <AuthLayout
      title="Регистрация"
      subtitle="Создайте аккаунт для управления автоответами."
      footer={
        <>
          Уже есть аккаунт?{" "}
          <Link to="/login" className="font-semibold text-primary hover:underline">
            Войти
          </Link>
        </>
      }
    >
      <form className="flex flex-col gap-4" onSubmit={onSubmit}>
        <div className="space-y-1.5">
          <Label htmlFor="email">Email</Label>
          <div className="relative">
            <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input id="email" type="email" autoComplete="email" className="pl-9" {...register("email")} />
          </div>
          {errors.email && <p className="text-sm text-destructive">{errors.email.message}</p>}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="password">Пароль</Label>
          <div className="relative">
            <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="password"
              type="password"
              autoComplete="new-password"
              className="pl-9"
              {...register("password")}
            />
          </div>
          {errors.password && <p className="text-sm text-destructive">{errors.password.message}</p>}
        </div>
        <Button
          variant="grad"
          type="submit"
          className="mt-1 w-full"
          disabled={isSubmitting || register_.isPending}
        >
          {register_.isPending ? "Создаём…" : "Создать аккаунт"}
        </Button>
      </form>
    </AuthLayout>
  )
}

function extractMessage(err: unknown): string | undefined {
  if (typeof err === "object" && err !== null) {
    const e = err as { response?: { data?: { message?: string } } }
    return e.response?.data?.message
  }
  return undefined
}
