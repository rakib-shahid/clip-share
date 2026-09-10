import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react"
import { Archive, ArrowLeft, ExternalLink, KeyRound, Pencil, Power, RefreshCw, RotateCcw, ShieldCheck, Trash2, UserRound } from "lucide-react"
import { request, type Session, type User, type UserStorageSummary } from "@/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Modal } from "@/components/ui/modal"

type Action = { type: "edit" | "password" | "disable" | "enable" | "archive" | "restore" | "delete-library"; user: User }

type Props = {
  session: Session
  onClose: () => void
  onUserChanged: (user: User) => void
  onOpenLibrary: (user: User) => void
}

export function UserManagementDialog({ session, onClose, onUserChanged, onOpenLibrary }: Props) {
  const [action, setAction] = useState<Action | null>(null)
  const [users, setUsers] = useState<UserStorageSummary[] | null>(null)
  const [error, setError] = useState("")
  const load = useCallback(async () => {
    setUsers(null)
    setError("")
    try {
      const result = await request<{ users: UserStorageSummary[] }>("/api/users")
      setUsers(result.users)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not load user storage.")
    }
  }, [])
  useEffect(() => { void load() }, [load])
  const active = users?.filter((user) => user.state !== "archived") ?? []
  const archived = users?.filter((user) => user.state === "archived") ?? []

  return <>
    <Modal title="Manage users" onClose={onClose} className="max-h-[90vh] max-w-5xl">
      <div className="max-h-[72vh] overflow-y-auto pr-1">
        <p className="mb-5 text-sm leading-6 text-slate-400">Change login settings and account access. Public clip links and libraries remain intact when an account is disabled or archived.</p>
        {!users && !error && <p className="py-10 text-center text-sm text-slate-500" role="status">Loading user storage…</p>}
        {error && <div className="rounded-xl border border-red-400/20 bg-red-400/10 p-4"><p className="text-sm text-red-200" role="alert">{error}</p><Button className="mt-3" size="sm" variant="secondary" onClick={() => void load()}><RefreshCw size={14} /> Retry</Button></div>}
        {users && <><UserList users={active} onAction={(type, user) => setAction({ type, user })} onOpenLibrary={onOpenLibrary} />
        {archived.length > 0 && <section className="mt-8 border-t border-white/[.08] pt-6">
          <p className="eyebrow">Archived users</p>
          <p className="mb-4 mt-2 text-sm text-slate-500">These users cannot log in, but their retained libraries remain available to you.</p>
          <UserList users={archived} onAction={(type, user) => setAction({ type, user })} onOpenLibrary={onOpenLibrary} />
        </section>}</>}
      </div>
    </Modal>
    {action && <UserActionDialog action={action} session={session} onClose={() => setAction(null)} onChanged={(user) => { onUserChanged(user); setAction(null); void load() }} />}
  </>
}

function UserList({ users, onAction, onOpenLibrary }: { users: UserStorageSummary[]; onAction: (type: Action["type"], user: User) => void; onOpenLibrary: (user: User) => void }) {
  return <div className="space-y-3">
    {users.map((user) => <article key={user.id} className="rounded-xl border border-white/[.08] bg-slate-950/70 p-4">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid size-10 shrink-0 place-items-center rounded-xl border border-sky-400/15 bg-sky-400/10 text-sky-300">{user.role === "admin" ? <ShieldCheck size={19} /> : <UserRound size={19} />}</div>
          <div className="min-w-0"><div className="flex items-center gap-2"><h3 className="truncate font-medium text-white">{user.username}</h3><StateBadge state={user.state} /></div><p className="mt-1 text-xs text-slate-500">{Math.round(user.storedFileLimitBytes / 1_000_000)} MB per stored clip · {user.role === "admin" ? "Super-admin" : "User"} · {formatStoredBytes(user.storedBytes)} used</p></div>
        </div>
        <div className="flex flex-wrap gap-1">
          <Button size="sm" variant="secondary" onClick={() => onAction("edit", user)}><Pencil size={14} /> Edit</Button>
          <Button size="sm" variant="secondary" onClick={() => onAction("password", user)}><KeyRound size={14} /> Reset password</Button>
          {user.role !== "admin" && user.state === "active" && <Button size="sm" variant="danger" onClick={() => onAction("disable", user)}><Power size={14} /> Disable</Button>}
          {user.role !== "admin" && user.state === "disabled" && <Button size="sm" variant="success" onClick={() => onAction("enable", user)}><Power size={14} /> Enable</Button>}
          {user.role !== "admin" && user.state !== "archived" && <Button size="sm" variant="warning" onClick={() => onAction("archive", user)}><Archive size={14} /> Archive</Button>}
          {user.state === "archived" && !user.libraryTrashed && <Button size="sm" variant="success" onClick={() => onAction("restore", user)}><RotateCcw size={14} /> Restore</Button>}
          {user.state === "archived" && !user.libraryTrashed && <Button size="sm" variant="danger" onClick={() => onAction("delete-library", user)}><Trash2 size={14} /> Delete library</Button>}
          {!user.libraryTrashed && <Button size="sm" variant="secondary" onClick={() => onOpenLibrary(user)}><ExternalLink size={14} /> Open library</Button>}
          {user.libraryTrashed && <span className="px-3 py-2 text-xs text-amber-200">Library is in recycle bin</span>}
        </div>
      </div>
    </article>)}
  </div>
}

function formatStoredBytes(bytes: number) {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB"]
  let value = bytes
  let unit = 0
  while (value >= 1000 && unit < units.length - 1) { value /= 1000; unit++ }
  const digits = unit === 0 ? 0 : unit === 3 ? 2 : 1
  return `${value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d)0$/, "$1")} ${units[unit]}`
}

