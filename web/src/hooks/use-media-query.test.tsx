import { act, renderHook } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { useIsNarrow, useMediaQuery } from "./use-media-query"

afterEach(() => vi.unstubAllGlobals())

describe("useMediaQuery", () => {
  it("uses an SSR-safe fallback when matchMedia is unavailable", () => {
    vi.stubGlobal("matchMedia", undefined)
    expect(renderHook(() => useMediaQuery("(min-width: 1px)", true)).result.current).toBe(true)
  })

  it("subscribes to modern change events and cleans up", () => {
    let listener: ((event: MediaQueryListEvent) => void) | undefined
    const add = vi.fn((_type, next) => { listener = next as (event: MediaQueryListEvent) => void })
    const remove = vi.fn()
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false, media: "", onchange: null, addEventListener: add, removeEventListener: remove, dispatchEvent: vi.fn() })))
    const hook = renderHook(() => useIsNarrow())
    expect(hook.result.current).toBe(false)
    act(() => listener?.({ matches: true } as MediaQueryListEvent))
    expect(hook.result.current).toBe(true)
    hook.unmount(); expect(remove).toHaveBeenCalledOnce()
  })
})
