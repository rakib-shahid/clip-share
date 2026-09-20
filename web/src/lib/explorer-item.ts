import type { ClipSummary, Folder, Session } from "@/api"

export type ItemCapabilities = Readonly<{
  open: boolean
  preview: boolean
  copyLink: boolean
  openPublicPage: boolean
  rename: boolean
  move: boolean
  copy: boolean
  moveToTrash: boolean
  cancelUpload: boolean
  dismissFailure: boolean
}>

type ItemStatus = Readonly<{
  label: string
  tone: "neutral" | "ready" | "active" | "failed" | "muted" | "unknown"
}>

type ItemProgress = Readonly<
  | { kind: "none"; value: null }
  | { kind: "indeterminate"; value: null }
  | { kind: "determinate"; value: number }
>

export type ExplorerFolderItem = Readonly<{
  kind: "folder"
  key: `folder:${number}`
  id: number
  name: string
  ownerUserId: number
  ownerUsername: string
  parentFolderId: number | null
  isRoot: boolean
  folderCount: number
  clipCount: number
  status: ItemStatus
  previewEligible: false
  progress: ItemProgress
  secondary: Readonly<{ kind: "folder-counts"; folderCount: number; clipCount: number }>
  capabilities: ItemCapabilities
}>

export type ExplorerClipItem = Readonly<{
  kind: "clip"
  key: `clip:${number}`
  id: number
  name: string
  state: string
  sizeBytes: number | null
  createdAt: string
  jobId: number | null
  progressValue: number | null
  errorMessage: string | null
  publicId: string
  status: ItemStatus
  previewEligible: boolean
  progress: ItemProgress
  secondary: Readonly<{ kind: "clip-metadata"; sizeBytes: number | null; createdAt: string }>
  capabilities: ItemCapabilities
}>

export type ExplorerItem = ExplorerFolderItem | ExplorerClipItem

const noCapabilities: ItemCapabilities = {
  open: false, preview: false, copyLink: false, openPublicPage: false, rename: false,
  move: false, copy: false, moveToTrash: false, cancelUpload: false, dismissFailure: false,
}

const clipStatuses: Record<string, ItemStatus> = {
  uploading: { label: "Uploading", tone: "active" },
  queued: { label: "Queued for processing", tone: "active" },
  validating: { label: "Validating upload", tone: "active" },
  processing: { label: "Processing video", tone: "active" },
  failed: { label: "Upload failed", tone: "failed" },
  ready: { label: "Ready", tone: "ready" },
  cancelled: { label: "Cancelled", tone: "muted" },
  unavailable: { label: "Unavailable", tone: "muted" },
}

export function folderExplorerItem(folder: Folder, role: Session["user"]["role"]): ExplorerFolderItem {
  return {
    kind: "folder",
    key: `folder:${folder.id}`,
    id: folder.id,
    name: folder.name,
    ownerUserId: folder.ownerUserId,
    ownerUsername: folder.ownerUsername,
    parentFolderId: folder.parentFolderId,
    isRoot: folder.isRoot,
    folderCount: folder.folderCount,
    clipCount: folder.clipCount,
    status: { label: "Folder", tone: "neutral" },
    previewEligible: false,
    progress: { kind: "none", value: null },
    secondary: { kind: "folder-counts", folderCount: folder.folderCount, clipCount: folder.clipCount },
    capabilities: {
      ...noCapabilities,
      open: true,
      rename: !folder.isRoot,
      move: !folder.isRoot,
      copy: role === "admin" && !folder.isRoot,
      moveToTrash: !folder.isRoot,
    },
  }
}

export function clipExplorerItem(clip: ClipSummary): ExplorerClipItem {
  const knownStatus = clipStatuses[clip.state]
  const status = knownStatus ?? { label: "Unknown status", tone: "unknown" as const }
  const active = ["queued", "validating", "processing"].includes(clip.state)
  const previewEligible = clip.state === "ready"
  let progress: ItemProgress = { kind: "none", value: null }
  if (active) progress = clip.progress === null || !Number.isFinite(clip.progress)
    ? { kind: "indeterminate", value: null }
    : { kind: "determinate", value: Math.min(100, Math.max(0, clip.progress)) }

  return {
    kind: "clip",
    key: `clip:${clip.id}`,
    id: clip.id,
    name: clip.title,
    state: clip.state,
    sizeBytes: clip.sizeBytes,
    createdAt: clip.createdAt,
    jobId: clip.jobId,
    progressValue: clip.progress,
    errorMessage: clip.errorMessage,
    publicId: clip.publicId,
    status,
    previewEligible,
    progress,
    secondary: { kind: "clip-metadata", sizeBytes: clip.sizeBytes, createdAt: clip.createdAt },
    capabilities: previewEligible
      ? { ...noCapabilities, open: true, preview: true, copyLink: true, openPublicPage: true, rename: true, move: true, moveToTrash: true }
      : active && clip.jobId !== null
        ? { ...noCapabilities, cancelUpload: true }
        : clip.state === "failed" && clip.jobId !== null
          ? { ...noCapabilities, dismissFailure: true }
          : { ...noCapabilities },
  }
}

export function explorerItems(folders: readonly Folder[], clips: readonly ClipSummary[], role: Session["user"]["role"]): ExplorerItem[] {
  return [...folders.map((folder) => folderExplorerItem(folder, role)), ...clips.map(clipExplorerItem)]
}
