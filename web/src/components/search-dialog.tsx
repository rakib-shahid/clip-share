import { useState, type FormEvent, type ReactNode } from "react";
import { Check, Copy, Folder, FolderOpen, Search, Video } from "lucide-react";
import { request, type SearchResult, type Session } from "@/api";
import {
  ClipPreviewThumbnail,
  VideoPreviewDialog,
} from "@/components/clip-preview";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { copyText } from "@/lib/clipboard";

type Props = {
  session: Session;
  onClose: () => void;
  onNavigate: (result: SearchResult) => void;
};

export function SearchDialog({ session, onClose, onNavigate }: Props) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [searched, setSearched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<SearchResult | null>(null);
  const [copiedID, setCopiedID] = useState<number | null>(null);

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
    await copyText(`${session.publicBaseURL}/c/${result.publicId}`);
    setCopiedID(result.id);
    window.setTimeout(
      () => setCopiedID((current) => (current === result.id ? null : current)),
      1500,
    );
  }

  return (
    <>
      <Modal
        title="Search libraries"
        onClose={onClose}
        className="max-h-[90vh] max-w-5xl"
      >
        <form className="flex gap-2" onSubmit={search} role="search">
          <Input
            aria-label="Search folder names and clip titles"
            placeholder="Search folder names and clip titles…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            maxLength={200}
            autoFocus
          />
          <Button disabled={loading}>
            <Search size={17} /> {loading ? "Searching…" : "Search"}
          </Button>
        </form>
        <p className="mt-2 text-xs text-slate-500">
          Searches every folder you can access. Recycle-bin items are excluded.
        </p>
        {error && (
          <p
            className="mt-4 rounded-lg border border-red-400/20 bg-red-400/10 px-3 py-2 text-sm text-red-200"
            role="alert"
          >
            {error}
          </p>
        )}
        <div className="mt-5 max-h-[62vh] overflow-y-auto pr-1">
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
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {results.map((result) => (
                <article
                  key={`${result.kind}-${result.id}`}
                  className="overflow-hidden rounded-xl border border-white/[.08] bg-slate-950"
                >
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
                  <div className="p-3">
                    <p className="text-[11px] font-semibold uppercase tracking-wide text-sky-300">
                      {result.kind}
                    </p>
                    <h3 className="mt-1 truncate font-medium text-white">
                      {result.name}
                    </h3>
                    <p
                      className="mt-1 truncate text-xs text-slate-500"
                      title={result.path}
                    >
                      {result.path}
                    </p>
                    {session.user.role === "admin" && (
                      <p className="mt-1 text-xs text-slate-600">
                        Owner: {result.ownerUsername}
                      </p>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-1 border-t border-white/[.06] bg-slate-950 px-2 py-2">
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
                  </div>
                </article>
              ))}
            </div>
          )}
          {truncated && (
            <p className="mt-4 rounded-lg border border-sky-400/15 bg-sky-400/[.06] px-3 py-2 text-sm text-sky-100">
              Showing the first 100 matches. Refine your search to narrow the
              results.
            </p>
          )}
        </div>
      </Modal>
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
    <div className="grid min-h-52 place-items-center rounded-xl border border-dashed border-white/10 bg-white/[.02] text-center">
      <div>
        <div className="mx-auto text-slate-700">{icon}</div>
        <h3 className="mt-3 font-medium text-slate-300">{title}</h3>
        <p className="mt-1 text-sm text-slate-600">{detail}</p>
      </div>
    </div>
  );
}
