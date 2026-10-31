import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { failure, fieldCodes } from '../auth/errors'
import { useConnection } from '../hooks/useConnection'
import { languageName, regionName } from '../lib/format'
import { Button, Icon, SegmentedControl, TextField } from '../ui'
import { FieldGroup, InsetField } from './Fields'
import { Card, LoadingCard, Page, useServer, useStepLabel } from './Page'
import { PickerSheet } from './PickerSheet'
import { laterKey, setFlag } from './step'
import { OfflineScreen } from './StatusScreen'
import { slide, useStepNav } from './transition'

type Library = components['schemas']['Library']
type LibraryType = components['schemas']['LibraryType']

export const maxFolders = 16

/** Languages offered for metadata; TMDB has many more, these cover most homes. */
const metadataLanguages = ['en', 'fr', 'de', 'es', 'it', 'pt', 'nl', 'ja', 'ko', 'zh']

function browserRegion(): string {
  try {
    return new Intl.Locale(navigator.language).maximize().region ?? 'US'
  } catch {
    return 'US'
  }
}

function overlaps(a: string, b: string): boolean {
  const slash = (p: string) => (p.endsWith('/') ? p : p + '/')
  return slash(a).startsWith(slash(b)) || slash(b).startsWith(slash(a))
}

/**
 * Step 2 of 4 (boards TO04, TO05): one or more libraries, folders picked
 * on the server's disks (F007 R7).
 */
