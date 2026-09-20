import type { ExplorerSort } from "@/lib/explorer-preferences"

export function folderPageURL(folderId: number, sort: ExplorerSort, cursor?: string | null) {
  const query = new URLSearchParams({ sort })
  if (cursor) query.set("cursor", cursor)
  return `/api/folders/${folderId}?${query.toString()}`
}
