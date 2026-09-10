import { useEffect, useState, type FormEvent, type ReactNode } from "react"
import { AlertTriangle, ChevronDown, Folder, KeyRound, LogIn, LogOut, Play, Plus, RotateCcw, Search, ShieldCheck, Trash2, Upload, Users, X } from "lucide-react"
import { request, type SearchResult, type Session, type User } from "./api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { LibraryExplorer } from "@/components/library-explorer"
import { ClipPreviewThumbnail, VideoPreviewDialog } from "@/components/clip-preview"
import { UserManagementDialog } from "@/components/user-management-dialog"
import { SearchDialog } from "@/components/search-dialog"
import { Modal } from "@/components/ui/modal"

type View = "loading" | "setup" | "login" | "app"

export default function App() {
  const [view, setView] = useState<View>("loading")
  const [session, setSession] = useState<Session | null>(null)

  useEffect(() => {
    Promise.all([
      request<{ setupRequired: boolean }>("/api/setup/status"),
      request<Session>("/api/auth/me").catch(() => null)
    ]).then(([setup, current]) => {
      if (setup.setupRequired) setView("setup")
      else if (current) { setSession(current); setView("app") }
      else setView("login")
    }).catch(() => setView("login"))
  }, [])

  if (view === "loading") return <Loading />
  if (view === "setup") return <AuthScreen mode="setup" onSuccess={(value) => { setSession(value); setView("app") }} />
  if (view === "login") return <AuthScreen mode="login" onSuccess={(value) => { setSession(value); setView("app") }} />
  if (!session) return null
  return <Shell session={session} onSessionUpdated={(user) => setSession((current) => current ? { ...current, user } : current)} onLogout={() => { setSession(null); setView("login") }} />
}

function Brand() {
  return <a href="/"><div className="flex items-center gap-3"><div className="grid size-10 place-items-center rounded-xl bg-sky-400 text-slate-950"><Upload size={20} strokeWidth={2.5} /></div><div><div className="font-semibold tracking-tight text-white">Clip Share</div><div className="text-xs text-slate-500">Your clips, ready to share.</div></div></div></a>
}

function Loading() { return <main className="grid min-h-screen place-items-center"><div className="size-7 animate-spin rounded-full border-2 border-slate-700 border-t-sky-400" role="status" aria-label="Loading Clip Share" /></main> }

function AuthScreen({ mode, onSuccess }: { mode: "setup" | "login"; onSuccess: (session: Session) => void }) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [setupToken, setSetupToken] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError("")
    try {
      const body = mode === "setup" ? { setupToken, username, password } : { username, password }
      const result = await request<Session>(mode === "setup" ? "/api/setup" : "/api/auth/login", { method: "POST", body: JSON.stringify(body) })
      onSuccess(result)
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Something went wrong.") }
    finally { setBusy(false) }
  }

  return <main className="relative grid min-h-screen place-items-center overflow-hidden px-4 py-12">
    <div className="ambient" />
    <div className="relative w-full max-w-md">
      <div className="mb-8 flex justify-center"><Brand /></div>
      <Card>
        <CardHeader><p className="eyebrow">{mode === "setup" ? "First run" : "Welcome back"}</p><h1 className="mt-2 text-2xl font-semibold tracking-tight text-white">{mode === "setup" ? "Create your admin account" : "Sign in to your library"}</h1><p className="mt-2 text-sm leading-6 text-slate-400">{mode === "setup" ? "Use the setup token from your Docker environment. This page closes after the first account is created." : "Enter the credentials your administrator created for you."}</p></CardHeader>
        <CardContent><form className="space-y-4" onSubmit={submit}>
          {mode === "setup" && <Field label="Setup token"><Input type="password" autoComplete="off" value={setupToken} onChange={(e) => setSetupToken(e.target.value)} required /></Field>}
          <Field label="Username"><Input autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} minLength={3} maxLength={32} required autoFocus={mode === "login"} /></Field>
          <Field label="Password"><Input type="password" autoComplete={mode === "setup" ? "new-password" : "current-password"} value={password} onChange={(e) => setPassword(e.target.value)} minLength={8} maxLength={128} required /></Field>
          {error && <p className="rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">{error}</p>}
          <Button className="w-full" size="lg" variant="success" disabled={busy}>{mode === "setup" ? <ShieldCheck size={18} /> : <LogIn size={18} />}{busy ? "Please wait…" : mode === "setup" ? "Create administrator" : "Sign in"}</Button>
        </form></CardContent>
      </Card>
    </div>
  </main>
}

