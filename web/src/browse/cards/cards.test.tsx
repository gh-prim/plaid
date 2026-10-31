import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import type { components } from '../../api/schema'
import { fakeApi } from '../../test/fakeApi'
import { renderUi } from '../../test/renderUi'
import { Row } from '../Row'
import { LandscapeCard } from './LandscapeCard'
import { PosterCard } from './PosterCard'

type ItemSummary = components['schemas']['ItemSummary']

const item = (extra: Partial<ItemSummary> = {}): ItemSummary => ({
  id: 'm1',
  kind: 'movie',
  displayTitle: 'Dune: Part Two',
  matchStatus: 'matched',
  genres: [],
  fileCount: 1,
  childCount: 0,
  technical: { hdrFormats: [], audioLanguages: [] },
  year: 2024,
  poster: { imageId: 'po1' },
  backdrop: { imageId: 'bd1' },
  ...extra,
})

const inRouter = (ui: React.ReactElement, lang?: string) =>
  renderUi(<MemoryRouter>{ui}</MemoryRouter>, lang)

describe('PosterCard', () => {
  it('is a link named by the title, with a poster at the displayed width', async () => {
    await inRouter(<PosterCard item={item()} />)
    const link = screen.getByRole('link', { name: /Dune: Part Two/ })
    expect(link).toHaveAttribute('href', '/item/m1')
    const img = link.querySelector('img')!
    expect(img).toHaveAttribute('loading', 'lazy')
    expect(img.getAttribute('srcset')).toContain('/api/v1/images/po1?w=185 185w')
    expect(img.getAttribute('srcset')).not.toContain('w=780')
    expect(within(link).getByText('2024')).toBeInTheDocument()
  })

  it('shows the missing-artwork plate with the title', async () => {
    await inRouter(<PosterCard item={item({ poster: undefined })} />)
    expect(screen.getByRole('link').querySelector('img')).toBeNull()
    expect(screen.getAllByText('Dune: Part Two')).toHaveLength(2)
  })

  it('marks a watched movie and a show’s new episodes', async () => {
    const { rerender } = await inRouter(
      <PosterCard item={item({ userState: { positionMs: 0, watched: true, playCount: 1 } })} />,
    )
    expect(screen.getByText('Watched')).toBeInTheDocument()
    rerender(
      <MemoryRouter>
        <PosterCard
          item={item({
            kind: 'show',
            userState: {
              positionMs: 0,
              watched: false,
              playCount: 0,
              watchedEpisodes: 3,
              totalEpisodes: 13,
            },
          })}
        />
      </MemoryRouter>,
    )
    expect(screen.getByText('10 new')).toBeInTheDocument()
  })

  it('shows the progress thread with its percentage in words', async () => {
    await inRouter(
      <PosterCard
        item={item({
          userState: { positionMs: 3_734_000, durationMs: 9_960_000, watched: false, playCount: 0 },
        })}
      />,
    )
    expect(screen.getByRole('progressbar', { name: '37 % watched' })).toHaveAttribute(
      'aria-valuenow',
      '37',
    )
  })
})

describe('LandscapeCard', () => {
  const episode = item({
    id: 'e4',
    kind: 'episode',
    displayTitle: 'There’s No Disgrace Like Home',
    seasonNumber: 1,
    episodeNumber: 4,
    still: { imageId: 'st4' },
    show: { id: 'simpsons', displayTitle: 'The Simpsons' },
    userState: { positionMs: 780_000, durationMs: 1_380_000, watched: false, playCount: 0 },
  })

  it('names the show and the episode, with the time left, and plays directly', async () => {
    await inRouter(<LandscapeCard item={episode} />)
    const link = screen.getByRole('link', { name: /The Simpsons/ })
    expect(link).toHaveAttribute('href', '/watch/e4?t=780')
    expect(
      within(link).getByText('S1 E4 · There’s No Disgrace Like Home · 10m left'),
    ).toBeInTheDocument()
    expect(link.querySelector('img')?.getAttribute('src')).toContain('/images/st4')
  })

  it('says it in French', async () => {
    await inRouter(<LandscapeCard item={episode} />, 'fr-FR')
    expect(
      screen.getByText('S1 É4 · There’s No Disgrace Like Home · Encore 10min'),
    ).toBeInTheDocument()
  })

  it('marks it watched from its menu, then goes to the show', async () => {
    const calls = fakeApi({ 'PUT /items/e4/watched': { status: 204 } })
    const onChange = vi.fn()
    await inRouter(<LandscapeCard item={episode} onChange={onChange} />)
    await userEvent.click(screen.getByRole('button', { name: 'More actions for The Simpsons' }))
    expect(screen.getByRole('menuitem', { name: 'Go to show' })).toHaveAttribute(
      'href',
      '/item/simpsons',
    )
    await userEvent.click(screen.getByRole('menuitem', { name: 'Mark as watched' }))
    expect(calls.map((c) => `${c.method} ${c.path}`)).toEqual(['PUT /items/e4/watched'])
    expect(onChange).toHaveBeenCalled()
  })
})

describe('Row', () => {
  it('is a titled list with See all', async () => {
    await inRouter(
      <Row
        title="Recently added in Movies"
        seeAll="/library/lm?sort=added"
        page={{ items: [item(), item({ id: 'm2', displayTitle: 'Barbie' })] }}
      />,
    )
    const row = screen.getByRole('region', { name: 'Recently added in Movies' })
    expect(within(row).getAllByRole('listitem')).toHaveLength(2)
    expect(within(row).getByRole('link', { name: /See all/ })).toHaveAttribute(
      'href',
      '/library/lm?sort=added',
    )
  })

  it('loads the next page when its end comes into view', async () => {
    let fire: (entries: { isIntersecting: boolean }[]) => void = () => {}
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(cb: typeof fire) {
          fire = cb
        }
        observe() {}
        disconnect() {}
      },
    )
    const more = vi.fn(async () => ({ items: [item({ id: 'm3', displayTitle: 'Casablanca' })] }))
    await inRouter(<Row title="Drama" page={{ items: [item()], nextCursor: 'c1' }} more={more} />)
    fire([{ isIntersecting: true }])
    expect(await screen.findByRole('link', { name: /Casablanca/ })).toBeInTheDocument()
    expect(more).toHaveBeenCalledWith('c1')
  })
})
