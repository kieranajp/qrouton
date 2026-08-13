import { defineConfig } from "vite";

export default defineConfig({
  optimizeDeps: { entries: ["tests/viewport.html"] },
});
