import { useEffect, useId, useRef, type ReactNode } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { cn } from "@/lib/utils"

type ModalProps = {
  title: string
  onClose: () => void
  children: ReactNode
  className?: string
  nested?: boolean
  dismissible?: boolean
}

export function Modal({ title, onClose, children, className, nested = false, dismissible = true }: ModalProps) {
  const titleID = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const backdropPointerStarted = useRef(false)

  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const focusables = () => Array.from(dialogRef.current?.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])') ?? [])
    const focusInitial = () => focusables()[0]?.focus()
    const isTopmost = () => Array.from(document.querySelectorAll('[role="dialog"][aria-modal="true"]')).at(-1) === dialogRef.current
    const onKeyDown = (event: KeyboardEvent) => {
      if (!isTopmost()) return
      if (event.key === "Escape" && dismissible) { event.preventDefault(); onClose(); return }
      if (event.key !== "Tab") return
      const items = focusables()
      if (items.length === 0) { event.preventDefault(); return }
      const first = items[0]; const last = items[items.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    const frame = window.requestAnimationFrame(focusInitial)
    document.addEventListener("keydown", onKeyDown)
    return () => { window.cancelAnimationFrame(frame); document.removeEventListener("keydown", onKeyDown); previous?.focus() }
  }, [dismissible, onClose])

  useEffect(() => {
    if (!nested) return
    const parent = Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"][aria-modal="true"]')).at(-2)
    if (!parent) return
    const previousAriaHidden = parent.getAttribute("aria-hidden")
    const wasInert = parent.hasAttribute("inert")
    parent.setAttribute("aria-hidden", "true")
    parent.setAttribute("inert", "")
    return () => {
      if (previousAriaHidden === null) parent.removeAttribute("aria-hidden")
      else parent.setAttribute("aria-hidden", previousAriaHidden)
      if (!wasInert) parent.removeAttribute("inert")
    }
  }, [nested])

  function closeFromBackdrop() {
    if (dismissible && Array.from(document.querySelectorAll('[role="dialog"][aria-modal="true"]')).at(-1) === dialogRef.current) onClose()
  }

  return createPortal(<div ref={dialogRef} className={cn("fixed inset-0 grid place-items-center bg-black/75 px-4 backdrop-blur-sm", nested ? "z-[60]" : "z-50")} role="dialog" aria-modal="true" aria-labelledby={titleID}
    onPointerDown={(event) => { backdropPointerStarted.current = event.target === event.currentTarget }}
    onPointerUp={(event) => { if (backdropPointerStarted.current && event.target === event.currentTarget) closeFromBackdrop(); backdropPointerStarted.current = false }}
    onPointerCancel={() => { backdropPointerStarted.current = false }}>
    <Card className={cn("w-full max-w-lg overflow-hidden", className)}>
      <CardHeader className="flex flex-row items-center justify-between gap-4">
        <h2 id={titleID} className="truncate text-xl font-semibold text-white">{title}</h2>
        <Button type="button" variant="ghost" size="sm" disabled={!dismissible} onClick={onClose} aria-label={`Close ${title}`}><X size={17} /></Button>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  </div>, document.body)
}
