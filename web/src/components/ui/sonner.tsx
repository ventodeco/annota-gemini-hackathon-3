"use client"

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import type { CSSProperties } from "react"
import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"

type SonnerTheme = NonNullable<ToasterProps["theme"]>
type ToasterStyle = CSSProperties & Record<`--${string}`, string>

function isSonnerTheme(theme: string): theme is SonnerTheme {
  return theme === "light" || theme === "dark" || theme === "system"
}

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()
  const sonnerTheme = isSonnerTheme(theme) ? theme : "system"
  const style: ToasterStyle = {
    "--normal-bg": "var(--popover)",
    "--normal-text": "var(--popover-foreground)",
    "--normal-border": "var(--border)",
    "--border-radius": "var(--radius)",
    top: "60px",
  }

  return (
    <Sonner
      theme={sonnerTheme}
      className="toaster group"
      position="top-center"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={style}
      {...props}
    />
  )
}

export { Toaster }