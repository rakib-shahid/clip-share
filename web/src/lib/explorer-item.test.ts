import { describe, expect, it } from "vitest"
import type { ClipSummary, Folder } from "@/api"
import { clipExplorerItem, explorerItems, folderExplorerItem } from "./explorer-item"

const folder: Folder = { id: 7, ownerUserId: 2, ownerUsername: "alice", parentFolderId: 1, name: "Videos", isRoot: false, folderCount: 3, clipCount: 4 }
const clip: ClipSummary = { id: 7, title: "Clip", state: "ready", sizeBytes: 120, createdAt: "2026-09-12T12:00:00Z", jobId: null, progress: null, errorMessage: null, publicId: "public" }

describe("explorer item model", () => {
  it("maps folders without losing identity, ownership, or direct counts", () => {
    expect(folderExplorerItem(folder, "user")).toMatchObject({
      kind: "folder", key: "folder:7", id: 7, name: "Videos", ownerUserId: 2, ownerUsername: "alice",
      parentFolderId: 1, folderCount: 3, clipCount: 4, previewEligible: false,
      secondary: { kind: "folder-counts", folderCount: 3, clipCount: 4 },
      capabilities: { open: true, rename: true, move: true, copy: false, moveToTrash: true },
    })
    expect(folderExplorerItem(folder, "admin").capabilities.copy).toBe(true)
    expect(folderExplorerItem({ ...folder, isRoot: true }, "admin").capabilities).toMatchObject({ rename: false, move: false, copy: false, moveToTrash: false })
  })

  it.each([
    ["uploading", "Uploading", "active"],
    ["queued", "Queued for processing", "active"],
    ["validating", "Validating upload", "active"],
    ["processing", "Processing video", "active"],
    ["failed", "Upload failed", "failed"],
    ["ready", "Ready", "ready"],
    ["cancelled", "Cancelled", "muted"],
    ["unavailable", "Unavailable", "muted"],
    ["future-state", "Unknown status", "unknown"],
  ])("maps %s to a safe visible status", (state, label, tone) => {
    const item = clipExplorerItem({ ...clip, state })
    expect(item.status).toEqual({ label, tone })
    expect(item.previewEligible).toBe(state === "ready")
    if (state === "future-state") expect(item.capabilities).toEqual(expect.objectContaining({ preview: false, cancelUpload: false, dismissFailure: false, moveToTrash: false }))
  })

  it("derives ready, active, and failed capabilities from state and job identity", () => {
    expect(clipExplorerItem(clip).capabilities).toMatchObject({ open: true, preview: true, copyLink: true, openPublicPage: true, rename: true, move: true, moveToTrash: true })
    expect(clipExplorerItem({ ...clip, state: "queued", jobId: 20, progress: null })).toMatchObject({ progress: { kind: "indeterminate", value: null }, capabilities: { cancelUpload: true } })
    expect(clipExplorerItem({ ...clip, state: "processing", jobId: 20, progress: 42 })).toMatchObject({ progress: { kind: "determinate", value: 42 }, progressValue: 42, capabilities: { cancelUpload: true } })
    expect(clipExplorerItem({ ...clip, state: "processing", jobId: null, progress: null }).capabilities.cancelUpload).toBe(false)
    expect(clipExplorerItem({ ...clip, state: "failed", jobId: 21 }).capabilities.dismissFailure).toBe(true)
    expect(clipExplorerItem({ ...clip, state: "failed", jobId: null }).capabilities.dismissFailure).toBe(false)
  })

  it("preserves null metadata, creates kind-prefixed keys, and never mutates inputs", () => {
    const originalFolder = structuredClone(folder)
    const originalClip = structuredClone(clip)
    const items = explorerItems([folder], [{ ...clip, sizeBytes: null, createdAt: "", progress: null }], "user")
    expect(items.map((item) => item.key)).toEqual(["folder:7", "clip:7"])
    expect(items[1]).toMatchObject({ sizeBytes: null, createdAt: "", jobId: null, progressValue: null })
    expect(folder).toEqual(originalFolder)
    expect(clip).toEqual(originalClip)
  })
})
