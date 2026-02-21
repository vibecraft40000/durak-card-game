import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    port: 5173,
    host: "0.0.0.0",
    allowedHosts: true,
    proxy: {
      "/api": {
        target: "http://localhost:8090",
        changeOrigin: true,
      },
      "/auth": {
        target: "http://localhost:8090",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://localhost:8090",
        ws: true,
        changeOrigin: true,
      },
      "/health": {
        target: "http://localhost:8090",
        changeOrigin: true,
      },
    },
  },
});
