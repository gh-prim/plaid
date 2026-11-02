import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import {
  formatBytes,
  formatDuration,
  formatNumber,
  formatRelative,
  formatTime,
  languageName,
  regionName,
} from '../lib/format'
import { mountCommand, setfaclCommand } from '../lib/shellQuote'
import { PickerSheet } from '../onboarding/PickerSheet'
import { useServer } from '../onboarding/Page'
import { Button, CommandBlock, Dialog, EmptyState, Icon, TextField } from '../ui'
import { TopBar, useAdmin } from './AdminLayout'
import { LibrarySharing, useSharing } from './Sharing'
import { Group, Loading, Row, Section } from './v2'

type Library = components['schemas']['Library']
type Folder = components['schemas']['LibraryFolder']

/** "1,240 movies · 1,263 files" or "312 shows · 8,064 episodes · 8,070 files". */
function counts(t: ReturnType<typeof useTranslation>['t'], l: Library): string {
  return l.type === 'movies'
    ? t('libraries.counts.movies', { movies: l.counts.movies, files: l.counts.files })
    : t('libraries.counts.shows', {
        shows: l.counts.shows,
        episodes: l.counts.episodes,
        files: l.counts.files,
      })
}

function scanLine(t: ReturnType<typeof useTranslation>['t'], locale: string, l: Library): string {
  if (l.scan.state !== 'idle' || l.scan.queued) return t('scan.state.short.scanning')
  return l.lastScanAt
    ? t('libraries.lastScan', { time: formatRelative(locale, new Date(l.lastScanAt)) })
    : ''
}

/** Folders of a library that can't be read right now. */
const troubled = (l: Library) => l.folders.filter((f) => f.status !== 'ok')

/** Libraries rescan every 15 minutes unless set otherwise (F004-R19). */
const defaultInterval = 900

/**
 * Libraries (canvas step 21, board AdmEN02): four numbers, then every
 * library as a table row with its type, size, folder, last scan and who it
 * is shared with (F007 R20, F009).
 */
