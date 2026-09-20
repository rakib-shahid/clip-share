import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { SearchResult, Session } from "@/api"
import { SearchDialog } from "./search-dialog"

const requestMock = vi.hoisted(() => vi.fn())
vi.mock("@/api", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/api")>()), request: requestMock }))

const session = (role: "user" | "admin" = "user"): Session => ({ user: { id: 7, username: "alice", role, state: "active", storedFileLimitBytes: 50_000_000, rootFolderId: 1, libraryTrashed: false }, csrfToken: "csrf", publicBaseURL: "http://example.test" })
const results: SearchResult[] = [
  { kind: "clip", id: 2, name: "Second", ownerUserId: 8, ownerUsername: "bob", folderId: 4, path: "Bob / Clips", publicId: "p", state: "ready" },
  { kind: "folder", id: 1, name: "First", ownerUserId: 7, ownerUsername: "alice", folderId: 1, path: "Alice / First" },
]

describe("SearchDialog", () => {
  beforeEach(() => { requestMock.mockReset(); localStorage.clear() })

  it("uses a labelled Field and validates blank searches without a request", async () => {
    const user = userEvent.setup()
    render(<SearchDialog session={session()} onClose={() => undefined} onNavigate={() => undefined} />)
    expect(screen.getByLabelText("Folder or clip name")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Search" }))
    expect(requestMock).not.toHaveBeenCalled()
    expect(screen.getByText("No matches")).toBeInTheDocument()
  })

  it("preserves API order, shows administrator owners, and navigates exactly once", async () => {
    requestMock.mockResolvedValue({ results, truncated: false })
    const user = userEvent.setup(); const navigate = vi.fn()
    render(<SearchDialog session={session("admin")} onClose={() => undefined} onNavigate={navigate} />)
    await user.type(screen.getByLabelText("Folder or clip name"), "match")
    await user.click(screen.getByRole("button", { name: "Search" }))
    const names = await screen.findAllByTitle(/Bob \/ Clips|Alice \/ First/)
    expect(names.map((node) => node.textContent)).toEqual(["Bob / Clips", "Alice / First"])
    expect(screen.getByText("Owner: bob")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Show in folder" }))
    expect(navigate).toHaveBeenCalledOnce()
    expect(navigate).toHaveBeenCalledWith(results[0])
    expect(screen.queryByRole("button", { name: /sort/i })).not.toBeInTheDocument()
  })

  it("persists its view independently and announces the 100-result cap", async () => {
    requestMock.mockResolvedValue({ results, truncated: true })
    const user = userEvent.setup()
    const first = render(<SearchDialog session={session()} onClose={() => undefined} onNavigate={() => undefined} />)
    await user.click(screen.getByRole("radio", { name: "List view" }))
    await user.type(screen.getByLabelText("Folder or clip name"), "match")
    await user.click(screen.getByRole("button", { name: "Search" }))
    expect(await screen.findByText(/first 100 matches/i)).toBeInTheDocument()
    first.unmount()
    render(<SearchDialog session={session()} onClose={() => undefined} onNavigate={() => undefined} />)
    await waitFor(() => expect(screen.getByRole("radio", { name: "List view" })).toHaveAttribute("data-state", "on"))
  })

  it("keeps explicit loading, request-error, and empty-result states", async () => {
    let reject!: (reason: Error) => void
    requestMock.mockImplementationOnce(() => new Promise((_resolve, rejectRequest) => { reject = rejectRequest }))
    const user = userEvent.setup()
    render(<SearchDialog session={session()} onClose={() => undefined} onNavigate={() => undefined} />)
    await user.type(screen.getByLabelText("Folder or clip name"), "missing")
    await user.click(screen.getByRole("button", { name: "Search" }))
    expect(screen.getByRole("status", { name: "Searching" })).toBeInTheDocument()
    reject(new Error("Search unavailable"))
    expect(await screen.findByRole("alert")).toHaveTextContent("Search unavailable")
    requestMock.mockResolvedValueOnce({ results: [], truncated: false })
    await user.click(screen.getByRole("button", { name: "Search" }))
    expect(await screen.findByText("No matches")).toBeInTheDocument()
  })
})
