import { useCallback, useEffect, useRef, useState, type FormEvent } from "react"
import { ArrowLeft, Check, ChevronDown, ChevronRight, CircleX, CloudUpload, Copy, ExternalLink, Folder, FolderInput, FolderPlus, Home, RotateCcw, Save, Trash2, Video } from "lucide-react"
import { request, type Folder as FolderRecord, type FolderDeletionSummary, type FolderPage, type Session, type User } from "@/api"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { UploadDialog } from "@/components/upload-dialog"
import { EditorDialog } from "@/components/editor-dialog"
import { ClipPreview } from "@/components/clip-preview"
import { FolderPickerDialog } from "@/components/folder-picker-dialog"
import { ItemManagementBar } from "@/components/item-management-bar"
import { Modal } from "@/components/ui/modal"
import { copyText } from "@/lib/clipboard"

type ExplorerProps = {
  session: Session
  root: User
  users: User[]
  refreshToken?: number
  onBack?: () => void
  requestedFolder?: { folderID: number; token: number } | null
}

export function LibraryExplorer({ session, root, users, refreshToken = 0, onBack, requestedFolder }: ExplorerProps) {
  const [folderId, setFolderId] = useState(root.rootFolderId)
  const [page, setPage] = useState<FolderPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [renameFolder, setRenameFolder] = useState<FolderRecord | null>(null)
  const [moveFolder, setMoveFolder] = useState<FolderRecord | null>(null)
  const [copyFolder, setCopyFolder] = useState<FolderRecord | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [editorSessionID, setEditorSessionID] = useState<string | null>(null)
  const [editorLocalFile, setEditorLocalFile] = useState<File | null>(null)
  const lastRefreshToken = useRef(refreshToken)

  const load = useCallback((id: number, replace = false) => {
    setLoading(true); setError("")
    request<FolderPage>(`/api/folders/${id}`)
      .then((result) => { setPage(result); setFolderId(result.folder.id); const path = `/app/folders/${result.breadcrumbs.slice(1).map((crumb) => encodeURIComponent(crumb.name)).join("/") || "root"}`; (replace ? window.history.replaceState : window.history.pushState).call(window.history, { folderID: result.folder.id }, "", path) })
      .catch((reason) => setError(reason instanceof Error ? reason.message : "Could not load this folder."))
      .finally(() => setLoading(false))
  }, [])

  async function loadMore() {
    if (!page?.nextCursor) return
    setLoadingMore(true); setError("")
    try {
      const next = await request<FolderPage>(`/api/folders/${page.folder.id}?cursor=${encodeURIComponent(page.nextCursor)}`)
      setPage({ ...next, folders: [...page.folders, ...next.folders], clips: [...page.clips, ...next.clips] })
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Could not load more items.") }
    finally { setLoadingMore(false) }
  }

  useEffect(() => { const onPop = (event: PopStateEvent) => { const id = Number(event.state?.folderID); if (Number.isInteger(id) && id > 0) load(id, true) }; window.addEventListener("popstate", onPop); const initialID = requestedFolder?.folderID ?? root.rootFolderId; setFolderId(initialID); load(initialID, true); return () => window.removeEventListener("popstate", onPop) }, [load, requestedFolder?.folderID, requestedFolder?.token, root.rootFolderId])

  useEffect(() => {
    if (lastRefreshToken.current === refreshToken) return
    lastRefreshToken.current = refreshToken
    load(folderId, true)
  }, [folderId, load, refreshToken])

  useEffect(() => {
    if (!page?.clips.some((clip) => ["queued", "processing", "validating"].includes(clip.state))) return
    const timer = window.setInterval(() => {
      request<FolderPage>(`/api/folders/${folderId}`).then(setPage).catch(() => undefined)
    }, 2000)
    return () => window.clearInterval(timer)
  }, [folderId, page?.clips])

  const title = page?.folder.isRoot ? (session.user.role === "admin" ? `${page.folder.ownerUsername}'s library` : "Your library") : page?.folder.name

  return <>
    <div className="mb-7">
      <div className="mb-5 flex flex-wrap items-center gap-1 text-sm text-slate-400">
        {onBack && <button className="mr-2 inline-flex items-center gap-1 rounded-md px-2 py-1 hover:bg-white/[.06] hover:text-white" onClick={onBack}><ArrowLeft size={15} /> All libraries</button>}
        {page?.breadcrumbs.map((crumb, index) => <span key={crumb.id} className="inline-flex items-center gap-1">
          {index > 0 && <ChevronRight size={14} className="text-slate-600" />}
          <button className="rounded-md px-2 py-1 hover:bg-white/[.06] hover:text-white" onClick={() => load(crumb.id)}>{index === 0 ? <span className="inline-flex items-center gap-1.5"><Home size={14} />{session.user.role === "admin" ? crumb.ownerUsername : "Library"}</span> : crumb.name}</button>
        </span>)}
      </div>
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>{session.user.role === "admin" && root.id !== session.user.id && <div className="mb-3 inline-flex items-center gap-2 rounded-lg border border-sky-400/20 bg-sky-400/10 px-3 py-2 text-xs font-semibold text-sky-200" role="status"><span aria-hidden="true">●</span> Administrator view · {root.username}'s library</div>}<p className="eyebrow">{session.user.role === "admin" && root.id !== session.user.id ? `Managing ${root.username}` : "Folder explorer"}</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">{title ?? "Library"}</h1><p className="mt-2 text-sm text-slate-400" aria-live="polite">{page ? `${page.folders.length} folders · ${page.clips.length} clips` : "Loading contents…"}</p></div>
        <div className="flex gap-2"><Button variant="secondary" onClick={() => setCreateOpen(true)} disabled={!page}><FolderPlus size={17} /> New folder</Button><Button variant="success" onClick={() => setUploadOpen(true)} disabled={!page}><CloudUpload size={17} /> Upload clip</Button></div>
      </div>
    </div>

    {error && <p className="mb-5 rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">{error}</p>}
    {loading ? <ExplorerLoading /> : page && <>
      {page.folders.length === 0 && page.clips.length === 0 ? <EmptyFolder onCreate={() => setCreateOpen(true)} /> :
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {page.folders.map((folder) => <FolderCardV2 key={folder.id} folder={folder} session={session} onOpen={() => load(folder.id)} onRename={() => setRenameFolder(folder)} onMove={() => setMoveFolder(folder)} onCopy={session.user.role === "admin" ? () => setCopyFolder(folder) : undefined} onDeleted={() => load(folderId)} />)}
          {page.clips.map((clip) => <div className="flex h-full flex-col" key={clip.id}><ClipCard clip={clip} session={session} /><ClipManagementV2 clip={clip} currentFolder={page.folder} session={session} users={users} onChanged={() => load(folderId)} /></div>)}
        </div>}{page.nextCursor && <div className="mt-7 flex justify-center"><Button variant="secondary" onClick={loadMore} disabled={loadingMore}><ChevronDown size={16} />{loadingMore ? "Loading…" : "Load more"}</Button></div>}
    </>}

    {createOpen && page && <NameDialog title="Create folder" action="Create folder" onClose={() => setCreateOpen(false)} onSubmit={async (name) => { await request("/api/folders", { method: "POST", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ parentFolderId: page.folder.id, name }) }); setCreateOpen(false); load(folderId) }} />}
    {renameFolder && <NameDialog title="Rename folder" action="Save name" initialValue={renameFolder.name} onClose={() => setRenameFolder(null)} onSubmit={async (name) => { await request(`/api/folders/${renameFolder.id}`, { method: "PATCH", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ name }) }); setRenameFolder(null); load(folderId) }} />}
    {moveFolder && <MoveDialog source={moveFolder} session={session} users={users} onClose={() => setMoveFolder(null)} onMoved={() => { setMoveFolder(null); load(folderId) }} />}
    {copyFolder && page && <CopyFolderDialog source={copyFolder} initialFolderID={page.folder.id} session={session} users={users} onClose={() => setCopyFolder(null)} onCopied={() => { setCopyFolder(null); load(folderId) }} />}
    {uploadOpen && page && <UploadDialog session={session} currentFolder={page.folder} users={users} onClose={() => setUploadOpen(false)} onQueued={() => {setUploadOpen(false);load(folderId)}} onUploaded={(sessionID,file) => { setUploadOpen(false);setEditorLocalFile(file);setEditorSessionID(sessionID) }} />}
    {editorSessionID && <EditorDialog session={session} sessionID={editorSessionID} localFile={editorLocalFile} users={users} onClose={() => {setEditorSessionID(null);setEditorLocalFile(null)}} onFinalized={() => { setEditorSessionID(null);setEditorLocalFile(null); load(folderId) }} />}
  </>
}

