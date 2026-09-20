export function formatRelativeUploadTime(timestamp: number, now: number, locale?: string): string {
  const elapsed = Math.max(0, now - timestamp)
  const relative = new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" })
  if (elapsed < 60_000) return "Just now"
  if (elapsed < 60 * 60_000) return relative.format(-Math.floor(elapsed / 60_000), "minute")

  const uploaded = new Date(timestamp)
  const current = new Date(now)
  const uploadedDay = new Date(uploaded.getFullYear(), uploaded.getMonth(), uploaded.getDate()).getTime()
  const currentDay = new Date(current.getFullYear(), current.getMonth(), current.getDate()).getTime()
  if (Math.round((currentDay - uploadedDay) / 86_400_000) === 1) return capitalize(relative.format(-1, "day"))
  if (elapsed < 24 * 60 * 60_000) return relative.format(-Math.floor(elapsed / (60 * 60_000)), "hour")
  return new Intl.DateTimeFormat(locale, {
    month: "short",
    day: "numeric",
    ...(uploaded.getFullYear() === current.getFullYear() ? {} : { year: "numeric" as const }),
  }).format(uploaded)
}

function capitalize(value: string) {
  return value.length === 0 ? value : value[0].toLocaleUpperCase() + value.slice(1)
}
