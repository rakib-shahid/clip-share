import { describe, expect, it } from "vitest"
import type { ClipSummary, FolderPage, JobStatus } from "@/api"
import { activeJobIDChunks, appendFolderPage, jobStatusesChangeSortKey, mergeJobStatuses } from "./explorer-coordination"

const clip = (id: number, state = "processing", sizeBytes: number | null = null): ClipSummary => ({ id, title: `Clip ${id}`, state, sizeBytes, createdAt: "2026-01-01T00:00:00Z", jobId: id, progress: null, errorMessage: null, publicId: `p${id}` })
const page = (clips: ClipSummary[]): FolderPage => ({ folder: { id: 1, ownerUserId: 1, ownerUsername: "alice", parentFolderId: null, name: "Library", isRoot: true, folderCount: 0, clipCount: clips.length }, breadcrumbs: [], folders: [], clips, nextCursor: null, totalFolderCount: 0, totalClipCount: clips.length, totalItemCount: clips.length })

describe("explorer coordination", () => {
  it("chunks active jobs at 100 and excludes terminal jobs", () => {
    const chunks = activeJobIDChunks(page([...Array.from({ length: 205 }, (_, index) => clip(index + 1)), clip(999, "ready")]))
    expect(chunks.map((chunk) => chunk.length)).toEqual([100, 100, 5])
  })

  it("resets only when the authoritative selected sort key changes", () => {
    const current = page([clip(1, "processing", null)])
    const status: JobStatus = { jobId: 1, clipId: 1, state: "ready", progress: 100, errorMessage: null, sizeBytes: 500 }
    expect(jobStatusesChangeSortKey("latest", current, [status])).toBe(false)
    expect(jobStatusesChangeSortKey("name_asc", current, [status])).toBe(false)
    expect(jobStatusesChangeSortKey("state", current, [status])).toBe(true)
    expect(jobStatusesChangeSortKey("size_desc", current, [status])).toBe(true)
    expect(mergeJobStatuses(current, [status]).clips[0]).toMatchObject({ state: "ready", progress: 100, sizeBytes: 500 })
  })

  it("deduplicates 240 accumulated items while preserving page order", () => {
    const first = page(Array.from({ length: 180 }, (_, index) => clip(index + 1)))
    const next = page(Array.from({ length: 61 }, (_, index) => clip(index + 180)))
    const merged = appendFolderPage(first, next)
    expect(merged.clips).toHaveLength(240)
    expect(merged.clips.map((item) => item.id)).toEqual(Array.from({ length: 240 }, (_, index) => index + 1))
  })
})
