import { describe, expect, it, vi } from "vitest"
import { createPreferenceStore, defaultPreferences, preferenceStorageKey } from "./explorer-preferences"

function memoryStorage(initial: Record<string, string> = {}): Storage {
  const values = new Map(Object.entries(initial))
  return {
    get length() { return values.size },
    clear: () => values.clear(),
    getItem: (key) => values.get(key) ?? null,
    key: (index) => [...values.keys()][index] ?? null,
    removeItem: (key) => { values.delete(key) },
    setItem: (key, value) => { values.set(key, value) },
  }
}

describe("explorer preferences", () => {
  it("uses actor-scoped defaults and round trips valid independent settings", () => {
    const storage = memoryStorage()
    const first = createPreferenceStore(41, { storage, events: null })
    expect(first.get()).toEqual(defaultPreferences())
    expect(first.update({ explorer: { view: "list", sort: "size_desc" }, search: { view: "list" } })).toEqual([
      "explorer.view", "explorer.sort", "search.view",
    ])
    expect(JSON.parse(storage.getItem(preferenceStorageKey(41))!)).toEqual({
      version: 1, explorer: { view: "list", sort: "size_desc" }, search: { view: "list" }, trash: { view: "grid" },
    })
    expect(createPreferenceStore(41, { storage, events: null }).get().explorer).toEqual({ view: "list", sort: "size_desc" })
    expect(createPreferenceStore(42, { storage, events: null }).get()).toEqual(defaultPreferences())
  })

  it("defaults malformed and legacy data while preserving valid version-one fields", () => {
    const key = preferenceStorageKey(7)
    for (const raw of ["not json", JSON.stringify({ version: 0, explorer: { view: "list" } }), JSON.stringify([1, 2])]) {
      expect(createPreferenceStore(7, { storage: memoryStorage({ [key]: raw }), events: null }).get()).toEqual(defaultPreferences())
    }
    const partial = `{"version":1,"explorer":{"view":"list","sort":"bogus"},"search":null,"trash":{"view":"list"},"__proto__":{"polluted":true}}`
    expect(createPreferenceStore(7, { storage: memoryStorage({ [key]: partial }), events: null }).get()).toEqual({
      explorer: { view: "list", sort: "latest" }, search: { view: "grid" }, trash: { view: "list" },
    })
    expect(({} as { polluted?: boolean }).polluted).toBeUndefined()
  })

  it("survives denied reads and writes while retaining valid updates in memory", () => {
    const denied = memoryStorage()
    denied.getItem = () => { throw new Error("denied") }
    denied.setItem = () => { throw new Error("quota") }
    const store = createPreferenceStore(3, { storage: denied, events: null })
    expect(store.get()).toEqual(defaultPreferences())
    store.update({ explorer: { view: "list" } })
    expect(store.get().explorer.view).toBe("list")
  })

  it("applies valid cross-tab updates without echoing and reports sort separately from view", () => {
    const storage = memoryStorage()
    const events = new EventTarget()
    const setItem = vi.spyOn(storage, "setItem")
    const listener = vi.fn()
    const store = createPreferenceStore(9, { storage, events: events as unknown as Window })
    const unsubscribe = store.subscribe(listener)
    events.dispatchEvent(new StorageEvent("storage", {
      key: preferenceStorageKey(9),
      newValue: JSON.stringify({ version: 1, explorer: { view: "list", sort: "oldest" }, search: { view: "grid" }, trash: { view: "grid" } }),
    }))
    expect(listener).toHaveBeenCalledWith(expect.objectContaining({ explorer: { view: "list", sort: "oldest" } }), ["explorer.view", "explorer.sort"])
    expect(setItem).not.toHaveBeenCalled()

    events.dispatchEvent(new StorageEvent("storage", { key: preferenceStorageKey(10), newValue: "{}" }))
    events.dispatchEvent(new StorageEvent("storage", { key: preferenceStorageKey(9), newValue: JSON.stringify({ version: 2 }) }))
    expect(listener).toHaveBeenCalledTimes(1)
    unsubscribe()
    store.destroy()
    events.dispatchEvent(new StorageEvent("storage", {
      key: preferenceStorageKey(9),
      newValue: JSON.stringify({ version: 1, explorer: { view: "grid", sort: "latest" }, search: { view: "grid" }, trash: { view: "grid" } }),
    }))
    expect(listener).toHaveBeenCalledTimes(1)
  })
})
