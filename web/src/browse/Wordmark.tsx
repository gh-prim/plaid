/**
 * The PLAID logo (canvas "PLAID Design", step 14, "Woven play"): the play
 * triangle crossed by one warp and one weft thread, on its rounded tile,
 * then "plaid" drawn in strokes, the i dotted with a small play button.
 * Never translated. `size` is the lettering's x-height basis, in px, as
 * the old text wordmark's font size was.
 */
export function Wordmark({ size }: { size?: number }) {
  return (
    <span
      className="br-wordmark"
      style={size ? { fontSize: size } : undefined}
      role="img"
      aria-label="PLAID"
    >
      <PlayMark className="br-wordmark__mark" />
      <svg className="br-wordmark__name" viewBox="-2 14 245 122" aria-hidden="true">
        <g fill="none" stroke="currentColor" strokeWidth="12">
          <circle cx="31" cy="75" r="19" />
          <path d="M6 56 V130" />
          <path d="M78 20 V84 Q78 100 94 100" />
          <circle cx="125" cy="75" r="19" />
          <path d="M144 56 V100" />
          <path d="M170 56 V100" />
          <circle cx="214" cy="75" r="19" />
          <path d="M233 20 V100" />
        </g>
        <path
          d="M164 26 L180 35 L164 44 Z"
          fill="currentColor"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinejoin="round"
        />
      </svg>
    </span>
  )
}

/** The woven play button on its tile: the app icon, at any size. */
export function PlayMark({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 100 100" aria-hidden="true">
      <rect width="100" height="100" rx="24" fill="var(--wm-tile)" />
      <path
        d="M33 22 L81 50 L33 78 Z"
        fill="var(--wm-ink)"
        stroke="var(--wm-ink)"
        strokeWidth="6"
        strokeLinejoin="round"
      />
      <rect x="46" y="0" width="5" height="100" fill="var(--wm-tile)" />
      <rect x="0" y="47.5" width="100" height="5" fill="var(--wm-tile)" />
    </svg>
  )
}
