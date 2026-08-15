/// <reference types="vitest/config" />
import path from "node:path";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

const repoRoot = path.resolve(__dirname, "../..");

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, repoRoot, "");
  const apiTarget = env.VITE_API_PROXY || "http://127.0.0.1:8081";
  return {
    plugins: [react()],
    envDir: repoRoot,
    server: {
      host: "127.0.0.1",
      port: 5173,
      proxy: {
        "/api": { target: apiTarget, changeOrigin: true },
        "/media": { target: apiTarget, changeOrigin: true },
      },
    },
    test: { environment: "jsdom" },
  };
});
