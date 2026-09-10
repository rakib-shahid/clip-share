import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 rounded-lg border text-sm font-semibold shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300/70 disabled:pointer-events-none disabled:opacity-50",
  { variants: { variant: { default: "border-sky-300/60 bg-transparent text-slate-100 hover:border-sky-300/80 hover:bg-sky-300/10 [&_svg]:text-sky-300", success: "border-emerald-300/60 bg-transparent text-slate-100 hover:border-emerald-300/80 hover:bg-emerald-300/10 [&_svg]:text-emerald-300", danger: "border-rose-300/60 bg-transparent text-slate-100 hover:border-rose-300/80 hover:bg-rose-300/10 [&_svg]:text-rose-300", warning: "border-amber-300/60 bg-transparent text-slate-100 hover:border-amber-300/80 hover:bg-amber-300/10 [&_svg]:text-amber-300", secondary: "border-white/20 bg-transparent text-slate-200 hover:border-white/30 hover:bg-white/[.06] [&_svg]:text-slate-300", ghost: "border-transparent bg-transparent text-slate-300 shadow-none hover:bg-white/[.06] hover:text-white [&_svg]:text-slate-400" }, size: { default: "h-10 px-4", sm: "h-9 px-3", lg: "h-12 px-5" } }, defaultVariants: { variant: "default", size: "default" } }
)

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> { asChild?: boolean }
export function Button({ className, variant, size, asChild = false, ...props }: ButtonProps) {
  const Comp = asChild ? Slot : "button"
  return <Comp className={cn(buttonVariants({ variant, size }), className)} {...props} />
}
