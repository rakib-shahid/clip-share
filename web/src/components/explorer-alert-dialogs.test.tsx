import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { request } from "@/api"
import { ClipManagementV2, FolderDeleteDialog } from "./library-explorer"

vi.mock("@/api", async (original) => ({ ...(await original<typeof import("@/api")>()), request: vi.fn() }))
const requestMock = vi.mocked(request)
const session = { user: { id: 1, username: "User", role: "user" as const, state: "active" as const, storedFileLimitBytes: 50_000_000, rootFolderId: 1, libraryTrashed: false }, csrfToken: "csrf", publicBaseURL: "http://test" }
const folder = { id: 1, ownerUserId: 1, ownerUsername: "User", parentFolderId: null, name: "Folder", isRoot: true, folderCount: 0, clipCount: 0 }

beforeEach(() => requestMock.mockReset())

describe("explorer destructive Alert Dialogs", () => {
  it("moves a ready clip to trash exactly once", async () => {
    const user = userEvent.setup(); const changed = vi.fn(); requestMock.mockResolvedValue(undefined)
    render(<ClipManagementV2 clip={{ id: 7, title: "Clip", state: "ready", sizeBytes: 1, createdAt: "", jobId: null, progress: null, errorMessage: null, publicId: "p" }} currentFolder={folder} session={session} users={[session.user]} onChanged={changed} />)
    await user.click(screen.getByRole("button", { name: "Actions for Clip" }))
    await user.click(screen.getByRole("menuitem", { name: "Move to recycle bin" }))
    expect(screen.getByRole("alertdialog", { name: "Move clip to recycle bin" })).toBeVisible()
    await user.click(screen.getByRole("button", { name: "Move to recycle bin" }))
    await waitFor(() => expect(requestMock).toHaveBeenCalledOnce())
    expect(requestMock).toHaveBeenCalledWith("/api/clips/7", expect.objectContaining({ method: "DELETE" }))
    expect(changed).toHaveBeenCalledOnce()
  })

  it.each([["processing", "Cancel processing", "/api/jobs/12"], ["failed", "Dismiss failed upload", "/api/jobs/12/failure"]])("uses the distinct %s job route", async (state, opener, path) => {
    const user = userEvent.setup(); requestMock.mockResolvedValue(undefined)
    render(<ClipManagementV2 clip={{ id: 7, title: "Clip", state, sizeBytes: null, createdAt: "", jobId: 12, progress: null, errorMessage: null, publicId: "" }} currentFolder={folder} session={session} users={[session.user]} onChanged={() => undefined} />)
    await user.click(screen.getByRole("button", { name: opener }))
    const action = screen.getByRole("button", { name: state === "failed" ? "Dismiss" : "Cancel upload" })
    await user.dblClick(action)
    await waitFor(() => expect(requestMock).toHaveBeenCalledOnce())
    expect(requestMock).toHaveBeenCalledWith(path, expect.objectContaining({ method: "DELETE" }))
  })

  it("loads recursive totals and gates folder deletion on acknowledgement", async () => {
    const user = userEvent.setup(); requestMock.mockResolvedValueOnce({ folderCount: 2, clipCount: 3, totalItems: 5, storedBytes: 1000 }).mockResolvedValueOnce(undefined)
    const deleted = vi.fn(); render(<FolderDeleteDialog folder={folder} session={session} onClose={() => undefined} onDeleted={deleted} />)
    const action = await screen.findByRole("button", { name: "Move to recycle bin" })
    expect(action).toBeDisabled(); expect(screen.getByText("2")).toBeVisible(); expect(screen.getByText("3")).toBeVisible()
    await user.click(screen.getByRole("checkbox")); expect(action).toBeEnabled(); await user.click(action)
    await waitFor(() => expect(requestMock).toHaveBeenCalledTimes(2)); expect(deleted).toHaveBeenCalledOnce()
  })
})
