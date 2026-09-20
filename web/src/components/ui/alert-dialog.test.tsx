import { useState } from "react"
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "./alert-dialog"

function Example() {
  return <AlertDialog>
    <AlertDialogTrigger>Delete clip</AlertDialogTrigger>
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>Delete this clip?</AlertDialogTitle>
        <AlertDialogDescription>The clip will move to the recycle bin.</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>Keep clip</AlertDialogCancel>
        <AlertDialogAction className="border-rose-300/60">Delete clip</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
}

describe("AlertDialog", () => {
  it("provides labelled alert semantics and initially focuses the safe action", async () => {
    const user = userEvent.setup()
    render(<Example />)
    await user.click(screen.getByRole("button", { name: "Delete clip" }))
    const dialog = screen.getByRole("alertdialog", { name: "Delete this clip?" })
    expect(dialog).toHaveAccessibleDescription("The clip will move to the recycle bin.")
    expect(dialog).toHaveClass("dialog-motion")
    await waitFor(() => expect(screen.getByRole("button", { name: "Keep clip" })).toHaveFocus())
  })

  it("traps Tab, ignores outside interaction, and restores focus after cancel", async () => {
    const user = userEvent.setup()
    render(<Example />)
    const trigger = screen.getByRole("button", { name: "Delete clip" })
    await user.click(trigger)
    const cancel = screen.getByRole("button", { name: "Keep clip" })
    const action = within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete clip" })
    action.focus()
    await user.tab()
    expect(cancel).toHaveFocus()
    fireEvent.pointerDown(document.body)
    expect(screen.getByRole("alertdialog")).toBeVisible()
    await user.click(cancel)
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it("supports controlled busy state that rejects every close request", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    function BusyExample() {
      const [open] = useState(true)
      return <AlertDialog open={open} onOpenChange={onOpenChange}>
        <AlertDialogContent>
          <AlertDialogTitle>Deleting clip</AlertDialogTitle>
          <AlertDialogDescription>Please wait for deletion to finish.</AlertDialogDescription>
          <AlertDialogCancel disabled>Cancel</AlertDialogCancel>
          <AlertDialogAction disabled>Delete</AlertDialogAction>
        </AlertDialogContent>
      </AlertDialog>
    }
    render(<BusyExample />)
    await user.keyboard("{Escape}")
    fireEvent.pointerDown(document.body)
    expect(screen.getByRole("alertdialog", { name: "Deleting clip" })).toBeVisible()
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
