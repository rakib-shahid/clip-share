import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import type { ClipSummary } from "@/api"
import { clipExplorerItem } from "@/lib/explorer-item"
import { TooltipProvider } from "./ui/tooltip"
import { ClipGridItem } from "./clip-grid-item"
import { Copy } from "lucide-react"
import type { ItemAction } from "@/lib/item-actions"

const clip = (state: string, progress: number | null = null): ClipSummary => ({ id: 4, title: "Long clip title", state, sizeBytes: state === "ready" ? 1200 : null, createdAt: "2026-09-12T12:00:00Z", jobId: 9, progress, errorMessage: state === "failed" ? "Safe failure detail" : null, publicId: "public-id" })
const show = (state: string, progress: number | null = null, props: { onPreview?: () => void; overflow?: React.ReactNode; actions?: ItemAction[] } = {}) => render(<TooltipProvider><ClipGridItem item={clipExplorerItem(clip(state, progress))} {...props} /></TooltipProvider>)

describe("ClipGridItem", () => {
  it("renders a lazy poster and one ready preview action without video", async () => {
    const user = userEvent.setup(); const preview = vi.fn(); const copyLink = vi.fn()
    const actions: ItemAction[] = [{ id: "copy-link", label: "Copy link", group: 0, icon: Copy, run: copyLink }]
    const { container } = show("ready", null, { onPreview: preview, actions })
    const image = container.querySelector("img")!
    expect(container.querySelector('[data-slot="item"]')).toHaveClass("clip-card", "overflow-hidden", "rounded-2xl")
    expect(image).toHaveAttribute("src", "/m/public-id/poster")
    expect(image).toHaveAttribute("alt", "")
    expect(image).toHaveAttribute("loading", "lazy")
    expect(container.querySelector("video")).toBeNull()
    await user.click(screen.getByRole("button", { name: "Preview Long clip title" }))
    expect(preview).toHaveBeenCalledTimes(1)
    expect(screen.queryByText("Ready")).not.toBeInTheDocument()
    await user.click(screen.getAllByRole("button", { name: "Copy link for Long clip title" })[0])
    expect(copyLink).toHaveBeenCalledTimes(1)
    await user.click(screen.getByRole("button", { name: "Actions for Long clip title" }))
    expect(screen.getByRole("menuitem", { name: "Copy link" })).toBeInTheDocument()
    expect(screen.getAllByText("1 KB").length).toBeGreaterThan(0)
  })

  it.each(["queued", "validating", "processing"])("renders %s without preview eligibility", (state) => {
    const { container } = show(state)
    expect(screen.getAllByText(clipExplorerItem(clip(state)).status.label).length).toBeGreaterThan(0)
    expect(screen.queryByRole("button", { name: /Preview/ })).not.toBeInTheDocument()
    expect(container.querySelector("img,video")).toBeNull()
    expect(screen.getAllByRole("status").length).toBeGreaterThan(0)
  })

  it("clamps finite progress for rendering and ARIA", () => {
    const { container } = show("processing", 140)
    expect(container.querySelector('[data-slot="grid-details"] [role="progressbar"]')).toHaveAttribute("aria-valuenow", "100")
  })

  it("shows a non-color failure badge and safe bounded error text", () => {
    show("failed")
    expect(screen.getAllByText("Upload failed").length).toBeGreaterThan(0)
    expect(screen.getByText("Safe failure detail")).toHaveClass("line-clamp-2")
    expect(screen.queryByRole("button", { name: /Preview/ })).not.toBeInTheDocument()
  })

  it("exposes desktop list columns and one compact mobile summary", () => {
    const { container } = show("ready")
    expect(container.querySelector('[data-slot="list-state"]')).toBeInTheDocument()
    expect(container.querySelector('[data-slot="list-size"]')).toHaveTextContent("1 KB")
    expect(container.querySelector('[data-slot="list-date"] time')).toHaveAttribute("dateTime", clip("ready").createdAt)
    expect(container.querySelector('[data-slot="mobile-secondary"]')).toHaveClass("hidden")
  })

  it("isolates overflow actions from preview", async () => {
    const user = userEvent.setup(); const preview = vi.fn(); const action = vi.fn()
    const { container } = show("ready", null, { onPreview: preview, overflow: <button onClick={action}>Actions</button> })
    await user.click(screen.getByRole("button", { name: "Actions" }))
    expect(action).toHaveBeenCalledOnce(); expect(preview).not.toHaveBeenCalled()
    expect(container.querySelector("button button")).toBeNull()
  })
})
