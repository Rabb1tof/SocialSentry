import axios, {
  type AxiosError,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from "axios"

import { useAuthStore } from "@/store/auth"

interface RetryFlag {
  _retried?: boolean
}
type RetryableConfig = InternalAxiosRequestConfig & RetryFlag

export const api = axios.create({
  baseURL: "/api/v1",
  withCredentials: true, // include the refresh cookie
})

// Attach Bearer token to every request when present.
api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) {
    config.headers = config.headers ?? {}
    config.headers["Authorization"] = `Bearer ${token}`
  }
  return config
})

// On 401, try /auth/refresh once and retry the original request.
api.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error: AxiosError) => {
    const original = error.config as RetryableConfig | undefined
    if (!original || original._retried) {
      throw error
    }
    const status = error.response?.status
    const url = original.url ?? ""

    // Don't try to refresh on the refresh endpoint itself, or on /auth/login.
    if (url.endsWith("/auth/refresh") || url.endsWith("/auth/login")) {
      throw error
    }

    if (status === 401) {
      original._retried = true
      try {
        const refreshResp = await axios.post<{ data: { access_token: string } }>(
          "/api/v1/auth/refresh",
          null,
          { withCredentials: true },
        )
        const newToken = refreshResp.data?.data?.access_token
        if (newToken) {
          useAuthStore.getState().setAccessToken(newToken)
          original.headers = original.headers ?? {}
          original.headers["Authorization"] = `Bearer ${newToken}`
          return api.request(original)
        }
      } catch {
        useAuthStore.getState().clear()
      }
    }

    throw error
  },
)
