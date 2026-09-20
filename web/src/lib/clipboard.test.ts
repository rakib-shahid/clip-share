import { afterEach, describe, expect, it, vi } from "vitest"
import { copyText } from "./clipboard"

describe("copyText", () => {
  afterEach(() => vi.restoreAllMocks())

  it("awaits the Clipboard API and propagates failure", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("denied"))
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } })
    await expect(copyText("https://example.test/c/id")).rejects.toThrow("denied")
    expect(writeText).toHaveBeenCalledWith("https://example.test/c/id")
  })

  it("uses the HTTP fallback and removes its temporary input", async () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined })
    Object.defineProperty(document, "execCommand", { configurable: true, value: vi.fn(() => true) })
    await copyText("public link")
    expect(document.execCommand).toHaveBeenCalledWith("copy")
    expect(document.querySelector("textarea")).toBeNull()
  })
})
