import { useState } from "react"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "./dialog"

function DialogHarness({ onAutoFocus }: { onAutoFocus?: () => void }) {
  const [open, setOpen] = useState(false)
  const [value, setValue] = useState("")
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => setOpen(nextOpen)}>
      <DialogTrigger>Open details</DialogTrigger>
      <DialogContent onOpenAutoFocus={onAutoFocus}>
        <DialogTitle>Clip details</DialogTitle>
        <DialogDescription>Rename the selected clip.</DialogDescription>
        <label>Title<input value={value} onChange={(event) => setValue(event.target.value)} /></label>
        <button type="button">Save</button>
      </DialogContent>
    </Dialog>
  )
}

describe("Dialog", () => {
  it("labels content and focuses once without refocusing on parent rerenders", async () => {
    const user = userEvent.setup()
    const onAutoFocus = vi.fn()
    render(<DialogHarness onAutoFocus={onAutoFocus} />)

    await user.click(screen.getByRole("button", { name: "Open details" }))
    const dialog = screen.getByRole("dialog", { name: "Clip details" })
    expect(dialog).toHaveClass("dialog-motion")
    expect(dialog).toHaveAccessibleDescription("Rename the selected clip.")
    const input = screen.getByRole("textbox", { name: "Title" })
    await waitFor(() => expect(input).toHaveFocus())

    await user.type(input, "A revised title")
    expect(input).toHaveFocus()
    expect(onAutoFocus).toHaveBeenCalledOnce()
    expect(screen.getByRole("button", { name: "Close" })).toBeVisible()
  })

  it("contains forward and reverse Tab focus", async () => {
    const user = userEvent.setup()
    render(<DialogHarness />)
    await user.click(screen.getByRole("button", { name: "Open details" }))

    const input = screen.getByRole("textbox", { name: "Title" })
    const close = screen.getByRole("button", { name: "Close" })
    close.focus()
    await user.tab()
    expect(input).toHaveFocus()
    await user.tab({ shift: true })
    expect(close).toHaveFocus()
  })

  it("closes with Escape and restores focus to its trigger", async () => {
    const user = userEvent.setup()
    render(<DialogHarness />)
    const trigger = screen.getByRole("button", { name: "Open details" })
    await user.click(trigger)
    await user.keyboard("{Escape}")

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it("closes when the overlay receives a pointer interaction", async () => {
    const user = userEvent.setup()
    render(<DialogHarness />)
    await user.click(screen.getByRole("button", { name: "Open details" }))
    const dialog = screen.getByRole("dialog")
    const overlay = dialog.previousElementSibling
    expect(overlay).not.toBeNull()
    await user.click(overlay as Element)
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })

  it("keeps the parent mounted while only the topmost nested dialog handles Escape", async () => {
    const user = userEvent.setup()
    render(
      <Dialog defaultOpen>
        <DialogContent>
          <DialogTitle>Parent dialog</DialogTitle>
          <DialogDescription>Parent content.</DialogDescription>
          <Dialog>
            <DialogTrigger>Open warning</DialogTrigger>
            <DialogContent>
              <DialogTitle>Nested warning</DialogTitle>
              <DialogDescription>Nested content.</DialogDescription>
            </DialogContent>
          </Dialog>
        </DialogContent>
      </Dialog>,
    )

    const nestedTrigger = screen.getByRole("button", { name: "Open warning" })
    await user.click(nestedTrigger)
    expect(screen.getAllByRole("dialog", { hidden: true })).toHaveLength(2)
    fireEvent.keyDown(document.activeElement ?? document.body, { key: "Escape" })

    expect(screen.getByRole("dialog", { name: "Parent dialog" })).toBeVisible()
    expect(screen.queryByRole("dialog", { name: "Nested warning" })).not.toBeInTheDocument()
    await waitFor(() => expect(nestedTrigger).toHaveFocus())
  })
})
