import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import App from "./App"
import { ThemeProvider } from "next-themes"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { AppMotionProvider } from "@/components/motion-provider"
import "./index.css"

createRoot(document.getElementById("root")!).render(<StrictMode><ThemeProvider attribute="class" forcedTheme="dark"><AppMotionProvider><TooltipProvider delayDuration={300}><App /><Toaster richColors closeButton /></TooltipProvider></AppMotionProvider></ThemeProvider></StrictMode>)
