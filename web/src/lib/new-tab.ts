export function openInNewTab(url: string): Window | null {
  const opened = window.open(url, "_blank", "noopener,noreferrer")
  if (opened) opened.opener = null
  return opened
}
