import { useEffect, useRef, useState } from "react"
import { CloudUpload } from "lucide-react"
import { AnimatePresence, m, useReducedMotion } from "motion/react"
import { uiAnimationDurationSeconds } from "@/lib/motion"

export function ViewportFileDrop({ enabled = true, destinationLabel, onFile }: { enabled?: boolean; destinationLabel: string; onFile: (file: File) => void }) {
  const [active, setActive] = useState(false)
  const dragDepth = useRef(0)
  const reducedMotion = useReducedMotion()

  useEffect(() => {
    if (!enabled) return
    const isFileDrag = (event: DragEvent) => Array.from(event.dataTransfer?.types ?? []).includes("Files")
    const reset = () => { dragDepth.current = 0; setActive(false) }
    const enter = (event: DragEvent) => {
      if (!isFileDrag(event)) return
      event.preventDefault()
      dragDepth.current += 1
      setActive(true)
    }
    const over = (event: DragEvent) => {
      if (!isFileDrag(event)) return
      event.preventDefault()
      if (event.dataTransfer) event.dataTransfer.dropEffect = "copy"
      setActive(true)
    }
    const leave = (event: DragEvent) => {
      if (!isFileDrag(event)) return
      dragDepth.current = Math.max(0, dragDepth.current - 1)
      if (dragDepth.current === 0) setActive(false)
    }
    const drop = (event: DragEvent) => {
      if (!isFileDrag(event)) return
      event.preventDefault()
      event.stopPropagation()
      const selected = event.dataTransfer?.files[0]
      reset()
      if (selected) onFile(selected)
    }
    window.addEventListener("dragenter", enter, true)
    window.addEventListener("dragover", over, true)
    window.addEventListener("dragleave", leave, true)
    window.addEventListener("drop", drop, true)
    window.addEventListener("dragend", reset, true)
    return () => {
      window.removeEventListener("dragenter", enter, true)
      window.removeEventListener("dragover", over, true)
      window.removeEventListener("dragleave", leave, true)
      window.removeEventListener("drop", drop, true)
      window.removeEventListener("dragend", reset, true)
      dragDepth.current = 0
    }
  }, [enabled, onFile])

  return <AnimatePresence>{active && <m.div className="pointer-events-none fixed inset-0 z-[100] grid place-items-center bg-slate-950/75 p-5 backdrop-blur-md" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: reducedMotion ? 0 : uiAnimationDurationSeconds }} role="status" aria-live="polite" data-global-file-drop-overlay><m.div className="grid min-h-72 w-full max-w-2xl place-items-center rounded-3xl border-2 border-dashed border-sky-300/70 bg-sky-400/[.08] p-8 text-center shadow-2xl shadow-sky-950/50" initial={reducedMotion ? false : { opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.98 }} transition={{ duration: reducedMotion ? 0 : uiAnimationDurationSeconds }}><div><span className="mx-auto grid size-16 place-items-center rounded-2xl bg-sky-400 text-slate-950 shadow-lg shadow-sky-400/20"><CloudUpload size={32} /></span><p className="mt-5 text-xl font-semibold text-white">Drop video to upload</p><p className="mt-2 text-sm text-sky-100/75">It will be added to {destinationLabel}. You can review the title and upload options first.</p></div></m.div></m.div>}</AnimatePresence>
}
