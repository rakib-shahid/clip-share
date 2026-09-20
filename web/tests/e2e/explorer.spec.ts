import { expect, test, type Page } from "@playwright/test"
import AxeBuilder from "@axe-core/playwright"
import { uiAnimationDurationMS } from "../../src/lib/motion"

const alice = { id: 1, username: "alice", role: "user", state: "active", storedFileLimitBytes: 500_000_000, rootFolderId: 1, libraryTrashed: false }
const admin = { id: 9, username: "admin", role: "admin", state: "active", storedFileLimitBytes: 500_000_000, rootFolderId: 9, libraryTrashed: false }
const folder = (id: number, name: string, parentFolderId: number | null = 1) => ({ id, ownerUserId: 1, ownerUsername: "alice", parentFolderId, name, isRoot: id === 1, folderCount: 0, clipCount: 0 })
const clip = (id: number) => ({ id, title: `Clip ${String(id).padStart(3, "0")}`, state: "ready", sizeBytes: id * 1_000, createdAt: new Date(Date.UTC(2026, 8, 13, 12, id % 60)).toISOString(), jobId: null, progress: null, errorMessage: null, publicId: `public-${id}` })

async function mockAuthenticated(page: Page, role: "user" | "admin" = "user") {
  const actor = role === "admin" ? admin : alice
  await page.route("**/api/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (url.pathname === "/api/setup/status") return route.fulfill({ json: { setupRequired: false } })
    if (url.pathname === "/api/auth/me") return route.fulfill({ json: { user: actor, csrfToken: "csrf", publicBaseURL: "http://example.test" } })
    if (url.pathname === "/api/users") return route.fulfill({ json: { users: [admin, alice] } })
    if (url.pathname === "/api/trash") return route.fulfill({ json: { items: [] } })
    if (url.pathname.startsWith("/api/folders/")) {
      const id = Number(url.pathname.split("/").at(-1))
      const cursor = url.searchParams.get("cursor")
      const root = folder(1, "Library", null)
      if (id !== 1) return route.fulfill({ json: { folder: folder(id, id === 2 ? "Projects" : "Archive"), breadcrumbs: [root, folder(id, id === 2 ? "Projects" : "Archive")], folders: [], clips: [], nextCursor: null, totalFolderCount: 0, totalClipCount: 0, totalItemCount: 0 } })
      const clips = cursor ? [clip(61)] : Array.from({ length: 60 }, (_, index) => clip(index + 1))
      return route.fulfill({ json: { folder: root, breadcrumbs: [root], folders: cursor ? [] : [folder(2, "Projects"), folder(3, "Archive")], clips, nextCursor: cursor ? null : "page-two", totalFolderCount: 2, totalClipCount: 61, totalItemCount: 63 } })
    }
    if (url.pathname === "/api/jobs/statuses") return route.fulfill({ json: { jobs: [] } })
    return route.fulfill({ status: request.method() === "GET" ? 404 : 204, json: request.method() === "GET" ? { error: { message: "Not mocked" } } : undefined })
  })
}

test("authenticated explorer supports pagination, list layout, and both action menus @cross-browser", async ({ page, browserName }) => {
  test.slow()
  await mockAuthenticated(page)
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Your library" })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText("2 folders · 61 clips")).toBeVisible()
  await page.getByRole("radio", { name: "List view" }).click()
  await expect(page.getByText("Status / type")).toBeVisible()
  await page.getByRole("button", { name: "Load more" }).click()
  await expect(page.getByTitle("Clip 061")).toBeVisible()
  await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0)

  const actions = page.getByRole("button", { name: "Actions for Clip 001" })
  await actions.click()
  await expect(page.getByRole("menuitem", { name: "Copy link" })).toBeVisible()
  await page.keyboard.press("Escape")
  await page.getByTitle("Clip 001").click({ button: "right" })
  await expect(page.getByRole("menuitem", { name: "Copy link" })).toBeVisible()
  await page.keyboard.press("Escape")
  await expect(page.getByRole("menuitem", { name: "Copy link" })).toHaveCount(0)
  if (browserName === "chromium") {
    await page.getByRole("button", { name: "Preview Clip 001" }).focus()
    await page.keyboard.press("Shift+F10")
    await expect(page.getByRole("menuitem", { name: "Rename" })).toBeVisible()
    await page.getByRole("menuitem", { name: "Rename" }).click()
    const title = page.getByLabel("Clip title")
    await title.fill("Continuously typed title")
    await expect(title).toBeFocused()
    await page.getByRole("button", { name: "Back" }).click()
    await actions.click()
    await page.getByRole("menuitem", { name: "Move to recycle bin" }).click()
    await expect(page.getByRole("alertdialog", { name: "Move clip to recycle bin" })).toBeVisible()
    await page.getByRole("button", { name: "Back" }).click()
  }

  const results = await new AxeBuilder({ page }).analyze()
  expect(results.violations.filter((violation) => violation.impact === "critical")).toEqual([])
})

