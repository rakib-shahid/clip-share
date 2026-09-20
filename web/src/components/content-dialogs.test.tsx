import { useState } from "react"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import { TrashDialog } from "@/App"
import { VideoPreviewDialog } from "./clip-preview"
import { SearchDialog } from "./search-dialog"
import { UserManagementDialog } from "./user-management-dialog"

const session = {
  user: { id: 1, username: "Admin", role: "admin" as const, state: "active" as const, storedFileLimitBytes: 50_000_000, rootFolderId: 1, libraryTrashed: false },
  csrfToken: "csrf",
  publicBaseURL: "http://example.test",
}

afterEach(() => vi.unstubAllGlobals())

describe("content Dialog migrations", () => {
  it("focuses video controls and returns focus to the preview opener", async () => {
    const user = userEvent.setup()
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Preview clip</button>{open && <VideoPreviewDialog title="Demo clip" posterSrc="/poster" videoSrc="/video" onClose={() => setOpen(false)} />}</>
    }
    const { container } = render(<Harness />)
    const opener = screen.getByRole("button", { name: "Preview clip" })
    await user.click(opener)
    expect(screen.getByRole("dialog", { name: "Demo clip" })).toHaveClass("max-w-4xl")
    await waitFor(() => expect(container.ownerDocument.querySelector("video")).toHaveFocus())
    await user.keyboard("{Escape}")
    await waitFor(() => expect(opener).toHaveFocus())
  })

  it("preserves Search requests and closes only the topmost nested preview", async () => {
    const user = userEvent.setup()
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ results: [{ kind: "clip", id: 9, name: "Found clip", ownerUserId: 1, ownerUsername: "Admin", folderId: 1, path: "/Found clip", publicId: "public-9", state: "ready" }], truncated: false }), { status: 200, headers: { "Content-Type": "application/json" } }))
    vi.stubGlobal("fetch", fetchMock)
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Open search</button>{open && <SearchDialog session={session} onClose={() => setOpen(false)} onNavigate={() => undefined} />}</>
    }
    render(<Harness />)
    const opener = screen.getByRole("button", { name: "Open search" })
    await user.click(opener)
    const input = screen.getByRole("textbox", { name: "Search folder names and clip titles" })
    await waitFor(() => expect(input).toHaveFocus())
    await user.type(input, "  Found  ")
    await user.click(screen.getByRole("button", { name: "Search" }))
    await screen.findByText("Found clip")
    expect(fetchMock).toHaveBeenCalledWith("/api/search?q=Found", expect.any(Object))

    const previewOpener = screen.getByRole("button", { name: "Preview Found clip" })
    await user.click(previewOpener)
    expect(screen.getAllByRole("dialog", { hidden: true })).toHaveLength(2)
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.getByRole("dialog", { name: "Search libraries" })).toBeVisible())
    expect(previewOpener).toHaveFocus()
    await user.keyboard("{Escape}")
    await waitFor(() => expect(opener).toHaveFocus())
  })

  it("keeps recycle-bin loading inside a bounded Dialog and restores focus", async () => {
    const user = userEvent.setup()
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200, headers: { "Content-Type": "application/json" } })))
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Open recycle bin</button>{open && <TrashDialog session={session} onClose={() => setOpen(false)} onLibraryChanged={() => undefined} />}</>
    }
    render(<Harness />)
    const opener = screen.getByRole("button", { name: "Open recycle bin" })
    await user.click(opener)
    const dialog = screen.getByRole("dialog", { name: "Recycle bin" })
    expect(dialog).toHaveClass("max-h-[90dvh]", "overflow-hidden")
    await screen.findByText("Recycle bin is empty.")
    await user.keyboard("{Escape}")
    await waitFor(() => expect(opener).toHaveFocus())
  })

  it("loads Manage users in a bounded Dialog and restores focus", async () => {
    const user = userEvent.setup()
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ users: [{ ...session.user, storedBytes: 0 }] }), { status: 200, headers: { "Content-Type": "application/json" } })))
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Open user manager</button>{open && <UserManagementDialog session={session} onClose={() => setOpen(false)} onUserChanged={() => undefined} onOpenLibrary={() => undefined} />}</>
    }
    render(<Harness />)
    const opener = screen.getByRole("button", { name: "Open user manager" })
    await user.click(opener)
    expect(screen.getByRole("dialog", { name: "Manage users" })).toHaveClass("max-h-[90dvh]", "overflow-hidden")
    await screen.findByText(/Super-admin/)
    await user.keyboard("{Escape}")
    await waitFor(() => expect(opener).toHaveFocus())
  })
})
