import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'

type LetterFacet = components['schemas']['LetterFacet']

/**
 * The letter index (F008-R15): on the right edge when sorted by title,
 * hidden on phones. A letter jumps to its first item, from the facets'
 * counts (letters come in title order).
 */
export function LetterIndex({
  letters,
  desc,
  total,
  onJump,
}: {
  letters: LetterFacet[]
  desc: boolean
  total: number
  onJump: (index: number) => void
}) {
  const { t } = useTranslation()
  const starts = new Map<string, number>()
  let at = 0
  for (const l of letters) {
    starts.set(l.letter, desc ? total - at - l.count : at)
    at += l.count
  }
  return (
    <nav className="br-letters" aria-label={t('grid.letters')}>
      {letters.map((l) => (
        <button
          key={l.letter}
          type="button"
          className="br-letters__item pl-focusable"
          onClick={() => onJump(Math.max(0, starts.get(l.letter) ?? 0))}
        >
          {l.letter}
        </button>
      ))}
    </nav>
  )
}
