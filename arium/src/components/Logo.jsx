// The arium mark: a geometric "a" with a terminal cursor, matching the
// "arium_" wordmark. Same drawing as public/favicon.svg, but the tile and
// glyph follow the theme so it sits naturally in light and dark mode.
export default function Logo({ className = 'h-7 w-7' }) {
  return (
    <svg viewBox="0 0 64 64" className={className} aria-hidden="true">
      <rect width="64" height="64" rx="14" className="fill-text-h" />
      <circle cx="23" cy="32.5" r="12" fill="none" strokeWidth="7" className="stroke-bg" />
      <rect x="33" y="16.5" width="7" height="31.5" rx="3.5" className="fill-bg" />
      <rect x="45" y="41.5" width="12" height="6.5" rx="1.5" className="fill-accent" />
    </svg>
  )
}
