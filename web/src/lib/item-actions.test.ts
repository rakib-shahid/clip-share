import { describe, expect, it, vi } from "vitest"
import { clipExplorerItem, folderExplorerItem } from "./explorer-item"
import { itemActions, type ItemActionHandlers } from "./item-actions"

const handlers = new Proxy({}, { get: () => vi.fn() }) as ItemActionHandlers

describe("itemActions", () => {
  it("uses the accepted action matrix", () => {
    const folder = { id: 1, ownerUserId: 1, ownerUsername: "owner", parentFolderId: 2, name: "Folder", isRoot: false, folderCount: 0, clipCount: 0 }
    expect(itemActions(folderExplorerItem(folder, "user"), handlers).map((action) => action.id)).toEqual(["open", "rename", "move", "trash"])
    expect(itemActions(folderExplorerItem(folder, "admin"), handlers).map((action) => action.id)).toEqual(["open", "rename", "move", "copy", "trash"])
    const clip = { id: 2, title: "Clip", state: "ready", sizeBytes: 4, createdAt: "2026-01-01T00:00:00Z", jobId: 3, progress: 100, errorMessage: null, publicId: "public" }
    expect(itemActions(clipExplorerItem(clip), handlers).map((action) => action.id)).toEqual(["preview", "copy-link", "public-page", "rename", "move", "trash"])
    expect(itemActions(clipExplorerItem({ ...clip, state: "processing", progress: 50 }), handlers).map((action) => action.id)).toEqual(["cancel"])
    expect(itemActions(clipExplorerItem({ ...clip, state: "failed" }), handlers).map((action) => action.id)).toEqual(["dismiss"])
  })
})
