import type { ReactNode } from "react"
import { domAnimation, LazyMotion, MotionConfig } from "motion/react"

export function AppMotionProvider({ children }: { children: ReactNode }) {
  return <MotionConfig reducedMotion="user"><LazyMotion features={domAnimation} strict>{children}</LazyMotion></MotionConfig>
}
