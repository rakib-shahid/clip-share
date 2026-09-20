import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { ExplorerItemSummary, ExplorerToolbar } from "./explorer-toolbar"

function renderToolbar(overrides: Partial<React.ComponentProps<typeof ExplorerToolbar>> = {}) {
  const props: React.ComponentProps<typeof ExplorerToolbar> = {
    view: "grid",
    sort: "latest",
    disabled: false,
    busy: false,
    onViewChange: vi.fn(),
    onSortChange: vi.fn(),
    onCreateFolder: vi.fn(),
    onUploadClip: vi.fn(),
    ...overrides,
  }
  render(<ExplorerToolbar {...props} />)
  return props
}

describe("ExplorerToolbar", () => {
  it("groups creation actions and invokes each exactly once", async () => {
    const user = userEvent.setup()
    const props = renderToolbar()
    expect(screen.getByRole("group", { name: "Create items" })).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /New folder/ }))
    await user.click(screen.getByRole("button", { name: /Upload clip/ }))
    expect(props.onCreateFolder).toHaveBeenCalledTimes(1)
    expect(props.onUploadClip).toHaveBeenCalledTimes(1)
    expect(screen.getByRole("button", { name: /Upload clip/ })).toHaveClass("border-l", "border-l-emerald-300/60")
    expect(screen.getByRole("button", { name: /Upload clip/ })).toHaveAttribute("data-group-left-border")
  })

  it("exposes a single selected view and changes it without invoking sort", async () => {
    const user = userEvent.setup()
    const props = renderToolbar()
    expect(screen.getByRole("radio", { name: "Grid view" })).toHaveAttribute("data-state", "on")
    await user.click(screen.getByRole("radio", { name: "List view" }))
    expect(props.onViewChange).toHaveBeenCalledWith("list")
    expect(props.onSortChange).not.toHaveBeenCalled()
  })

  it("uses an accessible seven-option sort radio menu and ignores the active option", async () => {
    const user = userEvent.setup()
    const props = renderToolbar()
    await user.click(screen.getByRole("button", { name: "Sort: Latest upload" }))
    expect(screen.getAllByRole("menuitemradio")).toHaveLength(7)
    expect(screen.getByRole("menuitemradio", { name: "Latest upload" })).toHaveAttribute("data-state", "checked")
    await user.click(screen.getByRole("menuitemradio", { name: "Latest upload" }))
    expect(props.onSortChange).not.toHaveBeenCalled()
    await user.click(screen.getByRole("button", { name: "Sort: Latest upload" }))
    await user.click(screen.getByRole("menuitemradio", { name: "Oldest upload" }))
    expect(props.onSortChange).toHaveBeenCalledWith("oldest")
  })

  it("preserves disabled creation and busy sort states and wraps controls", () => {
    renderToolbar({ disabled: true, busy: true })
    expect(screen.getByRole("button", { name: /New folder/ })).toBeDisabled()
    expect(screen.getByRole("button", { name: /Upload clip/ })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Sort: Latest upload" })).toBeDisabled()
    expect(screen.getByLabelText("Folder explorer controls")).toHaveClass("flex-wrap")
    expect(screen.getByLabelText("Folder explorer controls")).toHaveAttribute("aria-busy", "true")
  })

  it("announces authoritative zero and singular totals", () => {
    const { rerender } = render(<ExplorerItemSummary folderCount={0} clipCount={0} />)
    expect(screen.getByText("0 folders · 0 clips")).toBeInTheDocument()
    rerender(<ExplorerItemSummary folderCount={1} clipCount={1} />)
    expect(screen.getByText("1 folder · 1 clip")).toBeInTheDocument()
  })
})