function Field({ label, children }: { label: string; children: ReactNode }) { return <label className="block"><span className="mb-2 block text-sm font-medium text-slate-300">{label}</span>{children}</label> }

function Shell({ session, onSessionUpdated, onLogout }: { session: Session; onSessionUpdated: (user: User) => void; onLogout: () => void }) {
  const [users, setUsers] = useState<User[]>([])
  const [selectedLibrary, setSelectedLibrary] = useState<User | null>(session.user.role === "user" ? session.user : null)
  const [showCreate, setShowCreate] = useState(false)
  const [error, setError] = useState("")
  const [trashOpen, setTrashOpen] = useState(false)
  const [libraryRefresh, setLibraryRefresh] = useState(0)
  const [manageUsersOpen, setManageUsersOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [accountOpen, setAccountOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [requestedFolder, setRequestedFolder] = useState<{ folderID: number; token: number } | null>(null)

  useEffect(() => {
    if (session.user.role === "admin") request<{ users: User[] }>("/api/users").then((result) => setUsers(result.users)).catch((reason) => setError(reason.message))
  }, [session.user.role])

  async function logout() {
    await request<void>("/api/auth/logout", { method: "POST", headers: { "X-CSRF-Token": session.csrfToken } }).catch(() => undefined)
    onLogout()
  }

  function replaceUser(user: User) {
    setUsers((current) => current.map((candidate) => candidate.id === user.id ? user : candidate))
    setSelectedLibrary((current) => current?.id === user.id ? user : current)
    if (user.id === session.user.id) onSessionUpdated(user)
  }

  function openManagedLibrary(user: User) {
    setRequestedFolder(null)
    setSelectedLibrary(user)
    setManageUsersOpen(false)
  }

  function openSearchResult(result: SearchResult) {
    const owner = (users.length ? users : [session.user]).find((user) => user.id === result.ownerUserId)
    if (!owner) {
      setError("That library is no longer available.")
      return
    }
    setRequestedFolder((current) => ({ folderID: result.folderId, token: (current?.token ?? 0) + 1 }))
    setSelectedLibrary(owner)
    setSearchOpen(false)
  }

  return <div className="min-h-screen">
    <header className="border-b border-white/[.07] bg-slate-950/70 backdrop-blur-xl"><div className="mx-auto flex max-w-7xl items-center justify-between px-5 py-4"><Brand /><div className="flex items-center gap-1 sm:gap-2"><Button variant="ghost" size="sm" onClick={() => setSearchOpen(true)} aria-label="Search libraries"><Search size={16} /><span className="hidden lg:inline">Search</span></Button><Button variant="ghost" size="sm" onClick={() => setTrashOpen(true)} aria-label="Open recycle bin"><Trash2 size={16} /><span className="hidden lg:inline">Recycle bin</span></Button><div className="relative"><Button variant="ghost" size="sm" onClick={() => setAccountOpen((open) => !open)} aria-expanded={accountOpen} aria-haspopup="menu"><span className="hidden text-right sm:block"><span className="block text-sm font-medium text-slate-200">{session.user.username}</span><span className="block text-xs capitalize text-slate-500">{session.user.role}</span></span><ChevronDown size={16} /></Button>{accountOpen && <div className="absolute right-0 z-40 mt-2 w-48 rounded-xl border border-white/[.1] bg-slate-950 p-1 shadow-2xl" role="menu"><Button className="w-full justify-start" size="sm" variant="ghost" role="menuitem" onClick={() => { setAccountOpen(false); setPasswordOpen(true) }}><KeyRound size={15} /> Change password</Button><Button className="w-full justify-start" size="sm" variant="ghost" role="menuitem" onClick={logout}><LogOut size={15} /> Log out</Button></div>}</div></div></div></header>
    <main className="mx-auto max-w-7xl px-5 py-10">
      {selectedLibrary ? <LibraryExplorer session={session} root={selectedLibrary} users={users.length ? users : [session.user]} refreshToken={libraryRefresh} requestedFolder={requestedFolder} onBack={session.user.role === "admin" ? () => { setRequestedFolder(null); setSelectedLibrary(null) } : undefined} /> : <><div className="mb-8 flex flex-col gap-5 sm:flex-row sm:items-end sm:justify-between"><div><p className="eyebrow">All libraries</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">Choose a user space</h1><p className="mt-2 max-w-xl text-sm leading-6 text-slate-400">Open any library to organize its folders. Your personal admin library remains your default upload destination.</p></div><div className="flex flex-wrap gap-2"><Button variant="ghost" onClick={() => setManageUsersOpen(true)}><Users size={17} /> Manage users</Button><Button variant="secondary" onClick={() => setShowCreate(true)}><Plus size={17} /> New user</Button></div></div>{error && <p className="mb-5 text-sm text-red-300">{error}</p>}<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{users.filter((user) => user.state !== "archived").map((user) => <LibraryCard key={user.id} user={user} onOpen={() => { setRequestedFolder(null); setSelectedLibrary(user) }} />)}</div></>}
    </main>
    {showCreate && <CreateUserDialog csrf={session.csrfToken} onClose={() => setShowCreate(false)} onCreated={(user) => { setUsers((current) => [...current, user]); setShowCreate(false) }} />}
    {trashOpen && <TrashDialog session={session} onClose={() => setTrashOpen(false)} onLibraryChanged={() => setLibraryRefresh((current) => current + 1)} />}
    {manageUsersOpen && <UserManagementDialog session={session} onClose={() => setManageUsersOpen(false)} onUserChanged={replaceUser} onOpenLibrary={openManagedLibrary} />}
    {searchOpen && <SearchDialog session={session} onClose={() => setSearchOpen(false)} onNavigate={openSearchResult} />}
    {passwordOpen && <ChangePasswordDialog session={session} onClose={() => setPasswordOpen(false)} />}
  </div>
}

type TrashItem = {
  id: number
  kind: "clip" | "folder"
  name: string
  ownerUserId: number
  ownerUsername: string
  deletedAt: string
  publicId: string | null
}

function ChangePasswordDialog({ session, onClose }: { session: Session; onClose: () => void }) {
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError("")
    try {
      await request<void>("/api/me/password", { method: "POST", headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify({ currentPassword, newPassword, confirmPassword }) })
      setCurrentPassword("")
      setNewPassword("")
      setConfirmPassword("")
      onClose()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not change your password.")
    } finally {
      setBusy(false)
    }
  }

  return <Modal title="Change password" onClose={onClose} dismissible={!busy}>
    <form className="space-y-4" onSubmit={submit}>
      <p className="text-sm leading-6 text-slate-400">Your existing browser sessions will remain signed in until they expire.</p>
      <Field label="Current password"><Input type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} minLength={8} maxLength={128} required autoFocus /></Field>
      <Field label="New password"><Input type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} minLength={8} maxLength={128} required /></Field>
      <Field label="Confirm new password"><Input type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} minLength={8} maxLength={128} required /></Field>
      {error && <p className="text-sm text-red-300" role="alert">{error}</p>}
      <div className="flex justify-end gap-2 pt-2"><Button type="button" variant="danger" disabled={busy} onClick={onClose}><X size={15} /> Cancel</Button><Button variant="success" disabled={busy}><KeyRound size={15} />{busy ? "Saving…" : "Change password"}</Button></div>
    </form>
  </Modal>
}

