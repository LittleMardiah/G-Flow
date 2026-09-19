import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

const strokeLine: IconProps = {
  fill: "none",
  strokeWidth: 1.8,
  strokeLinecap: "round",
  strokeLinejoin: "round",
};

export function GitHubIcon(props: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
      <path d="M9 18c-4.51 2-5-2-7-2" />
      <path d="M22 6.5c-2-1.5-4-1.5-6-1.5" />
    </svg>
  );
}

export function LinkedInIcon(props: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d="M16 8a6 6 0 0 1 6 6v7h-4v-7a2 2 0 0 0-2-2 2 2 0 0 0-2 2v7h-4v-7a6 6 0 0 1 6-6z" />
      <rect width="4" height="12" x="2" y="9" />
      <circle cx="4" cy="4" r="2" />
    </svg>
  );
}

export function RideIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" stroke="currentColor" {...strokeLine} aria-hidden="true" {...props}>
      <path d="M19 17h2c.6 0 1-.4 1-1v-3c0-.9-.7-1.7-1.5-1.9C18.7 10.6 16 10 16 10s-1.3-1.4-2.2-2.3c-.5-.4-1.1-.7-1.8-.7H5c-.6 0-1.1.4-1.4.9l-1.4 2.9A3.7 3.7 0 0 0 2 12v4c0 .6.4 1 1 1h2" />
      <circle cx="7" cy="17" r="2" />
      <path d="M9 17h6" />
      <circle cx="17" cy="17" r="2" />
    </svg>
  );
}

export function FoodIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" stroke="currentColor" {...strokeLine} aria-hidden="true" {...props}>
      <path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2" />
      <path d="M7 2v20" />
      <path d="M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3Zm0 0v7" />
    </svg>
  );
}

export function SendIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" stroke="currentColor" {...strokeLine} aria-hidden="true" {...props}>
      <path d="M11 21.73a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73z" />
      <path d="M12 22V12" />
      <path d="m3.3 7 8.7 5 8.7-5" />
    </svg>
  );
}

export function WalletIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" stroke="currentColor" {...strokeLine} aria-hidden="true" {...props}>
      <path d="M19 7V4a1 1 0 0 0-1-1H5a2 2 0 0 0 0 4h15a1 1 0 0 1 1 1v4h-3a2 2 0 0 0 0 4h3a1 1 0 0 0 1-1v-2a1 1 0 0 0-1-1" />
      <path d="M3 5v14a2 2 0 0 0 2 2h15a1 1 0 0 0 1-1v-4" />
    </svg>
  );
}

export function GoIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="Go" {...props}>
      <circle cx="15.5" cy="5.5" r="3.5" fill="#00ADD8" />
      <rect x="3" y="12.5" width="11" height="3.4" rx="1.7" fill="#00ADD8" />
      <path
        d="M20.6 14.2l-3.4 3.4 3.4 3.4"
        fill="none"
        stroke="#00ADD8"
        strokeWidth="2.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function FlutterIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="Flutter" {...props}>
      <path
        d="M5.5 8.1 2.6 11l9.2 9.2h.1l3.1 3.3-.4-3.2 4.7-4.7h-8.2L5.5 8.1z"
        fill="#0175C2"
      />
      <path d="M5.5 8.1 8.6 5l9.4 9.2h-3.9L5.5 8.1z" fill="#049BF0" />
      <path d="m5.5 8.1 3.1-3.1h4.1L8.6 9.1l-3.1-1z" fill="#042B59" />
    </svg>
  );
}

export function NextIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="Next.js" {...props}>
      <path
        d="M12 1a11 11 0 1 0 0 22 11 11 0 0 0 0-22zm-1.9 15.6V7.4l5.4 5.4z"
        fill="#111827"
      />
      <path
        d="M15.6 6.4v9.8l-1.6-1.9V8.4zM6.5 8.3 14.4 16.1l-1.2.7-8.9-8.6v7.5h-1V6.8v0z"
        fill="#ffffff"
        opacity="0.92"
      />
    </svg>
  );
}

export function SupabaseIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="Supabase" {...props}>
      <path d="M13.6 1.2 3.4 12.5h9.2l-1.8 10.3.4-.4L20.6 11h-8.4l1.4-9.8z" fill="#3ECF8E" />
      <path d="M13.6 1.2 3.4 12.5h9.2l-1.8 10.3.4-.4L20.6 11h-8.4l1.4-9.8z" fill="#1E1E1E" opacity="0.25" />
    </svg>
  );
}

export function RedisIcon(props: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="Redis" {...props}>
      <path
        d="M12 2.5 21.5 12 12 21.5 2.5 12z"
        fill="none"
        stroke="#DC382D"
        strokeWidth="2.2"
        strokeLinejoin="round"
      />
      <path
        d="M7.4 12.2h9.2M9.8 8.9h6.8M9.8 15.4h6.8"
        fill="none"
        stroke="#DC382D"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}