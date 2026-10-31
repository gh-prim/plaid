/** The brand wordmark, never translated. */
const before = 'pla'
const after = 'd'
/** A dotless i: Foxy is its dot. */
const dotless = 'ı'

/**
 * The PLAID wordmark (V2): "plaid" in bold sans, Foxy the fox in place of
 * the dot on the i (a dotless ı under it).
 */
export function Wordmark({ size }: { size?: number }) {
  return (
    <span className="br-wordmark" style={size ? { fontSize: size } : undefined} aria-label="PLAID">
      <span aria-hidden="true">
        {before}
        <span className="br-wordmark__i">
          {dotless}
          <Foxy />
        </span>
        {after}
      </span>
    </span>
  )
}

/** Foxy, the fox of the brand. */
export function Foxy({ className }: { className?: string }) {
  return (
    <svg className={className ?? 'br-foxy'} viewBox="0 0 100 100" aria-hidden="true">
      <path d="M25 47 C14 33 9 15 15 4 C30 3 42 14 46 28 Z" fill="var(--fx-tan)" />
      <path
        d="M28 41 C20 31 17 19 20 11 C29 12 37 20 40 29 Z"
        fill="var(--fx-cream)"
        opacity="0.22"
      />
      <path
        d="M25 47 C14 33 9 15 15 4 C30 3 42 14 46 28"
        fill="none"
        stroke="var(--fx-ink)"
        strokeOpacity="0.6"
        strokeWidth="2.2"
      />
      <path d="M55 28 C61 14 77 7 92 11 C98 24 93 39 79 47 Z" fill="var(--fx-tan)" />
      <path
        d="M61 29 C66 20 77 15 88 16 C91 25 87 34 79 40 Z"
        fill="var(--fx-cream)"
        opacity="0.22"
      />
      <path
        d="M55 28 C61 14 77 7 92 11 C98 24 93 39 79 47"
        fill="none"
        stroke="var(--fx-ink)"
        strokeOpacity="0.6"
        strokeWidth="2.2"
      />
      <path
        d="M50 24 C68 24 80 36 81 52 C82 66 74 76 64 82 C59 86 54 88 50 88 C46 88 41 86 36 82 C26 76 18 66 19 52 C20 36 32 24 50 24 Z"
        fill="var(--fx-tan)"
      />
      <path
        d="M28 33 C34 27 42 24 50 24 C58 24 66 27 72 33 C65 31 57 32 50 36 C43 32 35 31 28 33 Z"
        fill="var(--fx-ink)"
        opacity="0.16"
      />
      <path
        d="M50 53 C58 53 63 60 64 67 C65 78 58 87 50 87 C42 87 35 78 36 67 C37 60 42 53 50 53 Z"
        fill="var(--fx-cream)"
      />
      <path
        d="M27 72 C30 79 36 84 42 86 C38 80 36 76 36 71 Z M73 72 C70 79 64 84 58 86 C62 80 64 76 64 71 Z"
        fill="var(--fx-cream)"
        opacity="0.7"
      />
      <ellipse cx="50" cy="64" rx="12" ry="8" fill="var(--fx-ink)" opacity="0.1" />
      <ellipse
        cx="38.5"
        cy="40"
        rx="3.6"
        ry="2.2"
        fill="var(--fx-cream)"
        opacity="0.8"
        transform="rotate(-18 38.5 40)"
      />
      <ellipse
        cx="61.5"
        cy="40"
        rx="3.6"
        ry="2.2"
        fill="var(--fx-cream)"
        opacity="0.8"
        transform="rotate(18 61.5 40)"
      />
      <circle cx="35" cy="48.5" r="6.3" fill="var(--fx-ink)" />
      <circle cx="65" cy="48.5" r="6.3" fill="var(--fx-ink)" />
      <circle cx="36.8" cy="46.4" r="1.8" fill="var(--fx-cream)" />
      <circle cx="66.8" cy="46.4" r="1.8" fill="var(--fx-cream)" />
      <path
        d="M28.5 45 C31 42.6 36 41.6 40.5 42.4 M59.5 42.4 C64 41.6 69 42.6 71.5 45"
        fill="none"
        stroke="var(--fx-ink)"
        strokeWidth="1.6"
        strokeLinecap="round"
        opacity="0.5"
      />
      <path
        d="M41.5 64.5 C41.5 58.5 58.5 58.5 58.5 64.5 C58.5 69.5 53.5 72.5 50 72.5 C46.5 72.5 41.5 69.5 41.5 64.5 Z"
        fill="var(--fx-ink)"
      />
      <path
        d="M45 61.6 C46.6 60.7 48.6 60.5 50.2 60.9"
        fill="none"
        stroke="var(--fx-cream)"
        strokeWidth="1.2"
        strokeLinecap="round"
        opacity="0.6"
      />
      <path
        d="M50 72.5 L50 75.5 M46.5 77 C48.5 78.2 51.5 78.2 53.5 77"
        fill="none"
        stroke="var(--fx-ink)"
        strokeWidth="1.8"
        strokeLinecap="round"
      />
    </svg>
  )
}
