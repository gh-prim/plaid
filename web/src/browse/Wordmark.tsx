/** The brand wordmark, never translated. */
const wordmark = 'plaid'

/** The PLAID wordmark, in the serif (the Foxy dot waits for F002). */
export function Wordmark({ size }: { size?: number }) {
  return (
    <span className="br-wordmark" style={size ? { fontSize: size } : undefined} aria-label="PLAID">
      {wordmark}
    </span>
  )
}
