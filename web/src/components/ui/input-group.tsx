"use client";

import { Input, type InputProps } from "@/components/ui/input";
import type * as React from "react";
import { cn } from "@/lib/utils";

// InputGroup composes an Input with inline addons (icons, text, buttons)
// inside a single bordered, focusable container.
export function InputGroup({
  className,
  ...props
}: React.ComponentProps<"div">): React.ReactElement {
  return (
    <div
      className={cn(
        "flex w-full items-stretch rounded-lg border border-input bg-background shadow-xs/5 transition-shadow focus-within:ring-[3px] focus-within:ring-ring/24",
        className,
      )}
      data-slot="input-group"
      {...props}
    />
  );
}

export function InputGroupInput({
  className,
  ...props
}: InputProps): React.ReactElement {
  return (
    <Input
      unstyled
      className={cn("min-w-0 flex-1", className)}
      data-slot="input-group-input"
      {...props}
    />
  );
}

export function InputGroupAddon({
  className,
  ...props
}: React.ComponentProps<"span">): React.ReactElement {
  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center px-3 text-muted-foreground",
        className,
      )}
      data-slot="input-group-addon"
      {...props}
    />
  );
}

export function InputGroupText({
  className,
  ...props
}: React.ComponentProps<"span">): React.ReactElement {
  return (
    <span
      className={cn("text-sm text-muted-foreground", className)}
      data-slot="input-group-text"
      {...props}
    />
  );
}
