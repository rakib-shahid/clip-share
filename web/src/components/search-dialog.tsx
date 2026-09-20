import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from "react";
import { Check, Copy, Folder, FolderOpen, Search, Video } from "lucide-react";
import { request, type SearchResult, type Session } from "@/api";
import {
  ClipPreviewThumbnail,
  VideoPreviewDialog,
} from "@/components/clip-preview";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Field, FieldLabel } from "@/components/ui/field";
import { copyText } from "@/lib/clipboard";
import { createPreferenceStore } from "@/lib/explorer-preferences";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Grid2X2, List } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Item, ItemActions, ItemContent, ItemDescription, ItemTitle } from "@/components/ui/item";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";

type Props = {
  session: Session;
  onClose: () => void;
  onNavigate: (result: SearchResult) => void;
};

export function SearchDialog({ session, onClose, onNavigate }: Props) {
  const [open, setOpen] = useState(true);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [searched, setSearched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<SearchResult | null>(null);
  const [copiedID, setCopiedID] = useState<number | null>(null);
  const preferenceStore = useMemo(() => createPreferenceStore(session.user.id), [session.user.id]);
  const [view, setView] = useState(preferenceStore.get().search.view);
  useEffect(() => { const unsubscribe = preferenceStore.subscribe((next) => setView(next.search.view)); return () => { unsubscribe(); preferenceStore.destroy() } }, [preferenceStore]);
  const returnFocusRef = useRef(
    document.activeElement instanceof HTMLElement ? document.activeElement : null,
  );

  async function search(event: FormEvent) {
    event.preventDefault();
    const trimmed = query.trim();
    if (!trimmed) {
      setResults([]);
      setTruncated(false);
      setSearched(true);
      setError("");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const response = await request<{
        results: SearchResult[];
        truncated: boolean;
      }>(`/api/search?q=${encodeURIComponent(trimmed)}`);
      setResults(response.results);
      setTruncated(response.truncated);
      setSearched(true);
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not search the library.",
      );
    } finally {
      setLoading(false);
    }
  }

  async function copyLink(result: SearchResult) {
    if (!result.publicId) return;
    try {
      await copyText(`${session.publicBaseURL}/c/${result.publicId}`);
      setCopiedID(result.id);
      toast.success("Link copied");
      window.setTimeout(
        () => setCopiedID((current) => (current === result.id ? null : current)),
        1500,
      );
    } catch {
      toast.error("Could not copy link");
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        className="max-h-[90dvh] max-w-5xl overflow-hidden"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          returnFocusRef.current?.focus();
          onClose();
        }}
      >
        <DialogHeader>
          <DialogTitle>Search libraries</DialogTitle>
          <DialogDescription>
            Searches every folder you can access. Recycle-bin items are excluded.
          </DialogDescription>
        </DialogHeader>
        <form className="flex items-end gap-2" onSubmit={search} role="search">
          <Field className="min-w-0 flex-1 gap-1.5">
          <FieldLabel htmlFor="library-search">Folder or clip name</FieldLabel>
          <Input
            id="library-search"
            aria-label="Search folder names and clip titles"
            placeholder="Search folder names and clip titles…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            maxLength={200}
            autoFocus
          />
          </Field>
          <Button disabled={loading}>
            <Search size={17} /> {loading ? "Searching…" : "Search"}
          </Button>
        </form>
        <div className="mt-3 flex justify-end"><ToggleGroup type="single" value={view} onValueChange={(value) => { if (value === "grid" || value === "list") preferenceStore.update({ search: { view: value } }) }} aria-label="Search result view"><ToggleGroupItem value="grid" aria-label="Grid view"><Grid2X2 /></ToggleGroupItem><ToggleGroupItem value="list" aria-label="List view"><List /></ToggleGroupItem></ToggleGroup></div>
        {error && (
          <p
            className="mt-4 rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200"
            role="alert"
          >
            {error}
          </p>
        )}
        <div className="min-h-0 overflow-y-auto pr-1">
          {loading && <div className={view === "grid" ? "grid gap-3 sm:grid-cols-2 lg:grid-cols-3" : "grid gap-2"} role="status" aria-label="Searching"><Skeleton className="h-28" /><Skeleton className="h-28" /><Skeleton className="h-28" /></div>}
          {!searched && !loading && (
            <SearchEmpty
              icon={<Search size={28} />}
              title="Find a folder or clip"
              detail="Enter any part of its name. Search is not case-sensitive."
            />
          )}
          {searched && !loading && results.length === 0 && (
            <SearchEmpty
              icon={<Search size={28} />}
              title="No matches"
              detail="Try a shorter or different part of the name."
            />
          )}
          {results.length > 0 && (
            <ul className={view === "grid" ? "grid gap-3 sm:grid-cols-2 lg:grid-cols-3" : "grid gap-2"} data-view={view} aria-label="Search results">
              {results.map((result) => (
                <li
                  key={`${result.kind}-${result.id}`}
                  className="min-w-0"
                >
                  <Item variant="outline" className="h-full flex-col items-stretch overflow-hidden rounded-xl bg-slate-950 p-0">
                  {result.kind === "clip" &&
                  result.state === "ready" &&
                  result.publicId ? (
                    <ClipPreviewThumbnail
                      title={result.name}
                      posterSrc={`/m/${result.publicId}/poster`}
                      onPreview={() => setPreview(result)}
                    />
                  ) : (
                    <div className="grid aspect-video place-items-center bg-slate-900 text-sky-300">
                      {result.kind === "folder" ? (
                        <Folder size={38} />
                      ) : (
                        <Video size={34} />
                      )}
                    </div>
                  )}
                  <ItemContent className="w-full p-3">
                    <Badge variant="secondary">{result.kind}</Badge>
                    <ItemTitle><span className="truncate">{result.name}</span></ItemTitle>
                    <ItemDescription title={result.path}>{result.path}</ItemDescription>
                    {session.user.role === "admin" && (
                      <p className="mt-1 text-xs text-slate-600">
                        Owner: {result.ownerUsername}
                      </p>
                    )}
                  </ItemContent>
                  <ItemActions className="flex w-full flex-wrap gap-1 border-t border-white/[.06] bg-slate-950 px-2 py-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => onNavigate(result)}
                    >
                      <FolderOpen size={14} />{" "}
                      {result.kind === "folder"
                        ? "Open folder"
                        : "Show in folder"}
                    </Button>
                    {result.kind === "clip" &&
                      result.state === "ready" &&
                      result.publicId && (
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => copyLink(result)}
                        >
                          {copiedID === result.id ? (
                            <Check size={14} />
                          ) : (
                            <Copy size={14} />
                          )}{" "}
                          {copiedID === result.id ? "Copied" : "Copy link"}
                        </Button>
                      )}
                  </ItemActions>
                  </Item>
                </li>
              ))}
            </ul>
          )}
          {truncated && (
            <p className="mt-4 rounded-lg border border-sky-400/15 bg-sky-400/[.06] px-3 py-2 text-sm text-sky-100">
              Showing the first 100 matches. Refine your search to narrow the
              results.
            </p>
          )}
        </div>
      </DialogContent>
      </Dialog>
      {preview?.publicId && (
        <VideoPreviewDialog
          title={preview.name}
          posterSrc={`/m/${preview.publicId}/poster`}
          videoSrc={`/m/${preview.publicId}/video`}
          nested
          onClose={() => setPreview(null)}
        />
      )}
    </>
  );
}

function SearchEmpty({
  icon,
  title,
  detail,
}: {
  icon: ReactNode;
  title: string;
  detail: string;
}) {
  return (
    <Empty className="min-h-52 border border-white/10 bg-white/[.02]"><EmptyHeader><EmptyMedia>{icon}</EmptyMedia><EmptyTitle>{title}</EmptyTitle><EmptyDescription>{detail}</EmptyDescription></EmptyHeader></Empty>
  );
}
