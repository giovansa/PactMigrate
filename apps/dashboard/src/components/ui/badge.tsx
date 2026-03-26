import * as React from "react";

import { cn } from "@/lib/utils";

export type BadgeProps = React.HTMLAttributes<HTMLDivElement> & {
  variant?: "default" | "secondary" | "outline" | "success" | "danger";
};

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  const variantClass =
    variant === "default"
      ? "bg-primary text-primary-foreground hover:bg-primary/90"
      : variant === "secondary"
        ? "bg-secondary text-secondary-foreground hover:bg-secondary/80"
        : variant === "outline"
          ? "border border-border text-foreground bg-background"
          : variant === "success"
            ? "bg-emerald-600 text-white hover:bg-emerald-600/90"
            : "bg-destructive text-destructive-foreground hover:bg-destructive/90";

  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors",
        variantClass,
        className,
      )}
      {...props}
    />
  );
}

