import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react"
import { ArrowLeft, ChevronDown, CircleX, Copy, Folder, FolderInput, FolderPlus, RotateCcw, Save, Trash2 } from "lucide-react"
import { request, type Folder as FolderRecord, type FolderDeletionSummary, type FolderPage, type JobStatus, type Session, type User } from "@/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"
import { UploadDialog } from "@/components/upload-dialog"
import { EditorDialog } from "@/components/editor-dialog"
import { VideoPreviewDialog } from "@/components/clip-preview"
import { FolderPickerDialog } from "@/components/folder-picker-dialog"
import { ItemDropdownActions } from "@/components/item-action-menus"
import { ExplorerItemSummary, ExplorerToolbar } from "@/components/explorer-toolbar"
import { createPreferenceStore } from "@/lib/explorer-preferences"
import { folderPageURL } from "@/lib/folder-page-url"
import { ExplorerBreadcrumbs } from "@/components/explorer-breadcrumbs"
import { FolderGridItem } from "@/components/folder-grid-item"
import { folderExplorerItem } from "@/lib/explorer-item"
import { explorerGridClasses, explorerListClasses } from "@/lib/explorer-layout"
import { ClipGridItem } from "@/components/clip-grid-item"
import { clipExplorerItem } from "@/lib/explorer-item"
import { itemActions } from "@/lib/item-actions"
import { copyText } from "@/lib/clipboard"
import { toast } from "sonner"
import { AnimatePresence, m, useReducedMotion } from "motion/react"
import { Skeleton } from "@/components/ui/skeleton"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Spinner } from "@/components/ui/spinner"
import { collectionVariants, initialItemVariants, layoutTransition, type NavigationDirection } from "@/lib/motion"
import { activeJobIDChunks, appendFolderPage, jobStatusesChangeSortKey, mergeJobStatuses } from "@/lib/explorer-coordination"
import { openInNewTab } from "@/lib/new-tab"
import { ViewportFileDrop } from "@/components/viewport-file-drop"
import { waitForMinimumPending } from "@/lib/minimum-pending"

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
  const [appendError, setAppendError] = useState("")
  const appendGeneration = useRef(0)
  const appendAbort = useRef<AbortController | null>(null)
  const loadingMoreRef = useRef(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [droppedFile, setDroppedFile] = useState<File | null>(null)
  const [editorSessionID, setEditorSessionID] = useState<string | null>(null)
  const [editorLocalFile, setEditorLocalFile] = useState<File | null>(null)
  const lastRefreshToken = useRef(refreshToken)
  const requestGeneration = useRef(0)
  const pageAbort = useRef<AbortController | null>(null)
  const committedLocation = useRef({ url: window.location.href, state: window.history.state })
  const pendingFocus = useRef<{ key: string; index: number } | null>(null)
  const preferenceStore = useMemo(() => createPreferenceStore(session.user.id), [session.user.id])
  const [preferences, setPreferences] = useState(() => preferenceStore.get())
  const [navigationDirection, setNavigationDirection] = useState<NavigationDirection>("neutral")
  const [staggerInitialItems, setStaggerInitialItems] = useState(true)
  const hasCommittedPage = useRef(false)
  const reducedMotion = useReducedMotion()
  const reducedMotionRef = useRef(Boolean(reducedMotion))
  reducedMotionRef.current = Boolean(reducedMotion)
  const sortRef = useRef(preferences.explorer.sort)
  sortRef.current = preferences.explorer.sort

  const openUploadPicker = useCallback(() => {
    setDroppedFile(null)
    setUploadOpen(true)
  }, [])
  const openDroppedFile = useCallback((file: File) => {
    setDroppedFile(file)
    setUploadOpen(true)
  }, [])

  const load = useCallback((id: number, replace = false, fromHistory = false, direction: NavigationDirection = "neutral") => {
    const startedAt = performance.now()
    const generation = ++requestGeneration.current
    const sort = sortRef.current
    appendGeneration.current++
    appendAbort.current?.abort()
    loadingMoreRef.current = false
    setLoadingMore(false); setAppendError("")
    pageAbort.current?.abort()
    const controller = new AbortController(); pageAbort.current = controller
    setLoading(true); setError("")
    request<FolderPage>(folderPageURL(id, sort), { signal: controller.signal })
      .then(async (result) => {
        await waitForMinimumPending(startedAt, reducedMotionRef.current)
        if (generation !== requestGeneration.current || controller.signal.aborted || sort !== sortRef.current || result.folder.id !== id) return
        setNavigationDirection(direction)
        setStaggerInitialItems(!hasCommittedPage.current)
        hasCommittedPage.current = true
        setPage(result); setFolderId(result.folder.id)
        const path = `/app/folders/${result.breadcrumbs.slice(1).map((crumb) => encodeURIComponent(crumb.name)).join("/") || "root"}`
        const state = { folderID: result.folder.id }
        ;(replace ? window.history.replaceState : window.history.pushState).call(window.history, state, "", path)
        committedLocation.current = { url: window.location.href, state }
      })
      .catch((reason) => {
        if (generation !== requestGeneration.current || controller.signal.aborted || reason instanceof DOMException && reason.name === "AbortError") return
        setError(reason instanceof Error ? reason.message : "Could not load this folder.")
        if (fromHistory) window.history.replaceState(committedLocation.current.state, "", committedLocation.current.url)
      })
      .finally(() => { if (generation === requestGeneration.current) setLoading(false) })
  }, [])

  async function loadMore() {
    if (!page?.nextCursor || loadingMoreRef.current) return
    const startedAt = performance.now()
    const generation = ++appendGeneration.current
    const committedFolder = page.folder.id; const committedSort = preferences.explorer.sort; const cursor = page.nextCursor
    const controller = new AbortController(); appendAbort.current?.abort(); appendAbort.current = controller
    loadingMoreRef.current = true
    setLoadingMore(true); setAppendError("")
    try {
      const next = await request<FolderPage>(folderPageURL(committedFolder, committedSort, cursor), { signal: controller.signal })
      await waitForMinimumPending(startedAt, reducedMotionRef.current)
      if (generation !== appendGeneration.current || controller.signal.aborted || folderId !== committedFolder || sortRef.current !== committedSort) return
      setPage((current) => {
        if (!current || current.folder.id !== committedFolder) return current
        return appendFolderPage(current, next)
      })
    } catch (reason) { await waitForMinimumPending(startedAt, reducedMotionRef.current); if (generation === appendGeneration.current && !controller.signal.aborted) setAppendError(reason instanceof Error ? reason.message : "Could not load more items.") }
    finally { if (generation === appendGeneration.current) { loadingMoreRef.current = false; setLoadingMore(false) } }
  }

  function reloadAfterMutation(key: string) {
    const node = document.querySelector<HTMLElement>(`[data-item-key="${key}"]`)
    const collection = node?.parentElement
    pendingFocus.current = { key, index: node && collection ? Array.from(collection.children).indexOf(node) : 0 }
    load(folderId, true)
  }

  useEffect(() => {
    const target = pendingFocus.current
    if (!target || loading) return
    pendingFocus.current = null
    window.requestAnimationFrame(() => {
      const retained = document.querySelector<HTMLElement>(`[data-item-key="${target.key}"] [aria-label^="Actions for"]`)
      if (retained) { retained.focus(); return }
      const items = Array.from(document.querySelectorAll<HTMLElement>("[data-item-key]"))
      const successor = items[Math.min(target.index, Math.max(0, items.length - 1))]?.querySelector<HTMLElement>("button")
      ;(successor ?? document.querySelector<HTMLElement>("[data-slot=empty] button") ?? document.querySelector<HTMLElement>("h1"))?.focus()
    })
  }, [loading, page])

  useEffect(() => { const onPop = (event: PopStateEvent) => { const id = Number(event.state?.folderID); if (Number.isInteger(id) && id > 0) load(id, true, true, "back") }; window.addEventListener("popstate", onPop); const initialID = requestedFolder?.folderID ?? root.rootFolderId; setFolderId(initialID); hasCommittedPage.current = false; load(initialID, true); return () => { window.removeEventListener("popstate", onPop); pageAbort.current?.abort(); appendAbort.current?.abort() } }, [load, requestedFolder?.folderID, requestedFolder?.token, root.rootFolderId])

  useEffect(() => {
    setPreferences(preferenceStore.get())
    const unsubscribe = preferenceStore.subscribe((next) => setPreferences(next))
    return () => { unsubscribe(); preferenceStore.destroy() }
  }, [preferenceStore])

  const previousSort = useRef(preferences.explorer.sort)
  useEffect(() => {
    if (previousSort.current === preferences.explorer.sort) return
    previousSort.current = preferences.explorer.sort
    load(folderId, true)
  }, [folderId, load, preferences.explorer.sort])

  useEffect(() => {
    if (lastRefreshToken.current === refreshToken) return
    lastRefreshToken.current = refreshToken
    load(folderId, true)
  }, [folderId, load, refreshToken])

  useEffect(() => {
    if (!page?.clips.some((clip) => ["queued", "processing", "validating"].includes(clip.state))) return
    let stopped = false
    const controllers: AbortController[] = []
    const navigationGeneration = requestGeneration.current
    const committedFolder = page.folder.id
    const committedSort = preferences.explorer.sort
    const poll = async () => {
      const chunks = activeJobIDChunks(page)
      const jobs = chunks.flat()
      const statuses: JobStatus[] = []
      try { for (const chunk of chunks) { const controller = new AbortController(); controllers.push(controller); const result = await request<{ jobs: JobStatus[] }>(`/api/jobs/statuses?ids=${chunk.join(",")}`, { signal: controller.signal }); statuses.push(...result.jobs) } } catch { return }
      if (stopped || navigationGeneration !== requestGeneration.current || committedFolder !== folderId || committedSort !== sortRef.current) return
      const found = new Set(statuses.map((status) => status.jobId))
      if (jobs.some((id) => !found.has(id))) { load(folderId, true); return }
      const sortChanges = jobStatusesChangeSortKey(committedSort, page, statuses)
      if (sortChanges) { load(folderId, true); return }
      setPage((current) => current ? mergeJobStatuses(current, statuses) : current)
    }
    const timer = window.setInterval(() => void poll(), 2000)
    return () => { stopped = true; controllers.forEach((controller) => controller.abort()); window.clearInterval(timer) }
  }, [folderId, load, page, preferences.explorer.sort])

  const title = page?.folder.isRoot ? (session.user.role === "admin" ? `${page.folder.ownerUsername}'s library` : "Your library") : page?.folder.name

  return <>
    <div className="mb-7">
      {page && <ExplorerBreadcrumbs crumbs={page.breadcrumbs} administrator={session.user.role === "admin"} onNavigate={(id) => load(id, false, false, "back")} onBack={onBack} />}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>{session.user.role === "admin" && root.id !== session.user.id && <div className="mb-3 inline-flex items-center gap-2 rounded-lg border border-sky-400/20 bg-sky-400/10 px-3 py-2 text-xs font-semibold text-sky-200" role="status"><span aria-hidden="true">●</span> Administrator view · {root.username}'s library</div>}<p className="eyebrow">{session.user.role === "admin" && root.id !== session.user.id ? `Managing ${root.username}` : "Folder explorer"}</p><h1 tabIndex={-1} className="mt-2 text-3xl font-semibold tracking-tight text-white focus:outline-none">{title ?? "Library"}</h1>{page ? <ExplorerItemSummary folderCount={page.totalFolderCount} clipCount={page.totalClipCount} /> : <p className="mt-2 text-sm text-slate-400" aria-live="polite">Loading contents…</p>}</div>
        <ExplorerToolbar view={preferences.explorer.view} sort={preferences.explorer.sort} disabled={!page} busy={loading} onViewChange={(view) => preferenceStore.update({ explorer: { view } })} onSortChange={(sort) => preferenceStore.update({ explorer: { sort } })} onCreateFolder={() => setCreateOpen(true)} onUploadClip={openUploadPicker} onRefresh={() => load(folderId, true)} />
      </div>
    </div>

    {error && <div className="mb-5 flex flex-wrap items-center gap-3 rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert"><span>{error}</span><Button size="sm" variant="secondary" onClick={() => load(folderId, true)}><RotateCcw size={15} /> Retry</Button></div>}
    {loading && page && <p className="fixed bottom-5 left-1/2 z-40 inline-flex -translate-x-1/2 items-center gap-2 rounded-full border border-sky-300/20 bg-slate-950/90 px-4 py-2 text-sm text-sky-200 shadow-xl shadow-black/30 backdrop-blur-md" role="status"><span className="size-4 animate-spin rounded-full border-2 border-sky-300 border-r-transparent" /> Updating folder…</p>}
    {loading && !page ? <ExplorerLoading view={preferences.explorer.view} /> : page && <div aria-busy={loading} inert={loading ? true : undefined} className={loading ? "pointer-events-none opacity-70 transition-opacity" : "transition-opacity"}>
      {page.totalItemCount === 0 ? <EmptyFolder onCreate={() => setCreateOpen(true)} onUpload={openUploadPicker} /> :
        <><div className={preferences.explorer.view === "list" ? "explorer-list-head" : "sr-only"} aria-hidden={preferences.explorer.view !== "list"}><span>Name</span><span>Status / type</span><span>Size / contents</span><span>Uploaded</span><span>Actions</span></div><AnimatePresence mode="popLayout"><m.ul key={page.folder.id} layout={!reducedMotion} transition={layoutTransition} initial="hidden" animate="visible" exit="exit" variants={collectionVariants(navigationDirection, Boolean(reducedMotion))} className={preferences.explorer.view === "grid" ? explorerGridClasses : explorerListClasses} data-view={preferences.explorer.view} aria-label="Folder contents">
          {page.folders.map((folder, index) => <m.li layout={!reducedMotion} transition={layoutTransition} initial="hidden" animate="visible" variants={initialItemVariants(index, staggerInitialItems, Boolean(reducedMotion))} key={`folder:${folder.id}`} data-item-key={`folder:${folder.id}`}><FolderCardV2 folder={folder} session={session} onOpen={() => load(folder.id, false, false, "forward")} onRename={() => setRenameFolder(folder)} onMove={() => setMoveFolder(folder)} onCopy={session.user.role === "admin" ? () => setCopyFolder(folder) : undefined} onDeleted={() => reloadAfterMutation(`folder:${folder.id}`)} /></m.li>)}
          {page.clips.map((clip, index) => <m.li layout={!reducedMotion} transition={layoutTransition} initial="hidden" animate="visible" variants={initialItemVariants(page.folders.length + index, staggerInitialItems, Boolean(reducedMotion))} key={`clip:${clip.id}`} data-item-key={`clip:${clip.id}`}><ClipCardV2 clip={clip} currentFolder={page.folder} session={session} users={users} onChanged={() => reloadAfterMutation(`clip:${clip.id}`)} /></m.li>)}
        </m.ul></AnimatePresence></>}{(page.nextCursor || appendError) && <div className="mt-7 flex flex-col items-center gap-2">{appendError && <p className="text-sm text-red-300" role="alert">{appendError}</p>}<Button variant="secondary" onClick={loadMore} disabled={loadingMore}>{loadingMore ? <Spinner /> : <ChevronDown size={16} />}{loadingMore ? "Loading…" : appendError ? "Retry load more" : "Load more"}</Button></div>}
    </div>}

    {createOpen && page && <NameDialog title="Create folder" action="Create folder" onClose={() => setCreateOpen(false)} onSubmit={async (name) => { await request("/api/folders", { method: "POST", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ parentFolderId: page.folder.id, name }) }); toast.success("Folder created"); load(folderId, true) }} />}
    {renameFolder && <NameDialog title="Rename folder" action="Save name" initialValue={renameFolder.name} onClose={() => setRenameFolder(null)} onSubmit={async (name) => { await request(`/api/folders/${renameFolder.id}`, { method: "PATCH", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ name }) }); toast.success("Folder renamed"); reloadAfterMutation(`folder:${renameFolder.id}`) }} />}
    {moveFolder && <MoveDialog source={moveFolder} session={session} users={users} onClose={() => setMoveFolder(null)} onMoved={() => { const key = `folder:${moveFolder.id}`; setMoveFolder(null); toast.success("Folder moved"); reloadAfterMutation(key) }} />}
    {copyFolder && page && <CopyFolderDialog source={copyFolder} initialFolderID={page.folder.id} session={session} users={users} onClose={() => setCopyFolder(null)} onCopied={() => { const key = `folder:${copyFolder.id}`; setCopyFolder(null); toast.success("Folder copied"); reloadAfterMutation(key) }} />}
    {page && <ViewportFileDrop enabled={!uploadOpen && !editorSessionID} destinationLabel={page.folder.isRoot ? `${page.folder.ownerUsername}'s library` : page.folder.name} onFile={openDroppedFile} />}
    {uploadOpen && page && <UploadDialog session={session} currentFolder={page.folder} users={users} initialFile={droppedFile} onClose={() => {setUploadOpen(false);setDroppedFile(null)}} onQueued={() => {setUploadOpen(false);setDroppedFile(null);toast.success("Upload queued");load(folderId, true)}} onUploaded={(sessionID,file) => { setUploadOpen(false);setDroppedFile(null);setEditorLocalFile(file);setEditorSessionID(sessionID) }} />}
    {editorSessionID && <EditorDialog session={session} sessionID={editorSessionID} localFile={editorLocalFile} users={users} onClose={() => {setEditorSessionID(null);setEditorLocalFile(null)}} onFinalized={() => { setEditorSessionID(null);setEditorLocalFile(null); load(folderId, true) }} />}
  </>
}

