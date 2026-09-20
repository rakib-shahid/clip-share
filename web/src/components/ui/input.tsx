import * as React from "react"
import { cn } from "cn"

export function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return <input type={type} data-slot="input" className={cn("h-11 w-full rounded-lg border border-white/10 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none placeholder:text-slate-600 focus:border-sky-400/70 focus:ring-2 focus:ring-sky-400/20", className)} {...props} />
}