test("compact list switches precisely below 640px without horizontal overflow", async ({ page }) => {
  await mockAuthenticated(page)
  await page.setViewportSize({ width: 640, height: 720 })
  await page.goto("/")
  await page.getByRole("radio", { name: "List view" }).click()
  await expect(page.getByText("Status / type")).toBeVisible()
  await page.setViewportSize({ width: 639, height: 720 })
  await expect(page.getByText("Status / type")).toBeHidden()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false)
})

test("fast refresh settles its transition without shifting the collection", async ({ page }) => {
  await mockAuthenticated(page)
  await page.goto("/")
  const firstItem = page.locator("[data-item-key]").first()
  await expect(firstItem).toBeVisible()
  await page.evaluate(() => {
    const item = document.querySelector<HTMLElement>("[data-item-key]")!
    const originalY = item.getBoundingClientRect().y
    const probe = { seen: false, shifted: false, startedAt: performance.now(), endedAt: null as number | null, observer: null as MutationObserver | null }
    probe.observer = new MutationObserver(() => {
      const updating = Array.from(document.querySelectorAll("[role=status]")).some((node) => node.textContent?.includes("Updating folder"))
      probe.shifted ||= Math.abs(item.getBoundingClientRect().y - originalY) > 0.5
      if (updating) probe.seen = true
      else if (probe.seen && probe.endedAt === null) { probe.endedAt = performance.now(); probe.observer?.disconnect() }
    })
    probe.observer.observe(document.body, { childList: true, subtree: true })
    ;(window as Window & { __refreshProbe?: typeof probe }).__refreshProbe = probe
  })
  await page.getByRole("button", { name: "Refresh" }).click()
  await page.waitForFunction(() => (window as Window & { __refreshProbe?: { endedAt: number | null } }).__refreshProbe?.endedAt !== null)
  const result = await page.evaluate(() => {
    const probe = (window as Window & { __refreshProbe?: { seen: boolean; shifted: boolean; startedAt: number; endedAt: number | null } }).__refreshProbe!
    return { seen: probe.seen, shifted: probe.shifted, duration: probe.endedAt! - probe.startedAt }
  })
  expect(result.seen).toBe(true)
  expect(result.duration).toBeGreaterThanOrEqual(uiAnimationDurationMS - 50)
  expect(result.shifted).toBe(false)
})

test("dropping a file anywhere opens upload in the active folder", async ({ page }) => {
  await mockAuthenticated(page)
  await page.goto("/")
  await page.getByRole("button", { name: "Open folder Projects" }).click()
  await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible()
  await page.evaluate(() => {
    const transfer = new DataTransfer()
    transfer.items.add(new File(["video"], "viewport-drop.mp4", { type: "video/mp4" }))
    ;(window as Window & { __clipShareDrop?: DataTransfer }).__clipShareDrop = transfer
    window.dispatchEvent(new DragEvent("dragenter", { dataTransfer: transfer, bubbles: true, cancelable: true }))
  })
  await expect(page.getByText("Drop video to upload")).toBeVisible()
  await expect(page.getByText(/added to Projects/)).toBeVisible()
  await page.evaluate(() => {
    const target = window as Window & { __clipShareDrop?: DataTransfer }
    window.dispatchEvent(new DragEvent("drop", { dataTransfer: target.__clipShareDrop, bubbles: true, cancelable: true }))
    delete target.__clipShareDrop
  })
  await expect(page.getByRole("dialog", { name: "Upload a video" })).toBeVisible()
  await expect(page.getByText("viewport-drop.mp4")).toBeVisible()
  await expect(page.getByLabel("Clip title")).toHaveValue("viewport-drop")
  await expect(page.getByText("alice / Projects")).toBeVisible()
})

