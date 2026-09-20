import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { Eye, Trash2 } from "lucide-react"
import { ItemContextSurface, ItemDropdownActions } from "./item-action-menus"
import type { ItemAction } from "@/lib/item-actions"

const run = vi.fn()
const actions: ItemAction[] = [{ id: "preview", label: "Preview", group: 0, icon: Eye, run }, { id: "trash", label: "Move to recycle bin", group: 1, icon: Trash2, destructive: true, run }]

describe("item action menus", () => {
  it("renders and runs the dropdown descriptors once", async () => {
    const user = userEvent.setup(); render(<ItemDropdownActions actions={actions} />)
    await user.click(screen.getByRole("button", { name: "Item actions" })); await user.click(screen.getByRole("menuitem", { name: "Preview" }))
    expect(run).toHaveBeenCalledOnce()
  })
  it("opens the same actions from a context surface", async () => {
    const user = userEvent.setup(); render(<ItemContextSurface actions={actions}><button>Item</button></ItemContextSurface>)
    await user.pointer({ keys: "[MouseRight]", target: screen.getByRole("button", { name: "Item" }) })
    expect(screen.getByRole("menuitem", { name: "Preview" })).toBeInTheDocument()
    expect(screen.getByRole("menuitem", { name: "Move to recycle bin" })).toHaveAttribute("data-destructive", "true")
  })
  it.each(["{ContextMenu}", "{Shift>}{F10}{/Shift}"])("opens from the keyboard shortcut %s", async (shortcut) => {
    const user = userEvent.setup(); render(<ItemContextSurface actions={actions}><button>Keyboard item</button></ItemContextSurface>)
    const item = screen.getByRole("button", { name: "Keyboard item" }); item.focus()
    await user.keyboard(shortcut)
    expect(screen.getByRole("menuitem", { name: "Preview" })).toBeInTheDocument()
  })
  it.each(["{Shift>}{F10}{/Shift}", "{ContextMenu}"])("opens from the keyboard shortcut %s", async (shortcut) => {
    const user = userEvent.setup(); render(<ItemContextSurface actions={actions}><button>Keyboard item</button></ItemContextSurface>)
    screen.getByRole("button", { name: "Keyboard item" }).focus()
    await user.keyboard(shortcut)
    expect(screen.getByRole("menuitem", { name: "Preview" })).toBeInTheDocument()
  })
})
