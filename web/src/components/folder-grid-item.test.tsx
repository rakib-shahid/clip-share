import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import type { Folder } from "@/api"
import { explorerGridClasses } from "@/lib/explorer-layout"
import { folderExplorerItem } from "@/lib/explorer-item"
import { FolderGridItem } from "./folder-grid-item"

function item(folderCount = 0, clipCount = 0, name = "Projects", role: "admin" | "user" = "user") {
  const folder: Folder = { id: 4, ownerUserId: 9, ownerUsername: "alice", parentFolderId: 1, name, isRoot: false, folderCount, clipCount }
  return folderExplorerItem(folder, role)
}

describe("FolderGridItem", () => {
  it("activates its one primary action once by pointer, Enter, and Space", async () => {
    const user = userEvent.setup()
    const open = vi.fn()
    render(<FolderGridItem item={item()} onOpen={open} />)
    const primary = screen.getByRole("button", { name: "Open folder Projects" })
    await user.click(primary)
    primary.focus()
    await user.keyboard("{Enter}")
    await user.keyboard(" ")
    expect(open).toHaveBeenCalledTimes(3)
    expect(primary).toHaveClass("focus-visible:ring-2")
  })

  it("isolates the separate action slot without nested buttons", async () => {
    const user = userEvent.setup()
    const open = vi.fn()
    const overflow = vi.fn()
    const { container } = render(<FolderGridItem item={item()} onOpen={open} overflow={<button type="button" onClick={overflow}>More actions</button>} />)
    await user.click(screen.getByRole("button", { name: "More actions" }))
    expect(overflow).toHaveBeenCalledTimes(1)
    expect(open).not.toHaveBeenCalled()
    expect(container.querySelector("button button")).toBeNull()
  })

  it.each([[0, 0, "0 folders · 0 clips"], [1, 1, "1 folder · 1 clip"], [2, 3, "2 folders · 3 clips"]])("renders direct counts with grammar", (folders, clips, label) => {
    const { container } = render(<FolderGridItem item={item(folders as number, clips as number)} onOpen={() => undefined} />)
    expect(container.querySelector('[data-slot="grid-secondary"]')).toHaveTextContent(label as string)
  })

  it("exposes aligned desktop cells and a CSS-only mobile summary", () => {
    const { container } = render(<FolderGridItem item={item(2, 4)} onOpen={() => undefined} />)
    expect(container.querySelector('[data-slot="list-state"]')).toHaveTextContent("Folder")
    expect(container.querySelector('[data-slot="list-size"]')).toHaveTextContent("2 folders · 4 clips")
    expect(container.querySelector('[data-slot="list-date"]')).toHaveAttribute("aria-label", "Not applicable")
    expect(container.querySelector('[data-slot="mobile-secondary"]')).toHaveClass("hidden")
  })

  it("keeps a long Unicode name complete for accessibility while truncating its title", () => {
    const name = "旅行の映像 — a very long folder title intended to overflow the visible card"
    render(<FolderGridItem item={item(1, 2, name, "admin")} onOpen={() => undefined} />)
    expect(screen.getByRole("button", { name: `Open folder ${name}` })).toBeInTheDocument()
    expect(screen.getByTitle(name)).toHaveClass("truncate")
  })

  it("keeps the icon and left-aligned text together at the top", () => {
    const { container } = render(<FolderGridItem item={item()} onOpen={() => undefined} />)
    const primary = screen.getByRole("button", { name: "Open folder Projects" })
    expect(primary).toHaveClass("items-start", "text-left")
    expect(container.querySelector('[data-slot="item-content"]')).toHaveClass("pt-0.5", "text-left")
    expect(container.querySelector('[data-slot="item-content"]')).not.toHaveClass("self-center")
  })

  it("locks the one/two/three/four-column max-width grid contract", () => {
    expect(explorerGridClasses).toContain("max-w-7xl")
    expect(explorerGridClasses).toContain("grid-cols-1")
    expect(explorerGridClasses).toContain("sm:grid-cols-2")
    expect(explorerGridClasses).toContain("lg:grid-cols-3")
    expect(explorerGridClasses).toContain("xl:grid-cols-4")
    expect(explorerGridClasses).not.toMatch(/md:grid-cols|2xl:grid-cols/)
  })

  it("does not invent folder size, date, selection, or drag affordances", () => {
    render(<FolderGridItem item={item(2, 4)} onOpen={() => undefined} />)
    expect(screen.queryByText(/bytes|uploaded/i)).not.toBeInTheDocument()
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument()
  })
})
