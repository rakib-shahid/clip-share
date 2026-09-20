import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { Folder, FolderPage, Session } from "@/api"
import { UploadDialog } from "./upload-dialog"
import { AppMotionProvider } from "./motion-provider"

const api = vi.hoisted(() => ({ request: vi.fn(), uploadVideo: vi.fn() }))
vi.mock("@/api", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/api")>()), ...api }))

const folder: Folder = { id: 2, ownerUserId: 1, ownerUsername: "alice", parentFolderId: null, name: "Library", isRoot: true, folderCount: 0, clipCount: 0 }
const page: FolderPage = { folder, breadcrumbs: [folder], folders: [], clips: [], nextCursor: null, totalFolderCount: 0, totalClipCount: 0, totalItemCount: 0 }
const session: Session = { user: { id: 1, username: "alice", role: "user", state: "active", storedFileLimitBytes: 500_000_000, rootFolderId: 2, libraryTrashed: false }, csrfToken: "csrf", publicBaseURL: "http://example.test" }

function show(overrides: Partial<React.ComponentProps<typeof UploadDialog>> = {}) {
  const props = { session, currentFolder: folder, users: [session.user], onClose: vi.fn(), onQueued: vi.fn(), onUploaded: vi.fn(), ...overrides }
  const result = render(<AppMotionProvider><UploadDialog {...props} /></AppMotionProvider>)
  return { ...result, props }
}

describe("UploadDialog Attachment", () => {
  beforeEach(() => {
    api.request.mockReset().mockResolvedValue(page)
    api.uploadVideo.mockReset()
    vi.stubGlobal("URL", { ...URL, createObjectURL: vi.fn(() => "blob:preview"), revokeObjectURL: vi.fn() })
  })

  it("selects, replaces, removes, and drops files without exposing a source path", async () => {
    const user = userEvent.setup()
    const { unmount } = show()
    const input = document.querySelector('input[type="file"]') as HTMLInputElement
    const longName = `${"match-".repeat(30)}.mp4`
    await user.upload(input, new File(["one"], longName, { type: "video/mp4" }))
    expect(screen.getByTitle(longName)).toHaveClass("truncate")
    expect(screen.getByRole("button", { name: "Replace selected video" })).toBeEnabled()
    expect(document.body).not.toHaveTextContent("C:\\")
    await user.click(screen.getByRole("button", { name: "Remove selected video" }))
    expect(screen.getByRole("button", { name: /Choose or drop one video/i })).toBeInTheDocument()
    fireEvent.drop(screen.getByRole("button", { name: /Choose or drop one video/i }), { dataTransfer: { files: [new File(["two"], "dropped.webm", { type: "video/webm" })] } })
    expect(screen.getByText("dropped.webm")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Replace selected video" }))
    await user.upload(input, new File(["three"], "replacement.mov", { type: "video/quicktime" }))
    expect(screen.getByText("replacement.mov")).toBeInTheDocument()
    unmount()
    expect(URL.revokeObjectURL).toHaveBeenCalled()
  })

  it("selects and replaces a file dropped anywhere while the dialog is open", async () => {
    show()
    const first = new File(["one"], "anywhere-first.mp4", { type: "video/mp4" })
    const firstTransfer = { types: ["Files"], files: [first], dropEffect: "none" }
    fireEvent.dragEnter(window, { dataTransfer: firstTransfer })
    expect(screen.getByText("Drop video to upload")).toBeInTheDocument()
    fireEvent.drop(window, { dataTransfer: firstTransfer })
    expect(screen.getByText("anywhere-first.mp4")).toBeInTheDocument()
    expect(screen.getByDisplayValue("anywhere-first")).toBeInTheDocument()

    const replacement = new File(["two"], "anywhere-replacement.webm", { type: "video/webm" })
    const replacementTransfer = { types: ["Files"], files: [replacement], dropEffect: "none" }
    fireEvent.dragEnter(document.body, { dataTransfer: replacementTransfer })
    fireEvent.drop(document.body, { dataTransfer: replacementTransfer })
    expect(screen.getByText("anywhere-replacement.webm")).toBeInTheDocument()
    expect(screen.getByDisplayValue("anywhere-replacement")).toBeInTheDocument()
  })

  it("uses measured upload progress then indeterminate validation", async () => {
    let finish!: (value: object) => void
    api.uploadVideo.mockImplementation(async (_form, _csrf, onProgress: (value: number) => void) => {
      onProgress(45)
      await new Promise<object>((resolve) => { finish = resolve })
      return {}
    })
    const user = userEvent.setup()
    const onQueued = vi.fn()
    show({ onQueued })
    await user.upload(document.querySelector('input[type="file"]') as HTMLInputElement, new File(["video"], "clip.mp4", { type: "video/mp4" }))
    await user.click(screen.getByRole("button", { name: "Upload clip" }))
    expect(await screen.findByRole("progressbar", { name: "Upload progress" })).toHaveAttribute("aria-valuenow", "45")
    const progress = api.uploadVideo.mock.calls[0][2] as (value: number) => void
    progress(100)
    expect(await screen.findByText(/Validating media/)).toBeInTheDocument()
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument()
    finish({})
    await waitFor(() => expect(onQueued).toHaveBeenCalledOnce())
  })

  it("returns to an actionable error state and can retry", async () => {
    api.uploadVideo.mockRejectedValueOnce(new Error("Network interrupted")).mockResolvedValueOnce({})
    const user = userEvent.setup()
    const onQueued = vi.fn()
    show({ onQueued })
    await user.upload(document.querySelector('input[type="file"]') as HTMLInputElement, new File(["video"], "retry.mp4", { type: "video/mp4" }))
    await user.click(screen.getByRole("button", { name: "Upload clip" }))
    expect(await screen.findByRole("alert")).toHaveTextContent("Network interrupted")
    expect(screen.getByRole("button", { name: "Upload clip" })).toBeEnabled()
    await user.click(screen.getByRole("button", { name: "Upload clip" }))
    await waitFor(() => expect(onQueued).toHaveBeenCalledOnce())
  })

  it("confirms and aborts an in-flight upload", async () => {
    api.uploadVideo.mockImplementation((_form: FormData, _csrf: string, _progress: (value: number) => void, signal: AbortSignal) => new Promise((_resolve, reject) => {
      signal.addEventListener("abort", () => reject(new DOMException("cancelled", "AbortError")))
    }))
    const user = userEvent.setup(); const onClose = vi.fn()
    show({ onClose })
    await user.upload(document.querySelector('input[type="file"]') as HTMLInputElement, new File(["video"], "cancel.mp4", { type: "video/mp4" }))
    await user.click(screen.getByRole("button", { name: "Upload clip" }))
    await user.click(screen.getByRole("button", { name: "Cancel upload" }))
    expect(await screen.findByRole("alertdialog", { name: "Cancel this upload?" })).toBeInTheDocument()
    await user.click(screen.getAllByRole("button", { name: "Cancel upload" }).at(-1)!)
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
  })
})
