import type { ReactNode } from "react";
import { Reveal } from "@/components/Reveal";

type SectionHeaderProps = {
  badge?: ReactNode;
  title: ReactNode;
  subtitle?: ReactNode;
  align?: "left" | "center";
  className?: string;
};

export function SectionHeader({
  badge,
  title,
  subtitle,
  align = "center",
  className = "",
}: SectionHeaderProps) {
  const alignCls =
    align === "center" ? "mx-auto text-center" : "text-left";

  return (
    <Reveal className={`max-w-2xl ${alignCls} ${className}`}>
      {badge && (
        <span className="inline-flex items-center gap-2 rounded-button border border-neutral-200 bg-accent/70 px-4 py-1.5 text-xs font-bold uppercase tracking-widest text-secondary-dark">
          <span className="h-1.5 w-1.5 rounded-full bg-accent" />
          {badge}
        </span>
      )}
      <h2 className="mt-5 text-3xl font-extrabold tracking-tight text-text-primary sm:text-4xl">
        {title}
      </h2>
      {subtitle && (
        <p className="mt-4 text-base leading-relaxed text-text-support sm:text-lg">
          {subtitle}
        </p>
      )}
    </Reveal>
  );
}
