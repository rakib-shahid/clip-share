import { defineConfig, devices } from "@playwright/test"

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  fullyParallel: true,
  reporter: process.env.CI ? "line" : "list",
  use: {
    baseURL: process.env.CLIP_SHARE_E2E_URL ?? "http://127.0.0.1:5173",
    trace: "retain-on-failure",
    screenshot: "only-on-failure"
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", grep: /@cross-browser/, use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", grep: /@cross-browser/, use: { ...devices["Desktop Safari"] } },
    { name: "touch", grep: /@touch/, use: { ...devices["Pixel 7"] } },
  ]
})
