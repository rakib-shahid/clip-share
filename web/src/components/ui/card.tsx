import * as React from "react"
import { cn } from "cn"

export function Card({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card" className={cn("rounded-2xl border border-white/[.08] bg-slate-900/70 shadow-2xl shadow-black/20", className)} {...props} /> }
export function CardHeader({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card-header" className={cn("p-6 pb-2", className)} {...props} /> }
export function CardTitle({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card-title" className={cn("font-semibold leading-none", className)} {...props} /> }
export function CardDescription({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card-description" className={cn("text-sm text-slate-400", className)} {...props} /> }
export function CardAction({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card-action" className={cn("self-start justify-self-end", className)} {...props} /> }
export function CardContent({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card-content" className={cn("p-6", className)} {...props} /> }
export function CardFooter({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="card-footer" className={cn("flex items-center p-6 pt-0", className)} {...props} /> }
