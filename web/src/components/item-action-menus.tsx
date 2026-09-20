import { cloneElement, useState, type HTMLAttributes, type KeyboardEvent, type ReactElement } from "react"
import { MoreHorizontal } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from "@/components/ui/context-menu"
import type { ItemAction } from "@/lib/item-actions"

export function ItemDropdownActions({ actions, label = "Item actions" }: { actions: ItemAction[]; label?: string }) {
  if (actions.length === 0) return null
  return <DropdownMenu><DropdownMenuTrigger asChild><Button type="button" size="sm" variant="ghost" className="ml-auto w-9 px-0" aria-label={label}><MoreHorizontal /></Button></DropdownMenuTrigger><DropdownMenuContent align="end">{renderActions(actions, "dropdown")}</DropdownMenuContent></DropdownMenu>
}

export function ItemContextSurface({ actions, children }: { actions: ItemAction[]; children: ReactElement<HTMLAttributes<HTMLElement>> }) {
  const [open, setOpen] = useState(false)
  if (actions.length === 0) return children
  const originalKeyDown = children.props.onKeyDown
  const trigger = cloneElement(children, {
    onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
      originalKeyDown?.(event)
      if (event.defaultPrevented || !(event.key === "ContextMenu" || event.key === "F10" && event.shiftKey)) return
      event.preventDefault()
      const visibleTrigger = event.currentTarget.querySelector<HTMLButtonElement>('[aria-label^="Actions for"]')
      if (visibleTrigger) {
        visibleTrigger.click()
        return
      }
      const bounds = event.currentTarget.getBoundingClientRect()
      event.currentTarget.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: bounds.left + Math.min(24, bounds.width / 2), clientY: bounds.top + Math.min(24, bounds.height / 2) }))
      setOpen(true)
    },
  })
  return <ContextMenu open={open} onOpenChange={setOpen}><ContextMenuTrigger asChild>{trigger}</ContextMenuTrigger><ContextMenuContent>{renderActions(actions, "context")}</ContextMenuContent></ContextMenu>
}

function renderActions(actions: ItemAction[], kind: "dropdown" | "context") {
  const result: ReactElement[] = []
  actions.forEach((action, index) => {
    if (index > 0 && action.group !== actions[index - 1].group) result.push(kind === "dropdown" ? <DropdownMenuSeparator key={`s-${action.group}`} /> : <ContextMenuSeparator key={`s-${action.group}`} />)
    const Icon = action.icon
    result.push(kind === "dropdown"
      ? <DropdownMenuItem key={action.id} disabled={action.disabled} className={action.destructive ? "text-rose-300 focus:bg-rose-400/10 focus:text-rose-200" : undefined} onSelect={() => void action.run()}><Icon />{action.label}</DropdownMenuItem>
      : <ContextMenuItem key={action.id} disabled={action.disabled} destructive={action.destructive} onSelect={() => void action.run()}><Icon />{action.label}</ContextMenuItem>)
  })
  return result
}
