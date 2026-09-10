import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { UserManagementDialog } from "./user-management-dialog"

const session = {
  user: { id: 1, username: "Admin", role: "admin" as const, state: "active" as const, storedFileLimitBytes: 50_000_000, rootFolderId: 1, libraryTrashed: false },
  csrfToken: "csrf",
  publicBaseURL: "http://example.test"
}

afterEach(() => vi.unstubAllGlobals())

describe("UserManagementDialog", () => {
  it("loads and displays a fresh storage snapshot", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ users: [{ ...session.user, storedBytes: 0 }, { id: 2, username: "User1", role: "user", state: "active", storedFileLimitBytes: 50_000_000, rootFolderId: 2, libraryTrashed: false, storedBytes: 1_380_000_000 }] }), { status: 200, headers: { "Content-Type": "application/json" } })))
    render(<UserManagementDialog session={session} onClose={() => undefined} onUserChanged={() => undefined} onOpenLibrary={() => undefined} />)
    expect(screen.getByRole("status")).toHaveTextContent("Loading user storage")
    await waitFor(() => expect(screen.getByText(/1\.38 GB used/)).toBeInTheDocument())
    expect(screen.getByText(/50 MB per stored clip · User · 1\.38 GB used/)).toBeInTheDocument()
    expect(screen.getByText(/Super-admin · 0 B used/)).toBeInTheDocument()
  })

  it("offers retry after a failed snapshot request", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { message: "Unavailable" } }), { status: 500, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ users: [{ ...session.user, storedBytes: 0 }] }), { status: 200, headers: { "Content-Type": "application/json" } }))
    vi.stubGlobal("fetch", fetchMock)
    render(<UserManagementDialog session={session} onClose={() => undefined} onUserChanged={() => undefined} onOpenLibrary={() => undefined} />)
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Unavailable"))
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    await waitFor(() => expect(screen.getByText(/0 B used/)).toBeInTheDocument())
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
