import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it } from "vitest"
import { toast } from "sonner"
import { Badge } from "./badge"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "./empty"
import { Progress } from "./progress"
import { Skeleton } from "./skeleton"
import { Spinner } from "./spinner"
import { Toaster } from "./sonner"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "./tooltip"

describe("status and feedback primitives", () => {
  it("exposes status text, measured progress, and presentation primitives", () => {
    render(<><Badge>Ready</Badge><Spinner aria-label="Processing" /><Progress aria-label="Upload progress" value={42} max={100} /><Skeleton data-testid="skeleton" /><Empty><EmptyHeader><EmptyTitle>No clips</EmptyTitle><EmptyDescription>Upload your first clip.</EmptyDescription></EmptyHeader></Empty></>)
    expect(screen.getByText("Ready")).toBeVisible(); expect(screen.getByRole("status", { name: "Processing" })).not.toHaveTextContent("%")
    expect(screen.getByRole("progressbar", { name: "Upload progress" })).toHaveAttribute("aria-valuenow", "42"); expect(screen.getByText("No clips")).toBeVisible(); expect(screen.getByTestId("skeleton")).toHaveClass("animate-pulse")
  })

  it("opens supplemental Tooltip content from keyboard focus", async () => {
    const user = userEvent.setup(); render(<TooltipProvider delayDuration={0}><Tooltip><TooltipTrigger asChild><button>Clip info</button></TooltipTrigger><TooltipContent>Uploaded yesterday</TooltipContent></Tooltip></TooltipProvider>)
    await user.tab(); expect(await screen.findByRole("tooltip")).toHaveTextContent("Uploaded yesterday")
  })

  it("announces and dismisses a toast", async () => {
    const user = userEvent.setup(); render(<Toaster closeButton theme="dark" />); toast("Folder restored")
    expect(await screen.findByText("Folder restored")).toBeVisible(); await user.click(screen.getByRole("button", { name: "Close toast" })); await waitFor(() => expect(screen.queryByText("Folder restored")).not.toBeInTheDocument())
  })
})