export function Libraries() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { data, refresh, toast } = useAdmin()
  const sharing = useSharing()
  const { data: stats } = usePolling(
    async () => (await api.GET('/server/stats')).data ?? null,
    60_000,
  )
  const [scanning, setScanning] = useState(false)
  // A next scan already due shows as now: the clock as the page opened.
  const [now] = useState(() => Date.now())
  const locale = i18n.resolvedLanguage ?? 'en'
  const lng = i18n.language
  const n = (v: number) => formatNumber(lng, v)
  const libs = data?.libraries ?? []
  const titles = libs.reduce(
    (a, l) => a + (l.type === 'movies' ? l.counts.movies : l.counts.shows),
    0,
  )
  const unmatched = libs.reduce((a, l) => a + l.counts.unrecognized, 0)
  const next = libs
    .filter((l) => l.lastScanAt)
    .map(
      (l) => new Date(l.lastScanAt!).getTime() + (l.scanIntervalSeconds ?? defaultInterval) * 1000,
    )
    .sort((a, b) => a - b)[0]
  const interval = Math.min(...libs.map((l) => l.scanIntervalSeconds ?? defaultInterval))
  const drive = stats?.drives.find((d) => d.available && d.freeBytes !== undefined)

  async function scanAll() {
    setScanning(true)
    await Promise.all(
      libs.map((l) =>
        api
          .POST('/libraries/{libraryId}/scan', {
            params: { path: { libraryId: l.id } },
            body: { full: false },
          })
          .catch(() => undefined),
      ),
    )
    setScanning(false)
    toast(t('admin.libraries.scanStarted', { count: libs.length }))
    await refresh()
  }

  /** Who sees a library: household groups, groups sharing it, people on their own. */
  function sharedWith(l: Library): string {
    if (!sharing.data) return ''
    const groups = sharing.data.groups.filter((g) => g.household || g.libraryIds.includes(l.id))
    const own = sharing.data.users.filter(
      (u) => u.role !== 'admin' && u.extraLibraryIds.includes(l.id),
    ).length
    if (!groups.length && !own) return t('sharing.library.nobody')
    const [first, ...rest] = groups
    return [
      first?.name,
      rest.length ? t('admin.libraries.moreGroups', { count: rest.length }) : undefined,
      own ? t('admin.libraries.morePeople', { count: own }) : undefined,
    ]
      .filter(Boolean)
      .join(l.householdOnly ? ' ' : ', ')
  }

  return (
    <>
      <TopBar title={t('libraries.title')}>
        {libs.length > 0 && (
          <Button variant="secondary" size="sm" loading={scanning} onClick={() => void scanAll()}>
            {t('admin.libraries.scanAll')}
          </Button>
        )}
        <Button size="sm" onClick={() => void navigate('/setup/libraries')}>
          <Icon name="plus" size={15} />
          {t('admin.libraries.add')}
        </Button>
      </TopBar>
      <div className="ax-body">
        {!data && <Loading />}
        {data && !libs.length && <EmptyState title={t('libraries.empty')} />}
        {libs.length > 0 && (
          <>
            <section className="ax-kpis">
              <div className="ax-card ax-kpi">
                <p>{t('admin.libraries.titles')}</p>
                <strong>{n(titles)}</strong>
                <span>{t('admin.libraries.inLibraries', { count: libs.length })}</span>
              </div>
              <Link to="/admin/unmatched" className="ax-card ax-kpi pl-focusable">
                <p>{t('admin.nav.unmatched')}</p>
                <strong>{n(unmatched)}</strong>
                <span>{t('admin.libraries.toReview')}</span>
              </Link>
              <div className="ax-card ax-kpi">
                <p>{t('admin.libraries.nextScan')}</p>
                <strong>{next ? formatTime(lng, new Date(Math.max(next, now))) : '—'}</strong>
                <span>
                  {t('admin.libraries.every', { duration: formatDuration(lng, interval * 1000) })}
                </span>
              </div>
              <div className="ax-card ax-kpi">
                <p>{t('admin.libraries.freeSpace')}</p>
                <strong>{drive ? formatBytes(lng, drive.freeBytes!) : '—'}</strong>
                <span className="ax-mono" title={drive?.path}>
                  {drive?.path ?? t('admin.dash.unknown')}
                </span>
              </div>
            </section>
            <section className="ax-card" aria-labelledby="ax-libs">
              <header className="ax-card__head">
                <h2 id="ax-libs">{t('admin.libraries.all')}</h2>
                <span>{n(libs.length)}</span>
              </header>
              <table className="ax-table ax-table--rows">
                <thead>
                  <tr>
                    <th aria-hidden="true" />
                    <th>{t('admin.libraries.colLibrary')}</th>
                    <th>{t('admin.libraries.colType')}</th>
                    <th>{t('admin.libraries.colTitles')}</th>
                    <th>{t('admin.libraries.colSize')}</th>
                    <th>{t('admin.libraries.colFolder')}</th>
                    <th>{t('admin.libraries.colScan')}</th>
                    <th>{t('admin.libraries.colShared')}</th>
                    <th aria-hidden="true" />
                  </tr>
                </thead>
                <tbody>
                  {libs.map((l) => {
                    const trouble = troubled(l)
                    const busy = l.scan.state !== 'idle' || l.scan.queued
                    return (
                      <tr key={l.id} onClick={() => void navigate(`/admin/libraries/${l.id}`)}>
                        <td>
                          <span className="ax-libicon">
                            <Icon name={l.type === 'movies' ? 'play' : 'tv'} size={14} />
                          </span>
                        </td>
                        <td className="ax-strong">
                          <Link
                            to={`/admin/libraries/${l.id}`}
                            className="pl-focusable"
                            onClick={(e) => e.stopPropagation()}
                          >
                            {l.name}
                          </Link>
                        </td>
                        <td className="ax-dim">
                          {l.type === 'movies' ? t('library.type.movies') : t('library.type.shows')}
                        </td>
                        <td className="ax-strong">
                          {l.type === 'movies'
                            ? n(l.counts.movies)
                            : t('admin.libraries.showsEpisodes', {
                                shows: n(l.counts.shows),
                                episodes: n(l.counts.episodes),
                              })}
                        </td>
                        <td className="ax-dim">{formatBytes(lng, l.counts.sizeBytes)}</td>
                        <td
                          className={trouble.length ? 'ax-warn' : 'ax-mono'}
                          title={l.folders.map((f) => f.path).join('\n')}
                        >
                          {trouble.length
                            ? t('admin.libraries.folderTrouble', {
                                count: trouble.length,
                                total: l.folders.length,
                              })
                            : (l.folders[0]?.path ?? '')}
                          {!trouble.length && l.folders.length > 1
                            ? ` +${l.folders.length - 1}`
                            : ''}
                        </td>
                        <td className="ax-dim">
                          {busy ? (
                            <span className="ax-scanning">
                              {t('scan.state.short.scanning')}
                              <span className="ax-bar ax-bar--thin" aria-hidden="true">
                                <span className="ax-bar__busy" />
                              </span>
                            </span>
                          ) : l.lastScanAt ? (
                            formatRelative(locale, new Date(l.lastScanAt))
                          ) : (
                            '—'
                          )}
                        </td>
                        <td className="ax-dim">{sharedWith(l)}</td>
                        <td>
                          <Icon name="chevronRight" size={14} />
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </section>
          </>
        )}
      </div>
    </>
  )
}

const isNetwork = (fs?: string) => !!fs && /^(nfs|cifs|smb|afp|fuse\.sshfs)/i.test(fs)

/** One library: folders, what to identify, scans and language (board TA03, F007 R20, R21, R22). */
export function LibraryPage() {
  const { t, i18n } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { serverName } = useServer()
  const { data, refresh, toast } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const lib = data?.libraries.find((l) => l.id === id)
  const [picking, setPicking] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [busy, setBusy] = useState<string>()
  const [language, setLanguage] = useState<{ lang: string; region: string }>()
  const back = { to: '/admin/libraries', label: t('libraries.title') }

  if (!lib)
    return (
      <Section title={t('libraries.title')} back={back} glyph="library">
        {data ? <EmptyState title={t('libraries.empty')} /> : <Loading />}
      </Section>
    )

  async function act(key: string, run: () => Promise<unknown>) {
    setBusy(key)
    try {
      await run()
      await refresh()
    } finally {
      setBusy(undefined)
    }
  }
  const patch = (body: components['schemas']['UpdateLibraryRequest']) =>
    api.PATCH('/libraries/{libraryId}', { params: { path: { libraryId: lib.id } }, body })
  const scan = (full: boolean) =>
    api.POST('/libraries/{libraryId}/scan', {
      params: { path: { libraryId: lib.id } },
      body: { full },
    })
  const remove = (f: Folder) =>
    act(`remove-${f.id}`, () =>
      patch({ folders: lib.folders.filter((x) => x.id !== f.id).map((x) => x.path) }),
    )
  const meta = (f: Folder) =>
    [
      f.watched ? t('folder.watch.liveShort') : t('folder.watch.periodicShort', { minutes: 15 }),
      `${isNetwork(f.fsType) ? t('folder.fs.network') : t('folder.fs.local')}${f.fsType ? ` (${f.fsType})` : ''}`,
    ].join(' · ')
  const gone = troubled(lib).length
  const check = (
    <div>
      <Button
        size="sm"
        loading={busy === 'check'}
        onClick={() => void act('check', () => scan(false))}
      >
        {t('picker.checkAgain')}
      </Button>
    </div>
  )

  return (
    <Section
      title={lib.name}
      back={back}
      glyph="library"
      sub={[counts(t, lib), scanLine(t, locale, lib)].filter(Boolean).join(' · ')}
      paneTitle={gone ? t('admin.library.unavailable', { count: gone }) : undefined}
      status={gone ? { text: t('folder.status.unavailable'), tone: 'warning' } : undefined}
      actions={
        <Button
          variant="secondary"
          size="sm"
          loading={busy === 'scan'}
          onClick={() => void act('scan', () => scan(false))}
        >
          <Icon name="refresh" size={14} />
          {t('libraries.scanNow')}
        </Button>
      }
      aside={
        <>
          <Group title={t('libraries.identifyReview')}>
            <Row
              to={`/admin/libraries/${lib.id}/unrecognized`}
              label={t('admin.settings.unrecognized')}
              value={formatNumber(i18n.language, lib.counts.unrecognized)}
              tone={lib.counts.unrecognized ? 'warning' : undefined}
              sub={t('libraries.unrecognized.help')}
            />
            <Row
              to={`/admin/libraries/${lib.id}/review`}
              label={t('review.needs.short')}
              sub={t('libraries.review.help')}
            />
          </Group>

          <LibrarySharing library={lib} onChange={() => void refresh()} />
        </>
      }
    >
      <Group
        title={t('library.folders.label')}
        foot={t('library.folders.count', { count: lib.folders.length })}
      >
        {lib.folders.map((f) =>
          f.status === 'unavailable' ? (
            <Row
              key={f.id}
              mono
              label={f.path}
              value={t('folder.status.unavailable')}
              tone="error"
              sub={meta(f)}
            >
              <p className="adm__text">{t('folder.unavailable.check')}</p>
              <CommandBlock command={mountCommand(f.path)} />
              {check}
            </Row>
          ) : f.status === 'unreadable' ? (
            <Row key={f.id} mono label={f.path} value={t('picker.noAccess')} tone="error">
              <p className="adm__text">{t('picker.unreadable.why', { name: f.path })}</p>
              <CommandBlock command={setfaclCommand(f.path)} />
              <p className="adm__help">{t('picker.unreadable.docker')}</p>
              {check}
            </Row>
          ) : (
            <Row
              key={f.id}
              mono
              label={f.path}
              value={t('folder.status.ok')}
              tone="ok"
              sub={meta(f)}
              control={
                lib.folders.length > 1 ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    loading={busy === `remove-${f.id}`}
                    aria-label={t('library.folders.remove', { path: f.path })}
                    onClick={() => void remove(f)}
                  >
                    {t('library.folders.removeShort')}
                  </Button>
                ) : undefined
              }
            />
          ),
        )}
        {lib.folders.length < 16 && (
          <Row
            label={t('admin.library.addFolder')}
            busy={busy === 'add'}
            onClick={() => setPicking(true)}
          />
        )}
      </Group>

      <Group
        title={t('admin.library.details')}
        foot={language ? undefined : t('libraries.language.help')}
      >
        {language ? (
          <Row label={t('library.metadata.language')}>
            <div className="adm__form">
              <TextField
                label={t('library.metadata.language')}
                value={language.lang}
                onValue={(v) => setLanguage({ ...language, lang: v.toLowerCase().slice(0, 2) })}
                maxLength={2}
              />
              <TextField
                label={t('library.metadata.region.label')}
                help={t('library.metadata.region.help')}
                value={language.region}
                onValue={(v) => setLanguage({ ...language, region: v.toUpperCase().slice(0, 2) })}
                maxLength={2}
              />
              <div className="adm__actions">
                <Button
                  loading={busy === 'lang'}
                  onClick={() =>
                    void act('lang', async () => {
                      await patch({
                        metadataLanguage: language.lang,
                        metadataRegion: language.region,
                      })
                      setLanguage(undefined)
                    })
                  }
                >
                  {t('common.save')}
                </Button>
                <Button variant="ghost" onClick={() => setLanguage(undefined)}>
                  {t('common.cancel')}
                </Button>
              </div>
            </div>
          </Row>
        ) : (
          <Row
            label={t('library.metadata.language')}
            value={t('admin.library.languageValue', {
              language: languageName(locale, lib.metadataLanguage),
              region: regionName(locale, lib.metadataRegion),
            })}
            onClick={() => setLanguage({ lang: lib.metadataLanguage, region: lib.metadataRegion })}
          />
        )}
      </Group>

      <Group
        title={t('libraries.scanning')}
        foot={t('libraries.fullRescan.help', { name: lib.name })}
      >
        <Row
          label={t('libraries.scanNow')}
          busy={busy === 'scan'}
          onClick={() => void act('scan', () => scan(false))}
        />
        <Row
          label={t('libraries.fullRescan.label')}
          busy={busy === 'full'}
          onClick={() => void act('full', () => scan(true))}
        />
      </Group>

      <Group>
        <Row danger label={t('libraries.delete.label')} onClick={() => setDeleting(true)} />
      </Group>

      {picking && (
        <PickerSheet
          serverName={serverName}
          onClose={() => setPicking(false)}
          onLost={() => setPicking(false)}
          onUse={(p) => {
            setPicking(false)
            void act('add', () => patch({ folders: [...lib.folders.map((f) => f.path), p] }))
          }}
        />
      )}
      {deleting && (
        <Dialog
          title={t('libraries.delete.confirm', { name: lib.name })}
          actions={
            <>
              <Button variant="ghost" onClick={() => setDeleting(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                onClick={() =>
                  void (async () => {
                    await api.DELETE('/libraries/{libraryId}', {
                      params: { path: { libraryId: lib.id } },
                    })
                    toast(t('libraries.deleted', { name: lib.name }))
                    await refresh()
                    await navigate('/admin/libraries')
                  })()
                }
              >
                {t('libraries.delete.label')}
              </Button>
            </>
          }
        />
      )}
    </Section>
  )
}
