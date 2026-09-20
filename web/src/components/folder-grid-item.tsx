import { Folder } from "lucide-react"
import { ItemContextSurface, ItemDropdownActions } from "@/components/item-action-menus"
import { Item, ItemActions, ItemContent, ItemDescription, ItemMedia, ItemTitle } from "@/components/ui/item"
import type { ExplorerFolderItem } from "@/lib/explorer-item"
import type { ItemAction } from "@/lib/item-actions"
import type { ReactNode } from "react"

export function FolderGridItem({ item, onOpen, actions = [], overflow }: { item: ExplorerFolderItem; onOpen: () => void; actions?: ItemAction[]; overflow?: ReactNode }) {
  return <ItemContextSurface actions={actions}><Item variant="outline" className="h-full flex-col items-stretch rounded-2xl bg-slate-950/40 p-0 transition-colors hover:border-sky-400/30 hover:bg-slate-900">
    <button type="button" data-slot="item-primary" className="flex min-h-36 flex-1 items-start gap-4 rounded-2xl p-4 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-sky-300/70" aria-label={`Open folder ${item.name}`} onClick={onOpen}>
      <ItemMedia variant="icon"><Folder aria-hidden="true" /></ItemMedia>
      <ItemContent className="min-w-0 pt-0.5 text-left">
        <ItemTitle><span className="block max-w-full truncate" title={item.name}>{item.name}</span></ItemTitle>
        <ItemDescription data-slot="grid-secondary">{countLabel(item.folderCount, "folder")} · {countLabel(item.clipCount, "clip")}</ItemDescription>
        <ItemDescription data-slot="mobile-secondary" className="hidden">Folder · {countLabel(item.folderCount, "folder")}, {countLabel(item.clipCount, "clip")}</ItemDescription>
      </ItemContent>
    </button>
    <span data-slot="list-state" className="explorer-list-cell hidden text-sm text-slate-300">Folder</span>
    <span data-slot="list-size" className="explorer-list-cell hidden text-sm text-slate-400">{countLabel(item.folderCount, "folder")} · {countLabel(item.clipCount, "clip")}</span>
    <span data-slot="list-date" className="explorer-list-cell hidden text-sm text-slate-600" aria-label="Not applicable">—</span>
    {(actions.length > 0 || overflow) && <ItemActions className="w-full px-4 pb-4" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>{overflow ?? <ItemDropdownActions actions={actions} label={`Actions for ${item.name}`} />}</ItemActions>}
  </Item></ItemContextSurface>
}

function countLabel(count: number, singular: string) {
  return `${count} ${count === 1 ? singular : `${singular}s`}`
}
