import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type SyntheticEvent,
  type WheelEvent as ReactWheelEvent,
} from "react";
import {
  AlertTriangle,
  ArrowLeft,
  ChevronsLeft,
  ChevronsRight,
  FolderInput,
  LoaderCircle,
  Play,
  RefreshCw,
  RotateCcw,
  Save,
  Volume2,
  VolumeX,
  Pencil,
  Trash,
  Check
} from "lucide-react";
import { request, type Session, type User } from "@/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { FolderPickerDialog } from "@/components/folder-picker-dialog";

type Audio = {
  index: number;
  codec: string;
  channels: number;
  channelLayout?: string;
  sampleRate?: number;
  language?: string;
  default: boolean;
  usable: boolean;
  unusableReason?: string;
  included: boolean;
  gainDb: number;
};
type Editor = {
  id: string;
  state: "editing_session" | "finalizing";
  editRevision: number;
  title: string;
  destinationFolderId: number;
  durationMs: number;
  sourceSizeBytes: number;
  sourceContainer: string;
  sourceVideoCodec: string;
  sourceWidth: number;
  sourceHeight: number;
  sourceFrameRate: number;
  compressionRequested: boolean;
  qualityCrf: number;
  maxHeight: number;
  previewState: "none" | "rendering" | "ready" | "failed";
  previewStartMs: number;
  previewEndMs: number;
  previewRevision?: number;
  previewRendering: boolean;
  edit: { trimStartMs: number; trimEndMs: number; audio: Audio[] };
};

