import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { Session, User } from "@/api"
import { EditorDialog } from "./editor-dialog"

const requestMock = vi.hoisted(() => vi.fn())

vi.mock("@/api", async () => {
  const actual = await vi.importActual<typeof import("@/api")>("@/api")
  return { ...actual, request: requestMock }
})

const user: User = {
  id: 1,
  username: "editor",
  role: "user",
  state: "active",
  storedFileLimitBytes: 500_000_000,
  rootFolderId: 1,
  libraryTrashed: false,
}

const session: Session = {
  user,
  csrfToken: "csrf",
  publicBaseURL: "http://localhost:5173",
}

const editor = {
  id: "edit-1",
  state: "editing_session" as const,
  editRevision: 4,
  title: "Original title",
  destinationFolderId: 1,
  durationMs: 10_000,
  sourceSizeBytes: 2_000_000,
  sourceContainer: "mp4",
  sourceVideoCodec: "h264",
  sourceWidth: 1280,
  sourceHeight: 720,
  sourceFrameRate: 30,
  compressionRequested: false,
  qualityCrf: 23,
  maxHeight: 720,
  previewState: "ready" as const,
  previewStartMs: 0,
  previewEndMs: 10_000,
  previewRevision: 4,
  previewRendering: false,
  edit: { trimStartMs: 0, trimEndMs: 10_000, audio: [] },
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function metadataCalls() {
  return requestMock.mock.calls.filter(([path, options]) =>
    String(path).endsWith("/metadata") && (options as RequestInit | undefined)?.method === "PATCH",
  )
}

afterEach(() => {
  vi.useRealTimers()
  requestMock.mockReset()
  vi.restoreAllMocks()
})

describe("EditorDialog", () => {
  it("keeps title focus through typing, callback rerenders, heartbeat, and saving", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const userEvents = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const metadata = deferred<typeof editor>()
    requestMock.mockImplementation((path: string, options?: RequestInit) => {
      if (path.endsWith("/heartbeat")) return Promise.resolve(undefined)
      if (path.endsWith("/metadata") && options?.method === "PATCH") return metadata.promise
      return Promise.resolve(editor)
    })

    const onFinalized = vi.fn()
    const view = render(
      <EditorDialog session={session} sessionID="session-1" localFile={null} users={[user]} onClose={vi.fn()} onFinalized={onFinalized} />,
    )
    const editDetails = await screen.findByRole("button", { name: "Edit details" })
    await waitFor(() => expect(editDetails).toHaveFocus())
    await userEvents.click(editDetails)
    const input = screen.getByRole("textbox", { name: "Clip title" })
    expect(input).toHaveFocus()
    expect(screen.getAllByRole("dialog", { hidden: true })).toHaveLength(2)

    for (const character of "New title") {
      await userEvents.keyboard(character)
      view.rerender(
        <EditorDialog session={session} sessionID="session-1" localFile={null} users={[user]} onClose={() => undefined} onFinalized={onFinalized} />,
      )
      expect(input).toHaveFocus()
    }
    expect(input).toHaveValue("New title")
    expect(metadataCalls()).toHaveLength(0)

    await act(async () => {
      vi.advanceTimersByTime(60_000)
      await Promise.resolve()
    })
    expect(input).toHaveFocus()
    expect(requestMock.mock.calls.filter(([path]) => String(path).endsWith("/heartbeat"))).toHaveLength(1)

    await userEvents.keyboard("{Enter}")
    expect(metadataCalls()).toHaveLength(1)
    expect(input).toHaveFocus()
    expect(input).toBeEnabled()
    fireEvent.keyDown(input, { key: "Escape" })
    expect(screen.getByRole("dialog", { name: "Edit clip details" })).toBeVisible()
    expect(input).toHaveFocus()

    const submitted = JSON.parse((metadataCalls()[0][1] as RequestInit).body as string) as { title: string }
    expect(submitted.title).toBe("New title")
    await act(async () => metadata.resolve({ ...editor, title: "New title" }))
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Edit clip details" })).not.toBeInTheDocument())
    expect(editDetails).toHaveFocus()
    expect(metadataCalls()).toHaveLength(1)
  })

  it("cancels title edits with Escape without a metadata request", async () => {
    const userEvents = userEvent.setup()
    requestMock.mockResolvedValue(editor)
    render(<EditorDialog session={session} sessionID="session-1" localFile={null} users={[user]} onClose={vi.fn()} onFinalized={vi.fn()} />)
    const editDetails = await screen.findByRole("button", { name: "Edit details" })
    await userEvents.click(editDetails)
    await userEvents.keyboard("Replacement{Escape}")

    expect(screen.queryByRole("dialog", { name: "Edit clip details" })).not.toBeInTheDocument()
    expect(metadataCalls()).toHaveLength(0)
    expect(editDetails).toHaveFocus()
  })

  it("preserves Space-key playback and clean-session discard behavior", async () => {
    const userEvents = userEvent.setup()
    const onClose = vi.fn()
    requestMock.mockImplementation((_path: string, options?: RequestInit) =>
      options?.method === "DELETE" ? Promise.resolve(undefined) : Promise.resolve(editor),
    )
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue()
    render(<EditorDialog session={session} sessionID="session-1" localFile={null} users={[user]} onClose={onClose} onFinalized={vi.fn()} />)
    await screen.findByRole("button", { name: "Edit details" })

    fireEvent.keyDown(document.body, { key: " ", code: "Space" })
    expect(play).toHaveBeenCalledOnce()
    await userEvents.click(screen.getByRole("button", { name: "Discard upload" }))
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    expect(requestMock.mock.calls.some(([, options]) => (options as RequestInit | undefined)?.method === "DELETE")).toBe(true)
  })
})
