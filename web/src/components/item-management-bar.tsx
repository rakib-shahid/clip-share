import { Copy, FolderInput, Pencil, Trash } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

type ItemManagementBarProps = {
  onRename: () => void
  onMove: () => void
  onDelete: () => void
  onCopy?: () => void
}

export function ItemManagementBar({ onRename, onMove, onDelete, onCopy }: ItemManagementBarProps) {
  const hasCopy = Boolean(onCopy)
  const actionClass = hasCopy ? "w-full min-w-0 gap-1 px-1 text-xs" : undefined

  return <div className={cn(
    "-mt-3 grid rounded-b-2xl border-x border-b border-white/[.06] bg-slate-950 py-2",
    hasCopy ? "grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.25rem_minmax(0,1fr)] gap-1 px-2" : "grid-cols-3 gap-1 px-3",
  )}>
    <Button size="sm" variant="secondary" className={actionClass} onClick={onRename}><Pencil className="shrink-0" size={12} /> Rename</Button>
    <Button size="sm" variant="secondary" className={actionClass} onClick={onMove}><FolderInput className="shrink-0" size={12} /> Move</Button>
    {onCopy && <Button size="sm" variant="secondary" className="w-9 px-0" onClick={onCopy} aria-label="Copy folder" title="Copy folder"><Copy className="shrink-0" size={14} /></Button>}
    <Button size="sm" variant="danger" className={actionClass} onClick={onDelete}><Trash className="shrink-0" size={12} /> Delete</Button>
  </div>
}
