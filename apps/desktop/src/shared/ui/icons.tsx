// Small inline icons (stroke = currentColor) so they follow the theme.
import type { ReactNode } from "react";

type IconProps = { size?: number };

function IconFrame({ size = 16, children }: IconProps & { children: ReactNode }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {children}
    </svg>
  );
}

export const RefreshIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M21 12a9 9 0 1 1-3-6.7" />
    <path d="M21 4v5h-5" />
  </IconFrame>
);

export const SettingsIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <circle cx="12" cy="12" r="3" />
    <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
  </IconFrame>
);

export const BackIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M15 18l-6-6 6-6" />
  </IconFrame>
);

export const ExternalLinkIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M14 4h6v6" />
    <path d="M20 4l-9 9" />
    <path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />
  </IconFrame>
);

export const LinkIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" />
    <path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" />
  </IconFrame>
);

export const UnreadDotIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <circle cx="12" cy="12" r="4" fill="currentColor" />
    <circle cx="12" cy="12" r="9" />
  </IconFrame>
);

export const ChevronRightIcon = (iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M9 18l6-6-6-6" />
  </IconFrame>
);

export const PowerIcon =(iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M12 2v10" />
    <path d="M18.4 6.6a9 9 0 1 1-12.8 0" />
  </IconFrame>
);

export const CheckAllIcon =(iconProps: IconProps) => (
  <IconFrame {...iconProps}>
    <path d="M2 12l5 5L18 6" />
    <path d="M12 16l1 1 9-11" />
  </IconFrame>
);
