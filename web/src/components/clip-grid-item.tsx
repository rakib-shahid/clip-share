import { CircleAlert, Copy, FileVideo, Video } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Item, ItemActions, ItemContent, ItemDescription, ItemTitle } from "@/components/ui/item"
import { Progress } from "@/components/ui/progress"
import { Spinner } from "@/components/ui/spinner"
import { RelativeUploadTime } from "@/components/relative-upload-time"
import { ItemContextSurface, ItemDropdownActions } from "@/components/item-action-menus"
import type { ExplorerClipItem } from "@/lib/explorer-item"
import type { ItemAction } from "@/lib/item-actions"
import type { ReactNode } from "react"

export function ClipGridItem({ item, onPreview, actions = [], overflow }: { item: ExplorerClipItem; onPreview?: () => void; actions?: ItemAction[]; overflow?: ReactNode }) {
  const copyAction = item.state === "ready" ? actions.find((action) => action.id === "copy-link") : undefined
  const media = (
    <div data-slot="clip-media" className="relative aspect-video w-full overflow-hidden rounded-t-2xl bg-slate-950">
      {item.previewEligible
        ? <img src={`/m/${item.publicId}/poster`} alt="" loading="lazy" decoding="async" className="size-full object-cover" />
        : <div className="grid size-full place-items-center text-slate-500" aria-hidden="true">{item.state === "failed" ? <CircleAlert size={34} /> : item.state === "processing" ? <Video size={34} /> : <FileVideo size={34} />}</div>}
    </div>
  )
  const primaryDetails = <div data-slot="item-primary-details" className="min-w-0 p-4 pb-0 text-left">
    <ItemTitle className="min-w-0"><span className="truncate" title={item.name}>{item.name}</span></ItemTitle>
    <ItemDescription data-slot="mobile-secondary" className="hidden">{item.status.label}{item.sizeBytes !== null ? ` · ${formatBytes(item.sizeBytes)}` : ""}</ItemDescription>
  </div>
  const primary = item.previewEligible && onPreview
    ? <button type="button" data-slot="item-primary" className="block w-full shrink-0 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-sky-300/70" onClick={onPreview} aria-label={`Preview ${item.name}`}>{media}{primaryDetails}</button>
    : <div data-slot="item-primary" className="w-full shrink-0">{media}{primaryDetails}</div>
  const content = (
    <ItemContent data-slot="grid-details" className="min-h-20 w-full p-4 text-left">
      <div className="flex min-w-0 items-start justify-between gap-2">
        {copyAction
          ? <Button type="button" size="sm" variant="secondary" className="h-8 shrink-0 px-2.5" aria-label={`Copy link for ${item.name}`} onClick={() => void copyAction.run()}><Copy size={14} /> Copy link</Button>
          : <Badge variant={item.state === "failed" ? "destructive" : item.state === "ready" ? "default" : "secondary"}>{item.status.label}</Badge>}
      </div>
      {item.state === "failed" && item.errorMessage && <ItemDescription className="line-clamp-2 text-red-300">{item.errorMessage}</ItemDescription>}
      {item.progress.kind === "determinate" && <Progress className="mt-2" value={item.progress.value} aria-label={`${item.status.label} progress`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={item.progress.value} />}
      {item.progress.kind === "indeterminate" && <p className="mt-2 inline-flex items-center gap-2 text-xs text-sky-300" role="status"><Spinner aria-label={item.status.label} /> {item.status.label}</p>}
      <div className="mt-auto flex flex-wrap items-center gap-2 pt-2 text-xs text-slate-500">{item.sizeBytes !== null && <span>{formatBytes(item.sizeBytes)}</span>}<RelativeUploadTime createdAt={item.createdAt} /></div>
    </ItemContent>
  )
  return <ItemContextSurface actions={actions}><Item variant="outline" className="clip-card isolate h-full flex-col items-stretch overflow-hidden rounded-2xl bg-slate-950/40 p-0">
    {primary}
    {content}
    <div data-slot="list-state" className="explorer-list-cell hidden min-w-0 text-sm text-slate-300">
      {copyAction ? <Button type="button" size="sm" variant="secondary" className="h-8 px-2.5" aria-label={`Copy link for ${item.name}`} onClick={() => void copyAction.run()}><Copy size={14} /> Copy link</Button> : <><Badge variant={item.state === "failed" ? "destructive" : "secondary"}>{item.status.label}</Badge>{item.progress.kind === "determinate" && <Progress className="mt-1" value={item.progress.value} aria-label={`${item.status.label} progress`} />}</>}
    </div>
    <span data-slot="list-size" className="explorer-list-cell hidden text-sm text-slate-400">{item.sizeBytes === null ? "—" : formatBytes(item.sizeBytes)}</span>
    <span data-slot="list-date" className="explorer-list-cell hidden text-sm text-slate-400"><RelativeUploadTime createdAt={item.createdAt} /></span>
    {(actions.length > 0 || overflow) && <ItemActions className="w-full rounded-b-2xl px-4 pb-4" onClick={(event) => event.stopPropagation()}>{overflow ?? <ItemDropdownActions actions={actions} label={`Actions for ${item.name}`} />}</ItemActions>}
  </Item></ItemContextSurface>
}

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B"
  return bytes >= 1_000_000 ? `${(bytes / 1_000_000).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1_000))} KB`
}
