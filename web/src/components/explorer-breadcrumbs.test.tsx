import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import type { Folder } from "@/api"
import { TooltipProvider } from "./ui/tooltip"
import { ExplorerBreadcrumbs } from "./explorer-breadcrumbs"

function crumb(id: number, name: string): Folder {
  return { id, name, ownerUserId: 1, ownerUsername: "owner", parentFolderId: id === 1 ? null : id - 1, isRoot: id === 1, folderCount: 0, clipCount: 0 }
}

function renderBreadcrumbs(crumbs: Folder[], options: { administrator?: boolean; onBack?: () => void } = {}) {
  const onNavigate = vi.fn()
  render(<TooltipProvider delayDuration={0}><ExplorerBreadcrumbs crumbs={crumbs} administrator={options.administrator ?? false} onNavigate={onNavigate} onBack={options.onBack} /></TooltipProvider>)
  return onNavigate
}

describe("ExplorerBreadcrumbs", () => {
  it("renders a semantic root path whose current page is inert", () => {
    const navigate = renderBreadcrumbs([crumb(1, "Root")])
    expect(screen.getByRole("navigation", { name: "Breadcrumb" })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Library" })).toHaveAttribute("aria-current", "page")
    expect(screen.queryByRole("button", { name: "Library" })).not.toBeInTheDocument()
    expect(navigate).not.toHaveBeenCalled()
  })

  it("shows every member of paths up to four crumbs", () => {
    renderBreadcrumbs([crumb(1, "Root"), crumb(2, "Alpha"), crumb(3, "Beta"), crumb(4, "Gamma")])
    expect(screen.queryByRole("button", { name: "Show omitted folders" })).not.toBeInTheDocument()
    for (const name of ["Library", "Alpha", "Beta", "Gamma"]) expect(screen.getByText(name)).toBeInTheDocument()
  })

  it("collapses long paths to root, omitted menu, parent, and current in exact order", async () => {
    const user = userEvent.setup()
    const navigate = renderBreadcrumbs([crumb(1, "Root"), crumb(2, "First"), crumb(3, "Duplicate"), crumb(4, "Duplicate"), crumb(5, "Parent"), crumb(6, "Current")])
    const navigation = screen.getByRole("navigation", { name: "Breadcrumb" })
    expect(navigation).toHaveTextContent(/Library.*Parent.*Current/)
    expect(navigation).not.toHaveTextContent("First")
    await user.click(screen.getByRole("button", { name: "Show omitted folders" }))
    const items = screen.getAllByRole("menuitem")
    expect(items.map((item) => item.textContent)).toEqual(["First", "Duplicate", "Duplicate"])
    await user.click(items[2])
    expect(navigate).toHaveBeenCalledWith(4)
    await user.click(screen.getByRole("button", { name: "Parent" }))
    expect(navigate).toHaveBeenLastCalledWith(5)
    expect(screen.queryByRole("button", { name: "Current" })).not.toBeInTheDocument()
  })

  it("supports keyboard menu access and keeps administrator back separate", async () => {
    const user = userEvent.setup()
    const back = vi.fn()
    const navigate = renderBreadcrumbs([crumb(1, "Root"), crumb(2, "A"), crumb(3, "B"), crumb(4, "C"), crumb(5, "D")], { administrator: true, onBack: back })
    await user.click(screen.getByRole("button", { name: "All libraries" }))
    expect(back).toHaveBeenCalledTimes(1)
    expect(navigate).not.toHaveBeenCalled()
    const overflow = screen.getByRole("button", { name: "Show omitted folders" })
    overflow.focus()
    await user.keyboard("{Enter}")
    expect(screen.getAllByRole("menuitem")[0]).toHaveFocus()
  })

  it("contains long names visually while retaining full accessible and tooltip text", async () => {
    const user = userEvent.setup()
    const longName = "A folder name that is intentionally far too long for a narrow phone viewport"
    renderBreadcrumbs([crumb(1, "Root"), crumb(2, longName)])
    const current = screen.getByRole("link", { name: longName })
    expect(current).toHaveClass("truncate")
    current.focus()
    expect(await screen.findByRole("tooltip")).toHaveTextContent(longName)
    await user.keyboard("{Escape}")
  })
})
