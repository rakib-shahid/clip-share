export const explorerSorts = ["latest", "oldest", "name_asc", "name_desc", "size_desc", "size_asc", "state"] as const
export type ExplorerSort = (typeof explorerSorts)[number]
export type ExplorerView = "grid" | "list"

export type Preferences = {
  explorer: { view: ExplorerView; sort: ExplorerSort }
  search: { view: ExplorerView }
  trash: { view: ExplorerView }
}

export type PreferenceField = "explorer.view" | "explorer.sort" | "search.view" | "trash.view"
export type PreferenceListener = (preferences: Preferences, changedFields: readonly PreferenceField[]) => void

type StoredPreferences = Preferences & { version: 1 }
type StorageEvents = Pick<Window, "addEventListener" | "removeEventListener">

const defaults: Preferences = {
  explorer: { view: "grid", sort: "latest" },
  search: { view: "grid" },
  trash: { view: "grid" },
}

export function defaultPreferences(): Preferences {
  return clonePreferences(defaults)
}

export function preferenceStorageKey(actingUserId: number): string {
  return `clip-share.preferences.v1.${actingUserId}`
}

export function createPreferenceStore(
  actingUserId: number,
  environment: { storage?: Storage | null; events?: StorageEvents | null } = {},
) {
  const key = preferenceStorageKey(actingUserId)
  const storage = environment.storage === undefined ? browserStorage() : environment.storage
  const events = environment.events === undefined ? browserEvents() : environment.events
  let current = readPreferences(storage, key)
  const listeners = new Set<PreferenceListener>()
  let listening = false

  const onStorage = (event: Event) => {
    const storageEvent = event as StorageEvent
    if (storageEvent.key !== key || storageEvent.newValue === null) return
    const next = parsePreferences(storageEvent.newValue)
    if (!next) return
    const changed = changedFields(current, next)
    if (changed.length === 0) return
    current = next
    notify(listeners, current, changed)
  }
  const startListening = () => {
    if (listening) return
    try {
      events?.addEventListener("storage", onStorage)
      listening = events !== null
    } catch {
      // Browser privacy policies may deny event registration.
    }
  }
  const stopListening = () => {
    if (!listening) return
    try {
      events?.removeEventListener("storage", onStorage)
    } catch {
      // A failed cleanup must not make unmounting fail.
    }
    listening = false
  }

  return {
    key,
    get(): Preferences {
      return clonePreferences(current)
    },
    update(patch: Partial<{ explorer: Partial<Preferences["explorer"]>; search: Partial<Preferences["search"]>; trash: Partial<Preferences["trash"]> }>): readonly PreferenceField[] {
      const next = applyPatch(current, patch)
      const changed = changedFields(current, next)
      if (changed.length === 0) return changed
      current = next
      try {
        storage?.setItem(key, JSON.stringify(toStored(current)))
      } catch {
        // Keep the valid preference in memory when persistence is unavailable.
      }
      notify(listeners, current, changed)
      return changed
    },
    subscribe(listener: PreferenceListener): () => void {
      listeners.add(listener)
      if (listeners.size === 1) startListening()
      return () => {
        listeners.delete(listener)
        if (listeners.size === 0) stopListening()
      }
    },
    destroy(): void {
      listeners.clear()
      stopListening()
    },
  }
}

function browserStorage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage
  } catch {
    return null
  }
}

function browserEvents(): StorageEvents | null {
  return typeof window === "undefined" ? null : window
}

function readPreferences(storage: Storage | null, key: string): Preferences {
  try {
    const raw = storage?.getItem(key)
    return raw ? parsePreferences(raw) ?? defaultPreferences() : defaultPreferences()
  } catch {
    return defaultPreferences()
  }
}

function parsePreferences(raw: string): Preferences | null {
  try {
    const value: unknown = JSON.parse(raw)
    if (!isRecord(value) || value.version !== 1) return null
    const explorer = isRecord(value.explorer) ? value.explorer : {}
    const search = isRecord(value.search) ? value.search : {}
    const trash = isRecord(value.trash) ? value.trash : {}
    return {
      explorer: {
        view: isView(explorer.view) ? explorer.view : defaults.explorer.view,
        sort: isSort(explorer.sort) ? explorer.sort : defaults.explorer.sort,
      },
      search: { view: isView(search.view) ? search.view : defaults.search.view },
      trash: { view: isView(trash.view) ? trash.view : defaults.trash.view },
    }
  } catch {
    return null
  }
}

function applyPatch(current: Preferences, patch: Partial<{ explorer: Partial<Preferences["explorer"]>; search: Partial<Preferences["search"]>; trash: Partial<Preferences["trash"]> }>): Preferences {
  const explorer = isRecord(patch.explorer) ? patch.explorer : {}
  const search = isRecord(patch.search) ? patch.search : {}
  const trash = isRecord(patch.trash) ? patch.trash : {}
  return {
    explorer: {
      view: isView(explorer.view) ? explorer.view : current.explorer.view,
      sort: isSort(explorer.sort) ? explorer.sort : current.explorer.sort,
    },
    search: { view: isView(search.view) ? search.view : current.search.view },
    trash: { view: isView(trash.view) ? trash.view : current.trash.view },
  }
}

function changedFields(previous: Preferences, next: Preferences): PreferenceField[] {
  const changed: PreferenceField[] = []
  if (previous.explorer.view !== next.explorer.view) changed.push("explorer.view")
  if (previous.explorer.sort !== next.explorer.sort) changed.push("explorer.sort")
  if (previous.search.view !== next.search.view) changed.push("search.view")
  if (previous.trash.view !== next.trash.view) changed.push("trash.view")
  return changed
}

function notify(listeners: Set<PreferenceListener>, preferences: Preferences, changed: readonly PreferenceField[]) {
  for (const listener of listeners) listener(clonePreferences(preferences), changed)
}

function toStored(preferences: Preferences): StoredPreferences {
  return { version: 1, ...clonePreferences(preferences) }
}

function clonePreferences(preferences: Preferences): Preferences {
  return {
    explorer: { ...preferences.explorer },
    search: { ...preferences.search },
    trash: { ...preferences.trash },
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function isView(value: unknown): value is ExplorerView {
  return value === "grid" || value === "list"
}

function isSort(value: unknown): value is ExplorerSort {
  return typeof value === "string" && (explorerSorts as readonly string[]).includes(value)
}
