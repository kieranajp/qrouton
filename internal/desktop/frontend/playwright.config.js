import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  outputDir: "./node_modules/.cache/playwright-results",
  use: { baseURL: "http://127.0.0.1:4178" },
  webServer: {
    command: "npx vite --config playwright.vite.config.js --host 127.0.0.1 --port 4178",
    url: "http://127.0.0.1:4178/tests/viewport.html",
    reuseExistingServer: !process.env.CI,
  },
});
