import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"
import { Modal } from "./modal"

describe("Modal", () => {
  it("dismisses exactly once when pointer down and up stay on the backdrop", () => {
    const onClose = vi.fn()
    render(<Modal title="Test dialog" onClose={onClose}><p>Content</p></Modal>)
    const backdrop = screen.getByRole("dialog")
    fireEvent.pointerDown(backdrop)
    fireEvent.pointerUp(backdrop)
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("does not dismiss when a pointer drag crosses the card boundary", () => {
    const onClose = vi.fn()
    render(<Modal title="Test dialog" onClose={onClose}><p>Content</p></Modal>)
    const backdrop = screen.getByRole("dialog")
    const card = screen.getByText("Content").closest("div")!
    fireEvent.pointerDown(backdrop)
    fireEvent.pointerUp(card)
    expect(onClose).not.toHaveBeenCalled()
  })

  it("only the frontmost nested dialog handles Escape and makes its parent inert", async () => {
    const parentClose = vi.fn()
    const childClose = vi.fn()
    render(<><Modal title="Parent" onClose={parentClose}><button>Parent action</button></Modal><Modal title="Child" onClose={childClose} nested><button>Child action</button></Modal></>)
    await waitFor(() => expect(document.querySelectorAll('[role="dialog"]')[0]).toHaveAttribute("aria-hidden", "true"))
    const dialogs = document.querySelectorAll('[role="dialog"]')
    expect(dialogs[0]).toHaveAttribute("inert")
    fireEvent.keyDown(document, { key: "Escape" })
    expect(childClose).toHaveBeenCalledOnce()
    expect(parentClose).not.toHaveBeenCalled()
  })
})