test("an open upload dialog accepts a file dropped outside its drop box", async ({ page }) => {
  await mockAuthenticated(page)
  await page.goto("/")
  await page.getByRole("button", { name: "Upload clip" }).click()
  await expect(page.getByRole("dialog", { name: "Upload a video" })).toBeVisible()
  await page.evaluate(() => {
    const transfer = new DataTransfer()
    transfer.items.add(new File(["video"], "dialog-viewport-drop.webm", { type: "video/webm" }))
    const target = window as Window & { __clipShareDrop?: DataTransfer }
    target.__clipShareDrop = transfer
    document.body.dispatchEvent(new DragEvent("dragenter", { dataTransfer: transfer, bubbles: true, cancelable: true }))
  })
  await expect(page.getByText("Drop video to upload")).toBeVisible()
  await page.evaluate(() => {
    const target = window as Window & { __clipShareDrop?: DataTransfer }
    document.body.dispatchEvent(new DragEvent("drop", { dataTransfer: target.__clipShareDrop, bubbles: true, cancelable: true }))
    delete target.__clipShareDrop
  })
  await expect(page.getByText("dialog-viewport-drop.webm")).toBeVisible()
  await expect(page.getByLabel("Clip title")).toHaveValue("dialog-viewport-drop")
})

test("all server sort choices request page one without remounting for view changes", async ({ page }) => {
  await mockAuthenticated(page)
  const folderRequests: string[] = []
  page.on("request", (request) => { if (new URL(request.url()).pathname === "/api/folders/1") folderRequests.push(request.url()) })
  await page.goto("/")
  const labels = ["Oldest upload", "Name A–Z", "Name Z–A", "Largest size", "Smallest size", "Processing state", "Latest upload"]
  for (const label of labels) {
    await page.getByRole("button", { name: /^Sort:/ }).click()
    await page.getByRole("menuitemradio", { name: label }).click()
    await expect(page.getByRole("button", { name: `Sort: ${label}` })).toBeVisible()
    await page.getByRole("button", { name: "Load more" }).click()
    await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0)
  }
  expect(folderRequests.some((url) => url.includes("sort=oldest"))).toBeTruthy()
  expect(folderRequests.some((url) => url.includes("sort=size_desc"))).toBeTruthy()
  const beforeView = folderRequests.length
  await page.getByRole("radio", { name: "List view" }).click()
  expect(folderRequests).toHaveLength(beforeView)
})

test("320px touch explorer has no page overflow and uses Attachment plus Drawer @touch", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 720 })
  await mockAuthenticated(page)
  await page.goto("/")
  await page.getByRole("radio", { name: "List view" }).click()
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  expect(overflow).toBe(false)
  await page.getByRole("button", { name: "Upload clip" }).click()
  await page.locator('input[type="file"]').setInputFiles({ name: "a-very-long-private-video-filename-for-layout-testing.mp4", mimeType: "video/mp4", buffer: Buffer.from("video") })
  await expect(page.getByText("a-very-long-private-video-filename-for-layout-testing.mp4")).toBeVisible()
  await page.getByRole("button", { name: /Change/ }).click()
  await expect(page.getByText("Choose upload destination")).toBeVisible()
  await expect(page.locator('[data-vaul-drawer]')).toBeVisible()
})

test("administrator can enter an acting library with admin-only folder Copy @cross-browser", async ({ page }) => {
  await mockAuthenticated(page, "admin")
  await page.goto("/")
  await page.getByRole("button", { name: /alice/i }).click()
  await expect(page.getByText("Administrator view · alice's library")).toBeVisible()
  await page.getByRole("button", { name: "Actions for Projects" }).click()
  await expect(page.getByRole("menuitem", { name: "Copy" })).toBeVisible()
})

test("reduced motion removes explorer spatial transitions @cross-browser", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" })
  await mockAuthenticated(page)
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Your library" })).toBeVisible({ timeout: 15_000 })
  const durationMs = await page.evaluate(() => {
    const duration = getComputedStyle(document.querySelector('[data-item-key]')!).animationDuration
    return Number.parseFloat(duration) * (duration.endsWith("ms") ? 1 : 1_000)
  })
  expect(durationMs).toBeLessThanOrEqual(0.01)
})