function ClipCardV2({ clip, currentFolder, session, users, onChanged }: { clip: import("@/api").ClipSummary; currentFolder: FolderRecord; session: Session; users: User[]; onChanged: () => void }) {
  const [previewing, setPreviewing] = useState(false)
  const [dialog, setDialog] = useState<"rename" | "move" | "delete" | "cancel-job" | "dismiss-failure" | null>(null)
  const [jobBusy, setJobBusy] = useState(false)
  const [jobError, setJobError] = useState("")
  const item = clipExplorerItem(clip)
  const actions = itemActions(item, {
    preview: () => setPreviewing(true),
    "copy-link": async () => { try { await copyText(`${session.publicBaseURL}/c/${clip.publicId}`); toast.success("Link copied") } catch { toast.error("Could not copy link") } },
    "public-page": () => { openInNewTab(`${session.publicBaseURL}/c/${clip.publicId}`) },
    rename: () => setDialog("rename"), move: () => setDialog("move"), trash: () => setDialog("delete"),
    cancel: () => { setJobError(""); setDialog("cancel-job") }, dismiss: () => { setJobError(""); setDialog("dismiss-failure") },
  })
  async function submit(title?: string) {
    if (dialog === "rename") await request(`/api/clips/${clip.id}`, { method: "PATCH", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ title }) })
    if (dialog === "delete") await request(`/api/clips/${clip.id}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } })
    toast.success(dialog === "rename" ? "Clip renamed" : "Clip moved to recycle bin")
    setDialog(null); onChanged()
  }
  async function submitJobAction() {
    if (!clip.jobId) return
    setJobBusy(true); setJobError("")
    try { await request(dialog === "dismiss-failure" ? `/api/jobs/${clip.jobId}/failure` : `/api/jobs/${clip.jobId}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } }); toast.success(dialog === "dismiss-failure" ? "Failed upload dismissed" : "Upload cancelled"); setDialog(null); onChanged() }
    catch (reason) { setJobError(reason instanceof Error ? reason.message : "Could not update this upload.") }
    finally { setJobBusy(false) }
  }
  return <div className="flex h-full flex-col">
    <ClipGridItem item={item} onPreview={item.previewEligible ? () => setPreviewing(true) : undefined} actions={actions} />
    {previewing && <VideoPreviewDialog title={clip.title} posterSrc={`/m/${clip.publicId}/poster`} videoSrc={`/m/${clip.publicId}/video`} onClose={() => setPreviewing(false)} />}
    {dialog === "rename" && <NameDialog title="Rename clip" fieldLabel="Clip title" action="Save name" initialValue={clip.title} onClose={() => setDialog(null)} onSubmit={submit} />}
    {dialog === "move" && <MoveDialog source={{ id: clip.id, name: clip.title, ownerUserId: currentFolder.ownerUserId }} sourceKind="clip" initialFolderID={currentFolder.id} session={session} users={users} onClose={() => setDialog(null)} onMoved={() => { toast.success("Clip moved"); setDialog(null); onChanged() }} />}
    {dialog === "delete" && <AlertDialog open onOpenChange={(open) => { if (!open) setDialog(null) }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Move clip to recycle bin</AlertDialogTitle><AlertDialogDescription>This clip will be recoverable from the recycle bin.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel><ArrowLeft size={15} /> Back</AlertDialogCancel><AlertDialogAction className="border-rose-300/60" onClick={(event) => { event.preventDefault(); void submit() }}><Trash2 size={15} /> Move to recycle bin</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}
    {(dialog === "cancel-job" || dialog === "dismiss-failure") && <AlertDialog open onOpenChange={(open) => { if (!open && !jobBusy) setDialog(null) }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{dialog === "cancel-job" ? "Cancel this upload?" : "Dismiss failed upload?"}</AlertDialogTitle><AlertDialogDescription>{dialog === "cancel-job" ? "Processing will stop and all source and partial media will be removed. This cannot be recovered." : "Dismissing removes this private failure notice permanently."}</AlertDialogDescription></AlertDialogHeader>{jobError && <p className="text-sm text-red-300" role="alert">{jobError}</p>}<AlertDialogFooter><AlertDialogCancel disabled={jobBusy}>Back</AlertDialogCancel><AlertDialogAction className="border-rose-300/60" disabled={jobBusy} onClick={(event) => { event.preventDefault(); void submitJobAction() }}>{jobBusy ? "Working…" : dialog === "cancel-job" ? "Cancel upload" : "Dismiss"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}
  </div>
}

export function ClipManagementV2({ clip, currentFolder, session, users, onChanged }: { clip: import("@/api").ClipSummary; currentFolder: FolderRecord; session: Session; users: User[]; onChanged: () => void }) {
  const [dialog, setDialog] = useState<"rename" | "move" | "delete" | "cancel-job" | "dismiss-failure" | null>(null)
  const [jobBusy, setJobBusy] = useState(false)
  const [jobError, setJobError] = useState("")
  const jobID = clip.jobId

  async function submit(title?: string) {
    if (dialog === "rename") await request(`/api/clips/${clip.id}`, { method: "PATCH", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ title }) })
    if (dialog === "delete") await request(`/api/clips/${clip.id}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } })
    if (dialog !== "rename") setDialog(null)
    onChanged()
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
    {dialog === "dismiss-failure" && <AlertDialog open onOpenChange={(open) => { if (!open && !jobBusy) setDialog(null) }}><AlertDialogContent onEscapeKeyDown={(event) => { if (jobBusy) event.preventDefault() }}><AlertDialogHeader><AlertDialogTitle>Dismiss failed upload?</AlertDialogTitle><AlertDialogDescription>The source and partial media are already gone. Dismissing removes this private failure notice permanently; it will not create a recycle-bin item.</AlertDialogDescription></AlertDialogHeader>{jobError && <p className="text-sm text-red-300" role="alert">{jobError}</p>}<AlertDialogFooter><AlertDialogCancel disabled={jobBusy}><ArrowLeft size={15} /> Keep notice</AlertDialogCancel><AlertDialogAction className="border-rose-300/60" disabled={jobBusy} onClick={(event) => { event.preventDefault(); void submitJobAction() }}><CircleX size={15} />{jobBusy ? "Dismissing…" : "Dismiss"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}
  </>

  if (["queued", "validating", "processing"].includes(clip.state) && jobID) return <>
    <JobActionBar label="Cancel processing" onClick={() => { setJobError(""); setDialog("cancel-job") }} />
    {dialog === "cancel-job" && <AlertDialog open onOpenChange={(open) => { if (!open && !jobBusy) setDialog(null) }}><AlertDialogContent onEscapeKeyDown={(event) => { if (jobBusy) event.preventDefault() }}><AlertDialogHeader><AlertDialogTitle>Cancel this upload?</AlertDialogTitle><AlertDialogDescription>Processing will stop and all source and partial media will be removed. The clip will not enter the recycle bin and cannot be recovered.</AlertDialogDescription></AlertDialogHeader>{jobError && <p className="text-sm text-red-300" role="alert">{jobError}</p>}<AlertDialogFooter><AlertDialogCancel disabled={jobBusy}><ArrowLeft size={15} /> Keep processing</AlertDialogCancel><AlertDialogAction className="border-rose-300/60" disabled={jobBusy} onClick={(event) => { event.preventDefault(); void submitJobAction() }}><CircleX size={15} />{jobBusy ? "Cancelling…" : "Cancel upload"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}
  </>

  if (clip.state !== "ready") return null
  return <>
    <ItemDropdownActions actions={itemActions(clipExplorerItem(clip), { rename: () => setDialog("rename"), move: () => setDialog("move"), trash: () => setDialog("delete") })} label={`Actions for ${clip.title}`} />
    {dialog === "rename" && <NameDialog title="Rename clip" fieldLabel="Clip title" action="Save name" initialValue={clip.title} onClose={() => setDialog(null)} onSubmit={submit} />}
    {dialog === "move" && <MoveDialog source={{ id: clip.id, name: clip.title, ownerUserId: currentFolder.ownerUserId }} sourceKind="clip" initialFolderID={currentFolder.id} session={session} users={users} onClose={() => setDialog(null)} onMoved={() => { setDialog(null); onChanged() }} />}
    {dialog === "delete" && <AlertDialog open onOpenChange={(open) => { if (!open) setDialog(null) }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Move clip to recycle bin</AlertDialogTitle><AlertDialogDescription>This clip will be recoverable from the recycle bin.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel><ArrowLeft size={15} /> Back</AlertDialogCancel><AlertDialogAction className="border-rose-300/60" onClick={(event) => { event.preventDefault(); void submit() }}><Trash2 size={15} /> Move to recycle bin</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}
  </>
}

function JobActionBar({ label, onClick }: { label: string; onClick: () => void }) {
  return <div className="-mt-3 flex rounded-b-2xl border-x border-b border-white/[.06] bg-slate-950 px-3 py-2"><Button className="w-full" size="sm" variant="danger" onClick={onClick}><CircleX size={14} /> {label}</Button></div>
}

function FolderCardV2({ folder, session, onOpen, onRename, onMove, onCopy, onDeleted }: { folder: FolderRecord; session: Session; onOpen: () => void; onRename: () => void; onMove: () => void; onCopy?: () => void; onDeleted: () => void }) {
  const [confirming, setConfirming] = useState(false)
  const item = folderExplorerItem(folder, session.user.role)
  const actions = itemActions(item, { open: onOpen, rename: onRename, move: onMove, copy: onCopy, trash: () => setConfirming(true) })
  return <div className="flex h-full flex-col">
    <FolderGridItem item={item} onOpen={onOpen} actions={actions} />
    {confirming && <FolderDeleteDialog folder={folder} session={session} onClose={() => setConfirming(false)} onDeleted={() => { toast.success("Folder moved to recycle bin"); onDeleted() }} />}
  </div>
}

export function FolderDeleteDialog({ folder, session, onClose, onDeleted }: { folder: FolderRecord; session: Session; onClose: () => void; onDeleted: () => void }) {
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

  return <AlertDialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}><AlertDialogContent onEscapeKeyDown={(event) => { if (busy) event.preventDefault() }}>
    <AlertDialogHeader><AlertDialogTitle>{`Delete “${folder.name}”?`}</AlertDialogTitle><AlertDialogDescription>The complete folder subtree will move to the recycle bin. Review the recursive totals before continuing.</AlertDialogDescription></AlertDialogHeader>
    {loading && <div className="rounded-xl border border-white/[.08] bg-slate-950 p-4 text-sm text-slate-400" role="status">Calculating everything inside this folder…</div>}
    {!loading && loadError && <div className="rounded-xl border border-red-400/20 bg-red-400/10 p-4"><p className="text-sm text-red-200" role="alert">{loadError}</p><Button className="mt-3" size="sm" variant="secondary" onClick={() => setReloadToken((value) => value + 1)}><RotateCcw size={15} /> Retry summary</Button></div>}
    {summary && <>
      <div className="mt-4 grid grid-cols-3 gap-2">
        <DeletionStat label="Folders" value={summary.folderCount.toLocaleString()} />
        <DeletionStat label="Clips" value={summary.clipCount.toLocaleString()} />
        <DeletionStat label="Stored size" value={formatBytes(summary.storedBytes)} />
      </div>
    </>}
    <label className="mt-4 flex items-start gap-3 rounded-lg border border-white/[.08] bg-slate-950 p-3 text-sm text-slate-300"><input className="mt-1 size-4 accent-sky-400" type="checkbox" checked={acknowledged} disabled={!summary || loading} onChange={(event) => setAcknowledged(event.target.checked)} /> I understand all {summary?.totalItems.toLocaleString() ?? "calculated"} items will be removed from the active library.</label>
    {actionError && <p className="mt-3 text-sm text-red-300" role="alert">{actionError}</p>}
    <AlertDialogFooter><AlertDialogCancel disabled={busy}><ArrowLeft size={15} /> Back</AlertDialogCancel><AlertDialogAction className="border-rose-300/60" disabled={!summary || loading || !acknowledged || busy} onClick={(event) => { event.preventDefault(); void remove() }}><Trash2 size={15} />{busy ? "Moving…" : "Move to recycle bin"}</AlertDialogAction></AlertDialogFooter>
  </AlertDialogContent></AlertDialog>
}

function DeletionStat({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0 rounded-xl border border-white/[.08] bg-white/[.03] px-3 py-3"><p className="truncate text-xs text-slate-500">{label}</p><p className="mt-1 truncate text-sm font-semibold text-white" title={value}>{value}</p></div>
}

function ExplorerLoading({ view }: { view: "grid" | "list" }) {
  return <div className={view === "grid" ? explorerGridClasses : explorerListClasses} data-view={view} role="status" aria-label={`Loading folder contents in ${view} view`}>
    {[1, 2, 3, 4].map((item) => <Skeleton key={item} className={view === "grid" ? "aspect-[4/3] rounded-2xl border border-white/[.06]" : "h-[4.75rem] rounded-2xl border border-white/[.06]"} />)}
  </div>
}
function EmptyFolder({ onCreate, onUpload }: { onCreate: () => void; onUpload: () => void }) { return <Empty className="min-h-72 border border-white/10 bg-white/[.02]"><EmptyHeader><EmptyMedia variant="icon"><Folder /></EmptyMedia><EmptyTitle>This folder is empty</EmptyTitle><EmptyDescription>Create a folder or upload a clip to add your first video.</EmptyDescription></EmptyHeader><EmptyContent><div className="flex flex-wrap justify-center gap-2"><Button variant="secondary" onClick={onCreate}><FolderPlus size={17} /> Create folder</Button><Button variant="success" onClick={onUpload}>Upload clip</Button></div></EmptyContent></Empty> }

export function NameDialog({ title, fieldLabel = "Folder name", action, initialValue = "", onClose, onSubmit }: { title: string; fieldLabel?: string; action: string; initialValue?: string; onClose: () => void; onSubmit: (name: string) => Promise<void> }) {
  const [open, setOpen] = useState(true)
  const [name, setName] = useState(initialValue)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const submittingRef = useRef(false)
  const returnFocusRef = useRef(document.activeElement instanceof HTMLElement ? document.activeElement : null)
  const inputID = fieldLabel === "Clip title" ? "rename-clip-title" : "folder-name"
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (submittingRef.current || !name.trim()) return
    submittingRef.current = true
    setBusy(true)
    setError("")
    try {
      await onSubmit(name.trim())
      setOpen(false)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Something went wrong.")
    } finally {
      submittingRef.current = false
      setBusy(false)
    }
  }
  return <Dialog open={open} onOpenChange={(nextOpen) => { if (!nextOpen && !busy) setOpen(false) }}>
    <DialogContent showCloseButton={!busy} onCloseAutoFocus={(event) => { event.preventDefault(); returnFocusRef.current?.focus(); onClose() }} onEscapeKeyDown={(event) => { if (busy) event.preventDefault() }} onPointerDownOutside={(event) => { if (busy) event.preventDefault() }} onInteractOutside={(event) => { if (busy) event.preventDefault() }}>
      <DialogHeader><DialogTitle>{title}</DialogTitle><DialogDescription>{fieldLabel === "Clip title" ? "Choose the title shown in your library and public link." : "Choose a name for this folder."}</DialogDescription></DialogHeader>
      <form className="space-y-4" onSubmit={submit}>
        <FieldGroup className="gap-4"><Field><FieldLabel htmlFor={inputID}>{fieldLabel}</FieldLabel><Input id={inputID} value={name} onChange={(event) => setName(event.target.value)} maxLength={fieldLabel === "Clip title" ? 200 : 100} required autoFocus /></Field></FieldGroup>
        <FieldError>{error}</FieldError>
        <DialogFooter className="gap-2 sm:space-x-0"><Button type="button" variant="secondary" disabled={busy} onClick={() => setOpen(false)}><ArrowLeft size={15} /> Back</Button><Button type="submit" variant="success" disabled={busy || !name.trim()}><Save size={15} />{busy ? "Saving…" : action}</Button></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
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
