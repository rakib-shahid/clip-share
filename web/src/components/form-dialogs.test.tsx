import { useState } from "react"
import { act, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import type { Session, User } from "@/api"
import { ChangePasswordDialog, CreateUserDialog } from "@/App"
import { NameDialog } from "./library-explorer"

const requestMock = vi.hoisted(() => vi.fn())

vi.mock("@/api", async () => {
  const actual = await vi.importActual<typeof import("@/api")>("@/api")
  return { ...actual, request: requestMock }
})

const account: User = {
  id: 1,
  username: "admin",
  role: "admin",
  state: "active",
  storedFileLimitBytes: 500_000_000,
  rootFolderId: 1,
  libraryTrashed: false,
}

const session: Session = {
  user: account,
  csrfToken: "csrf-token",
  publicBaseURL: "http://localhost:5173",
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function overlayFor(dialog: HTMLElement) {
  const overlay = dialog.previousElementSibling
  if (!(overlay instanceof HTMLElement)) throw new Error("Dialog overlay was not rendered")
  return overlay
}

afterEach(() => {
  requestMock.mockReset()
})

describe("application form Dialogs", () => {
  it.each([
    ["folder", "Create folder", "Folder name", 100],
    ["clip", "Rename clip", "Clip title", 200],
  ])("submits one trimmed %s name while busy dismissal is blocked", async (_kind, title, fieldLabel, maxLength) => {
    const user = userEvent.setup()
    const pending = deferred<void>()
    const submitted = vi.fn((name: string) => { void name; return pending.promise })
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Open naming</button>{open && <NameDialog title={title} fieldLabel={fieldLabel} action="Save" initialValue="  Original  " onClose={() => setOpen(false)} onSubmit={async (name) => { await submitted(name); setOpen(false) }} />}</>
    }
    render(<Harness />)
    const opener = screen.getByRole("button", { name: "Open naming" })
    await user.click(opener)
    const input = screen.getByRole("textbox", { name: fieldLabel })
    expect(input).toHaveFocus()
    expect(input).toHaveAttribute("maxlength", String(maxLength))
    await user.clear(input)
    await user.type(input, "  Updated name  {Enter}{Enter}")

    expect(submitted).toHaveBeenCalledOnce()
    expect(submitted).toHaveBeenCalledWith("Updated name")
    const dialog = screen.getByRole("dialog", { name: title })
    await user.keyboard("{Escape}")
    await user.click(overlayFor(dialog))
    expect(screen.getByRole("dialog", { name: title })).toBeVisible()

    await act(async () => pending.resolve())
    await waitFor(() => expect(screen.queryByRole("dialog", { name: title })).not.toBeInTheDocument())
    expect(opener).toHaveFocus()
  })

  it("keeps a naming Dialog open for blank and server-error states", async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn().mockRejectedValue(new Error("Name already exists."))
    render(<NameDialog title="Create folder" action="Create" onClose={vi.fn()} onSubmit={onSubmit} />)
    const input = screen.getByRole("textbox", { name: "Folder name" })
    await user.keyboard("{Enter}")
    expect(onSubmit).not.toHaveBeenCalled()
    await user.type(input, "Existing{Enter}")
    expect(await screen.findByRole("alert")).toHaveTextContent("Name already exists.")
    expect(screen.getByRole("dialog", { name: "Create folder" })).toBeVisible()
  })

  it("creates a user once with a trimmed username and restores opener focus", async () => {
    const user = userEvent.setup()
    const pending = deferred<User>()
    requestMock.mockReturnValue(pending.promise)
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Open create user</button>{open && <CreateUserDialog csrf="csrf-token" onClose={() => setOpen(false)} onCreated={() => setOpen(false)} />}</>
    }
    render(<Harness />)
    const opener = screen.getByRole("button", { name: "Open create user" })
    await user.click(opener)
    const username = screen.getByRole("textbox", { name: "Username" })
    const password = screen.getByLabelText("Password")
    expect(username).toHaveFocus()
    expect(username).toHaveAttribute("autocomplete", "username")
    expect(password).toHaveAttribute("autocomplete", "new-password")
    await user.keyboard("{Enter}")
    expect(requestMock).not.toHaveBeenCalled()
    await user.type(username, "  Alice  ")
    await user.type(password, " passw0rd ")
    await user.keyboard("{Enter}{Enter}")
    expect(requestMock).toHaveBeenCalledOnce()
    const [, options] = requestMock.mock.calls[0] as [string, RequestInit]
    expect(options.headers).toEqual({ "X-CSRF-Token": "csrf-token" })
    expect(JSON.parse(options.body as string)).toEqual({ username: "Alice", password: " passw0rd ", storedFileLimitMb: 50 })
    await user.keyboard("{Escape}")
    expect(screen.getByRole("dialog", { name: "Create a user" })).toBeVisible()

    await act(async () => pending.resolve({ ...account, id: 2, username: "Alice", role: "user" }))
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Create a user" })).not.toBeInTheDocument())
    expect(opener).toHaveFocus()
  })

  it("shows create-user server errors and becomes dismissible again", async () => {
    const user = userEvent.setup()
    requestMock.mockRejectedValue(new Error("Username is unavailable."))
    const onClose = vi.fn()
    render(<CreateUserDialog csrf="csrf-token" onClose={onClose} onCreated={vi.fn()} />)
    await user.type(screen.getByRole("textbox", { name: "Username" }), "alice")
    await user.type(screen.getByLabelText("Password"), "passw0rd")
    await user.keyboard("{Enter}")
    expect(await screen.findByRole("alert")).toHaveTextContent("Username is unavailable.")
    await user.keyboard("{Escape}")
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("changes a password once, preserving password whitespace and blocking busy close", async () => {
    const user = userEvent.setup()
    const pending = deferred<void>()
    requestMock.mockReturnValue(pending.promise)
    function Harness() {
      const [open, setOpen] = useState(false)
      return <><button onClick={() => setOpen(true)}>Open password</button>{open && <ChangePasswordDialog session={session} onClose={() => setOpen(false)} />}</>
    }
    render(<Harness />)
    const opener = screen.getByRole("button", { name: "Open password" })
    await user.click(opener)
    const current = screen.getByLabelText("Current password")
    const next = screen.getByLabelText("New password")
    const confirm = screen.getByLabelText("Confirm new password")
    expect(current).toHaveFocus()
    expect(current).toHaveAttribute("autocomplete", "current-password")
    expect(next).toHaveAttribute("autocomplete", "new-password")
    await user.keyboard("{Enter}")
    expect(requestMock).not.toHaveBeenCalled()
    await user.type(current, "oldpass1")
    await user.type(next, " newpass1 ")
    await user.type(confirm, " newpass1 ")
    await user.keyboard("{Enter}{Enter}")
    expect(requestMock).toHaveBeenCalledOnce()
    const [, options] = requestMock.mock.calls[0] as [string, RequestInit]
    expect(JSON.parse(options.body as string)).toEqual({ currentPassword: "oldpass1", newPassword: " newpass1 ", confirmPassword: " newpass1 " })
    await user.keyboard("{Escape}")
    expect(screen.getByRole("dialog", { name: "Change password" })).toBeVisible()

    await act(async () => pending.resolve())
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Change password" })).not.toBeInTheDocument())
    expect(opener).toHaveFocus()
  })

  it("places password server errors beside the form and allows Escape afterward", async () => {
    const user = userEvent.setup()
    requestMock.mockRejectedValue(new Error("Current password is incorrect."))
    const onClose = vi.fn()
    render(<ChangePasswordDialog session={session} onClose={onClose} />)
    await user.type(screen.getByLabelText("Current password"), "oldpass1")
    await user.type(screen.getByLabelText("New password"), "newpass1")
    await user.type(screen.getByLabelText("Confirm new password"), "newpass1")
    await user.keyboard("{Enter}")
    expect(await screen.findByRole("alert")).toHaveTextContent("Current password is incorrect.")
    await user.keyboard("{Escape}")
    expect(onClose).toHaveBeenCalledOnce()
  })
})
