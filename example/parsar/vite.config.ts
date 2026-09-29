import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { homedir } from "node:os";
import { isAbsolute, join } from "node:path";

const home = process.env.OAC_DEV_HOME || join(homedir(), ".oac");
if (!isAbsolute(home)) throw new Error("OAC_DEV_HOME must be absolute.");
export default defineConfig({
  plugins: [react(), tailwindcss()],
  cacheDir: join(home, "cache", "parsar-example"),
  build: { outDir: join(home, "build", "parsar-example"), emptyOutDir: true },
  test: { include: ["src/**/*.test.ts"] },
});
