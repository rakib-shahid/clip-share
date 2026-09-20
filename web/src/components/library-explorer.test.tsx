import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Folder, FolderPage, Session } from "@/api"
import { AppMotionProvider } from "./motion-provider"
import { LibraryExplorer } from "./library-explorer"
import { TooltipProvider } from "./ui/tooltip"

const requestMock = vi.hoisted(() => vi.fn())
vi.mock("@/api", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/api")>()), request: requestMock }))

const root: Folder = { id: 1, ownerUserId: 1, ownerUsername: "alice", parentFolderId: null, name: "Library", isRoot: true, folderCount: 2, clipCount: 0 }
const projects: Folder = { ...root, id: 2, parentFolderId: 1, name: "Projects", isRoot: false, folderCount: 0 }
const archive: Folder = { ...projects, id: 3, name: "Archive" }
const folderPage = (folder: Folder, folders: Folder[] = []): FolderPage => ({ folder, breadcrumbs: folder.id === 1 ? [root] : [root, folder], folders, clips: [], nextCursor: null, totalFolderCount: folders.length, totalClipCount: 0, totalItemCount: folders.length })
const session: Session = { user: { id: 1, username: "alice", role: "user", state: "active", storedFileLimitBytes: 50_000_000, rootFolderId: 1, libraryTrashed: false }, csrfToken: "csrf", publicBaseURL: "http://example.test" }

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function show(requestedFolder: { folderID: number; token: number } | null = null) {
  return render(<AppMotionProvider><TooltipProvider><LibraryExplorer session={session} root={session.user} users={[session.user]} requestedFolder={requestedFolder} /></TooltipProvider></AppMotionProvider>)
}

describe("LibraryExplorer request coordination", () => {
  beforeEach(() => {
    requestMock.mockReset(); localStorage.clear(); window.history.replaceState({}, "", "/")
    vi.stubGlobal("URL", { ...URL, createObjectURL: vi.fn(() => "blob:preview"), revokeObjectURL: vi.fn() })
  })

  it("keeps committed contents inert during navigation and restores them after failure", async () => {
    const failure = deferred<FolderPage>()
    requestMock.mockResolvedValueOnce(folderPage(root, [projects, archive])).mockImplementationOnce(() => failure.promise)
    const user = userEvent.setup(); show()
    await user.click(await screen.findByRole("button", { name: "Open folder Projects" }))
    expect(screen.getByRole("button", { name: "Open folder Archive" }).closest("[inert]")).toBeInTheDocument()
    await act(async () => failure.reject(new Error("Folder unavailable")))
    expect(await screen.findByRole("alert")).toHaveTextContent("Folder unavailable")
    expect(screen.getByRole("button", { name: "Open folder Projects" }).closest("[inert]")).toBeNull()
    expect(window.location.pathname).toContain("root")
  })

  it("ignores a superseded response even when it resolves last", async () => {
    const slow = deferred<FolderPage>(); const fast = deferred<FolderPage>()
    requestMock.mockResolvedValueOnce(folderPage(root, [projects, archive])).mockImplementationOnce(() => slow.promise).mockImplementationOnce(() => fast.promise)
    const view = show()
    await screen.findByRole("button", { name: "Open folder Projects" })
    await userEvent.click(screen.getByRole("button", { name: "Open folder Projects" }))
    view.rerender(<AppMotionProvider><TooltipProvider><LibraryExplorer session={session} root={session.user} users={[session.user]} requestedFolder={{ folderID: 3, token: 1 }} /></TooltipProvider></AppMotionProvider>)
    await act(async () => fast.resolve(folderPage(archive)))
    expect(await screen.findByRole("heading", { name: "Archive" })).toBeInTheDocument()
    await act(async () => slow.resolve(folderPage(projects)))
    await waitFor(() => expect(screen.getByRole("heading", { name: "Archive" })).toBeInTheDocument())
    expect(screen.queryByRole("heading", { name: "Projects" })).not.toBeInTheDocument()
  })

  it("opens the upload flow for a file dropped anywhere and defaults to the current folder", async () => {
    requestMock.mockImplementation((url: string) => Promise.resolve(url.startsWith("/api/folders/2") ? folderPage(projects) : folderPage(root, [projects])))
    show()
    await userEvent.click(await screen.findByRole("button", { name: "Open folder Projects" }))
    await screen.findByRole("heading", { name: "Projects" })

    fireEvent.dragEnter(window, { dataTransfer: { types: ["text/plain"], files: [] } })
    expect(screen.queryByText("Drop video to upload")).not.toBeInTheDocument()

    const file = new File(["video"], "from-desktop.mp4", { type: "video/mp4" })
    const dataTransfer = { types: ["Files"], files: [file], dropEffect: "none" }
    fireEvent.dragEnter(window, { dataTransfer })
    expect(screen.getByText("Drop video to upload")).toBeInTheDocument()
    expect(screen.getByText(/added to Projects/)).toBeInTheDocument()
    fireEvent.dragOver(window, { dataTransfer })
    expect(dataTransfer.dropEffect).toBe("copy")
    fireEvent.drop(window, { dataTransfer })

    expect(await screen.findByRole("dialog", { name: "Upload a video" })).toBeInTheDocument()
    expect(screen.getByText("from-desktop.mp4")).toBeInTheDocument()
    expect(screen.getByDisplayValue("from-desktop")).toBeInTheDocument()
    expect(await screen.findByText("alice / Projects")).toBeInTheDocument()
    expect(screen.queryByText("Drop video to upload")).not.toBeInTheDocument()
  })
})
