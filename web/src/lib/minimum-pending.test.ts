import { describe, expect, it } from "vitest"
import { minimumPendingDurationMS, remainingPendingDuration } from "./minimum-pending"

describe("minimum pending duration", () => {
  it("holds fast operations for one complete UI transition window", () => {
    expect(minimumPendingDurationMS).toBe(150)
    expect(remainingPendingDuration(100, 150)).toBe(100)
    expect(remainingPendingDuration(100, 250)).toBe(0)
    expect(remainingPendingDuration(100, 150, true)).toBe(0)
  })
})
