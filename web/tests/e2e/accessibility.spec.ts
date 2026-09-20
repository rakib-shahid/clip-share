import { expect, test } from "@playwright/test"
import AxeBuilder from "@axe-core/playwright"

test("public shell has no critical accessibility violations @cross-browser", async ({ page }) => {
  await page.goto("/")
  await expect(page).toHaveTitle(/Clip Share/i)
  const results = await new AxeBuilder({ page }).analyze()
  expect(results.violations.filter((violation) => violation.impact === "critical")).toEqual([])
})

test("public shell remains usable at 320px @touch", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 720 })
  await page.goto("/")
  await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll")
  await expect(page).toHaveTitle(/Clip Share/i)
})

test("reduced motion keeps the shell available @cross-browser", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" })
  await page.goto("/")
  await expect(page).toHaveTitle(/Clip Share/i)
})
