import { render, screen } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { AppMotionProvider } from "@/components/motion-provider"
import { collectionVariants, fastTransition, initialItemVariants, layoutTransition, standardTransition, uiAnimationDurationMS, uiAnimationDurationSeconds } from "./motion"

describe("motion foundation", () => {
  it("configures a root provider without delaying ordinary input", () => {
    render(<AppMotionProvider><input aria-label="Title" /></AppMotionProvider>)
    expect(screen.getByRole("textbox", { name: "Title" })).toBeEnabled()
  })

  it("sources every shared transition from the single application duration", () => {
    expect(uiAnimationDurationMS).toBe(150)
    expect(uiAnimationDurationSeconds).toBe(0.15)
    for (const transition of [fastTransition, standardTransition, layoutTransition]) {
      expect(transition.duration).toBe(uiAnimationDurationSeconds); expect(transition).not.toHaveProperty("repeat")
    }
  })

  it("encodes navigation direction and caps initial stagger at eight items", () => {
    expect(collectionVariants("forward").hidden).toMatchObject({ x: 12 })
    expect(collectionVariants("back").hidden).toMatchObject({ x: -12 })
    expect(collectionVariants("forward", true).hidden).not.toHaveProperty("x")
    expect(initialItemVariants(7, true).hidden).toMatchObject({ opacity: 0 })
    expect(initialItemVariants(8, true).hidden).toEqual({ opacity: 1 })
    expect(initialItemVariants(2, true, true).hidden).not.toHaveProperty("y")
  })
})
