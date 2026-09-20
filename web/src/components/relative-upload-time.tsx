import { useCallback, useMemo, useSyncExternalStore } from "react"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { formatRelativeUploadTime } from "@/lib/relative-time"

let currentNow = Date.now()
let timer: number | null = null
const subscribers = new Set<() => void>()

function emitCurrentTime() {
  currentNow = Date.now()
  for (const subscriber of subscribers) subscriber()
}

function startTicker() {
  if (typeof document === "undefined" || document.hidden || timer !== null) return
  timer = window.setInterval(emitCurrentTime, 60_000)
}

function stopTicker() {
  if (timer === null) return
  window.clearInterval(timer)
  timer = null
}

function onVisibilityChange() {
  if (document.hidden) {
    stopTicker()
  } else {
    emitCurrentTime()
    startTicker()
  }
}

function subscribeToClock(callback: () => void) {
  subscribers.add(callback)
  if (subscribers.size === 1) {
    currentNow = Date.now()
    document.addEventListener("visibilitychange", onVisibilityChange)
    startTicker()
  }
  return () => {
    subscribers.delete(callback)
    if (subscribers.size === 0) {
      stopTicker()
      document.removeEventListener("visibilitychange", onVisibilityChange)
    }
  }
}

function useSharedMinuteClock(active: boolean) {
  const subscribe = useCallback((callback: () => void) => active ? subscribeToClock(callback) : () => undefined, [active])
  return useSyncExternalStore(subscribe, () => currentNow, () => currentNow)
}

export function RelativeUploadTime({ createdAt, locale, className }: { createdAt?: string | null; locale?: string; className?: string }) {
  const timestamp = useMemo(() => {
    if (!createdAt) return null
    const parsed = Date.parse(createdAt)
    return Number.isFinite(parsed) ? parsed : null
  }, [createdAt])
  const now = useSharedMinuteClock(timestamp !== null)
  if (timestamp === null) return <span className={className}>Upload date unavailable</span>

  const exact = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(timestamp))
  return <Tooltip>
    <TooltipTrigger asChild>
      <time className={className} dateTime={createdAt ?? undefined} tabIndex={0} aria-label={`Uploaded ${exact}`}>
        {formatRelativeUploadTime(timestamp, now, locale)}
      </time>
    </TooltipTrigger>
    <TooltipContent>{exact}</TooltipContent>
  </Tooltip>
}