export function EditorDialog({
  session,
  sessionID,
  localFile,
  users,
  onClose,
  onFinalized,
}: {
  session: Session;
  sessionID: string;
  localFile: File | null;
  users: User[];
  onClose: () => void;
  onFinalized: () => void;
}) {
  const [editor, setEditor] = useState<Editor | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [discarding, setDiscarding] = useState(false);
  const [previewToken, setPreviewToken] = useState(0);
  const [metadataOpen, setMetadataOpen] = useState(false);
  const [pickingDestination, setPickingDestination] = useState(false);
  const [playheadMS, setPlayheadMS] = useState(0);
  const previewRef = useRef<HTMLVideoElement>(null);
  const [titleDraft, setTitleDraft] = useState("");
  const [draft, setDraft] = useState<Editor["edit"] | null>(null);
  const [conflict, setConflict] = useState(false);
  const draftRef = useRef<Editor["edit"] | null>(null);
  const editorRef = useRef<Editor | null>(null);
  const saveTimerRef = useRef<number | null>(null);
  const [localURL, setLocalURL] = useState("");
  const [localPlaybackFailed, setLocalPlaybackFailed] = useState(false);
  const [showRenderedPreview, setShowRenderedPreview] = useState(false);
  const [previewStarting, setPreviewStarting] = useState(false);
  const [confirmingUnpreviewedFinalize, setConfirmingUnpreviewedFinalize] =
    useState(false);

  useEffect(() => {
    request<Editor>(`/api/uploads/${sessionID}`)
      .then(setEditor)
      .catch((reason) => {
        setError(reason.message);
        onClose();
      });
  }, [onClose, sessionID]);
  useEffect(() => {
    if (!localFile) return;
    const extension = localFile.name.split(".").pop()?.toLowerCase();
    const inferredType =
      extension === "mp4" || extension === "m4v"
        ? "video/mp4"
        : extension === "webm"
          ? "video/webm"
          : extension === "ogv" || extension === "ogg"
            ? "video/ogg"
            : localFile.type;
    // Some reverse-proxy/browser combinations leave a dragged File with an
    // empty or generic MIME type. A typed slice keeps the zero-copy local fast
    // path while giving the media element the correct container hint.
    const source =
      inferredType && inferredType !== localFile.type
        ? localFile.slice(0, localFile.size, inferredType)
        : localFile;
    const url = URL.createObjectURL(source);
    setLocalURL(url);
    return () => URL.revokeObjectURL(url);
  }, [localFile]);
  useEffect(() => {
    editorRef.current = editor;
  }, [editor]);
  useEffect(
    () => () => {
      if (saveTimerRef.current !== null)
        window.clearTimeout(saveTimerRef.current);
    },
    [],
  );
  useEffect(() => {
    const timer = window.setInterval(
      () =>
        request<void>(`/api/uploads/${sessionID}/heartbeat`, {
          method: "POST",
          headers: { "X-CSRF-Token": session.csrfToken },
        }).catch(() => undefined),
      60_000,
    );
    return () => window.clearInterval(timer);
  }, [session.csrfToken, sessionID]);
  useEffect(() => {
    if (!editor?.previewRendering) return;
    const timer = window.setInterval(
      () =>
        request<Editor>(`/api/uploads/${sessionID}`)
          .then((next) => {
            setEditor(next);
            if (!next.previewRendering && next.previewState === "ready") {
              setPreviewToken((value) => value + 1);
              setShowRenderedPreview(true);
            }
          })
          .catch(() => undefined),
      1000,
    );
    return () => window.clearInterval(timer);
  }, [editor?.previewRendering, sessionID]);
  useEffect(() => {
    const value = draft ?? editor?.edit;
    if (!editor || !value) return;
    const initial =
      value.trimStartMs === 0 &&
      value.trimEndMs === editor.durationMs &&
      value.audio.every(
        (track) =>
          track.included ===
            (track.usable &&
              (track.default ||
                value.audio.find((candidate) => candidate.usable)?.index ===
                  track.index)) && track.gainDb === 0,
      );
    if (initial) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [draft, editor]);
  const activeEdit = draft ?? editor?.edit;
  const selected = useMemo(
    () =>
      activeEdit?.audio.filter((track) => track.included && track.usable) ?? [],
    [activeEdit],
  );
  useEffect(() => {
    if (
      !editor ||
      !activeEdit ||
      discarding ||
      metadataOpen ||
      pickingDestination ||
      confirmingUnpreviewedFinalize
    )
      return;
    const onSpace = (event: KeyboardEvent) => {
      if ((event.code !== "Space" && event.key !== " ") || event.repeat) return;
      const target = event.target;
      const textInput =
        target instanceof HTMLInputElement &&
        !["button", "checkbox", "radio", "range", "submit"].includes(
          target.type,
        );
      if (
        textInput ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement ||
        (target instanceof HTMLElement && target.isContentEditable)
      )
        return;
      const player = previewRef.current;
      if (!player) return;
      event.preventDefault();
      event.stopPropagation();
      if (!player.paused) {
        player.pause();
        return;
      }
      const local = Boolean(
        localURL && !localPlaybackFailed && !showRenderedPreview,
      );
      const offset = local ? 0 : editor.previewStartMs;
      const start = Math.max(activeEdit.trimStartMs, offset);
      const end = Math.min(
        activeEdit.trimEndMs,
        local ? editor.durationMs : editor.previewEndMs,
      );
      const absolute = offset + Math.round(player.currentTime * 1000);
      if (absolute < start || absolute >= end) {
        player.currentTime = Math.max(0, (start - offset) / 1000);
        setPlayheadMS(start);
      }
      void player.play();
    };
    document.addEventListener("keydown", onSpace, true);
    return () => document.removeEventListener("keydown", onSpace, true);
  }, [
    activeEdit,
    confirmingUnpreviewedFinalize,
    discarding,
    editor,
    localPlaybackFailed,
    localURL,
    metadataOpen,
    pickingDestination,
    showRenderedPreview,
  ]);
  if (!editor)
    return (
      <Modal title="Preparing editor" onClose={onClose}>
        <p role="status" className="text-sm text-slate-400">
          Analyzing your private source…
        </p>
      </Modal>
    );
  const current = editor;
  const edit = activeEdit ?? current.edit;
  const showingLocalSource = Boolean(
    localURL && !localPlaybackFailed && !showRenderedPreview,
  );
  const playbackOffsetMS = showingLocalSource ? 0 : editor.previewStartMs;
  const playbackStartMS = Math.max(edit.trimStartMs, playbackOffsetMS);
  const playbackEndMS = Math.min(
    edit.trimEndMs,
    showingLocalSource ? editor.durationMs : editor.previewEndMs,
  );
  const dirty =
    edit.trimStartMs !== 0 ||
    edit.trimEndMs !== current.durationMs ||
    edit.audio.some(
      (track) =>
        track.included !==
          (track.usable &&
            (track.default ||
              edit.audio.find((candidate) => candidate.usable)?.index ===
                track.index)) || track.gainDb !== 0,
    );
  async function persistDraft(next = draftRef.current) {
    const base = editorRef.current;
    if (!base || !next) return base;
    setSaving(true);
    setError("");
    try {
      const updated = await request<Editor>(`/api/uploads/${sessionID}/edit`, {
        method: "PUT",
        headers: {
          "X-CSRF-Token": session.csrfToken,
          "If-Match": String(base.editRevision),
        },
        body: JSON.stringify(next),
      });
      editorRef.current = updated;
      setEditor(updated);
      if (sameEdit(draftRef.current, next)) {
        draftRef.current = null;
        setDraft(null);
      }
      return updated;
    } catch (reason) {
      const message =
        reason instanceof Error ? reason.message : "Could not save your edit.";
      setError(message);
      setConflict(message.includes("changed elsewhere"));
      return null;
    } finally {
      setSaving(false);
    }
  }
  function save(next: Editor["edit"]) {
    draftRef.current = next;
    setDraft(next);
    setEditor((value) => (value ? { ...value, edit: next } : value));
    setConflict(false);
    if (saveTimerRef.current !== null)
      window.clearTimeout(saveTimerRef.current);
    saveTimerRef.current = window.setTimeout(() => {
      saveTimerRef.current = null;
      void persistDraft(next);
    }, 400);
  }
  function patchAudio(index: number, patch: Partial<Audio>) {
    void save({
      ...edit,
      audio: edit.audio.map((track) =>
        track.index === index ? { ...track, ...patch } : track,
      ),
    });
  }
  async function finalize() {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    const hadDraft = draftRef.current !== null;
    const updated = await persistDraft();
    if (hadDraft && !updated) return;
    const base = updated ?? editorRef.current;
    if (!base || conflict) return;
    setSaving(true);
    setError("");
    try {
      await request(`/api/uploads/${sessionID}/finalize`, {
        method: "POST",
        headers: {
          "X-CSRF-Token": session.csrfToken,
          "If-Match": String(base.editRevision),
        },
      });
      onFinalized();
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not finalize this upload.",
      );
    } finally {
      setSaving(false);
    }
  }
  async function discard() {
    setSaving(true);
    try {
      await request<void>(`/api/uploads/${sessionID}`, {
        method: "DELETE",
        headers: { "X-CSRF-Token": session.csrfToken },
      });
      onClose();
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not discard this upload.",
      );
      setSaving(false);
    }
  }
  async function renderPreview(fullSource = false) {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    const hadDraft = draftRef.current !== null;
    const updated = await persistDraft();
    if (hadDraft && !updated) return;
    const base = updated ?? editorRef.current;
    if (!base || conflict) return;
    setSaving(true);
    setPreviewStarting(true);
    setError("");
    try {
      await request(`/api/uploads/${sessionID}/preview`, {
        method: "POST",
        headers: { "X-CSRF-Token": session.csrfToken },
        body: JSON.stringify({ playheadMs: playheadMS, fullSource }),
      });
      setEditor({
        ...base,
        previewRendering: true,
      });
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not render a preview.",
      );
    } finally {
      setPreviewStarting(false);
      setSaving(false);
    }
  }
  async function reloadRemote() {
    setSaving(true);
    try {
      const latest = await request<Editor>(`/api/uploads/${sessionID}`);
      editorRef.current = latest;
      setEditor(latest);
      draftRef.current = null;
      setDraft(null);
      setConflict(false);
      setError("");
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not reload this editor.",
      );
    } finally {
      setSaving(false);
    }
  }
  async function overwriteRemote() {
    setConflict(false);
    const next = draftRef.current;
    if (next) await persistDraft(next);
  }
  async function saveMetadata(
    destinationFolderId = current.destinationFolderId,
  ) {
    setSaving(true);
    setError("");
    try {
      const updated = await request<Editor>(
        `/api/uploads/${sessionID}/metadata`,
        {
          method: "PATCH",
          headers: {
            "X-CSRF-Token": session.csrfToken,
            "If-Match": String(current.editRevision),
          },
          body: JSON.stringify({
            title: titleDraft || current.title,
            destinationFolderId,
          }),
        },
      );
      setEditor(updated);
      setMetadataOpen(false);
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not save clip details.",
      );
    } finally {
      setSaving(false);
    }
  }
  function playerTime(event: SyntheticEvent<HTMLVideoElement>) {
    return (
      playbackOffsetMS + Math.round(event.currentTarget.currentTime * 1000)
    );
  }
  function seekPlayer(player: HTMLVideoElement, absoluteMS: number) {
    player.currentTime = Math.max(0, (absoluteMS - playbackOffsetMS) / 1000);
    setPlayheadMS(absoluteMS);
  }
  function restartPlayback(player: HTMLVideoElement) {
    seekPlayer(player, playbackStartMS);
    void player.play();
  }
  function constrainPlayback(event: SyntheticEvent<HTMLVideoElement>) {
    const player = event.currentTarget;
    const absolute = playerTime(event);
    if (absolute < playbackStartMS || absolute >= playbackEndMS) {
      restartPlayback(player);
      return;
    }
    setPlayheadMS(absolute);
  }
  function loopPlayback(event: SyntheticEvent<HTMLVideoElement>) {
    const player = event.currentTarget;
    const absolute = playerTime(event);
    if (absolute >= playbackEndMS - 20) {
      restartPlayback(player);
      return;
    }
    if (absolute < playbackStartMS) {
      seekPlayer(player, playbackStartMS);
      return;
    }
    setPlayheadMS(absolute);
  }
  const previewInProgress =
    previewStarting || editor.previewRendering;
  const previewIsCurrent =
    !draft &&
    editor.previewState === "ready" &&
    editor.previewRevision === editor.editRevision;
  return (
    <Modal
      title="Edit clip"
      onClose={() => (dirty ? setDiscarding(true) : void discard())}
      dismissible={!saving}
      className="max-h-[94vh] max-w-6xl"
    >
      <div className="grid max-h-[78vh] gap-6 overflow-y-auto pr-1 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <section>
          <p className="eyebrow">Source remains private until finalization</p>
          <div className="mt-1 flex items-center gap-3">
            <h2 className="truncate text-xl font-semibold text-white">
              {editor.title}
            </h2>
            <Button
              size="sm"
              variant="secondary"
              disabled={saving}
              onClick={() => {
                setTitleDraft(editor.title);
                setMetadataOpen(true);
              }}
            >
              <Pencil size={14} />
              Edit details
            </Button>
          </div>
          <p className="mt-2 text-sm text-slate-400">
            {formatTime(editor.durationMs)} · {editor.sourceWidth}×
            {editor.sourceHeight} · {editor.sourceFrameRate.toFixed(2)} fps ·{" "}
            {formatBytes(editor.sourceSizeBytes)}
          </p>
          <div className="mt-5 overflow-hidden rounded-xl border border-white/[.08] bg-slate-950 text-center text-sm text-slate-500">
            {showingLocalSource ? (
              <video
                ref={previewRef}
                className="aspect-video w-full"
                controls
                preload="metadata"
                src={localURL}
                onError={() => {
                  setLocalPlaybackFailed(true);
                  void renderPreview(true);
                }}
                onPlay={constrainPlayback}
                onSeeking={constrainPlayback}
                onTimeUpdate={loopPlayback}
                onEnded={(event) => restartPlayback(event.currentTarget)}
              />
            ) : editor.previewState === "ready" ? (
              <video
                ref={previewRef}
                key={previewToken}
                className="aspect-video w-full"
                controls
                src={`/api/uploads/${sessionID}/preview?v=${previewToken}`}
                onPlay={constrainPlayback}
                onSeeking={constrainPlayback}
                onTimeUpdate={loopPlayback}
                onEnded={(event) => restartPlayback(event.currentTarget)}
              />
            ) : (
              <div className="grid aspect-video place-items-center">
                <div>
                  <p className="font-medium text-slate-300">
                    Private render preview
                  </p>
                  <p className="mt-1 max-w-sm">
                    {editor.previewRendering
                      ? "Generating a browser-compatible preview…"
                      : localPlaybackFailed
                        ? "This source needs a browser-compatible proxy."
                        : "Preparing local playback…"}
                  </p>
                </div>
              </div>
            )}
          </div>
          <TrimTimeline
            durationMS={editor.durationMs}
            sourceURL={
              localPlaybackFailed && editor.previewState === "ready"
                ? `/api/uploads/${sessionID}/preview?v=${previewToken}`
                : localURL
            }
            sourceOffsetMS={localPlaybackFailed ? editor.previewStartMs : 0}
            edit={edit}
            playheadMS={playheadMS}
            disabled={saving}
            onSeek={(value) => {
              const bounded = Math.max(
                edit.trimStartMs,
                Math.min(edit.trimEndMs - 1, value),
              );
              setPlayheadMS(bounded);
              if (previewRef.current) seekPlayer(previewRef.current, bounded);
            }}
            onChange={(next) => {
              save(next);
              if (
                playheadMS < next.trimStartMs ||
                playheadMS >= next.trimEndMs
              ) {
                setPlayheadMS(next.trimStartMs);
                if (previewRef.current)
                  seekPlayer(previewRef.current, next.trimStartMs);
              }
            }}
          />
        </section>
        <aside className="rounded-xl border border-white/[.08] bg-white/[.025] p-4">
          <h3 className="font-medium text-white">Audio tracks</h3>
          <div className="mt-4 space-y-3">
            {editor.edit.audio.map((track) => (
              <div
                key={track.index}
                className="rounded-lg border border-white/[.08] bg-slate-950 p-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium text-slate-200">
                      Track {track.index}
                      {track.default && (
                        <span className="ml-2 text-xs text-sky-300">
                          Default
                        </span>
                      )}
                    </p>
                    <p className="text-xs text-slate-500">
                      {track.codec} · {track.channels || "?"} ch
                      {track.language ? ` · ${track.language}` : ""}
                    </p>
                  </div>
                  {track.usable ? (
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={saving}
                      onClick={() =>
                        patchAudio(track.index, { included: !track.included })
                      }
                    >
                      {track.included ? (
                        <>
                          <Volume2 size={14} /> Included
                        </>
                      ) : (
                        <>
                          <VolumeX size={14} /> Muted
                        </>
                      )}
                    </Button>
                  ) : (
                    <span className="text-xs text-red-300">
                      {track.unusableReason || "Unavailable"}
                    </span>
                  )}
                </div>
                {track.usable && (
                  <label className="mt-3 block text-xs text-slate-400">
                    Level{" "}
                    <span className="float-right text-sky-300">
                      {track.gainDb.toFixed(1)} dB
                    </span>
                    <input
                      className="mt-2 w-full accent-sky-400"
                      type="range"
                      min="-60"
                      max="12"
                      step="0.5"
                      value={track.gainDb}
                      disabled={saving}
                      onChange={(event) =>
                        patchAudio(track.index, {
                          gainDb: Number(event.target.value),
                        })
                      }
                    />
                  </label>
                )}
              </div>
            ))}
          </div>
          {selected.length === 0 && (
            <p className="mt-4 rounded-lg bg-amber-400/10 px-3 py-2 text-xs text-amber-200">
              This clip will be silent.
            </p>
          )}
        </aside>
      </div>
      {error && (
        <p className="mt-4 text-sm text-red-300" role="alert">
          {error}
        </p>
      )}
      {conflict && (
        <div
          className="mt-4 rounded-lg border border-amber-400/20 bg-amber-400/10 p-3 text-sm text-amber-100"
          role="alert"
        >
          <p>
            This editor changed elsewhere. Keep your local recipe or reload the
            saved version.
          </p>
          <div className="mt-3 flex gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => void reloadRemote()}
            >
              <RefreshCw size={14} />
              Reload saved edit
            </Button>
            <Button size="sm" variant="warning" onClick={() => void overwriteRemote()}>
              <Save size={14} />
              Overwrite with mine
            </Button>
          </div>
        </div>
      )}
      <p className="mt-3 text-xs text-slate-500" role="status">
        {saving && !previewInProgress
          ? "Saving…"
          : draft
            ? "Waiting to save…"
            : "Saved"}
      </p>
      <div className="mt-4 flex flex-wrap items-end justify-end gap-2">
        {previewInProgress && (
          <div
            className="mr-auto flex min-w-52 items-center gap-2"
            role="status"
            aria-live="polite"
          >
            <LoaderCircle
              className="shrink-0 animate-spin text-sky-300"
              size={17}
            />
            <div className="flex-1">
              <p className="mb-1 text-xs text-sky-200">Rendering preview…</p>
              <div
                className="h-1.5 overflow-hidden rounded-full bg-slate-800"
                role="progressbar"
                aria-label="Rendering preview"
              >
                <div className="h-full w-2/3 animate-pulse rounded-full bg-sky-400" />
              </div>
            </div>
          </div>
        )}
        {showRenderedPreview && localURL && !localPlaybackFailed && (
          <Button variant="secondary" onClick={() => setShowRenderedPreview(false)}>
            <ArrowLeft size={15} />
            Return to live source
          </Button>
        )}
        <Button
          variant="secondary"
          disabled={saving || previewInProgress || conflict}
          onClick={() => void renderPreview()}
        >
          <Play size={15} /> Render preview
        </Button>
        <Button
          variant="danger"
          disabled={saving}
          onClick={() => (dirty ? setDiscarding(true) : void discard())}
        >
          <Trash size={15} /> 
          Discard upload
        </Button>
        <Button
          variant="success"
          disabled={saving || previewInProgress || conflict}
          onClick={() =>
            previewIsCurrent
              ? void finalize()
              : setConfirmingUnpreviewedFinalize(true)
          }
        >
          <Check size={15} />
          Finalize upload
        </Button>
      </div>
      {confirmingUnpreviewedFinalize && (
        <Modal
          nested
          title="Finalize without a current preview?"
          onClose={() => setConfirmingUnpreviewedFinalize(false)}
          dismissible={!saving}
        >
          <div className="flex gap-3 text-sm text-slate-300">
            <AlertTriangle className="shrink-0 text-amber-300" size={20} />
            <p>
              Your latest audio and video settings have not been rendered into a
              preview. We recommend rendering one first to make sure the
              finalized clip looks and sounds correct.
            </p>
          </div>
          <div className="mt-5 flex flex-wrap justify-end gap-2">
            <Button
              variant="secondary"
              disabled={saving}
              onClick={() => setConfirmingUnpreviewedFinalize(false)}
            >
              <ArrowLeft size={15} />
              Back
            </Button>
            <Button
              variant="default"
              disabled={saving}
              onClick={() => {
                setConfirmingUnpreviewedFinalize(false);
                void renderPreview();
              }}
            >
              <Play size={15} /> Render preview
            </Button>
            <Button
              variant="warning"
              disabled={saving}
              onClick={() => {
                setConfirmingUnpreviewedFinalize(false);
                void finalize();
              }}
            >
              <AlertTriangle size={16} /> Finalize upload
            </Button>
          </div>
        </Modal>
      )}
      {discarding && (
        <Modal
          nested
          title="Discard this upload?"
          onClose={() => setDiscarding(false)}
        >
          <div className="flex gap-3 text-sm text-slate-400">
            <AlertTriangle className="shrink-0 text-amber-300" size={20} />
            Your trim and audio changes, source file, and temporary files will
            be deleted immediately.
          </div>
          <div className="mt-5 flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setDiscarding(false)}>
              <ArrowLeft size={15} />
              Keep editing
            </Button>
            <Button variant="danger" onClick={() => void discard()}><Trash size={15} /> Discard upload</Button>
          </div>
        </Modal>
      )}
      {metadataOpen && (
        <Modal
          nested
          title="Edit clip details"
          onClose={() => setMetadataOpen(false)}
          dismissible={!saving}
        >
          <label className="block text-sm text-slate-300">
            Clip title
            <Input
              className="mt-2"
              value={titleDraft}
              onChange={(event) => setTitleDraft(event.target.value)}
              maxLength={200}
              autoFocus
            />
          </label>
          <div className="mt-4 flex justify-end gap-2">
            <Button
              variant="danger"
              disabled={saving}
              onClick={() => setMetadataOpen(false)}
            >
              <ArrowLeft size={15} />
              Cancel
            </Button>
            <Button
              variant="success"
              disabled={saving || !titleDraft.trim()}
              onClick={() => void saveMetadata()}
            >
              <Save size={15} />
              Save title
            </Button>
            <Button
              variant="secondary"
              disabled={saving}
              onClick={() => setPickingDestination(true)}
            >
              <FolderInput size={15} />
              Change destination
            </Button>
          </div>
        </Modal>
      )}
      {pickingDestination && (
        <FolderPickerDialog
          title="Choose editor destination"
          session={session}
          users={users}
          initialID={editor.destinationFolderId}
          onClose={() => setPickingDestination(false)}
          onChoose={(folder) => {
            setPickingDestination(false);
            void saveMetadata(folder.id);
          }}
        />
      )}
    </Modal>
  );
}
function TrimTimeline({
  durationMS,
  sourceURL,
  sourceOffsetMS,
  edit,
  playheadMS,
  disabled,
  onSeek,
  onChange,
}: {
  durationMS: number;
  sourceURL: string;
  sourceOffsetMS: number;
  edit: Editor["edit"];
  playheadMS: number;
  disabled: boolean;
  onSeek: (value: number) => void;
  onChange: (edit: Editor["edit"]) => void;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{
    mode: "left" | "right" | "move";
    start: number;
    end: number;
    grabOffset: number;
    captureTarget: HTMLDivElement;
  } | null>(null);
  const dragViewportRef = useRef<{ start: number; duration: number } | null>(
    null,
  );
  const suppressClickRef = useRef(false);
  const [manualZoom, setManualZoom] = useState(1);
  const [panOffsetMS, setPanOffsetMS] = useState(0);
  const [dragViewport, setDragViewport] = useState<{
    start: number;
    duration: number;
  } | null>(null);
  const snap = (value: number) => Math.round(value / 100) * 100;
  const selectionDuration = Math.max(250, edit.trimEndMs - edit.trimStartMs);
  const minimumViewport = Math.min(durationMS, selectionDuration / 0.9);
  const automaticViewport = Math.min(durationMS, selectionDuration / 0.72);
  const settledViewDuration = Math.max(
    minimumViewport,
    Math.min(durationMS, automaticViewport * manualZoom),
  );
  const selectionCenter = (edit.trimStartMs + edit.trimEndMs) / 2;
  const settledViewStart = Math.max(
    0,
    Math.min(
      durationMS - settledViewDuration,
      selectionCenter + panOffsetMS - settledViewDuration / 2,
    ),
  );
  const viewDuration = dragViewport?.duration ?? settledViewDuration;
  const viewStart = dragViewport?.start ?? settledViewStart;
  const viewEnd = viewStart + viewDuration;
  const filmstrip = useFilmstrip(
    sourceURL,
    sourceOffsetMS,
    durationMS,
    viewStart,
    viewEnd,
  );
  function pointerDown(
    event: ReactPointerEvent<HTMLDivElement>,
    mode: "left" | "right" | "move",
  ) {
    if (disabled) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    suppressClickRef.current = false;
    const rect = trackRef.current?.getBoundingClientRect();
    const pointerTime = rect
      ? viewStart +
        ((event.clientX - rect.left) / rect.width) * viewDuration
      : mode === "right"
        ? edit.trimEndMs
        : edit.trimStartMs;
    const viewport = { start: viewStart, duration: viewDuration };
    dragViewportRef.current = viewport;
    setDragViewport(viewport);
    dragRef.current = {
      mode,
      start: edit.trimStartMs,
      end: edit.trimEndMs,
      grabOffset: pointerTime - edit.trimStartMs,
      captureTarget: event.currentTarget,
    };
  }
  function pointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    const drag = dragRef.current;
    const rect = trackRef.current?.getBoundingClientRect();
    let viewport = dragViewportRef.current;
    if (!drag || !rect || !viewport || rect.width <= 0) return;
    const edgeZone = Math.min(48, rect.width * 0.12);
    let panBy = 0;
    if (event.clientX < rect.left + edgeZone)
      panBy =
        -viewport.duration *
        0.035 *
        Math.min(1, (rect.left + edgeZone - event.clientX) / edgeZone);
    else if (event.clientX > rect.right - edgeZone)
      panBy =
        viewport.duration *
        0.035 *
        Math.min(1, (event.clientX - (rect.right - edgeZone)) / edgeZone);
    if (panBy !== 0) {
      const start = Math.max(
        0,
        Math.min(durationMS - viewport.duration, viewport.start + panBy),
      );
      viewport = { ...viewport, start };
      dragViewportRef.current = viewport;
      setDragViewport(viewport);
    }
    const fraction = Math.max(
      0,
      Math.min(1, (event.clientX - rect.left) / rect.width),
    );
    const pointerTime = snap(viewport.start + fraction * viewport.duration);
    suppressClickRef.current = true;
    if (drag.mode === "left")
      onChange({
        ...edit,
        trimStartMs: Math.max(0, Math.min(drag.end - 250, pointerTime)),
      });
    else if (drag.mode === "right")
      onChange({
        ...edit,
        trimEndMs: Math.min(
          durationMS,
          Math.max(drag.start + 250, pointerTime),
        ),
      });
    else {
      const length = drag.end - drag.start;
      const start = Math.max(
        0,
        Math.min(durationMS - length, pointerTime - drag.grabOffset),
      );
      onChange({ ...edit, trimStartMs: start, trimEndMs: start + length });
    }
  }
  function pointerUp(event: ReactPointerEvent<HTMLDivElement>) {
    const captureTarget = dragRef.current?.captureTarget;
    dragRef.current = null;
    dragViewportRef.current = null;
    setDragViewport(null);
    setPanOffsetMS(0);
    if (captureTarget?.hasPointerCapture(event.pointerId))
      captureTarget.releasePointerCapture(event.pointerId);
  }
  function wheel(event: ReactWheelEvent<HTMLDivElement>) {
    event.preventDefault();
    const horizontal =
      event.shiftKey || Math.abs(event.deltaX) > Math.abs(event.deltaY);
    if (horizontal && viewDuration < durationMS) {
      const delta = event.deltaX || event.deltaY;
      const desiredStart = Math.max(
        0,
        Math.min(
          durationMS - viewDuration,
          viewStart + (delta / (trackRef.current?.clientWidth || 1)) * viewDuration,
        ),
      );
      setPanOffsetMS(desiredStart + viewDuration / 2 - selectionCenter);
      return;
    }
    setManualZoom((current) =>
      Math.max(0.8, Math.min(20, current * (event.deltaY > 0 ? 1.2 : 1 / 1.2))),
    );
  }
  function nudge(edge: "left" | "right", amount: number) {
    if (edge === "left")
      onChange({
        ...edit,
        trimStartMs: Math.max(
          0,
          Math.min(edit.trimEndMs - 250, edit.trimStartMs + amount),
        ),
      });
    else
      onChange({
        ...edit,
        trimEndMs: Math.min(
          durationMS,
          Math.max(edit.trimStartMs + 250, edit.trimEndMs + amount),
        ),
      });
  }
  const left = ((edit.trimStartMs - viewStart) / viewDuration) * 100;
  const right = ((edit.trimEndMs - viewStart) / viewDuration) * 100;
  const play = Math.max(
    0,
    Math.min(100, ((playheadMS - viewStart) / viewDuration) * 100),
  );
  const filmstripDuration = filmstrip.endMS - filmstrip.startMS;
  const filmstripLeft = ((filmstrip.startMS - viewStart) / viewDuration) * 100;
  const filmstripWidth = (filmstripDuration / viewDuration) * 100;
  return (
    <div className="mt-5 rounded-xl border border-white/[.08] bg-slate-950/70 p-4">
      <div className="flex justify-between gap-3 text-sm">
        <span className="font-medium text-white">Trim selection</span>
        <span className="text-right text-sky-300">
          {formatTime(edit.trimEndMs - edit.trimStartMs)}{" "}
          <span className="ml-2 text-xs text-slate-500">
            {(durationMS / viewDuration).toFixed(1)}x zoom
          </span>
        </span>
      </div>
      <div
        ref={trackRef}
        className="relative mt-4 h-20 touch-none select-none overflow-hidden rounded-lg border border-white/10 bg-slate-900"
        onWheel={wheel}
        onPointerMove={pointerMove}
        onPointerUp={pointerUp}
        onPointerCancel={() => {
          dragRef.current = null;
          dragViewportRef.current = null;
          setDragViewport(null);
          setPanOffsetMS(0);
        }}
        onClick={(event) => {
          if (suppressClickRef.current) {
            suppressClickRef.current = false;
            return;
          }
          if (dragRef.current) return;
          const rect = event.currentTarget.getBoundingClientRect();
          onSeek(
            snap(
              viewStart +
                ((event.clientX - rect.left) / rect.width) * viewDuration,
            ),
          );
        }}
      >
        {filmstrip.images.length ? (
          <div
            className="absolute inset-y-0 flex"
            style={{ left: `${filmstripLeft}%`, width: `${filmstripWidth}%` }}
          >
            {filmstrip.images.map((image, index) => (
              <img
                key={index}
                src={image}
                alt=""
                className="h-full min-w-0 flex-1 object-cover opacity-70"
              />
            ))}
          </div>
        ) : (
          <div className="absolute inset-0 animate-pulse bg-gradient-to-r from-slate-800 via-slate-700 to-slate-800" />
        )}
        <div
          className="absolute inset-y-0 left-0 bg-black/65"
          style={{ width: `${left}%` }}
        />
        <div
          className="absolute inset-y-0 right-0 bg-black/65"
          style={{ width: `${100 - right}%` }}
        />
        <div
          data-role="selection"
          className="absolute inset-y-0 cursor-grab border-y-2 border-sky-300 bg-sky-300/10 active:cursor-grabbing"
          style={{ left: `${left}%`, width: `${right - left}%` }}
          onPointerDown={(event) => pointerDown(event, "move")}
        >
          <div
            className="absolute inset-y-0 -left-1 w-2 cursor-ew-resize rounded-l-sm bg-sky-300 shadow"
            role="slider"
            aria-label="Trim start"
            aria-valuemin={0}
            aria-valuemax={edit.trimEndMs - 250}
            aria-valuenow={edit.trimStartMs}
            tabIndex={0}
            onKeyDown={(event) => {
              if (event.key !== "ArrowLeft" && event.key !== "ArrowRight")
                return;
              event.preventDefault();
              nudge(
                "left",
                (event.key === "ArrowRight" ? 1 : -1) *
                  (event.shiftKey ? 1000 : 100),
              );
            }}
            onPointerDown={(event) => {
              event.stopPropagation();
              pointerDown(event, "left");
            }}
          />
          <div
            className="absolute inset-y-0 -right-1 w-2 cursor-ew-resize rounded-r-sm bg-sky-300 shadow"
            role="slider"
            aria-label="Trim end"
            aria-valuemin={edit.trimStartMs + 250}
            aria-valuemax={durationMS}
            aria-valuenow={edit.trimEndMs}
            tabIndex={0}
            onKeyDown={(event) => {
              if (event.key !== "ArrowLeft" && event.key !== "ArrowRight")
                return;
              event.preventDefault();
              nudge(
                "right",
                (event.key === "ArrowRight" ? 1 : -1) *
                  (event.shiftKey ? 1000 : 100),
              );
            }}
            onPointerDown={(event) => {
              event.stopPropagation();
              pointerDown(event, "right");
            }}
          />
        </div>
        {playheadMS >= viewStart && playheadMS <= viewEnd ? (
          <div
            className="pointer-events-none absolute inset-y-0 w-0.5 bg-white shadow"
            style={{ left: `${play}%` }}
          />
        ) : null}
      </div>
      <div className="mt-2 h-5" aria-hidden={viewDuration >= durationMS - 1}>
        {viewDuration < durationMS - 1 && !dragViewport ? (
          <div className="relative h-full rounded-md border border-white/[.06] bg-slate-950/70">
            <div className="absolute inset-x-2 top-1/2 h-1 -translate-y-1/2 rounded-full bg-slate-800" />
            <div
              className="pointer-events-none absolute top-1/2 h-2.5 -translate-y-1/2 rounded-sm border border-slate-400/40 bg-slate-500 shadow"
              style={{
                left: `${(viewStart / durationMS) * 100}%`,
                width: `${(viewDuration / durationMS) * 100}%`,
              }}
            />
            <input
              className="absolute inset-0 h-full w-full cursor-ew-resize opacity-0"
              type="range"
              min={0}
              max={durationMS - viewDuration}
              step={Math.max(1, Math.round(viewDuration / 500))}
              value={viewStart}
              aria-label="Scroll timeline"
              onChange={(event) => {
                const start = Number(event.target.value);
                setPanOffsetMS(start + viewDuration / 2 - selectionCenter);
              }}
            />
          </div>
        ) : null}
      </div>
      <div className="mt-2 flex justify-between text-xs text-slate-500">
        <span>{formatTime(viewStart)}</span>
        <span>Playhead {formatTime(playheadMS)}</span>
        <span>{formatTime(viewEnd)}</span>
      </div>
      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        <TimeInput
          label="In"
          value={edit.trimStartMs}
          minimum={0}
          maximum={edit.trimEndMs - 250}
          onChange={(value) => onChange({ ...edit, trimStartMs: value })}
        />
        <TimeInput
          label="Out"
          value={edit.trimEndMs}
          minimum={edit.trimStartMs + 250}
          maximum={durationMS}
          onChange={(value) => onChange({ ...edit, trimEndMs: value })}
        />
      </div>
      <div className="mt-4 flex flex-wrap gap-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={disabled || playheadMS >= edit.trimEndMs - 250}
          onClick={() => onChange({ ...edit, trimStartMs: playheadMS })}
        >
          <ChevronsLeft size={15} />
          Set In
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={disabled || playheadMS <= edit.trimStartMs + 250}
          onClick={() => onChange({ ...edit, trimEndMs: playheadMS })}
        >
          <ChevronsRight size={15} />
          Set Out
        </Button>
        <Button
          size="sm"
          variant="warning"
          disabled={disabled}
          onClick={() => {
            setManualZoom(1);
            setPanOffsetMS(0);
            onChange({ ...edit, trimStartMs: 0, trimEndMs: durationMS });
          }}
        >
          <RotateCcw size={15} /> Reset trim
        </Button>
      </div>
      <p className="mt-3 text-xs text-slate-500">
        Drag either edge to resize, drag the middle to move, click to seek, or
        use the scroll wheel to zoom. When zoomed in, use the timeline scrollbar or
        Shift+mouse wheel to move through the clip.
      </p>
    </div>
  );
}

