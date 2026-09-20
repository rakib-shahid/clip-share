import { uiAnimationDurationMS } from "./motion"

export const minimumPendingDurationMS = uiAnimationDurationMS

export function remainingPendingDuration(startedAt: number, now = performance.now(), reducedMotion = false) {
  if (reducedMotion) return 0
  return Math.max(0, minimumPendingDurationMS - (now - startedAt))
}

export async function waitForMinimumPending(startedAt: number, reducedMotion = false) {
  const remaining = remainingPendingDuration(startedAt, performance.now(), reducedMotion)
  if (remaining > 0) await new Promise<void>((resolve) => window.setTimeout(resolve, remaining))
}
