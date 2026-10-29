import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { formatRelative, languageName, regionName } from '../lib/format'
import { mountCommand } from '../lib/shellQuote'
import { PickerSheet } from '../onboarding/PickerSheet'
import { useServer } from '../onboarding/Page'
import {
  Button,
  CommandBlock,
  Dialog,
  EmptyState,
  FolderRow,
  LibraryRow,
  Mark,
  TextField,
} from '../ui'
import { Section, useAdmin } from './AdminLayout'

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

/** Every library, each with its counts, scan and folder trouble (board AP05, F007 R20). */
export function Libraries() {
  const { t, i18n } = useTranslation()
  const { data } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const libs = data?.libraries ?? []
  return (
    <Section>
      <div className="adm__head">
        <h2 className="adm__h2">{t('libraries.title')}</h2>
        <Link to="/setup/libraries" className="pl-btn pl-btn--secondary pl-btn--sm">
          {t('home.empty.action')}
        </Link>
      </div>
      {data && !libs.length && <EmptyState title={t('libraries.empty')} />}
      <div className="adm__rows">
        {libs.map((l) => {
          const gone = l.folders.find((f) => f.status !== 'ok')
          return (
            <LibraryRow
              key={l.id}
              to={`/admin/libraries/${l.id}`}
              name={l.name}
              counts={counts(t, l)}
              scan={scanLine(t, locale, l)}
              issue={
                gone
                  ? gone.status === 'unavailable'
                    ? t('folder.status.unavailable')
                    : t('picker.noAccess')
                  : undefined
              }
            />
          )
        })}
      </div>
    </Section>
  )
}

const isNetwork = (fs?: string) => !!fs && /^(nfs|cifs|smb|afp|fuse\.sshfs)/i.test(fs)

