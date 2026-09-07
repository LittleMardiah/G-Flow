"use client";

import type {
  AnchorHTMLAttributes,
  ButtonHTMLAttributes,
  ReactNode,
} from "react";

type Variant = "primary" | "secondary" | "outline" | "outlineWhite" | "ghost";
type Size = "sm" | "md" | "lg";

const base =
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-button font-semibold transition-all duration-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-60";

const variants: Record<Variant, string> = {
  primary:
    "bg-primary text-white shadow-[0_12px_30px_rgba(15,59,46,0.22)] hover:-translate-y-0.5 hover:bg-primary-light hover:shadow-soft",
  secondary:
    "bg-secondary text-white shadow-[0_12px_30px_rgba(212,168,83,0.28)] hover:-translate-y-0.5 hover:bg-secondary-dark hover:shadow-soft",
  outline:
    "border-2 border-secondary bg-transparent text-primary hover:-translate-y-0.5 hover:bg-secondary/10",
  outlineWhite:
    "border-2 border-white bg-transparent text-white hover:-translate-y-0.5 hover:bg-white/10",
  ghost: "bg-transparent text-primary hover:bg-accent",
};

const sizes: Record<Size, string> = {
  sm: "h-9 px-4 text-sm",
  md: "h-11 px-6 text-sm",
  lg: "h-13 px-8 text-base",
};

type CommonProps = {
  variant?: Variant;
  size?: Size;
  children: ReactNode;
};

type ButtonProps = CommonProps &
  ButtonHTMLAttributes<HTMLButtonElement> &
  Pick<AnchorHTMLAttributes<HTMLAnchorElement>, "href" | "target" | "rel">;

export function Button({
  variant = "primary",
  size = "md",
  className,
  children,
  href,
  target,
  rel,
  ...rest
}: ButtonProps) {
  const classes = `${base} ${variants[variant]} ${sizes[size]}${
    className ? ` ${className}` : ""
  }`;

  if (href !== undefined) {
    return (
      <a
        href={href}
        target={target}
        rel={rel}
        className={classes}
        {...(rest as AnchorHTMLAttributes<HTMLAnchorElement>)}
      >
        {children}
      </a>
    );
  }

  return (
    <button className={classes} {...rest}>
      {children}
    </button>
  );
}