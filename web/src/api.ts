export type User = { id: number; username: string; role: "admin" | "user"; state: "active" | "disabled" | "archived"; storedFileLimitBytes: number; rootFolderId: number; libraryTrashed: boolean }
export type UserStorageSummary = User & { storedBytes: number }
export type Session = { user: User; csrfToken: string; publicBaseURL: string }
export type Folder = { id: number; ownerUserId: number; ownerUsername: string; parentFolderId: number | null; name: string; isRoot: boolean; folderCount: number; clipCount: number }
export type FolderDeletionSummary = { folderCount: number; clipCount: number; totalItems: number; storedBytes: number }
export type ClipSummary = { id: number; title: string; state: string; sizeBytes: number | null; createdAt: string; jobId: number | null; progress: number | null; errorMessage: string | null; publicId: string }
export type FolderPage = { folder: Folder; breadcrumbs: Folder[]; folders: Folder[]; clips: ClipSummary[]; nextCursor: string | null }
export type SearchResult = { kind: "folder" | "clip"; id: number; name: string; ownerUserId: number; ownerUsername: string; folderId: number; path: string; publicId?: string; state?: string; sizeBytes?: number }
type APIError = { error?: { message?: string; conflicts?: Array<{ kind: string; path: string }> } }

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin", ...options, headers: { "Content-Type": "application/json", ...options.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as APIError
    const conflicts = body.error?.conflicts?.map((conflict) => conflict.path).join(", ")
    const message = body.error?.message ?? "Something went wrong."
    throw new Error(conflicts ? `${message} Conflicts: ${conflicts}` : message)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function uploadVideo<T>(form: FormData, csrfToken: string, onProgress: (percent: number) => void, signal?: AbortSignal): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const abortRequest = () => xhr.abort()
    const cleanup = () => signal?.removeEventListener("abort", abortRequest)
    xhr.open("POST", "/api/uploads")
    xhr.responseType = "json"
    xhr.setRequestHeader("X-CSRF-Token", csrfToken)
    xhr.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
    })
    xhr.addEventListener("load", () => {
      cleanup()
      if (xhr.status >= 200 && xhr.status < 300) resolve(xhr.response as T)
      else reject(new Error((xhr.response as APIError | null)?.error?.message ?? "The upload failed."))
    })
    xhr.addEventListener("error", () => { cleanup(); reject(new Error("The upload was interrupted.")) })
    xhr.addEventListener("abort", () => { cleanup(); reject(new DOMException("The upload was cancelled.", "AbortError")) })
    if (signal?.aborted) {
      reject(new DOMException("The upload was cancelled.", "AbortError"))
      return
    }
    signal?.addEventListener("abort", abortRequest, { once: true })
    xhr.send(form)
  })
}
