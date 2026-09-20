import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"
import { Button } from "@/components/ui/button"

const attachmentVariants = cva("group/attachment relative flex w-full min-w-0 items-center gap-2 rounded-xl border bg-card p-2 text-card-foreground transition-colors data-[state=error]:border-destructive/30 data-[state=idle]:border-dashed")
export function Attachment({ className, state = "done", ...props }: React.ComponentProps<"div"> & VariantProps<typeof attachmentVariants> & { state?: "idle" | "uploading" | "processing" | "error" | "done" }) { return <div data-slot="attachment" data-state={state} className={cn(attachmentVariants(), className)} {...props} /> }
export function AttachmentMedia({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="attachment-media" className={cn("flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted text-foreground [&_svg]:size-5", className)} {...props} /> }
export function AttachmentContent({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="attachment-content" className={cn("min-w-0 flex-1 leading-tight", className)} {...props} /> }
export function AttachmentTitle({ className, ...props }: React.ComponentProps<"span">) { return <span data-slot="attachment-title" className={cn("block truncate text-sm font-medium", className)} {...props} /> }
export function AttachmentDescription({ className, ...props }: React.ComponentProps<"span">) { return <span data-slot="attachment-description" className={cn("mt-0.5 block truncate text-xs text-muted-foreground", className)} {...props} /> }
export function AttachmentActions({ className, ...props }: React.ComponentProps<"div">) { return <div data-slot="attachment-actions" className={cn("relative z-20 flex shrink-0 items-center gap-1", className)} {...props} /> }
export function AttachmentAction(props: React.ComponentProps<typeof Button>) { return <Button type="button" variant="ghost" size="sm" {...props} /> }
