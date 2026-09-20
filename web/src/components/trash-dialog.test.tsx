import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Session } from "@/api"
import { TrashDialog } from "@/App"

const mocks = vi.hoisted(() => ({ request: vi.fn(), success: vi.fn(), error: vi.fn() }))
vi.mock("@/api", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/api")>()), request: mocks.request }))
vi.mock("sonner", () => ({ toast: { success: mocks.success, error: mocks.error }, Toaster: () => null }))

const session = (role: "user" | "admin" = "user"): Session => ({ user: { id: 1, username: "alice", role, state: "active", storedFileLimitBytes: 50_000_000, rootFolderId: 1, libraryTrashed: false }, csrfToken: "csrf", publicBaseURL: "http://example.test" })
const items = [
  { id: 3, kind: "clip", name: "Newest clip", ownerUserId: 1, ownerUsername: "alice", deletedAt: "2026-09-12T12:00:00Z", publicId: "public" },
  { id: 2, kind: "folder", name: "Older folder", ownerUserId: 2, ownerUsername: "bob", deletedAt: "2026-09-11T12:00:00Z", publicId: null },
]

describe("TrashDialog", () => {
  beforeEach(() => { mocks.request.mockReset(); mocks.success.mockReset(); mocks.error.mockReset(); localStorage.clear() })

  it("preserves API order and exposes matching pointer and context actions", async () => {
    mocks.request.mockResolvedValueOnce({ items })
    const user = userEvent.setup()
    render(<TrashDialog session={session()} onClose={() => undefined} onLibraryChanged={() => undefined} />)
    expect((await screen.findAllByTitle(/Newest clip|Older folder/)).map((node) => node.textContent)).toEqual(["Newest clip", "Older folder"])
    await user.hover(screen.getAllByText(/Deleted/)[0])
    expect(await screen.findByRole("tooltip")).toHaveTextContent(/expires/i)
    await user.click(screen.getByRole("button", { name: "Actions for Newest clip" }))
    expect(screen.getAllByRole("menuitem").map((node) => node.textContent)).toEqual(["Preview", "Restore"])
    await user.keyboard("{Escape}")
    await user.pointer({ keys: "[MouseRight]", target: screen.getByTitle("Newest clip") })
    expect(screen.getAllByRole("menuitem").map((node) => node.textContent)).toEqual(["Preview", "Restore"])
    expect(screen.queryByRole("menuitem", { name: "Delete permanently" })).not.toBeInTheDocument()
  })

  it("blocks duplicate restore and keeps a failure beside its item", async () => {
    let rejectRestore!: (reason: Error) => void
    mocks.request.mockResolvedValueOnce({ items }).mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectRestore = reject }))
    const user = userEvent.setup()
    render(<TrashDialog session={session()} onClose={() => undefined} onLibraryChanged={() => undefined} />)
    await screen.findByTitle("Newest clip")
    await user.click(screen.getByRole("button", { name: "Actions for Newest clip" }))
    await user.click(screen.getByRole("menuitem", { name: "Restore" }))
    await user.click(screen.getByRole("button", { name: "Actions for Newest clip" }))
    expect(screen.getByRole("menuitem", { name: "Restore" })).toHaveAttribute("data-disabled")
    await user.keyboard("{Escape}")
    rejectRestore(new Error("Restore conflict"))
    expect(await screen.findByRole("alert")).toHaveTextContent("Restore conflict")
    expect(screen.getByTitle("Newest clip").closest('[data-slot="item"]')).toHaveTextContent("Restore conflict")
    expect(mocks.request).toHaveBeenCalledTimes(2)
  })

  it("requires acknowledgement for administrator purge and reports success", async () => {
    mocks.request.mockResolvedValueOnce({ items }).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<TrashDialog session={session("admin")} onClose={() => undefined} onLibraryChanged={() => undefined} />)
    await screen.findByTitle("Older folder")
    await user.click(screen.getByRole("button", { name: "Actions for Older folder" }))
    await user.click(screen.getByRole("menuitem", { name: "Delete permanently" }))
    const confirm = screen.getByRole("button", { name: "Delete permanently" })
    expect(confirm).toBeDisabled()
    await user.click(screen.getByRole("checkbox"))
    await user.click(confirm)
    await waitFor(() => expect(mocks.success).toHaveBeenCalledWith("Permanently deleted Older folder"))
    expect(screen.queryByTitle("Older folder")).not.toBeInTheDocument()
  })
})