function ClipCard({ clip, session }: { clip: import("@/api").ClipSummary; session: Session }) {
  const [copied, setCopied] = useState(false)
  const shareURL = `${session.publicBaseURL}/c/${clip.publicId}`
  async function copyLink() { await copyText(shareURL); setCopied(true); window.setTimeout(() => setCopied(false), 1500) }
  return <Card className="overflow-hidden">
    {clip.state === "ready" ? <ClipPreview title={clip.title} posterSrc={`/m/${clip.publicId}/poster`} videoSrc={`/m/${clip.publicId}/video`} /> : <div className="grid aspect-video place-items-center bg-slate-950 text-slate-700"><Video size={32} /></div>}
    <div className="p-4"><h2 className="truncate font-medium text-white">{clip.title}</h2><div className="mt-2 flex items-center justify-between text-xs" aria-live="polite"><span className={clip.state === "ready" ? "text-emerald-300" : clip.state === "failed" ? "text-red-300" : "capitalize text-sky-300"}>{clip.state === "queued" ? "Queued for processing" : clip.state === "processing" ? "Processing video" : clip.state}</span>{clip.progress !== null && !["ready", "failed"].includes(clip.state) && <span className="text-slate-500">{clip.progress}%</span>}</div>{clip.progress !== null && !["ready", "failed"].includes(clip.state) && <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-800" role="progressbar" aria-label={`${clip.state} progress`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={clip.progress}><div className="h-full rounded-full bg-sky-400 transition-[width]" style={{ width: `${clip.progress}%` }} /></div>}{clip.errorMessage && <p className="mt-2 text-xs leading-5 text-red-300" role="alert">{clip.errorMessage}</p>}{clip.sizeBytes !== null && <p className="mt-2 text-xs text-slate-600">{formatBytes(clip.sizeBytes)}</p>}{clip.state === "ready" && <div className="mt-3 flex gap-2"><Button size="sm" variant="secondary" onClick={copyLink}>{copied ? <Check size={14} /> : <Copy size={14} />}{copied ? "Copied" : "Copy link"}</Button><Button size="sm" variant="ghost" asChild><a href={`/c/${clip.publicId}`} target="_blank" rel="noreferrer"><ExternalLink size={14} /> Public page</a></Button></div>}</div>
  </Card>
}

function ClipManagementV2({ clip, currentFolder, session, users, onChanged }: { clip: import("@/api").ClipSummary; currentFolder: FolderRecord; session: Session; users: User[]; onChanged: () => void }) {
  const [dialog, setDialog] = useState<"rename" | "move" | "delete" | "cancel-job" | "dismiss-failure" | null>(null)
  const [jobBusy, setJobBusy] = useState(false)
  const [jobError, setJobError] = useState("")
  const jobID = clip.jobId

  async function submit(title?: string) {
    if (dialog === "rename") await request(`/api/clips/${clip.id}`, { method: "PATCH", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ title }) })
    if (dialog === "delete") await request(`/api/clips/${clip.id}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } })
    setDialog(null); onChanged()
  }

  async function submitJobAction() {
    if (!jobID) return
    setJobBusy(true); setJobError("")
    try {
      const path = dialog === "dismiss-failure" ? `/api/jobs/${jobID}/failure` : `/api/jobs/${jobID}`
      await request(path, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } })
      setDialog(null); onChanged()
    } catch (reason) { setJobError(reason instanceof Error ? reason.message : "Could not update this upload.") }
    finally { setJobBusy(false) }
  }

  if (clip.state === "failed" && jobID) return <>
    <JobActionBar label="Dismiss failed upload" onClick={() => { setJobError(""); setDialog("dismiss-failure") }} />
    {dialog === "dismiss-failure" && <Modal title="Dismiss failed upload?" onClose={() => setDialog(null)} dismissible={!jobBusy}><p className="text-sm leading-6 text-slate-400">The source and partial media are already gone. Dismissing removes this private failure notice permanently; it will not create a recycle-bin item.</p>{jobError && <p className="mt-3 text-sm text-red-300" role="alert">{jobError}</p>}<div className="mt-5 flex justify-end gap-2"><Button variant="secondary" disabled={jobBusy} onClick={() => setDialog(null)}><ArrowLeft size={15} /> Keep notice</Button><Button variant="danger" disabled={jobBusy} onClick={submitJobAction}><CircleX size={15} />{jobBusy ? "Dismissing…" : "Dismiss"}</Button></div></Modal>}
  </>

  if (["queued", "validating", "processing"].includes(clip.state) && jobID) return <>
    <JobActionBar label="Cancel processing" onClick={() => { setJobError(""); setDialog("cancel-job") }} />
    {dialog === "cancel-job" && <Modal title="Cancel this upload?" onClose={() => setDialog(null)} dismissible={!jobBusy}><p className="text-sm leading-6 text-slate-400">Processing will stop and all source and partial media will be removed. The clip will not enter the recycle bin and cannot be recovered.</p>{jobError && <p className="mt-3 text-sm text-red-300" role="alert">{jobError}</p>}<div className="mt-5 flex justify-end gap-2"><Button variant="secondary" disabled={jobBusy} onClick={() => setDialog(null)}><ArrowLeft size={15} /> Keep processing</Button><Button variant="danger" disabled={jobBusy} onClick={submitJobAction}><CircleX size={15} />{jobBusy ? "Cancelling…" : "Cancel upload"}</Button></div></Modal>}
  </>

  if (clip.state !== "ready") return null
  return <>
    <ItemManagementBar onRename={() => setDialog("rename")} onMove={() => setDialog("move")} onDelete={() => setDialog("delete")} />
    {dialog === "rename" && <NameDialog title="Rename clip" fieldLabel="Clip title" action="Save name" initialValue={clip.title} onClose={() => setDialog(null)} onSubmit={submit} />}
    {dialog === "move" && <MoveDialog source={{ id: clip.id, name: clip.title, ownerUserId: currentFolder.ownerUserId }} sourceKind="clip" initialFolderID={currentFolder.id} session={session} users={users} onClose={() => setDialog(null)} onMoved={() => { setDialog(null); onChanged() }} />}
    {dialog === "delete" && <Modal title="Move clip to recycle bin" onClose={() => setDialog(null)}><p className="text-sm text-slate-400">This clip will be recoverable from the recycle bin.</p><div className="mt-5 flex justify-end gap-2"><Button variant="secondary" onClick={() => setDialog(null)}><ArrowLeft size={15} /> Back</Button><Button variant="danger" onClick={() => submit()}><Trash2 size={15} /> Move to recycle bin</Button></div></Modal>}
  </>
}

function JobActionBar({ label, onClick }: { label: string; onClick: () => void }) {
  return <div className="-mt-3 flex rounded-b-2xl border-x border-b border-white/[.06] bg-slate-950 px-3 py-2"><Button className="w-full" size="sm" variant="danger" onClick={onClick}><CircleX size={14} /> {label}</Button></div>
}

function FolderCardV2({ folder, session, onOpen, onRename, onMove, onCopy, onDeleted }: { folder: FolderRecord; session: Session; onOpen: () => void; onRename: () => void; onMove: () => void; onCopy?: () => void; onDeleted: () => void }) {
  const [confirming, setConfirming] = useState(false)
  return <div className="flex h-full flex-col">
    <Card className="group flex flex-1 flex-col overflow-hidden p-4 transition hover:border-sky-400/30 hover:bg-slate-900">
      <button className="block w-full flex-1 rounded-lg text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400" onClick={onOpen}>
        <div className="mb-7 grid size-12 place-items-center rounded-xl border border-sky-400/15 bg-sky-400/10 text-sky-300"><Folder /></div>
        <h2 className="truncate font-semibold text-white">{folder.name}</h2>
        <p className="mt-1 text-xs text-slate-500">{folder.folderCount} folders · {folder.clipCount} clips</p>
      </button>
    </Card>
    <ItemManagementBar onRename={onRename} onMove={onMove} onCopy={onCopy} onDelete={() => setConfirming(true)} />
    {confirming && <FolderDeleteDialog folder={folder} session={session} onClose={() => setConfirming(false)} onDeleted={onDeleted} />}
  </div>
}

function FolderDeleteDialog({ folder, session, onClose, onDeleted }: { folder: FolderRecord; session: Session; onClose: () => void; onDeleted: () => void }) {
  const [summary, setSummary] = useState<FolderDeletionSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState("")
  const [reloadToken, setReloadToken] = useState(0)
  const [acknowledged, setAcknowledged] = useState(false)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState("")

  useEffect(() => {
    let active = true
    setLoading(true)
    setLoadError("")
    setSummary(null)
    setAcknowledged(false)
    request<FolderDeletionSummary>(`/api/folders/${folder.id}/deletion-summary`)
      .then((result) => { if (active) setSummary(result) })
      .catch((reason) => { if (active) setLoadError(reason instanceof Error ? reason.message : "Could not calculate this folder's contents.") })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [folder.id, reloadToken])

  async function remove() {
    setBusy(true); setActionError("")
    try {
      await request(`/api/folders/${folder.id}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } })
      onClose(); onDeleted()
    } catch (reason) { setActionError(reason instanceof Error ? reason.message : "Could not delete this folder.") }
    finally { setBusy(false) }
  }

  return <Modal title={`Delete “${folder.name}”?`} onClose={onClose} dismissible={!busy}>
    {loading && <div className="rounded-xl border border-white/[.08] bg-slate-950 p-4 text-sm text-slate-400" role="status">Calculating everything inside this folder…</div>}
    {!loading && loadError && <div className="rounded-xl border border-red-400/20 bg-red-400/10 p-4"><p className="text-sm text-red-200" role="alert">{loadError}</p><Button className="mt-3" size="sm" variant="secondary" onClick={() => setReloadToken((value) => value + 1)}><RotateCcw size={15} /> Retry summary</Button></div>}
    {summary && <>
      <p className="text-sm leading-6 text-slate-400">The complete folder subtree will move to the recycle bin. Review the recursive totals before continuing.</p>
      <div className="mt-4 grid grid-cols-3 gap-2">
        <DeletionStat label="Folders" value={summary.folderCount.toLocaleString()} />
        <DeletionStat label="Clips" value={summary.clipCount.toLocaleString()} />
        <DeletionStat label="Stored size" value={formatBytes(summary.storedBytes)} />
      </div>
    </>}
    <label className="mt-4 flex items-start gap-3 rounded-lg border border-white/[.08] bg-slate-950 p-3 text-sm text-slate-300"><input className="mt-1 size-4 accent-sky-400" type="checkbox" checked={acknowledged} disabled={!summary || loading} onChange={(event) => setAcknowledged(event.target.checked)} /> I understand all {summary?.totalItems.toLocaleString() ?? "calculated"} items will be removed from the active library.</label>
    {actionError && <p className="mt-3 text-sm text-red-300" role="alert">{actionError}</p>}
    <div className="mt-5 flex justify-end gap-2"><Button variant="secondary" disabled={busy} onClick={onClose}><ArrowLeft size={15} /> Back</Button><Button variant="danger" disabled={!summary || loading || !acknowledged || busy} onClick={remove}><Trash2 size={15} />{busy ? "Moving…" : "Move to recycle bin"}</Button></div>
  </Modal>
}

