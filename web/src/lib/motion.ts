import type { Transition, Variants } from "motion/react"

// Change this one value to tune application Motion transitions and the minimum
// explorer loading/refresh presentation time together.
export const uiAnimationDurationMS = 150
export const uiAnimationDurationSeconds = uiAnimationDurationMS / 1_000

export const fastTransition: Transition = { duration: uiAnimationDurationSeconds, ease: [0.22, 1, 0.36, 1] }
export const standardTransition: Transition = { duration: uiAnimationDurationSeconds, ease: [0.22, 1, 0.36, 1] }
export const layoutTransition: Transition = { duration: uiAnimationDurationSeconds, ease: [0.22, 1, 0.36, 1] }

export type NavigationDirection = "forward" | "back" | "neutral"

export function collectionVariants(direction: NavigationDirection, reducedMotion = false): Variants {
  if (reducedMotion) return { hidden: { opacity: 0 }, visible: { opacity: 1, transition: fastTransition }, exit: { opacity: 0, transition: fastTransition } }
  const offset = direction === "forward" ? 12 : direction === "back" ? -12 : 0
  return { hidden: { opacity: 0, x: offset }, visible: { opacity: 1, x: 0, transition: standardTransition }, exit: { opacity: 0, x: -offset, transition: fastTransition } }
}

export function initialItemVariants(index: number, enabled: boolean, reducedMotion = false): Variants {
  if (!enabled || index >= 8) return { hidden: { opacity: 1 }, visible: { opacity: 1 } }
  return reducedMotion
    ? { hidden: { opacity: 0 }, visible: { opacity: 1, transition: fastTransition } }
    : { hidden: { opacity: 0, y: 6 }, visible: { opacity: 1, y: 0, transition: { ...fastTransition, delay: index * 0.015 } } }
}
