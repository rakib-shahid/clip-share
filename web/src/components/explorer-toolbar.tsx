import { ArrowUpDown, CloudUpload, FolderPlus, LayoutGrid, List, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuLabel, DropdownMenuRadioGroup,
  DropdownMenuRadioItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { ExplorerSort, ExplorerView } from "@/lib/explorer-preferences"

const sortLabels: Record<ExplorerSort, string> = {
  latest: "Latest upload",
  oldest: "Oldest upload",
  name_asc: "Name A–Z",
  name_desc: "Name Z–A",
  size_desc: "Largest size",
  size_asc: "Smallest size",
  state: "Processing state",
}

export function ExplorerItemSummary({ folderCount, clipCount }: { folderCount: number; clipCount: number }) {
  return <p className="mt-2 text-sm text-slate-400" aria-live="polite">
    {folderCount} {folderCount === 1 ? "folder" : "folders"} · {clipCount} {clipCount === 1 ? "clip" : "clips"}
  </p>
}

export function ExplorerToolbar({
  view, sort, disabled, busy, onViewChange, onSortChange, onCreateFolder, onUploadClip, onRefresh,
}: {
  view: ExplorerView
  sort: ExplorerSort
  disabled: boolean
  busy: boolean
  onViewChange: (view: ExplorerView) => void
  onSortChange: (sort: ExplorerSort) => void
  onCreateFolder: () => void
  onUploadClip: () => void
  onRefresh?: () => void
}) {
  return <div className="flex max-w-full flex-wrap items-center gap-2" aria-label="Folder explorer controls" aria-busy={busy}>
    <ButtonGroup aria-label="Create items">
      <Button variant="secondary" onClick={onCreateFolder} disabled={disabled}><FolderPlus size={17} /> New folder</Button>
      <Button variant="success" className="-ml-px border-l border-l-emerald-300/60" data-group-left-border onClick={onUploadClip} disabled={disabled}><CloudUpload size={17} /> Upload clip</Button>
    </ButtonGroup>

    <ToggleGroup type="single" value={view} onValueChange={(next) => next && onViewChange(next as ExplorerView)} variant="outline" aria-label="Explorer view">
      <ToggleGroupItem value="grid" aria-label="Grid view"><LayoutGrid /><span className="sr-only sm:not-sr-only">Grid</span></ToggleGroupItem>
      <ToggleGroupItem value="list" aria-label="List view"><List /><span className="sr-only sm:not-sr-only">List</span></ToggleGroupItem>
    </ToggleGroup>

    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" disabled={disabled || busy} aria-label={`Sort: ${sortLabels[sort]}`}><ArrowUpDown size={16} /> {sortLabels[sort]}</Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-52">
        <DropdownMenuLabel>Sort folder contents</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={sort} onValueChange={(next) => next !== sort && onSortChange(next as ExplorerSort)}>
          {(Object.entries(sortLabels) as [ExplorerSort, string][]).map(([value, label]) => <DropdownMenuRadioItem key={value} value={value}>{label}</DropdownMenuRadioItem>)}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
    {onRefresh && <Button variant="ghost" size="sm" onClick={onRefresh} disabled={busy} aria-label="Refresh folder"><RefreshCw size={16} /> Refresh</Button>}
  </div>
}
