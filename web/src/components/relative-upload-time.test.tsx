import { act, render, screen } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { TooltipProvider } from "./ui/tooltip"
import { RelativeUploadTime } from "./relative-upload-time"
import { formatRelativeUploadTime } from "../lib/relative-time"

describe("relative upload time", () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-09-12T18:30:00Z"))
  })

  afterEach(() => vi.useRealTimers())

  it("formats future skew and compact time/date boundaries", () => {
    const now = Date.now()
    expect(formatRelativeUploadTime(now + 10_000, now, "en-US")).toBe("Just now")
    expect(formatRelativeUploadTime(now - 30_000, now, "en-US")).toBe("Just now")
    expect(formatRelativeUploadTime(now - 5 * 60_000, now, "en-US")).toMatch(/5 min/)
    expect(formatRelativeUploadTime(now - 3 * 60 * 60_000, now, "en-US")).toMatch(/3 hr/)
    expect(formatRelativeUploadTime(Date.parse("2026-09-11T20:00:00Z"), now, "en-US")).toBe("Yesterday")
    expect(formatRelativeUploadTime(Date.parse("2025-01-02T12:00:00Z"), now, "en-US")).toMatch(/Jan 2, 2025/)
  })

  it("renders semantic exact time and an accessible tooltip", async () => {
    render(<TooltipProvider delayDuration={0}><RelativeUploadTime createdAt="2026-09-12T18:25:00Z" locale="en-US" /></TooltipProvider>)
    const time = screen.getByText(/ago|Just now/)
    expect(time.tagName).toBe("TIME")
    expect(time).toHaveAttribute("datetime", "2026-09-12T18:25:00Z")
    expect(time).toHaveAccessibleName(/Uploaded Sep 12, 2026/)
    act(() => { time.focus(); vi.runOnlyPendingTimers() })
    expect(screen.getByRole("tooltip")).toHaveTextContent(/Sep 12, 2026/)
  })

  it("renders invalid input without a tooltip", () => {
    render(<TooltipProvider><RelativeUploadTime createdAt="not-a-date" /></TooltipProvider>)
    expect(screen.getByText("Upload date unavailable").tagName).toBe("SPAN")
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument()
  })

  it("shares one timer, pauses while hidden, resumes, and cleans up", () => {
    let hidden = false
    vi.spyOn(document, "hidden", "get").mockImplementation(() => hidden)
    const interval = vi.spyOn(window, "setInterval")
    const clear = vi.spyOn(window, "clearInterval")
    const view = render(<TooltipProvider><RelativeUploadTime createdAt="2026-09-12T18:25:00Z" /><RelativeUploadTime createdAt="2026-09-12T17:25:00Z" /></TooltipProvider>)
    expect(interval).toHaveBeenCalledTimes(1)
    hidden = true
    act(() => document.dispatchEvent(new Event("visibilitychange")))
    expect(clear).toHaveBeenCalledTimes(1)
    hidden = false
    act(() => document.dispatchEvent(new Event("visibilitychange")))
    expect(interval).toHaveBeenCalledTimes(2)
    view.unmount()
    expect(clear).toHaveBeenCalledTimes(2)
  })
})