export function Libraries() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const go = useStepNav()
  const stepLabel = useStepLabel()
  const { serverName } = useServer()
  const { connection, lost, retryNow } = useConnection()
  const ui = i18n.resolvedLanguage ?? 'en'
  const [added, setAdded] = useState<Library[]>([])
  const [type, setType] = useState<LibraryType>('movies')
  const [name, setName] = useState('')
  const [named, setNamed] = useState(false)
  const [folders, setFolders] = useState<string[]>([])
  const [language, setLanguage] = useState(metadataLanguages.includes(ui) ? ui : 'en')
  const [region, setRegion] = useState(browserRegion)
  const [changing, setChanging] = useState(false)
  const [picking, setPicking] = useState(false)
  const [folderErrors, setFolderErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const [formOpen, setFormOpen] = useState(true)
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(async () => {
    try {
      const { data } = await api.GET('/libraries')
      if (data) setAdded(data)
    } catch {
      lost()
    }
  }, [lost])

  useEffect(() => {
    api
      .GET('/libraries')
      .then(({ data }) => {
        setLoaded(true)
        if (!data) return
        setAdded(data)
        if (data.length) setFormOpen(false)
      })
      .catch(lost)
  }, [lost])

  const shownName = named
    ? name
    : t(type === 'movies' ? 'library.type.movies' : 'library.type.shows')

  async function add() {
    setBusy(true)
    setFolderErrors({})
    try {
      const { data, error, response } = await api.POST('/libraries', {
        body: { name: shownName.trim(), type, folders },
      })
      if (error) {
        const f = failure(error, response)
        const errs: Record<string, string> = {}
        for (const [field, code] of Object.entries(fieldCodes(f))) {
          const i = Number(field.split('/')[1])
          const path = folders[i]
          if (path) errs[path] = code
        }
        if (f.error?.code === 'folder_overlap') {
          for (const p of folders) {
            const other = added.find((l) => l.folders.some((lf) => overlaps(lf.path, p)))
            if (other) errs[p] = `overlap:${other.name}`
          }
        }
        setFolderErrors(errs)
        return
      }
      if (data && (language !== data.metadataLanguage || region !== data.metadataRegion)) {
        await api.PATCH('/libraries/{libraryId}', {
          params: { path: { libraryId: data.id } },
          body: { metadataLanguage: language, metadataRegion: region },
        })
      }
      setFolders([])
      setNamed(false)
      await load()
      slide(() => setFormOpen(false))
    } catch {
      lost()
    } finally {
      setBusy(false)
    }
  }

  function folderError(path: string): string | undefined {
    const code = folderErrors[path]
    if (!code) return undefined
    if (code.startsWith('overlap:')) return t('errors.folder_overlap', { library: code.slice(8) })
    if (code === 'folder_unreadable') return t('errors.folder_unreadable')
    return t('errors.folder_not_found')
  }

  const shortState = (l: Library) => {
    if (l.scan.queued || l.scan.state === 'queued') return t('scan.state.queued')
    if (l.scan.state === 'idle') return t('scan.state.idle')
    return t('scan.state.short.scanning')
  }

  const later = () => {
    setFlag(laterKey)
    void navigate('/')
  }

  if (!loaded) {
    return (
      <Page step={2}>
        <LoadingCard />
        <OfflineScreen connection={connection} onRetry={() => void retryNow()} />
      </Page>
    )
  }

  return (
    <Page step={2}>
      <Card
        id={formOpen ? 'library-form' : 'libraries'}
        wide
        kicker={stepLabel(2)}
        title={t('onboarding.libraries.title')}
        body={t('onboarding.libraries.body', { serverName })}
      >
        {added.length > 0 && (
          <ul className="ob-fields__box ob__libs" aria-label={t('onboarding.libraries.added')}>
            {added.map((l) => (
              <li key={l.id}>
                <span className="ob__tick" aria-hidden="true">
                  <Icon name="check" size={14} />
                </span>
                <span className="ob__lib">
                  <b>{l.name}</b>
                  <code>{l.folders.map((f) => f.path).join(' · ')}</code>
                </span>
                <span className="ob__lib-state">{shortState(l)}</span>
              </li>
            ))}
          </ul>
        )}

        {formOpen ? (
          <div className="ob__form">
            <div className="ob__type">
              <SegmentedControl
                label={t('library.type.label')}
                value={type}
                options={(['movies', 'shows'] as const).map((v) => ({
                  value: v,
                  label: t(v === 'movies' ? 'library.type.movies' : 'library.type.shows'),
                }))}
                onChange={setType}
              />
            </div>
            <FieldGroup>
              <InsetField
                label={t('library.name')}
                value={shownName}
                onValue={(v) => {
                  setNamed(true)
                  setName(v)
                }}
                maxLength={64}
              />
            </FieldGroup>
            <div className="ob__folders">
              <div className="ob__folders-head">
                <span>{t('library.folders.label')}</span>
                <span>
                  {t('library.folders.count', { count: folders.length, max: maxFolders })}
                </span>
              </div>
              <div className="ob-fields__box">
                {folders.map((p) => (
                  <div key={p} className="ob__folder-row">
                    <div className="ob__folder">
                      <Icon name={folderError(p) ? 'error' : 'folder'} size={20} />
                      <code>{p}</code>
                      <Button
                        variant="icon"
                        icon="x"
                        size="sm"
                        label={t('library.folders.remove', { path: p })}
                        onClick={() => setFolders((f) => f.filter((x) => x !== p))}
                      />
                    </div>
                    {folderError(p) && <p className="ob__folder-error">{folderError(p)}</p>}
                  </div>
                ))}
                {folders.length < maxFolders && (
                  <button
                    type="button"
                    className="ob__addrow pl-focusable"
                    aria-expanded={picking}
                    onClick={() => setPicking(true)}
                  >
                    <Icon name="plus" size={20} />
                    {t('library.folders.add')}
                  </button>
                )}
              </div>
            </div>
            {changing ? (
              <div className="ob__metadata">
                <label className="pl pl-field">
                  <span className="pl-field__label">{t('library.metadata.language')}</span>
                  <select
                    className="pl-field__box"
                    value={language}
                    onChange={(e) => setLanguage(e.target.value)}
                  >
                    {metadataLanguages.map((l) => (
                      <option key={l} value={l}>
                        {languageName(ui, l)}
                      </option>
                    ))}
                  </select>
                </label>
                <TextField
                  label={t('library.metadata.region.label')}
                  help={t('library.metadata.region.help')}
                  value={region}
                  onValue={(v) => setRegion(v.toUpperCase().slice(0, 2))}
                  maxLength={2}
                  autoCapitalize="characters"
                />
              </div>
            ) : (
              <p className="ob__foot ob__foot--left">
                {t('library.metadata.summary', {
                  language: languageName(ui, language),
                  region: regionName(ui, region),
                })}{' '}
                ·{' '}
                <button type="button" className="ob__link" onClick={() => setChanging(true)}>
                  {t('common.change')}
                </button>
              </p>
            )}
            <Button
              block
              className="ob__cta"
              loading={busy}
              disabled={busy || folders.length === 0 || !shownName.trim()}
              onClick={() => void add()}
            >
              {t('library.add')}
            </Button>
            {added.length === 0 ? (
              <button type="button" className="ob__link" onClick={later}>
                {t('onboarding.libraries.later')}
              </button>
            ) : (
              <button
                type="button"
                className="ob__link"
                onClick={() => slide(() => setFormOpen(false), 'back')}
              >
                {t('common.cancel')}
              </button>
            )}
          </div>
        ) : (
          <div className="ob__form">
            <button
              type="button"
              className="ob-fields__box ob__addrow pl-focusable"
              onClick={() => slide(() => setFormOpen(true))}
            >
              <Icon name="plus" size={20} />
              {t('onboarding.libraries.another')}
            </button>
            <Button block className="ob__cta" onClick={() => go('/setup/scan')}>
              {t('onboarding.libraries.next')}
            </Button>
            <p className="ob__foot">{t('onboarding.libraries.foot')}</p>
          </div>
        )}
      </Card>

      {picking && (
        <PickerSheet
          serverName={serverName}
          onClose={() => setPicking(false)}
          onLost={lost}
          onUse={(p) => {
            setFolders((f) => (f.includes(p) ? f : [...f, p]))
            setPicking(false)
          }}
        />
      )}
      <OfflineScreen connection={connection} onRetry={() => void retryNow()} />
    </Page>
  )
}