function TrashDialog({ session, onClose, onLibraryChanged }: { session: Session; onClose: () => void; onLibraryChanged: () => void }) {
  const [items, setItems] = useState<TrashItem[]>([])
  const [loading, setLoading] = useState(true)
  const [busyKey, setBusyKey] = useState("")
  const [error, setError] = useState("")
  const [preview, setPreview] = useState<TrashItem | null>(null)
  const [purging, setPurging] = useState<TrashItem | null>(null)

  useEffect(() => {
    request<{ items: TrashItem[] }>("/api/trash")
      .then((result) => setItems(result.items))
      .catch((reason) => setError(reason instanceof Error ? reason.message : "Could not load the recycle bin."))
      .finally(() => setLoading(false))
  }, [])

  async function restore(item: TrashItem) {
    const key = `${item.kind}-${item.id}`
    setBusyKey(key); setError("")
    try {
      await request(`/api/trash/${item.kind}/${item.id}/restore`, { method: "POST", headers: { "X-CSRF-Token": session.csrfToken } })
      setItems((current) => current.filter((candidate) => `${candidate.kind}-${candidate.id}` !== key))
      onLibraryChanged()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not restore this item.")
    } finally {
      setBusyKey("")
    }
  }

  function removeItem(item: TrashItem) {
    setItems((current) => current.filter((candidate) => candidate.kind !== item.kind || candidate.id !== item.id))
    setPurging(null)
  }

  return <>
    <Modal title="Recycle bin" onClose={onClose} className="max-h-[90vh] max-w-4xl">
      <p className="eyebrow">Recoverable storage</p><p className="mb-5 mt-2 text-sm text-slate-400">You can restore your items for 30 days. The administrator retains them for 90 days.</p>
      <div className="max-h-[70vh] overflow-y-auto pr-1">
          {error && <p className="mb-4 rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">{error}</p>}
          {loading ? <p className="py-10 text-center text-sm text-slate-500">Loading recycle bin…</p> : items.length === 0 ? <div className="grid min-h-48 place-items-center text-center"><div><Trash2 className="mx-auto text-slate-700" size={32} /><p className="mt-3 text-sm text-slate-500">Recycle bin is empty.</p></div></div> :
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {items.map((item) => {
                const key = `${item.kind}-${item.id}`
                return <article key={key} className="overflow-hidden rounded-xl border border-white/[.08] bg-slate-950">
                  {item.kind === "clip" ? <ClipPreviewThumbnail title={item.name} posterSrc={`/api/trash/clip/${item.id}/poster`} onPreview={() => setPreview(item)} /> : <div className="grid aspect-video place-items-center bg-slate-900"><Folder size={38} className="text-sky-300" /></div>}
                  <div className="p-3"><p className="text-[11px] font-semibold uppercase tracking-wide text-sky-300">{item.kind}</p><h3 className="mt-1 truncate font-medium text-white">{item.name}</h3>{session.user.role === "admin" && <p className="mt-1 truncate text-xs text-slate-500">Owner: {item.ownerUsername}</p>}<p className="mt-1 text-xs text-slate-600">Deleted {new Date(item.deletedAt).toLocaleString()}</p></div>
                  <div className="flex flex-wrap gap-1 border-t border-white/[.06] bg-slate-950 px-2 py-2">
                    {item.kind === "clip" && <Button size="sm" variant="secondary" onClick={() => setPreview(item)}><Play size={14} /> Preview</Button>}
                    <Button size="sm" variant="success" disabled={busyKey === key} onClick={() => restore(item)}><RotateCcw size={14} />{busyKey === key ? "Restoring…" : "Restore"}</Button>
                    {session.user.role === "admin" && <Button size="sm" variant="danger" onClick={() => setPurging(item)}><Trash2 size={14} /> Delete permanently</Button>}
                  </div>
                </article>
              })}
            </div>}
      </div>
    </Modal>
    {preview && <VideoPreviewDialog title={preview.name} posterSrc={`/api/trash/clip/${preview.id}/poster`} videoSrc={`/api/trash/clip/${preview.id}/video`} nested onClose={() => setPreview(null)} />}
    {purging && <PurgeTrashDialog item={purging} session={session} onClose={() => setPurging(null)} onPurged={() => removeItem(purging)} />}
  </>
}