function StateBadge({ state }: { state: User["state"] }) {
  const tone = state === "active" ? "border-emerald-400/20 bg-emerald-400/10 text-emerald-200" : state === "disabled" ? "border-amber-400/20 bg-amber-400/10 text-amber-200" : "border-slate-400/20 bg-slate-400/10 text-slate-300"
  return <span className={`rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${tone}`}>{state}</span>
}

function UserActionDialog({ action, session, onClose, onChanged }: { action: Action; session: Session; onClose: () => void; onChanged: (user: User) => void }) {
  const [username, setUsername] = useState(action.user.username)
  const [limit, setLimit] = useState(Math.round(action.user.storedFileLimitBytes / 1_000_000))
  const [password, setPassword] = useState("")
  const [acknowledged, setAcknowledged] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const title = action.type === "edit" ? `Edit ${action.user.username}` : action.type === "password" ? `Reset ${action.user.username}'s password` : action.type === "archive" ? `Archive ${action.user.username}?` : action.type === "restore" ? `Restore ${action.user.username}` : action.type === "delete-library" ? `Delete ${action.user.username}'s retained library?` : action.type === "enable" ? `Enable ${action.user.username}?` : `Disable ${action.user.username}?`

  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError("")
    try {
      const base = `/api/users/${action.user.id}`
      if (action.type === "password") {
        await request<void>(`${base}/password`, mutation(session, { password }))
        onChanged(action.user)
        return
      }
      if (action.type === "delete-library") {
        await request<void>(`${base}/library`, mutation(session, {}, "DELETE"))
        onChanged({ ...action.user, libraryTrashed: true })
        return
      }
      const endpoint = action.type
      const body = action.type === "edit" ? { username, storedFileLimitMb: limit } : action.type === "restore" ? { password } : {}
      const user = await request<User>(action.type === "edit" ? base : `${base}/${endpoint}`, mutation(session, body, action.type === "edit" ? "PATCH" : "POST"))
      onChanged(user)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not update this user.")
    } finally {
      setBusy(false)
    }
  }

  return <Modal title={title} onClose={onClose} nested dismissible={!busy}>
    <form className="space-y-4" onSubmit={submit}>
      {action.type === "edit" && <><UserField label="Username"><Input value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={32} required autoFocus /></UserField><UserField label="Stored limit (MB)"><Input type="number" value={limit} onChange={(event) => setLimit(Number(event.target.value))} min={1} max={500} required /></UserField></>}
      {(action.type === "password" || action.type === "restore") && <><p className="text-sm leading-6 text-slate-400">{action.type === "restore" ? "Set a new password before restoring login access." : "The old password will stop working. Existing browser sessions are not signed out."}</p><UserField label="New password"><Input type="password" autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} minLength={8} maxLength={128} required autoFocus /></UserField></>}
      {(action.type === "disable" || action.type === "enable") && <p className="text-sm leading-6 text-slate-400">{action.type === "enable" ? "This user will be able to sign in and use their existing library again." : "This immediately blocks login and any already-open session. Their library and public clip links stay available."}</p>}
      {action.type === "archive" && <><p className="text-sm leading-6 text-slate-400">This immediately removes login access and moves the account out of the main library chooser. The library and public links are retained.</p><label className="flex items-start gap-3 rounded-lg border border-white/[.08] bg-slate-950 p-3 text-sm text-slate-300"><input className="mt-1 size-4 accent-sky-400" type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} /> I understand this account will be archived and unable to log in.</label></>}
      {action.type === "delete-library" && <><p className="text-sm leading-6 text-slate-400">This moves the entire retained library to the administrator recycle bin and immediately disables every public clip link. It can be restored there for 90 days.</p><label className="flex items-start gap-3 rounded-lg border border-red-400/20 bg-red-400/10 p-3 text-sm text-red-100"><input className="mt-1 size-4 accent-red-400" type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} /> I understand the library will no longer be available or publicly playable.</label></>}
      {error && <p className="rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200" role="alert">{error}</p>}
      <div className="flex justify-end gap-2 pt-2"><Button type="button" variant="secondary" disabled={busy} onClick={onClose}><ArrowLeft size={15} /> Back</Button><Button variant={action.type === "archive" ? "warning" : action.type === "delete-library" || action.type === "disable" ? "danger" : action.type === "restore" || action.type === "enable" || action.type === "edit" || action.type === "password" ? "success" : "default"} disabled={busy || ((action.type === "archive" || action.type === "delete-library") && !acknowledged)}>{action.type === "edit" ? <Pencil size={15} /> : action.type === "password" ? <KeyRound size={15} /> : action.type === "archive" ? <Archive size={15} /> : action.type === "delete-library" ? <Trash2 size={15} /> : action.type === "restore" ? <RotateCcw size={15} /> : <Power size={15} />}{busy ? "Saving…" : action.type === "edit" ? "Save changes" : action.type === "password" ? "Set password" : action.type === "archive" ? "Archive user" : action.type === "delete-library" ? "Move library to recycle bin" : action.type === "restore" ? "Restore user" : action.type === "enable" ? "Enable user" : "Disable user"}</Button></div>
    </form>
  </Modal>
}

function mutation(session: Session, body: object, method = "POST"): RequestInit {
  return { method, headers: { "X-CSRF-Token": session.csrfToken }, body: JSON.stringify(body) }
}

function UserField({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block"><span className="mb-2 block text-sm font-medium text-slate-300">{label}</span>{children}</label>
}