function DeletionStat({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0 rounded-xl border border-white/[.08] bg-white/[.03] px-3 py-3"><p className="truncate text-xs text-slate-500">{label}</p><p className="mt-1 truncate text-sm font-semibold text-white" title={value}>{value}</p></div>
}

function ExplorerLoading() { return <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4" role="status" aria-label="Loading folder contents">{[1, 2, 3, 4].map((item) => <div key={item} className="h-44 animate-pulse rounded-2xl border border-white/[.06] bg-white/[.03]" />)}</div> }
function EmptyFolder({ onCreate }: { onCreate: () => void }) { return <div className="grid min-h-72 place-items-center rounded-2xl border border-dashed border-white/10 bg-white/[.02] px-5 text-center"><div><div className="mx-auto grid size-14 place-items-center rounded-2xl bg-sky-400/10 text-sky-300"><Folder size={27} /></div><h2 className="mt-4 font-semibold text-white">This folder is empty</h2><p className="mt-2 text-sm text-slate-500">Create a folder or use Upload clip to add your first video.</p><Button className="mt-5" variant="secondary" onClick={onCreate}><FolderPlus size={17} /> Create folder</Button></div></div> }

function NameDialog({ title, fieldLabel = "Folder name", action, initialValue = "", onClose, onSubmit }: { title: string; fieldLabel?: string; action: string; initialValue?: string; onClose: () => void; onSubmit: (name: string) => Promise<void> }) {
  const [name, setName] = useState(initialValue); const [busy, setBusy] = useState(false); const [error, setError] = useState("")
  async function submit(event: FormEvent) { event.preventDefault(); setBusy(true); setError(""); try { await onSubmit(name) } catch (reason) { setError(reason instanceof Error ? reason.message : "Something went wrong.") } finally { setBusy(false) } }
  return <Modal title={title} onClose={onClose} dismissible={!busy}><form className="space-y-4" onSubmit={submit}><label className="block"><span className="mb-2 block text-sm font-medium text-slate-300">{fieldLabel}</span><Input value={name} onChange={(event) => setName(event.target.value)} maxLength={fieldLabel === "Clip title" ? 200 : 100} required autoFocus /></label>{error && <p className="text-sm text-red-300" role="alert">{error}</p>}<div className="flex justify-end gap-2"><Button type="button" variant="secondary" disabled={busy} onClick={onClose}><ArrowLeft size={15} /> Back</Button><Button variant="success" disabled={busy || !name.trim()}><Save size={15} />{busy ? "Saving…" : action}</Button></div></form></Modal>
}

function MoveDialog({ source, sourceKind = "folder", initialFolderID, session, users, onClose, onMoved }: { source: Pick<FolderRecord, "id" | "name" | "ownerUserId">; sourceKind?: "folder" | "clip"; initialFolderID?: number; session: Session; users: User[]; onClose: () => void; onMoved: () => void }) {
  const roots = session.user.role === "admin" ? users : [session.user]
  const initialRoot = roots.find((user) => user.id === source.ownerUserId) ?? roots[0]
  if (!initialRoot) return null
  return <FolderPickerDialog title={`Move “${source.name}”`} description="Choose a destination. Moving into another user's library is available only to the administrator." session={session} users={users} initialID={initialFolderID ?? initialRoot.rootFolderId} excludedFolderID={sourceKind === "folder" ? source.id : undefined} confirmLabel="Move here" confirmIcon={<FolderInput size={17} />} onClose={onClose} onChoose={async (folder) => {
    const path = sourceKind === "clip" ? `/api/clips/${source.id}/move` : `/api/folders/${source.id}/move`
    await request(path, { method: "POST", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ destinationFolderId: folder.id }) })
    onMoved()
  }} />
}

function CopyFolderDialog({ source, initialFolderID, session, users, onClose, onCopied }: { source: FolderRecord; initialFolderID: number; session: Session; users: User[]; onClose: () => void; onCopied: () => void }) {
  return <FolderPickerDialog title={`Copy “${source.name}”`} description="Choose where to place an independent copy. Every folder, clip, public link, and stored media file receives a new identity." session={session} users={users} initialID={initialFolderID} excludedFolderID={source.id} confirmLabel="Copy here" confirmIcon={<Copy size={17} />} onClose={onClose} onChoose={async (folder) => {
    await request(`/api/folders/${source.id}/copy`, { method: "POST", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ destinationFolderId: folder.id }) })
    onCopied()
  }} />
}
function formatBytes(bytes: number) { if (bytes === 0) return "0 B"; return bytes >= 1_000_000 ? `${(bytes / 1_000_000).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1_000))} KB` }
