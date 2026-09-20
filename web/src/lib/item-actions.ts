import type { ComponentType } from "react"
import { Copy, ExternalLink, Eye, FolderInput, FolderOpen, Pencil, Trash2, XCircle } from "lucide-react"
import type { ExplorerItem } from "@/lib/explorer-item"

export type ItemActionID = "open" | "preview" | "copy-link" | "public-page" | "rename" | "move" | "copy" | "trash" | "cancel" | "dismiss"
export type ItemAction = { id: ItemActionID; label: string; group: number; destructive?: boolean; disabled?: boolean; icon: ComponentType<{ className?: string }>; run: () => void | Promise<void> }
export type ItemActionHandlers = Partial<Record<ItemActionID, () => void | Promise<void>>>

export function itemActions(item: ExplorerItem, handlers: ItemActionHandlers): ItemAction[] {
  const add = (id: ItemActionID, label: string, group: number, icon: ItemAction["icon"], destructive = false) => handlers[id] ? [{ id, label, group, icon, destructive, run: handlers[id]! }] : []
  if (item.kind === "folder") return [
    ...add("open", "Open", 0, FolderOpen), ...add("rename", "Rename", 0, Pencil), ...add("move", "Move", 0, FolderInput),
    ...(item.capabilities.copy ? add("copy", "Copy", 0, Copy) : []), ...add("trash", "Move to recycle bin", 1, Trash2, true),
  ]
  if (item.capabilities.preview) return [
    ...add("preview", "Preview", 0, Eye), ...add("copy-link", "Copy link", 0, Copy), ...add("public-page", "Open public page", 0, ExternalLink),
    ...add("rename", "Rename", 1, Pencil), ...add("move", "Move", 1, FolderInput), ...add("trash", "Move to recycle bin", 2, Trash2, true),
  ]
  if (item.capabilities.cancelUpload) return add("cancel", "Cancel upload", 0, XCircle, true)
  if (item.capabilities.dismissFailure) return add("dismiss", "Dismiss failed upload", 0, Trash2, true)
  return []
}
