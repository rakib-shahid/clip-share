import { useCallback, useEffect, useState, type ReactNode } from "react"
import { ArrowLeft, Check, ChevronRight, Folder } from "lucide-react"
import { request, type Folder as FolderRecord, type FolderPage, type Session, type User } from "@/api"
import { Button } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"

type FolderPickerDialogProps = {
  title: string
  description?: string
  session: Session
  users: User[]
  initialID: number
  excludedFolderID?: number
  confirmLabel?: string
  confirmIcon?: ReactNode
  onClose: () => void
  onChoose: (folder: FolderRecord) => void | Promise<void>
}

export function FolderPickerDialog({ title, description, session, users, initialID, excludedFolderID, confirmLabel = "Choose this folder", confirmIcon = <Check size={17} />, onClose, onChoose }: FolderPickerDialogProps) {
  const roots = session.user.role === "admin" ? users : [session.user]
  const [page, setPage] = useState<FolderPage | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const browse = useCallback((id: number) => {
    setError("")
    request<FolderPage>(`/api/folders/${id}`).then(setPage).catch((reason) => setError(reason instanceof Error ? reason.message : "Could not load folders."))
  }, [])
  useEffect(() => { browse(initialID) }, [browse, initialID])
  const folders = page?.folders.filter((folder) => folder.id !== excludedFolderID) ?? []

  async function choose() {
    if (!page) return
    setBusy(true); setError("")
    try {
      await onChoose(page.folder)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not choose this destination.")
    } finally {
      setBusy(false)
    }
  }

  return <Modal title={title} onClose={onClose} nested>
    {description && <p className="mb-4 text-sm text-slate-400">{description}</p>}
    {session.user.role === "admin" && <div className="mb-4 flex gap-2 overflow-x-auto pb-1">{roots.map((user) => <Button key={user.id} type="button" size="sm" variant={page?.folder.ownerUserId === user.id ? "secondary" : "ghost"} onClick={() => browse(user.rootFolderId)}><Folder size={14} />{user.username}</Button>)}</div>}
    <div className="mb-4 flex flex-wrap items-center gap-1 text-xs text-slate-400">{page?.breadcrumbs.map((crumb, index) => <span key={crumb.id} className="inline-flex items-center gap-1">{index > 0 && <ChevronRight size={12} />}<button className="rounded px-1.5 py-1 hover:bg-white/[.06] hover:text-white" onClick={() => browse(crumb.id)}>{index === 0 ? "Library" : crumb.name}</button></span>)}</div>
    <div className="max-h-64 space-y-2 overflow-y-auto rounded-xl border border-white/[.08] bg-slate-950/60 p-2">{folders.map((folder) => <button key={folder.id} className="flex w-full items-center gap-3 rounded-lg p-3 text-left text-sm text-slate-200 hover:bg-white/[.06]" onClick={() => browse(folder.id)}><Folder size={18} className="text-sky-300" />{folder.name}<ChevronRight size={14} className="ml-auto text-slate-600" /></button>)}{page && folders.length === 0 && <p className="p-4 text-center text-sm text-slate-600">No folders inside this destination.</p>}</div>
    {error && <p className="mt-3 text-sm text-red-300" role="alert">{error}</p>}
    <div className="mt-5 flex justify-end gap-2"><Button variant="secondary" onClick={onClose}><ArrowLeft size={15} /> Back</Button><Button variant="success" onClick={choose} disabled={!page || busy}>{confirmIcon}{busy ? "Working…" : confirmLabel}</Button></div>
  </Modal>
}
