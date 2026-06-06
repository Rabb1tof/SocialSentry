import { Navigate, Outlet, createBrowserRouter, useLocation } from "react-router-dom"

import { useMe } from "@/api/auth"
import { useAuthStore } from "@/store/auth"
import { Skeleton } from "@/components/ui/skeleton"
import { DashboardLayout } from "@/components/layout/DashboardLayout"
import { LoginPage } from "@/pages/auth/Login"
import { RegisterPage } from "@/pages/auth/Register"
import { DashboardPage } from "@/pages/dashboard/Dashboard"
import { SubscriptionPage } from "@/pages/subscription/Subscription"
import { AdminUsersPage } from "@/pages/admin/AdminUsers"
import { AdminSubscriptionsPage } from "@/pages/admin/AdminSubscriptions"
import { AdminPlatformSettingsPage } from "@/pages/admin/AdminPlatformSettings"
import { AccountsListPage } from "@/pages/accounts/AccountsList"
import { TriggersListPage } from "@/pages/triggers/TriggersList"
import { TriggerFormPage } from "@/pages/triggers/TriggerForm"
import { AccountLogsPage } from "@/pages/logs/AccountLogs"

function ProtectedRoute() {
  const location = useLocation()
  const accessToken = useAuthStore((s) => s.accessToken)
  const { data: user, isLoading, isError } = useMe()

  if (!accessToken && !isLoading) {
    return <Navigate to={`/login?next=${encodeURIComponent(location.pathname)}`} replace />
  }
  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Skeleton className="h-24 w-72" />
      </div>
    )
  }
  if (isError || !user) {
    return <Navigate to={`/login?next=${encodeURIComponent(location.pathname)}`} replace />
  }
  return <Outlet />
}

function AdminRoute() {
  const { data: user, isLoading } = useMe()
  if (isLoading) {
    return (
      <div className="p-6">
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }
  if (!user || user.role !== "admin") {
    return <Navigate to="/dashboard" replace />
  }
  return <Outlet />
}

function PublicOnlyRoute() {
  const accessToken = useAuthStore((s) => s.accessToken)
  const { data: user } = useMe()
  if (accessToken && user) {
    return <Navigate to="/dashboard" replace />
  }
  return <Outlet />
}

export const router = createBrowserRouter([
  {
    element: <PublicOnlyRoute />,
    children: [
      { path: "/login", element: <LoginPage /> },
      { path: "/register", element: <RegisterPage /> },
    ],
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <DashboardLayout />,
        children: [
          { path: "/dashboard", element: <DashboardPage /> },
          { path: "/subscription", element: <SubscriptionPage /> },
          { path: "/accounts", element: <AccountsListPage /> },
          { path: "/accounts/:id/triggers", element: <TriggersListPage /> },
          { path: "/accounts/:id/triggers/new", element: <TriggerFormPage /> },
          { path: "/accounts/:id/triggers/:tid", element: <TriggerFormPage /> },
          { path: "/accounts/:id/logs", element: <AccountLogsPage /> },
          {
            element: <AdminRoute />,
            children: [
              { path: "/admin", element: <Navigate to="/admin/users" replace /> },
              { path: "/admin/users", element: <AdminUsersPage /> },
              { path: "/admin/subscriptions", element: <AdminSubscriptionsPage /> },
              { path: "/admin/platform-settings", element: <AdminPlatformSettingsPage /> },
            ],
          },
        ],
      },
    ],
  },
  {
    path: "/",
    element: <Navigate to="/dashboard" replace />,
  },
  {
    path: "*",
    element: <Navigate to="/dashboard" replace />,
  },
])