function PurgeTrashDialog({ item, session, onClose, onPurged }: { item: TrashItem; session: Session; onClose: () => void; onPurged: () => void }) {
  const [acknowledged, setAcknowledged] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  async function purge() {
    setBusy(true); setError("")
    try {
      await request(`/api/trash/${item.kind}/${item.id}`, { method: "DELETE", headers: { "X-CSRF-Token": session.csrfToken } })
      onPurged()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not permanently delete this item.")
    } finally {
      setBusy(false)
    }
  }
  return <Modal title="Delete permanently?" onClose={onClose} nested dismissible={!busy}><div className="flex items-start gap-3"><div className="grid size-10 shrink-0 place-items-center rounded-xl bg-red-400/10 text-red-300"><AlertTriangle size={20} /></div><p className="text-sm leading-6 text-slate-400">“{item.name}” and all media it contains will be removed immediately. This cannot be undone.</p></div><label className="mt-5 flex items-start gap-3 rounded-lg border border-white/[.08] bg-slate-950 p-3 text-sm text-slate-300"><input className="mt-1 size-4 accent-red-400" type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} /> I understand this deletion is permanent and cannot be recovered.</label>{error && <p className="mt-3 text-sm text-red-300" role="alert">{error}</p>}<div className="mt-5 flex justify-end gap-2"><Button variant="secondary" disabled={busy} onClick={onClose}><X size={15} /> Back</Button><Button variant="danger" disabled={!acknowledged || busy} onClick={purge}><Trash2 size={16} />{busy ? "Deleting…" : "Delete permanently"}</Button></div></Modal>
}

