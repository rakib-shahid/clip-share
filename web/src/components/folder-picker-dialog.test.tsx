import { act, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { request } from "@/api"
import { FolderPickerDialog } from "./folder-picker-dialog"

vi.mock("@/api", async (original) => ({ ...(await original<typeof import("@/api")>()), request: vi.fn() }))
const requestMock = vi.mocked(request)
const user = { id: 1, username: "User", role: "user" as const, state: "active" as const, storedFileLimitBytes: 1, rootFolderId: 1, libraryTrashed: false }
const session = { user, csrfToken: "csrf", publicBaseURL: "http://test" }
const root = { id: 1, ownerUserId: 1, ownerUsername: "User", parentFolderId: null, name: "Root", isRoot: true, folderCount: 2, clipCount: 0 }
const child = { ...root, id: 2, parentFolderId: 1, name: "Child", isRoot: false, folderCount: 0 }
const page = { folder: root, breadcrumbs: [root], folders: [child, { ...child, id: 3, name: "Excluded" }], clips: [], nextCursor: null }

function media(initial: boolean) {
  let listener: ((event: MediaQueryListEvent) => void) | undefined
  const value = { matches: initial, media: "", onchange: null, addEventListener: vi.fn((_type, next) => { listener = next }), removeEventListener: vi.fn(), dispatchEvent: vi.fn() }
  vi.stubGlobal("matchMedia", vi.fn(() => value)); return (matches: boolean) => act(() => listener?.({ matches } as MediaQueryListEvent))
}

beforeEach(() => { requestMock.mockReset(); vi.unstubAllGlobals() })

describe("FolderPickerDialog", () => {
  it("uses Dialog on desktop, filters exclusions, and chooses exactly once", async () => {
    media(false); requestMock.mockResolvedValue(page); let resolve!: () => void; const pending = new Promise<void>((done) => { resolve = done }); const choose = vi.fn(() => pending); const ui = userEvent.setup()
    render(<FolderPickerDialog title="Move folder" session={session} users={[user]} initialID={1} excludedFolderID={3} onClose={() => undefined} onChoose={choose} />)
    expect(await screen.findByRole("dialog", { name: "Move folder" })).toHaveClass("dialog-motion")
    expect(screen.getByRole("button", { name: "Child" })).toBeVisible(); expect(screen.queryByRole("button", { name: "Excluded" })).not.toBeInTheDocument()
    await ui.dblClick(screen.getByRole("button", { name: "Choose this folder" })); expect(choose).toHaveBeenCalledOnce(); expect(choose).toHaveBeenCalledWith(root); resolve(); await waitFor(() => expect(screen.getByRole("button", { name: "Choose this folder" })).toBeEnabled())
  })

  it("uses a bounded bottom Drawer on narrow screens", async () => {
    media(true); requestMock.mockResolvedValue(page)
    render(<FolderPickerDialog title="Choose destination" session={session} users={[user]} initialID={1} onClose={() => undefined} onChoose={() => undefined} />)
    expect(await screen.findByRole("dialog", { name: "Choose destination" })).toHaveClass("bottom-0", "max-h-[92dvh]")
  })

  it("preserves loaded folder state across a live shell change", async () => {
    const change = media(false); requestMock.mockResolvedValue(page)
    render(<FolderPickerDialog title="Choose destination" session={session} users={[user]} initialID={1} onClose={() => undefined} onChoose={() => undefined} />)
    await screen.findByRole("button", { name: "Child" }); change(true)
    expect(screen.getByRole("dialog", { name: "Choose destination" })).toHaveClass("bottom-0"); expect(screen.getByRole("button", { name: "Child" })).toBeVisible(); expect(requestMock).toHaveBeenCalledOnce()
  })
})
