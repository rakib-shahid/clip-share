import type { FolderPage, JobStatus } from "@/api"
import type { ExplorerSort } from "./explorer-preferences"

export function activeJobIDChunks(page: FolderPage, maximum = 100): number[][] {
  const ids = page.clips.flatMap((clip) => clip.jobId && ["queued", "processing", "validating"].includes(clip.state) ? [clip.jobId] : [])
  const chunks: number[][] = []
  for (let index = 0; index < ids.length; index += maximum) chunks.push(ids.slice(index, index + maximum))
  return chunks
}

export function jobStatusesChangeSortKey(sort: ExplorerSort, page: FolderPage, statuses: readonly JobStatus[]): boolean {
  return statuses.some((status) => {
    const clip = page.clips.find((candidate) => candidate.jobId === status.jobId)
    if (!clip) return false
    if (sort === "size_desc" || sort === "size_asc") return clip.sizeBytes !== status.sizeBytes
    if (sort === "state") return clip.state !== status.state
    return false
  })
}

export function mergeJobStatuses(page: FolderPage, statuses: readonly JobStatus[]): FolderPage {
  const byID = new Map(statuses.map((status) => [status.jobId, status]))
  return { ...page, clips: page.clips.map((clip) => {
    const status = clip.jobId ? byID.get(clip.jobId) : undefined
    return status ? { ...clip, state: status.state, progress: status.progress, errorMessage: status.errorMessage, sizeBytes: status.sizeBytes } : clip
  }) }
}

export function appendFolderPage(current: FolderPage, next: FolderPage): FolderPage {
  const folderIDs = new Set(current.folders.map((folder) => folder.id))
  const clipIDs = new Set(current.clips.map((clip) => clip.id))
  return { ...next, folders: [...current.folders, ...next.folders.filter((folder) => !folderIDs.has(folder.id))], clips: [...current.clips, ...next.clips.filter((clip) => !clipIDs.has(clip.id))] }
}
