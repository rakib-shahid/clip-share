import { useEffect, useState } from "react"

export function useMediaQuery(query: string, fallback = false) {
  const [matches, setMatches] = useState(() => typeof window !== "undefined" && typeof window.matchMedia === "function" ? window.matchMedia(query).matches : fallback)
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return
    const media = window.matchMedia(query)
    const update = (event: MediaQueryListEvent) => setMatches(event.matches)
    setMatches(media.matches)
    media.addEventListener("change", update)
    return () => media.removeEventListener("change", update)
  }, [query])
  return matches
}

export function useIsNarrow() {
  return useMediaQuery("(max-width: 639px)")
}
