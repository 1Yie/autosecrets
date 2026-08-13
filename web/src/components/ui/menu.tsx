"use client";

import { Menu as MenuPrimitive } from "@base-ui/react/menu";
import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

export const Menu = MenuPrimitive.Root;
export const MenuTrigger = MenuPrimitive.Trigger;
export const MenuGroup = MenuPrimitive.Group;

export function MenuSeparator({
  className,
  ...props
}: MenuPrimitive.Separator.Props): React.ReactElement {
  return (
    <MenuPrimitive.Separator
      className={cn("mx-2 my-1 h-px bg-border", className)}
      data-slot="menu-separator"
      {...props}
    />
  );
}

export function MenuGroupLabel({
  className,
  ...props
}: MenuPrimitive.GroupLabel.Props): React.ReactElement {
  return (
    <MenuPrimitive.GroupLabel
      className={cn(
        "flex items-center gap-2 px-2 py-1.5 text-xs text-muted-foreground",
        className,
      )}
      data-slot="menu-group-label"
      {...props}
    />
  );
}

export function MenuPopup({
  className,
  children,
  side = "top",
  sideOffset = 6,
  align = "start",
  alignOffset = 0,
  portalProps,
  ...props
}: MenuPrimitive.Popup.Props & {
  side?: MenuPrimitive.Positioner.Props["side"];
  sideOffset?: MenuPrimitive.Positioner.Props["sideOffset"];
  align?: MenuPrimitive.Positioner.Props["align"];
  alignOffset?: MenuPrimitive.Positioner.Props["alignOffset"];
  portalProps?: MenuPrimitive.Portal.Props;
}): React.ReactElement {
  return (
    <MenuPrimitive.Portal {...portalProps}>
      <MenuPrimitive.Positioner
        align={align}
        alignOffset={alignOffset}
        side={side}
        sideOffset={sideOffset}
        className="z-50 select-none"
        data-slot="menu-positioner"
      >
        <MenuPrimitive.Popup
          className={cn(
            "min-w-44 origin-(--transform-origin) rounded-lg border bg-popover p-1 text-foreground outline-none shadow-lg/5 not-dark:bg-clip-padding",
            className,
          )}
          data-slot="menu-popup"
          {...props}
        >
          {children}
        </MenuPrimitive.Popup>
      </MenuPrimitive.Positioner>
    </MenuPrimitive.Portal>
  );
}

export const menuItemVariants = cva(
  "flex w-full cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-base outline-none data-disabled:pointer-events-none data-disabled:opacity-64 data-highlighted:bg-accent data-highlighted:text-accent-foreground sm:text-sm [&_svg:not([class*='size-'])]:size-4.5 sm:[&_svg:not([class*='size-'])]:size-4 [&_svg]:pointer-events-none [&_svg]:shrink-0",
  {
    defaultVariants: { variant: "default" },
    variants: {
      variant: {
        default: "",
        destructive:
          "text-destructive data-highlighted:bg-destructive/10 data-highlighted:text-destructive",
      },
    },
  },
);

export function MenuItem({
  className,
  variant,
  ...props
}: MenuPrimitive.Item.Props &
  VariantProps<typeof menuItemVariants>): React.ReactElement {
  return (
    <MenuPrimitive.Item
      className={cn(menuItemVariants({ variant }), className)}
      data-slot="menu-item"
      {...props}
    />
  );
}