function TimeInput({
  label,
  value,
  minimum = 0,
  maximum,
  onChange,
}: {
  label: string;
  value: number;
  minimum?: number;
  maximum: number;
  onChange: (value: number) => void;
}) {
  const [text, setText] = useState(formatTimecode(value));
  useEffect(() => setText(formatTimecode(value)), [value]);
  return (
    <label className="text-sm text-slate-300">
      {label}
      <Input
        className="mt-2 font-mono"
        value={text}
        onChange={(event) => setText(event.target.value)}
        onBlur={() => {
          const next = parseTimecode(text);
          if (next !== null && next >= minimum && next <= maximum)
            onChange(next);
          else setText(formatTimecode(value));
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.currentTarget.blur();
            return;
          }
          if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
          event.preventDefault();
          const amount = event.shiftKey ? 1000 : 100;
          const next = Math.max(
            minimum,
            Math.min(
              maximum,
              value + (event.key === "ArrowUp" ? amount : -amount),
            ),
          );
          onChange(next);
        }}
        aria-label={`${label} time`}
      />
      <span className="mt-1 block text-xs text-slate-500">HH:MM:SS.mmm</span>
    </label>
  );
}
function formatTime(milliseconds: number) {
  const total = Math.max(0, Math.round(milliseconds));
  const seconds = Math.floor(total / 1000) % 60;
  const minutes = Math.floor(total / 60000);
  return `${minutes}:${String(seconds).padStart(2, "0")}.${String(total % 1000).padStart(3, "0")}`;
}
function formatBytes(bytes: number) {
  return bytes >= 1_000_000
    ? `${(bytes / 1_000_000).toFixed(1)} MB`
    : `${Math.ceil(bytes / 1_000)} KB`;
}
function sameEdit(left: Editor["edit"] | null, right: Editor["edit"]) {
  return left !== null && JSON.stringify(left) === JSON.stringify(right);
}
function formatTimecode(milliseconds: number) {
  const total = Math.max(0, Math.round(milliseconds));
  const hours = Math.floor(total / 3_600_000);
  const minutes = Math.floor(total / 60_000) % 60;
  const seconds = Math.floor(total / 1000) % 60;
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}.${String(total % 1000).padStart(3, "0")}`;
}
function parseTimecode(value: string) {
  const match = value.trim().match(/^([\d:]+)(?:\.(\d{1,3}))?$/);
  if (!match) return null;
  const parts = match[1].split(":").map(Number);
  if (
    parts.length < 1 ||
    parts.length > 3 ||
    parts.some((part) => !Number.isInteger(part) || part < 0)
  )
    return null;
  const seconds = parts.at(-1) ?? 0;
  const minutes = parts.length >= 2 ? (parts.at(-2) ?? 0) : 0;
  const hours = parts.length === 3 ? parts[0] : 0;
  if (minutes > 59 || seconds > 59) return null;
  const milliseconds = Number((match[2] ?? "").padEnd(3, "0"));
  return ((hours * 60 + minutes) * 60 + seconds) * 1000 + milliseconds;
}
function useFilmstrip(
  sourceURL: string,
  sourceOffsetMS: number,
  durationMS: number,
  viewStartMS: number,
  viewEndMS: number,
) {
  const [filmstrip, setFilmstrip] = useState<{
    sourceURL: string;
    startMS: number;
    endMS: number;
    images: string[];
  }>({ sourceURL: "", startMS: 0, endMS: 0, images: [] });
  useEffect(() => {
    if (!sourceURL || viewEndMS <= viewStartMS) return;
    let cancelled = false;
    let video: HTMLVideoElement | null = null;
    const viewDurationMS = viewEndMS - viewStartMS;
    const captureStartMS = Math.max(0, viewStartMS - viewDurationMS);
    const captureEndMS = Math.min(durationMS, viewEndMS + viewDurationMS);
    const timer = window.setTimeout(() => {
      if (cancelled) return;
      video = document.createElement("video");
      video.muted = true;
      video.playsInline = true;
      video.preload = "auto";
      const seek = (time: number) =>
        new Promise<void>((resolve, reject) => {
          const done = () => {
            video?.removeEventListener("seeked", done);
            video?.removeEventListener("error", failed);
            resolve();
          };
          const failed = () => {
            video?.removeEventListener("seeked", done);
            video?.removeEventListener("error", failed);
            reject();
          };
          video?.addEventListener("seeked", done, { once: true });
          video?.addEventListener("error", failed, { once: true });
          if (video) video.currentTime = time;
        });
      const load = () => {
        void (async () => {
          const frames: string[] = [];
          const canvas = document.createElement("canvas");
          canvas.width = 160;
          canvas.height = 90;
          const context = canvas.getContext("2d");
          if (!context || !video) return;
          try {
            for (let index = 0; index < 24; index++) {
              const absoluteMS =
                captureStartMS +
                ((captureEndMS - captureStartMS) * (index + 0.5)) / 24;
              const sourceSeconds = (absoluteMS - sourceOffsetMS) / 1000;
              await seek(
                Math.max(0, Math.min(video.duration - 0.001, sourceSeconds)),
              );
              context.drawImage(video, 0, 0, canvas.width, canvas.height);
              frames.push(canvas.toDataURL("image/jpeg", 0.68));
            }
            if (!cancelled)
              setFilmstrip({
                sourceURL,
                startMS: captureStartMS,
                endMS: captureEndMS,
                images: frames,
              });
          } catch {
            // Keep the last complete strip visible if replacement decoding fails.
          }
        })();
      };
      video.addEventListener("loadedmetadata", load, { once: true });
      video.src = sourceURL;
      video.load();
    }, 140);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
      if (video) {
        video.pause();
        video.removeAttribute("src");
        video.load();
      }
    };
  }, [durationMS, sourceOffsetMS, sourceURL, viewEndMS, viewStartMS]);
  return filmstrip.sourceURL === sourceURL
    ? filmstrip
    : { sourceURL, startMS: 0, endMS: 0, images: [] };
}
