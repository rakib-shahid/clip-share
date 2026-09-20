import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { request } from "@/api"
import { Shell } from "@/App"

vi.mock("@/api", async (original) => ({ ...(await original<typeof import("@/api")>()), request: vi.fn() }))
const requestMock = vi.mocked(request)
const session = { user: { id: 1, username: "User", role: "user" as const, state: "active" as const, storedFileLimitBytes: 1, rootFolderId: 1, libraryTrashed: false }, csrfToken: "csrf", publicBaseURL: "http://test" }

beforeEach(() => { requestMock.mockReset(); requestMock.mockResolvedValue({ folder: { id: 1, ownerUserId: 1, ownerUsername: "User", parentFolderId: null, name: "Root", isRoot: true, folderCount: 0, clipCount: 0 }, breadcrumbs: [], folders: [], clips: [], nextCursor: null }) })

describe("account Dropdown Menu", () => {
  it("opens by keyboard, navigates, escapes, and restores trigger focus", async () => {
    const user = userEvent.setup(); render(<Shell session={session} onSessionUpdated={() => undefined} onLogout={() => undefined} />)
    const trigger = screen.getByRole("button", { name: "Account menu for User" }); trigger.focus(); await user.keyboard("{Enter}")
    expect(screen.getByRole("menuitem", { name: "Change password" })).toHaveFocus(); await user.keyboard("{End}"); expect(screen.getByRole("menuitem", { name: "Log out" })).toHaveFocus(); await user.keyboard("{Escape}"); expect(trigger).toHaveFocus()
  })

  it("opens Change password and invokes logout once", async () => {
    const user = userEvent.setup(); const logout = vi.fn(); render(<Shell session={session} onSessionUpdated={() => undefined} onLogout={logout} />)
    await user.click(screen.getByRole("button", { name: "Account menu for User" })); await user.click(screen.getByRole("menuitem", { name: "Change password" })); expect(screen.getByRole("dialog", { name: "Change password" })).toBeVisible()
    await user.keyboard("{Escape}"); await user.click(screen.getByRole("button", { name: "Account menu for User" })); await user.click(screen.getByRole("menuitem", { name: "Log out" })); await waitFor(() => expect(logout).toHaveBeenCalledOnce())
  })
})
