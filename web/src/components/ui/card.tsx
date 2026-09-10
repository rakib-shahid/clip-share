import * as React from "react"
import { cn } from "@/lib/utils"

export function Card({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("rounded-2xl border border-white/[.08] bg-slate-900/70 shadow-2xl shadow-black/20", className)} {...props} /> }
export function CardHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("p-6 pb-2", className)} {...props} /> }
export function CardContent({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) { return <div className={cn("p-6", className)} {...props} /> }