/** One library: folders, what to identify, scans and language (board AP06, F007 R20, R21, R22). */
export function LibraryPage() {
  const { t, i18n } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { serverName } = useServer()
  const { data, refresh, toast } = useAdmin()
  const locale = i18n.resolvedLanguage ?? 'en'
  const lib = data?.libraries.find((l) => l.id === id)
  const [picking, setPicking] = useState(false)
  const [expanded, setExpanded] = useState<string>()
  const [deleting, setDeleting] = useState(false)
  const [busy, setBusy] = useState<string>()
  const [language, setLanguage] = useState<{ lang: string; region: string }>()

  if (!lib) return <Section>{data && <EmptyState title={t('libraries.empty')} />}</Section>

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
    act('remove', () =>
      patch({ folders: lib.folders.filter((x) => x.id !== f.id).map((x) => x.path) }),
    )
  const meta = (f: Folder) =>
    [
      t('folder.status.ok'),
      f.watched ? t('folder.watch.liveShort') : t('folder.watch.periodicShort', { minutes: 15 }),
      `${isNetwork(f.fsType) ? t('folder.fs.network') : t('folder.fs.local')}${f.fsType ? ` (${f.fsType})` : ''}`,
    ].join(' · ')
  const typeLabel = lib.type === 'movies' ? t('library.type.movies') : t('library.type.shows')

  return (
    <Section>
      <div>
        <p className="adm__kicker">{t('libraries.kicker', { type: typeLabel })}</p>
        <h2 className="adm__h2">{lib.name}</h2>
        <p className="adm__lead">
          {counts(t, lib)} · {scanLine(t, locale, lib)}
        </p>
      </div>

      <section className="adm__section" style={{ gap: 0 }}>
        <h3 className="adm__h3" style={{ marginBottom: 8 }}>
          {t('library.folders.label')}
        </h3>
        <ul className="adm__rows">
          {lib.folders.map((f) =>
            f.status === 'unavailable' ? (
              <li key={f.id} className="adm__row adm__row--alert">
                <Mark tone="error" />
                <div className="adm__row-body adm__row-body--spaced">
                  <div>
                    <p className="adm__path">{f.path}</p>
                    <p className="adm__word pl-tone--error">{t('folder.status.unavailable')}</p>
                  </div>
                  <p className="adm__text">{t('folder.unavailable.body', { path: f.path })}</p>
                  <p className="adm__text">{t('folder.unavailable.check')}</p>
                  <CommandBlock command={mountCommand(f.path)} />
                  <div>
                    <Button
                      loading={busy === 'check'}
                      onClick={() => void act('check', () => scan(false))}
                    >
                      {t('picker.checkAgain')}
                    </Button>
                  </div>
                </div>
              </li>
            ) : f.status === 'unreadable' ? (
              <FolderRow
                key={f.id}
                name={f.path}
                path={f.path}
                mono
                status="noAccess"
                checkVariant="primary"
                expanded={expanded === f.id}
                onToggle={() => setExpanded(expanded === f.id ? undefined : f.id)}
                onCheck={() => void act('check', () => scan(false))}
              />
            ) : (
              <li key={f.id} className="adm__row">
                <Mark tone="success" />
                <span className="adm__row-body">
                  <p className="adm__path">{f.path}</p>
                  <p className="adm__help">{meta(f)}</p>
                </span>
                {lib.folders.length > 1 && (
                  <Button variant="ghost" size="sm" onClick={() => void remove(f)}>
                    {t('library.folders.removeShort')}
                  </Button>
                )}
              </li>
            ),
          )}
          {lib.folders.length < 16 && (
            <li className="adm__row" style={{ paddingTop: 12 }}>
              <Button variant="secondary" onClick={() => setPicking(true)}>
                {t('library.folders.add')}
              </Button>
            </li>
          )}
        </ul>
      </section>

      <section className="adm__section" style={{ gap: 0 }}>
        <h3 className="adm__h3" style={{ marginBottom: 8 }}>
          {t('libraries.identifyReview')}
        </h3>
        <div className="adm__rows">
          <Link
            to={`/admin/libraries/${lib.id}/unrecognized`}
            className="adm__row adm__row--center"
          >
            <span className="adm__row-body">
              <span className="adm__row-title">
                {t('review.unrecognized.title', { count: lib.counts.unrecognized })}
              </span>
              <span className="adm__help">{t('libraries.unrecognized.help')}</span>
            </span>
            <span className="adm__chev">›</span>
          </Link>
          <Link to={`/admin/libraries/${lib.id}/review`} className="adm__row adm__row--center">
            <span className="adm__row-body">
              <span className="adm__row-title">{t('review.needs.short')}</span>
              <span className="adm__help">{t('libraries.review.help')}</span>
            </span>
            <span className="adm__chev">›</span>
          </Link>
        </div>
      </section>

      <section className="adm__section">
        <h3 className="adm__h3">{t('libraries.scanning')}</h3>
        <div className="adm__actions">
          <Button
            variant="secondary"
            loading={busy === 'scan'}
            onClick={() => void act('scan', () => scan(false))}
          >
            {t('libraries.scanNow')}
          </Button>
          <Button
            variant="secondary"
            loading={busy === 'full'}
            onClick={() => void act('full', () => scan(true))}
          >
            {t('libraries.fullRescan.label')}
          </Button>
        </div>
        <p className="adm__help">{t('libraries.fullRescan.help', { name: lib.name })}</p>
        {language ? (
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
            <p className="adm__help">{t('libraries.language.help')}</p>
            <div className="adm__actions">
              <Button
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
        ) : (
          <p className="adm__text">
            {t('library.metadata.summary', {
              language: languageName(locale, lib.metadataLanguage),
              region: regionName(locale, lib.metadataRegion),
            })}{' '}
            ·{' '}
            <button
              type="button"
              className="ob__link"
              onClick={() =>
                setLanguage({ lang: lib.metadataLanguage, region: lib.metadataRegion })
              }
            >
              {t('common.change')}
            </button>
          </p>
        )}
        <div>
          <Button variant="ghost" className="adm__danger" onClick={() => setDeleting(true)}>
            {t('libraries.delete.label')}
          </Button>
        </div>
      </section>

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
