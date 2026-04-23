/** Small inline SVGs — no extra deps */
export const iconClass = "icon";

export function IconLogo() {
  return (
    <svg className={`${iconClass} ${iconClass}--lg`} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M4 8a4 4 0 0 1 4-4h2a2 2 0 0 1 2 2v0a2 2 0 0 0 2 2h0a2 2 0 0 1 2 2v0a2 2 0 0 0 2 2h2a4 4 0 0 0 4-4V8a4 4 0 0 0-4-4h-2a2 2 0 0 0-2 2v0a2 2 0 0 1-2 2h-2a2 2 0 0 0-2-2V6a2 2 0 0 0-2-2H8a4 4 0 0 0-4 4v2Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path d="M8 14v4a2 2 0 0 0 2 2h1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconProject() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M4 5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path
        d="M14 5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-4a1 1 0 0 1-1-1V5Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path
        d="M4 15a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path
        d="M14 15a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-4a1 1 0 0 1-1-1v-4Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
    </svg>
  );
}

export function IconCompute() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3" y="4" width="18" height="6" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
      <rect x="3" y="14" width="18" height="6" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M7 7h.01M7 17h.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

export function IconImages() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="8.5" cy="8.5" r="1.5" fill="currentColor" />
      <path d="M21 15l-5-5-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconTerraform() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path d="M8 3h8v6H8V3Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
      <path d="M3 10h7v6H3v-6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
      <path d="M14 10h7v6h-7v-6Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
      <path d="M8 17h8v4H8v-4Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  );
}

export function IconActivity() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path d="M4 6h16M4 12h10M4 18h16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconChevronRight({ className }: { className?: string }) {
  return (
    <svg
      className={className ?? "icon icon--sm"}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden
    >
      <path d="M9 5l7 7-7 7" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function IconSearch() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="11" cy="11" r="6" stroke="currentColor" strokeWidth="1.5" />
      <path d="M20 20l-4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconBell() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M12 3a4 4 0 0 0-4 4v2.4c0 1.1-.3 2.1-.8 2.8L5 16h14l-2.2-3.8c-.5-.7-.8-1.7-.8-2.8V7a4 4 0 0 0-4-4Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path d="M10 20a2 2 0 0 0 4 0" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function IconSunMoon() {
  return (
    <svg className={iconClass} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M12 3a9 9 0 0 0 0 18 7 7 0 0 0 0-18ZM12 5v14a5 5 0 0 0 0-14Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

export function IconSettings() {
  return (
    <svg
      className={iconClass}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      aria-hidden
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.65.87.314.18.7.2 1.04.1l1.2-.3a1.125 1.125 0 0 1 1.3.4l.9 1.5c.2.3.1.7-.1 1l-1.2 1.5c-.3.3-.2.7 0 1.1.2.4.1.7-.1 1.1l-.9 1.5a1.125 1.125 0 0 1-1.3.4l-1.2-.3c-.34-.1-.72-.1-1.04.1-.33.2-.59.5-.65.9l-.21 1.3c-.09.5-.56.9-1.11.9H10.7c-.55 0-1.02-.4-1.11-.9l-.21-1.3c-.06-.4-.32-.7-.65-.9-.32-.1-.7-.1-1.04.1l-1.2.3a1.125 1.125 0 0 1-1.3-.4l-.9-1.5c-.2-.3-.1-.7.1-1.1.2-.4.1-.7-.1-1.1l.9-1.5a1.125 1.125 0 0 1 1.3-1.1l1.2.3c.34.1.72.1 1.04-.1.32-.2.59-.5.65-.9l.21-1.3z"
      />
      <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
    </svg>
  );
}
