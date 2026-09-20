import { readFileSync } from "node:fs"
import { describe, expect, it } from "vitest"
import { cn } from "@/lib/utils"

function read(relativePath: string) {
  return readFileSync(new URL(relativePath, import.meta.url), "utf8")
}

function cssHex(css: string, token: string) {
  const match = css.match(new RegExp(`--${token}:\\s*(#[0-9a-fA-F]{6});`))
  if (!match) throw new Error(`Missing opaque CSS token --${token}`)
  return match[1]
}

function luminance(hex: string) {
  const channels = hex.slice(1).match(/.{2}/g)?.map((value) => Number.parseInt(value, 16) / 255)
  if (!channels) throw new Error(`Invalid color ${hex}`)
  const linear = channels.map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4)
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2]
}

function contrast(first: string, second: string) {
  const high = Math.max(luminance(first), luminance(second))
  const low = Math.min(luminance(first), luminance(second))
  return (high + 0.05) / (low + 0.05)
}

describe("shadcn and theme baseline", () => {
  it("keeps the compatibility cn export while merging Tailwind classes", () => {
    expect(cn("px-2", undefined, "px-4")).toBe("px-4")
  })

  it("keeps dialogs centered with a smooth non-bouncing entrance", () => {
    const css = read("../index.css")
    const entrance = css.match(/@keyframes dialog-rise-in \{([\s\S]*?)\n\}/)?.[1] ?? ""
    expect(entrance).toContain("0%")
    expect(entrance).toContain("100%")
    expect(entrance).not.toMatch(/58%|78%|scale\(1\.00[1-9]/)
    expect(css).toContain('.dialog-motion { left: 50%; top: 50%; transform: translate(-50%, -50%); }')
    expect(css).toContain("animation: dialog-rise-in 180ms")
  })

  it("records the pinned Radix registry source", () => {
    const manifest = JSON.parse(read("../../shadcn-registry.json")) as {
      cliVersion: string
      provider: string
      style: string
      components: string[]
    }
    const configuration = JSON.parse(read("../../components.json")) as { style: string }

    expect(manifest).toMatchObject({ cliVersion: "4.21.0", provider: "radix" })
    expect(manifest.style).toBe(configuration.style)
    for (const component of manifest.components) {
      expect(read(`../components/ui/${component}.tsx`)).not.toHaveLength(0)
    }
  })

  it.each([
    ["normal", "foreground", "background"],
    ["muted", "muted-foreground", "background"],
    ["focus", "ring", "background"],
    ["success", "success-foreground", "success"],
    ["warning", "warning-foreground", "warning"],
    ["destructive", "destructive-foreground", "destructive"],
  ])("keeps %s text at WCAG AA contrast", (_name, foreground, background) => {
    const css = read("../index.css")
    expect(contrast(cssHex(css, foreground), cssHex(css, background))).toBeGreaterThanOrEqual(4.5)
  })
})
