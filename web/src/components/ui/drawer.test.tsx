import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { Drawer, DrawerClose, DrawerContent, DrawerDescription, DrawerFooter, DrawerHeader, DrawerTitle } from "./drawer"

describe("Drawer", () => {
  it("renders a labelled bottom sheet that fits a narrow viewport", async () => {
    const user = userEvent.setup()
    render(<Drawer defaultOpen><DrawerContent className="max-h-[90dvh]"><DrawerHeader><DrawerTitle>Choose folder</DrawerTitle><DrawerDescription>Select a destination.</DrawerDescription></DrawerHeader><DrawerFooter><DrawerClose>Cancel</DrawerClose></DrawerFooter></DrawerContent></Drawer>)
    const dialog = screen.getByRole("dialog", { name: "Choose folder" })
    expect(dialog).toHaveAccessibleDescription("Select a destination.")
    expect(dialog).toHaveClass("inset-x-0", "bottom-0", "max-h-[90dvh]")
    await user.click(screen.getByRole("button", { name: "Cancel" }))
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })

  it("allows a controlled busy consumer to reject close", async () => {
    const user = userEvent.setup(); const change = vi.fn()
    render(<Drawer open onOpenChange={change} dismissible={false}><DrawerContent><DrawerHeader><DrawerTitle>Moving</DrawerTitle><DrawerDescription>Please wait.</DrawerDescription></DrawerHeader><DrawerClose disabled>Cancel</DrawerClose></DrawerContent></Drawer>)
    await user.keyboard("{Escape}")
    expect(screen.getByRole("dialog", { name: "Moving" })).toBeVisible()
  })
})
