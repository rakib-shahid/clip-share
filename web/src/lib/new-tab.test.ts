import { afterEach, describe, expect, it, vi } from "vitest"
import { openInNewTab } from "./new-tab"

describe("openInNewTab", () => {
  afterEach(() => vi.restoreAllMocks())

  it("uses a new tab with both isolation features and clears opener", () => {
    const opened = { opener: window } as unknown as Window
    const spy = vi.spyOn(window, "open").mockReturnValue(opened)
    expect(openInNewTab("https://example.test/c/public")).toBe(opened)
    expect(spy).toHaveBeenCalledWith("https://example.test/c/public", "_blank", "noopener,noreferrer")
    expect(opened.opener).toBeNull()
  })

  it("safely accepts a popup-blocked null result", () => {
    vi.spyOn(window, "open").mockReturnValue(null)
    expect(openInNewTab("https://example.test/c/public")).toBeNull()
  })
})
