import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const controlApiTarget = env.KIWIGUARD_CONSOLE_API_TARGET || "http://localhost:8081";

  return {
    plugins: [react()],
    resolve: {
      alias: {
        app: fileURLToPath(new URL("./src/app", import.meta.url)),
        console: fileURLToPath(new URL("./src/console", import.meta.url)),
        features: fileURLToPath(new URL("./src/features", import.meta.url)),
        platform: fileURLToPath(new URL("./src/platform", import.meta.url)),
        shared: fileURLToPath(new URL("./src/shared", import.meta.url))
      }
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes("node_modules/@carbon")) return "carbon";
            if (id.includes("node_modules/@tanstack")) return "query";
            if (id.includes("node_modules/lucide-react")) return "icons";
            if (id.includes("node_modules/react") || id.includes("node_modules/react-dom")) return "react";
          }
        }
      }
    },
    server: {
      port: 5173,
      proxy: {
        "/api": controlApiTarget
      }
    }
  };
});
