import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import path from "path"

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    host: "0.0.0.0",
    port: 3000,
    // On Windows + Docker bind mounts, inotify events don't propagate from the host into the
    // Linux container, so HMR never sees file edits. Enable polling there (set VITE_USE_POLLING
    // in docker-compose) — native `npm run dev` on the host keeps fast native watching.
    watch: {
      usePolling: process.env.VITE_USE_POLLING === "true",
      interval: 300,
    },
    proxy: {
      "/api": {
        target: process.env.VITE_API_URL || "http://api:8080",
        changeOrigin: true,
      },
      // Realtime WebSocket — proxied to the API with ws upgrade enabled.
      "/ws": {
        target: process.env.VITE_API_URL || "http://api:8080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
