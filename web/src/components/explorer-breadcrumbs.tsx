import { ArrowLeft, Home } from "lucide-react"
import type { Folder } from "@/api"
import { Button } from "@/components/ui/button"
import { Breadcrumb, BreadcrumbEllipsis, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from "@/components/ui/breadcrumb"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

export function ExplorerBreadcrumbs({ crumbs, administrator, onNavigate, onBack }: {
  crumbs: Folder[]
  administrator: boolean
  onNavigate: (folderId: number) => void
  onBack?: () => void
}) {
  const long = crumbs.length > 4
  const omitted = long ? crumbs.slice(1, -2) : []
  const visible = long ? [crumbs[0], ...crumbs.slice(-2)] : crumbs

  return <div className="mb-5 flex min-w-0 flex-wrap items-center gap-2 text-slate-400">
    {onBack && <Button type="button" variant="ghost" size="sm" onClick={onBack}><ArrowLeft size={15} /> All libraries</Button>}
    <Breadcrumb aria-label="Breadcrumb" className="min-w-0 max-w-full">
      <BreadcrumbList className="max-w-full flex-nowrap gap-1 overflow-hidden sm:gap-1">
        {visible.map((crumb, index) => {
          const originalIndex = long && index > 0 ? crumbs.length - (visible.length - index) : index
          const current = originalIndex === crumbs.length - 1
          return <BreadcrumbSequence key={crumb.id} separator={index > 0 || (long && index === 1)}>
            {long && index === 1 && <OverflowCrumbs crumbs={omitted} onNavigate={onNavigate} />}
            <Crumb crumb={crumb} root={originalIndex === 0} administrator={administrator} current={current} onNavigate={onNavigate} />
          </BreadcrumbSequence>
        })}
      </BreadcrumbList>
    </Breadcrumb>
  </div>
}

function BreadcrumbSequence({ separator, children }: { separator: boolean; children: React.ReactNode }) {
  return <>{separator && <BreadcrumbSeparator />}{children}</>
}

function OverflowCrumbs({ crumbs, onNavigate }: { crumbs: Folder[]; onNavigate: (id: number) => void }) {
  return <>
    <BreadcrumbItem>
      <DropdownMenu>
        <DropdownMenuTrigger asChild><button type="button" className="rounded-md hover:bg-white/[.06] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300/70" aria-label="Show omitted folders"><BreadcrumbEllipsis /></button></DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {crumbs.map((crumb) => <DropdownMenuItem key={crumb.id} onSelect={() => onNavigate(crumb.id)}>{crumb.name}</DropdownMenuItem>)}
        </DropdownMenuContent>
      </DropdownMenu>
    </BreadcrumbItem>
    <BreadcrumbSeparator />
  </>
}

function Crumb({ crumb, root, administrator, current, onNavigate }: { crumb: Folder; root: boolean; administrator: boolean; current: boolean; onNavigate: (id: number) => void }) {
  const label = root ? (administrator ? crumb.ownerUsername : "Library") : crumb.name
  const content = <span className="inline-flex max-w-40 items-center gap-1.5 truncate sm:max-w-56">{root && <Home size={14} className="shrink-0" />}<span className="truncate">{label}</span></span>
  return <BreadcrumbItem className="min-w-0">
    <Tooltip>
      <TooltipTrigger asChild>
        {current
          ? <BreadcrumbPage className="min-w-0 truncate" tabIndex={0}>{content}</BreadcrumbPage>
          : <BreadcrumbLink asChild><button type="button" className="min-w-0 rounded-md px-2 py-1 hover:bg-white/[.06] hover:text-white" onClick={() => onNavigate(crumb.id)}>{content}</button></BreadcrumbLink>}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  </BreadcrumbItem>
}
