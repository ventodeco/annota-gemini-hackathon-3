import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const buttonVariants = cva(
  `
    inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium
    whitespace-nowrap transition-colors
    focus-visible:ring-1 focus-visible:ring-(--ring) focus-visible:outline-none
    disabled:pointer-events-none disabled:opacity-50
    [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0
  `,
  {
    variants: {
      variant: {
        default:
          `
            bg-gray-900 text-white shadow-sm
            hover:bg-gray-800
          `,
        destructive:
          `
            bg-(--destructive) text-(--destructive-foreground) shadow-sm
            hover:bg-(--destructive)/90
          `,
        outline:
          `
            border border-(--input) bg-(--background) shadow-sm
            hover:bg-(--accent) hover:text-(--accent-foreground)
          `,
        secondary:
          `
            border border-gray-200 bg-white text-gray-900 shadow-sm
            hover:bg-gray-50
          `,
        ghost: "hover:bg-(--accent) hover:text-(--accent-foreground)",
        link: `
          text-(--primary) underline-offset-4
          hover:underline
        `,
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 rounded-md px-3 text-xs",
        lg: "h-10 rounded-md px-8",
        icon: "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => {
    return (
      <button
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

export { Button }
