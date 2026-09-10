import { expect, test } from "@playwright/test"
import AxeBuilder from "@axe-core/playwright"

test("public shell has no critical accessibility violations", async ({ page }) => {
  await page.goto("/")
  await expect(page).toHaveTitle(/Clip Share/i)
  const results = await new AxeBuilder({ page }).analyze()
  expect(results.violations.filter((violation) => violation.impact === "critical")).toEqual([])
})
