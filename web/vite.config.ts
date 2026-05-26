import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const proxyTarget = process.env.VITE_PROXY_TARGET ?? "http://localhost:8787";
const devPort = Number(process.env.VITE_DEV_PORT ?? 7878);
const devHost = process.env.VITE_DEV_HOST ?? "localhost";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: devHost,
    port: devPort,
    proxy: {
      "/api": proxyTarget,
      "/v1": proxyTarget
    }
  }
});
