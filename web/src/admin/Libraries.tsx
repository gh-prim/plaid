import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { formatNumber, formatRelative, languageName, regionName } from '../lib/format'
import { mountCommand, setfaclCommand } from '../lib/shellQuote'
import { PickerSheet } from '../onboarding/PickerSheet'
import { useServer } from '../onboarding/Page'
import { Button, CommandBlock, Dialog, EmptyState, TextField } from '../ui'
import { useAdmin } from './AdminLayout'
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

/** Every library, with its count, scan and folder trouble (board TA02, F007 R20). */
export function Libraries() {
  const { t, i18n } = useTranslation()
  const { data, refresh, toast } = useAdmin()
  const [scanning, setScanning] = useState(false)
  const locale = i18n.resolvedLanguage ?? 'en'
  const n = (v: number) => formatNumber(i18n.language, v)
  const libs = data?.libraries ?? []
  const titles = libs.reduce(
    (a, l) => a + (l.type === 'movies' ? l.counts.movies : l.counts.shows),
    0,
  )
  const files = libs.reduce((a, l) => a + l.counts.files, 0)
  const gone = libs.reduce((a, l) => a + troubled(l).length, 0)

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

  return (
    <Section
      title={t('libraries.title')}
      glyph="libraries"
      paneTitle={data ? t('admin.libraries.count', { count: libs.length }) : undefined}
      paneSub={data ? t('admin.libraries.facts', { titles, files }) : undefined}
      status={
        gone
          ? { text: t('admin.libraries.unavailable', { count: gone }), tone: 'warning' }
          : undefined
      }
    >
      {!data && <Loading />}
      {data && !libs.length && <EmptyState title={t('libraries.empty')} />}
      {libs.length > 0 && (
        <Group>
          {libs.map((l) => {
            const trouble = troubled(l)
            return (
              <Row
                key={l.id}
                to={`/admin/libraries/${l.id}`}
                label={l.name}
                value={n(l.type === 'movies' ? l.counts.movies : l.counts.shows)}
                sub={
                  trouble.length
                    ? t('admin.libraries.folderTrouble', {
                        count: trouble.length,
                        total: l.folders.length,
                      })
                    : [counts(t, l), scanLine(t, locale, l)].filter(Boolean).join(' · ')
                }
                subTone={trouble.length ? 'warning' : undefined}
              />
            )
          })}
        </Group>
      )}
      {data && (
        <Group>
          <Row to="/setup/libraries" label={t('admin.libraries.add')} />
          {libs.length > 0 && (
            <Row
              label={t('admin.libraries.scanAll')}
              busy={scanning}
              onClick={() => void scanAll()}
            />
          )}
        </Group>
      )}
    </Section>
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
      paneTitle={lib.name}
      paneSub={[counts(t, lib), scanLine(t, locale, lib)].filter(Boolean).join(' · ')}
      status={
        gone
          ? { text: t('admin.library.unavailable', { count: gone }), tone: 'warning' }
          : undefined
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
