import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg border border-transparent text-sm font-medium transition-[background-color,box-shadow,transform,color] duration-200 disabled:pointer-events-none disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground border-primary/30 shadow-[0_10px_30px_rgba(15,23,42,0.18)] active:translate-y-0.5 active:shadow-[inset_0_3px_8px_rgba(15,23,42,0.35)] dark:bg-primary/80 dark:border-primary/40",
        destructive:
          "bg-destructive text-destructive-foreground border-destructive/30 shadow-[0_10px_24px_rgba(0,0,0,0.18)] active:translate-y-0.5 active:shadow-[inset_0_3px_8px_rgba(0,0,0,0.35)] dark:bg-destructive/85 dark:border-destructive/50",
        secondary:
          "bg-secondary text-secondary-foreground border-secondary/30 shadow-[0_6px_18px_rgba(15,23,42,0.14)] active:translate-y-0.5 active:shadow-[inset_0_2px_5px_rgba(15,23,42,0.25)] dark:bg-secondary/60 dark:border-secondary/50 dark:text-secondary-foreground",
        outline:
          "border-border bg-background text-foreground shadow-[0_4px_16px_rgba(15,23,42,0.08)] active:translate-y-0.5 active:shadow-[inset_0_2px_6px_rgba(15,23,42,0.2)] dark:border-border/40 dark:bg-transparent dark:text-foreground",
        ghost:
          "bg-transparent text-foreground shadow-none active:translate-y-0.5 dark:text-muted-foreground",
        navbar:
          "bg-transparent text-muted-foreground shadow-none active:translate-y-0.5",
        link:
          "bg-transparent text-primary underline-offset-4 px-0 font-semibold tracking-wide shadow-none active:translate-y-0.5 dark:text-primary",
      },
      size: {
        default: "h-10 px-5 py-2.5",
        sm: "h-8 rounded-md px-3",
        lg: "h-11 px-6 text-base",
        icon: "h-10 w-10",
        "icon-sm": "h-8 w-8",
        "icon-lg": "h-12 w-12",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : "button"

  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
