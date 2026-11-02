import { describe, expect, it } from 'vitest'
import { guessEpisode } from './Review'

describe('guessEpisode', () => {
  it.each([
    ['/s/Game of Thrones/Game.Of.Thrones.Saison.1.MULTI/01 - L Hiver Vient.mkv', '1', '1'],
    ['/s/Show/Show.S02E05E06.1080p.mkv', '2', '5-6'],
    ['/s/Show/Season 3/Show 3x07.mkv', '3', '7'],
    ['/s/Show/Specials/Making of.mkv', '', ''],
  ])('%s', (path, season, episodes) => {
    expect(guessEpisode(path)).toEqual({ season, episodes })
  })
})
