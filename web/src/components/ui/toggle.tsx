"use client"

/* eslint-disable react-refresh/only-export-components */

import * as React from "react"
import * as TogglePrimitive from "@radix-ui/react-toggle"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"

const toggleVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium outline-none transition-[color,box-shadow] hover:bg-white/[.06] hover:text-white focus-visible:ring-2 focus-visible:ring-sky-300/70 disabled:pointer-events-none disabled:opacity-50 data-[state=on]:bg-sky-300/15 data-[state=on]:text-sky-200 [&_svg]:pointer-events-none [&_svg]:shrink-0",
  {
    variants: {
      variant: { default: "bg-transparent", outline: "border border-white/20 bg-transparent shadow-sm" },
      size: { default: "h-10 min-w-10 px-3", sm: "h-9 min-w-9 px-2", lg: "h-12 min-w-12 px-4" },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
)

function Toggle({ className, variant, size, ...props }: React.ComponentProps<typeof TogglePrimitive.Root> & VariantProps<typeof toggleVariants>) {
  return <TogglePrimitive.Root data-slot="toggle" className={cn(toggleVariants({ variant, size, className }))} {...props} />
}

export { Toggle, toggleVariants }