function LibraryCard({ user, onOpen }: { user: User; onOpen: () => void }) {
  return <button className="rounded-2xl text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400" onClick={onOpen}><Card className="group p-5 transition hover:border-sky-400/30 hover:bg-slate-900"><div className="mb-8 flex items-start justify-between"><div className="grid size-12 place-items-center rounded-xl border border-sky-400/15 bg-sky-400/10 text-sky-300">{user.role === "admin" ? <ShieldCheck /> : <Folder />}</div><span className="rounded-full border border-white/10 px-2.5 py-1 text-[11px] font-medium capitalize text-slate-400">{user.state}</span></div><h2 className="font-semibold text-white">{user.username}</h2><p className="mt-1 text-sm text-slate-500">{Math.round(user.storedFileLimitBytes / 1_000_000)} MB per stored clip</p></Card></button>
}

function CreateUserDialog({ csrf, onClose, onCreated }: { csrf: string; onClose: () => void; onCreated: (user: User) => void }) {
  const [username, setUsername] = useState(""); const [password, setPassword] = useState(""); const [limit, setLimit] = useState(50); const [error, setError] = useState(""); const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) { event.preventDefault(); setBusy(true); setError(""); try { const user = await request<User>("/api/users", { method: "POST", headers: { "X-CSRF-Token": csrf }, body: JSON.stringify({ username, password, storedFileLimitMb: limit }) }); onCreated(user) } catch (reason) { setError(reason instanceof Error ? reason.message : "Something went wrong.") } finally { setBusy(false) } }
  return <Modal title="Create a user" onClose={onClose} dismissible={!busy} className="max-w-md"><p className="eyebrow mb-2">Administrator</p><p className="mb-5 text-sm text-slate-400">Set their login and maximum stored size per clip.</p><form className="space-y-4" onSubmit={submit}><Field label="Username"><Input value={username} onChange={(e) => setUsername(e.target.value)} minLength={3} maxLength={32} required autoFocus /></Field><Field label="Password"><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} minLength={8} maxLength={128} required /></Field><Field label="Stored limit (MB)"><Input type="number" min={1} max={500} value={limit} onChange={(e) => setLimit(Number(e.target.value))} required /></Field>{error && <p className="text-sm text-red-300" role="alert">{error}</p>}<div className="flex justify-end gap-2 pt-2"><Button type="button" variant="danger" disabled={busy} onClick={onClose}><X size={15} /> Cancel</Button><Button variant="success" disabled={busy}><Users size={17} />{busy ? "Creating…" : "Create user"}</Button></div></form></Modal>
}
