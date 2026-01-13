import * as React from "react"
import { cn } from "@/lib/utils"

export type SpinnerProps = React.HTMLAttributes<HTMLDivElement> & {
  // Spinner-specific props extend HTMLDivElement attributes
}

const Spinner = React.forwardRef<HTMLDivElement, SpinnerProps>(
  ({ className, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          `
            inline-block animate-spin rounded-full border-2
            border-(--foreground) border-t-transparent
          `,
          className
        )}
        {...props}
      />
    )
  }
)
Spinner.displayName = "Spinner"

export { Spinner }
