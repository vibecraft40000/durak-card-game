import type { ReactNode } from "react";

type IconProps = {
  size?: number;
  className?: string;
};

function IconBase({
  children,
  size = 20,
  className,
}: IconProps & { children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export function PlayIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M6 7L14 4L18 7L14 20L6 17V7Z" stroke="currentColor" strokeWidth="1.7" />
      <path d="M14 4V20" stroke="currentColor" strokeWidth="1.7" />
    </IconBase>
  );
}

export function PlusIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M12 5V19" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M5 12H19" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </IconBase>
  );
}

export function UserIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="12" cy="8" r="3.5" stroke="currentColor" strokeWidth="1.7" />
      <path
        d="M5.5 19C6.5 15.8 9 14.5 12 14.5C15 14.5 17.5 15.8 18.5 19"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
    </IconBase>
  );
}

/** Classic gear / cog icon */
export function SettingsIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.7" />
      <path
        d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1Z"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </IconBase>
  );
}

export function UsersIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="9" cy="9" r="3" stroke="currentColor" strokeWidth="1.7" />
      <circle cx="17" cy="8" r="2" stroke="currentColor" strokeWidth="1.5" />
      <path d="M4.5 19C5 16.5 7 15 9.5 15C12 15 14 16.5 14.5 19" stroke="currentColor" strokeWidth="1.7" />
      <path d="M15 15.5C16.5 15.7 18 16.8 18.5 19" stroke="currentColor" strokeWidth="1.5" />
    </IconBase>
  );
}

export function BackIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M14.5 6.5L9 12L14.5 17.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </IconBase>
  );
}

export function TrashIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M7 7H17" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      <path d="M9 7V5.8C9 5.35 9.35 5 9.8 5H14.2C14.65 5 15 5.35 15 5.8V7" stroke="currentColor" strokeWidth="1.7" />
      <path d="M8.5 9V17.2C8.5 17.65 8.85 18 9.3 18H14.7C15.15 18 15.5 17.65 15.5 17.2V9" stroke="currentColor" strokeWidth="1.7" />
      <path d="M10.5 10.5V16" stroke="currentColor" strokeWidth="1.5" />
      <path d="M13.5 10.5V16" stroke="currentColor" strokeWidth="1.5" />
    </IconBase>
  );
}

export function ChevronRightIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M9 6.5L14.5 12L9 17.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </IconBase>
  );
}

export function DotsHorizontalIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="6.5" cy="12" r="1.5" fill="currentColor" />
      <circle cx="12" cy="12" r="1.5" fill="currentColor" />
      <circle cx="17.5" cy="12" r="1.5" fill="currentColor" />
    </IconBase>
  );
}

export function TrendUpIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M5 16L10 11L13 14L19 8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M15 8H19V12" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </IconBase>
  );
}

/** Figma: Filter */
export function FilterIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path
        d="M4 6h16M4 12h10M4 18h6"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
    </IconBase>
  );
}

/** Withdraw: simple minus */
export function WithdrawIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M5 12h14" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </IconBase>
  );
}

/** Figma: lucide:plus (Ввод) */
export function DepositIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </IconBase>
  );
}

/** Figma: Calendar */
export function CalendarIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <rect x="3" y="4" width="18" height="18" rx="2" stroke="currentColor" strokeWidth="2" />
      <path d="M3 10h18M7 4v4M17 4v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </IconBase>
  );
}

/** Figma: Home Indicator (line) */
export function HomeIndicatorIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <rect x="6" y="10" width="12" height="4" rx="2" fill="currentColor" opacity="0.4" />
    </IconBase>
  );
}

/** Moon icon (dark theme) */
export function MoonIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path
        d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </IconBase>
  );
}

/** Sun icon (light theme) */
/** Close (X) for modal */
export function CloseIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </IconBase>
  );
}

/** Crypto Bot: light blue circle with white V (like Telegram) */
export function CryptoBotIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="12" cy="12" r="11" fill="#2AABEE" />
      <path d="M7 8l5 8 5-8" stroke="white" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </IconBase>
  );
}

export function SunIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="12" cy="12" r="4" stroke="currentColor" strokeWidth="1.7" />
      <path
        d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
    </IconBase>
  );
}
