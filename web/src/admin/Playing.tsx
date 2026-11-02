import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { formatClock, formatMbps } from '../lib/format'
import { Button, Dialog, StreamSpeed } from '../ui'
import { streamArt, streamPoster, useAdmin } from './AdminLayout'
import { Spark } from './Dashboard'
import { Loading, Section } from './v2'
import { methodText, reasonText } from './reasons'
import { slow } from './summary'

type ActivePlayback = components['schemas']['ActivePlayback']

/** Playing now refreshes every 5 s while visible (F007 R19). */
export const playingEvery = 5000

async function load() {
  const [p, s] = await Promise.all([api.GET('/playback'), api.GET('/settings/playback')])
  return {
    streams: p.data?.sessions ?? [],
    remoteMaxBitrate: s.data?.remoteMaxBitrate ?? 0,
    maxTranscodes: s.data?.maxTranscodes,
  }
}

/** Slowest first: a stream that can't keep up, then transcodes, then the rest. */
export function bySeverity(a: ActivePlayback, b: ActivePlayback): number {
  const rank = (s: ActivePlayback) =>
    slow(s)
      ? 0
      : s.session.method === 'transcode'
        ? 1
        : s.session.method === 'direct_stream'
          ? 2
          : 3
  return rank(a) - rank(b) || (a.speed ?? 9) - (b.speed ?? 9)
}

/** Two minutes of bitrate at one reading per poll (board AdmV2 07's curve). */
const keep = (2 * 60_000) / playingEvery

/**
 * Keeps each stream's last bitrates between polls, oldest first, so a card
 * can draw its curve; streams that ended are dropped.
 */
function useBitrates(streams: ActivePlayback[]) {
  const [history, setHistory] = useState<Record<string, number[]>>({})
  const [seen, setSeen] = useState<ActivePlayback[]>()
  if (seen !== streams) {
    // Adjusting state while rendering, on a new poll result only.
    setSeen(streams)
    const next: Record<string, number[]> = {}
    for (const s of streams) {
      const prev = history[s.session.id] ?? []
      next[s.session.id] = s.bitrateBps != null ? [...prev, s.bitrateBps].slice(-keep) : prev
    }
    setHistory(next)
  }
  return history
}

/**
 * Now playing (board AdmV2 07): one card per stream, slowest first, with
 * how it's sent and why, where it is, its bitrate over two minutes, and Stop.
 */
export function Playing() {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const { refresh: refreshSummary, toast } = useAdmin()
  const { data, refresh } = usePolling(load, playingEvery)
  const [stopping, setStopping] = useState<ActivePlayback>()
  const streams = useMemo(() => [...(data?.streams ?? [])].sort(bySeverity), [data])
  const rates = useBitrates(streams)
  const converting = streams.filter((s) => s.session.method === 'transcode').length
  const limitMbps = data?.remoteMaxBitrate ? data.remoteMaxBitrate / 1e6 : undefined

  async function stop(s: ActivePlayback) {
    setStopping(undefined)
    await api.DELETE('/playback/{sessionId}', { params: { path: { sessionId: s.session.id } } })
    toast(t('playing.stopped', { title: s.itemTitle, user: s.profileName }))
    await Promise.all([refresh(), refreshSummary()])
  }

  return (
    <Section
      title={t('playing.title')}
      sub={
        data && data.maxTranscodes != null
          ? `${t('admin.summary.playing', { count: streams.length })} · ${t('playing.capacity', { count: converting, max: data.maxTranscodes })}`
          : undefined
      }
    >
      {!data && <Loading />}
      {data && streams.length === 0 && (
        <section className="ax-card adm__empty">
          <p className="title">{t('playing.empty.title')}</p>
          <p className="adm__text">{t('playing.empty.label')}</p>
        </section>
      )}
      <ul className="ax-plays">
        {streams.map((s) => {
          const isSlow = slow(s)
          const reasons = s.session.reasons.map((r) =>
            reasonText(t, r, { remote: s.remote, limitMbps }),
          )
          const advice =
            isSlow && !s.session.hardwareAcceleration
              ? t('playing.slow.advice.software', { user: s.profileName })
              : undefined
          const curve = rates[s.session.id] ?? []
          const poster = streamArt(s, 'backdrop', 400) ?? streamPoster(s, 154)
          return (
            <li key={s.session.id}>
              <article className={`ax-card ax-play${isSlow ? ' is-slow' : ''}`}>
                <div className="ax-play__still">
                  {poster ? <img src={poster} alt="" /> : <i className="ax-poster" />}
                </div>
                <div className="ax-play__body">
                  <div className="ax-play__head">
                    <h2>{s.itemTitle}</h2>
                    <span
                      className={`ax-chip ${s.session.method === 'transcode' ? 'is-warning' : 'is-ok'}`}
                    >
                      {methodText(t, s.session.method)}
                      {s.session.method === 'transcode' &&
                        (s.session.hardwareAcceleration
                          ? ` · ${t('playing.hw.hardware', { accelerator: s.session.hardwareAcceleration.toUpperCase() })}`
                          : ` · ${t('playing.hw.software')}`)}
                    </span>
                    {isSlow && (
                      <span className="ax-chip is-error">{t('admin.playing.stutter')}</span>
                    )}
                  </div>
                  <p className="ax-play__who">
                    {[
                      s.profileName,
                      s.deviceName,
                      t(s.remote ? 'playing.where.remote' : 'playing.where.local'),
                      s.session.quality === 'original' ? undefined : s.session.quality,
                    ]
                      .filter(Boolean)
                      .join(t('card.separator'))}
                  </p>
                  {(reasons.length > 0 || advice) && (
                    <p className="ax-play__why">
                      {[...reasons, advice]
                        .filter((x) => x !== undefined)
                        .map((x) => (
                          <span key={x}>{x}</span>
                        ))}
                    </p>
                  )}
                  <div className="ax-play__pos">
                    <span className="ax-bar">
                      <span
                        style={{
                          width: `${s.session.durationMs ? (100 * s.positionMs) / s.session.durationMs : 0}%`,
                        }}
                      />
                    </span>
                    <small>
                      {t('playing.position', {
                        position: formatClock(s.positionMs),
                        duration: formatClock(s.session.durationMs),
                      })}
                    </small>
                  </div>
                  <div className="ax-play__foot">
                    <span className="ax-play__rate">
                      <b>{s.bitrateBps != null ? formatMbps(lng, s.bitrateBps) : '—'}</b>
                      {curve.length > 1 && <Spark values={curve} />}
                      <small>{t('admin.playing.lastTwo')}</small>
                      {s.speed != null && s.session.method === 'transcode' && (
                        <StreamSpeed speed={s.speed} />
                      )}
                    </span>
                    <Button
                      variant="secondary"
                      size="sm"
                      aria-label={t('admin.playing.stopFor', {
                        title: s.itemTitle,
                        user: s.profileName,
                      })}
                      onClick={() => setStopping(s)}
                    >
                      {t('playing.stop.label')}
                    </Button>
                  </div>
                </div>
              </article>
            </li>
          )
        })}
      </ul>
      {stopping && (
        <Dialog
          title={t('playing.stop.confirm.label', {
            title: stopping.itemTitle,
            user: stopping.profileName,
          })}
          actions={
            <>
              <Button variant="ghost" onClick={() => setStopping(undefined)}>
                {t('playing.stop.confirm.cancel')}
              </Button>
              <Button onClick={() => void stop(stopping)}>
                {t('playing.stop.confirm.action')}
              </Button>
            </>
          }
        />
      )}
    </Section>
  )
}
