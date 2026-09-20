import { useRef, useState } from "react"
import { Play } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { cn } from "@/lib/utils"

type ClipPreviewProps = {
  title: string
  posterSrc: string
  videoSrc: string
  className?: string
  nested?: boolean
}

export function ClipPreview({ title, posterSrc, videoSrc, className, nested = false }: ClipPreviewProps) {
  const [open, setOpen] = useState(false)
  return <>
    <ClipPreviewThumbnail title={title} posterSrc={posterSrc} className={className} onPreview={() => setOpen(true)} />
    {open && <VideoPreviewDialog title={title} posterSrc={posterSrc} videoSrc={videoSrc} nested={nested} onClose={() => setOpen(false)} />}
  </>
}

export function ClipPreviewThumbnail({ title, posterSrc, className, onPreview }: Pick<ClipPreviewProps, "title" | "posterSrc" | "className"> & { onPreview: () => void }) {
  return <button className={cn("group relative block aspect-video w-full overflow-hidden bg-black focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-sky-400", className)} onClick={onPreview} aria-label={`Preview ${title}`}>
      <img className="h-full w-full object-cover opacity-90 transition group-hover:opacity-60 group-focus-visible:opacity-60" src={posterSrc} alt="" />
      <span className="absolute inset-0 grid place-items-center opacity-80 transition md:opacity-0 md:group-hover:opacity-100 md:group-focus-visible:opacity-100">
        <span className="grid size-11 place-items-center rounded-full bg-sky-400 text-slate-950 shadow-lg shadow-black/40"><Play size={19} fill="currentColor" /></span>
      </span>
    </button>
}

export function VideoPreviewDialog({ title, posterSrc, videoSrc, nested, onClose }: Omit<ClipPreviewProps, "className"> & { onClose: () => void }) {
  const [open, setOpen] = useState(true)
  const videoRef = useRef<HTMLVideoElement>(null)
  const returnFocusRef = useRef(document.activeElement instanceof HTMLElement ? document.activeElement : null)
  void nested
  return <Dialog open={open} onOpenChange={setOpen}>
    <DialogContent
      className="max-h-[90dvh] max-w-4xl overflow-y-auto"
      onOpenAutoFocus={(event) => { event.preventDefault(); videoRef.current?.focus() }}
      onCloseAutoFocus={(event) => { event.preventDefault(); returnFocusRef.current?.focus(); onClose() }}
    >
      <DialogHeader>
        <DialogTitle className="truncate pr-8">{title}</DialogTitle>
        <DialogDescription className="sr-only">Video preview with playback controls.</DialogDescription>
      </DialogHeader>
      <video ref={videoRef} className="max-h-[70dvh] w-full rounded-xl bg-black" controls playsInline preload="metadata" poster={posterSrc} src={videoSrc} tabIndex={0} />
    </DialogContent>
  </Dialog>
}
