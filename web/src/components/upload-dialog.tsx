import { useEffect, useRef, useState, type DragEvent, type FormEvent } from "react"
import { ArrowLeft, CloudUpload, FileVideo, Folder, Info, Scissors, Upload, X } from "lucide-react"
import { request, uploadVideo, type Folder as FolderRecord, type FolderPage, type Session, type User } from "@/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { FolderPickerDialog } from "@/components/folder-picker-dialog"
import { Modal } from "@/components/ui/modal"

const qualityValues = [30, 27, 24, 21, 18]
const qualityLabels = ["Smallest file", "Smaller", "Balanced", "Higher quality", "Highest quality"]
const resolutionValues = [480, 720, 1080]

export function UploadDialog({ session, currentFolder, users, onClose, onQueued, onUploaded }: { session: Session; currentFolder: FolderRecord; users: User[]; onClose: () => void; onQueued: () => void; onUploaded: (sessionID: string, file: File) => void }) {
  const defaultDestination = currentFolder.id
  const [destination, setDestination] = useState<FolderRecord | null>(null)
  const [pickingDestination, setPickingDestination] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [title, setTitle] = useState("")
  const [compress, setCompress] = useState(false)
  const [quality, setQuality] = useState(2)
  const [resolution, setResolution] = useState(2)
  const [phase, setPhase] = useState<"idle" | "uploading" | "validating">("idle")
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState("")
  const [cancelConfirming, setCancelConfirming] = useState(false)
  const [openingEditor, setOpeningEditor] = useState(false)
  const [localPreviewURL, setLocalPreviewURL] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => { request<FolderPage>(`/api/folders/${defaultDestination}`).then((page) => setDestination(page.folder)).catch((reason) => setError(reason.message)) }, [defaultDestination])
  useEffect(() => { if(!file){setLocalPreviewURL("");return}const url=URL.createObjectURL(file);setLocalPreviewURL(url);return()=>URL.revokeObjectURL(url) },[file])
  const compressionRequired = Boolean(file && session.user.role !== "admin" && file.size > session.user.storedFileLimitBytes)

  function chooseFile(selected?: File) {
    if (!selected) return
    setFile(selected); setError("")
    const lastDot = selected.name.lastIndexOf(".")
    setTitle(lastDot > 0 ? selected.name.slice(0, lastDot) : selected.name)
    setCompress(session.user.role !== "admin" && selected.size > session.user.storedFileLimitBytes)
  }
  function drop(event: DragEvent) { event.preventDefault(); chooseFile(event.dataTransfer.files[0]) }

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!file || !destination) return
    const editingRequested = ((event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null)?.value === "trim"
    setOpeningEditor(editingRequested)
    setError(""); setPhase("uploading"); setProgress(0)
    if (editingRequested) {
      try {
        const existing = await request<{ id: string } | undefined>(`/api/uploads/resumable?destinationFolderId=${destination.id}&sourceSizeBytes=${file.size}&title=${encodeURIComponent(title)}`)
        if (existing?.id) {
          onUploaded(existing.id, file)
          return
        }
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : "Could not check for saved edits.")
        setPhase("idle"); setOpeningEditor(false)
        return
      }
    }
    const form = new FormData()
    // Metadata is intentionally appended first so the server can reserve the
    // title and disk capacity before it starts consuming the large file part.
    form.append("title", title)
    form.append("destinationFolderId", String(destination.id))
    form.append("compressionRequested", String(compressionRequired || compress))
    form.append("qualityCrf", String(qualityValues[quality]))
    form.append("maxHeight", String(resolutionValues[resolution]))
    form.append("editingRequested", String(editingRequested))
    form.append("video", file, file.name)
    const controller = new AbortController()
    abortRef.current = controller
    try {
      const result = await uploadVideo<{ id?: string }>(form, session.csrfToken, (value) => { setProgress(value); if (value === 100) setPhase("validating") }, controller.signal)
      if (editingRequested && result.id) onUploaded(result.id, file)
      else onQueued()
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") { onClose(); return }
      setError(reason instanceof Error ? reason.message : "The upload failed."); setPhase("idle");setOpeningEditor(false)
    } finally { abortRef.current = null }
  }

  const busy = phase !== "idle"
  function requestClose() { if (busy) setCancelConfirming(true); else onClose() }
  if(openingEditor&&file)return <Modal title="Edit clip" onClose={requestClose} dismissible={!cancelConfirming} className="max-h-[94vh] max-w-6xl"><div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]"><section><p className="eyebrow">Uploading and analyzing privately</p><h2 className="mt-1 truncate text-xl font-semibold text-white">{title}</h2><div className="mt-5 overflow-hidden rounded-xl border border-white/[.08] bg-slate-950">{localPreviewURL?<video className="aspect-video w-full" src={localPreviewURL} controls preload="metadata" />:<div className="grid aspect-video place-items-center text-sm text-slate-500">Preparing local preview…</div>}</div><div className="mt-4" role="status"><div className="mb-2 flex justify-between text-sm"><span className="text-slate-300">{phase==="uploading"?"Uploading source…":"Analyzing tracks and preparing editor…"}</span><span className="text-slate-500">{progress}%</span></div><div className="h-2 overflow-hidden rounded-full bg-slate-800"><div className="h-full rounded-full bg-sky-400 transition-[width]" style={{width:`${progress}%`}} /></div></div></section><aside className="rounded-xl border border-white/[.08] bg-white/[.025] p-4"><h3 className="font-medium text-white">Editor controls</h3><p className="mt-2 text-sm leading-6 text-slate-400">You can inspect the local video now. The trim filmstrip and detected audio tracks will appear as soon as server analysis finishes.</p></aside></div>{error&&<p className="mt-4 text-sm text-red-300" role="alert">{error}</p>}{cancelConfirming&&<Modal nested title="Cancel this upload?" onClose={()=>setCancelConfirming(false)}><p className="text-sm text-slate-400">The transfer and all temporary data will be discarded.</p><div className="mt-5 flex justify-end gap-2"><Button variant="secondary" onClick={()=>setCancelConfirming(false)}><ArrowLeft size={15} /> Keep editing</Button><Button variant="danger" onClick={()=>{setCancelConfirming(false);abortRef.current?.abort()}}><X size={15} /> Cancel upload</Button></div></Modal>}</Modal>
  return <Modal title="Upload a video" onClose={requestClose} className="max-h-[90vh] max-w-2xl">
      <p className="eyebrow mb-5">New clip</p><form className="max-h-[70vh] space-y-5 overflow-y-auto pr-1" onSubmit={submit}>
        <input ref={inputRef} className="sr-only" type="file" accept=".mp4,.m4v,.mov,.mkv,.webm,.avi,.wmv" onChange={(event) => chooseFile(event.target.files?.[0])} />
        <button type="button" className="grid min-h-36 w-full place-items-center rounded-xl border border-dashed border-white/15 bg-slate-950/50 p-5 text-center transition hover:border-sky-400/40 hover:bg-sky-400/[.03] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400" onClick={() => inputRef.current?.click()} onDragOver={(event) => event.preventDefault()} onDrop={drop} disabled={busy}>
          {file ? <div><FileVideo className="mx-auto text-sky-300" size={30} /><p className="mt-3 max-w-md truncate font-medium text-white">{file.name}</p><p className="mt-1 text-sm text-slate-500">{formatBytes(file.size)} · Click or drop to replace</p></div> : <div><Upload className="mx-auto text-sky-300" size={30} /><p className="mt-3 font-medium text-white">Choose or drop one video</p><p className="mt-1 text-sm text-slate-500">MP4, MOV, MKV, WebM, AVI, or WMV · Maximum 500 MB</p></div>}
        </button>

        <label className="block"><span className="mb-2 block text-sm font-medium text-slate-300">Clip title</span><Input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={200} required disabled={busy} /></label>
        <div><span className="mb-2 block text-sm font-medium text-slate-300">Destination</span><button type="button" className="flex h-11 w-full items-center gap-3 rounded-lg border border-white/10 bg-slate-950/70 px-3 text-left text-sm text-slate-200 hover:border-sky-400/30" onClick={() => setPickingDestination(true)} disabled={busy}><Folder size={17} className="text-sky-300" /><span className="truncate">{destination ? `${destination.ownerUsername} / ${destination.isRoot ? "Library" : destination.name}` : "Loading…"}</span><span className="ml-auto text-xs text-slate-500">Change</span></button></div>

        <div className="rounded-xl border border-white/[.08] bg-white/[.025] p-4">
          <label className="flex items-start gap-3"><input className="mt-1 size-4 accent-sky-400" type="checkbox" checked={compressionRequired || compress} onChange={(event) => setCompress(event.target.checked)} disabled={compressionRequired || busy} /><span><span className="flex items-center gap-2 text-sm font-medium text-white">Compress <Info size={14} className="text-slate-500" aria-hidden="true" /></span><span className="mt-1 block text-xs leading-5 text-slate-500">{compressionRequired && file ? `This file is ${formatBytes(file.size)}; your stored-file limit is ${formatBytes(session.user.storedFileLimitBytes)}. Compression is required.` : "Optional for this file. Leave off to avoid unnecessary quality loss when possible."}</span></span></label>
          {(compressionRequired || compress) && <div className="mt-5 grid gap-5 sm:grid-cols-2"><Slider label="Quality" value={quality} maximum={4} onChange={setQuality} valueLabel={qualityLabels[quality]} disabled={busy} /><Slider label="Maximum resolution" value={resolution} maximum={2} onChange={setResolution} valueLabel={`${resolutionValues[resolution]}p`} disabled={busy} /></div>}
        </div>

        {busy && <div aria-live="polite"><div className="mb-2 flex justify-between text-sm"><span className="text-slate-300">{phase === "uploading" ? "Uploading bytes…" : "Upload complete · validating media…"}</span><span className="text-slate-500">{progress}%</span></div><div className="h-2 overflow-hidden rounded-full bg-slate-800"><div className="h-full rounded-full bg-sky-400 transition-[width]" style={{ width: `${progress}%` }} /></div></div>}
        {error && <p className="rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">{error}</p>}
        <div className="flex flex-wrap justify-end gap-2"><Button type="button" variant="danger" onClick={requestClose}><X size={16} />{busy ? "Cancel upload" : "Cancel"}</Button><Button value="trim" disabled={busy || !file || !title.trim() || !destination}><Scissors size={16} />{busy ? "Working…" : "Trim clip"}</Button><Button value="upload" variant="success" disabled={busy || !file || !title.trim() || !destination}><CloudUpload size={16} />{busy ? "Working…" : "Upload clip"}</Button></div>
      </form>
    {pickingDestination && <FolderPickerDialog title="Choose upload destination" session={session} users={users} initialID={destination?.id ?? defaultDestination} onClose={() => setPickingDestination(false)} onChoose={(folder) => { setDestination(folder); setPickingDestination(false) }} />}
    {cancelConfirming && <Modal nested title="Cancel this upload?" onClose={() => setCancelConfirming(false)}><p className="text-sm leading-6 text-slate-400">The transfer or initial validation will stop. Its temporary source and reserved space will be removed, and nothing will enter the recycle bin.</p><div className="mt-5 flex justify-end gap-2"><Button variant="secondary" onClick={() => setCancelConfirming(false)}><ArrowLeft size={15} /> Keep uploading</Button><Button variant="danger" onClick={() => { setCancelConfirming(false); abortRef.current?.abort() }}><X size={15} /> Cancel upload</Button></div></Modal>}
  </Modal>
}

function Slider({ label, value, maximum, onChange, valueLabel, disabled }: { label: string; value: number; maximum: number; onChange: (value: number) => void; valueLabel: string; disabled: boolean }) { return <label><span className="flex justify-between text-xs font-medium text-slate-400"><span>{label}</span><span className="text-sky-300">{valueLabel}</span></span><input className="mt-3 w-full accent-sky-400" type="range" min={0} max={maximum} step={1} value={value} onChange={(event) => onChange(Number(event.target.value))} disabled={disabled} aria-valuetext={valueLabel} /></label> }
function formatBytes(bytes: number) { if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(bytes >= 10_000_000 ? 0 : 1)} MB`; if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(1)} KB`; return `${bytes} bytes` }
