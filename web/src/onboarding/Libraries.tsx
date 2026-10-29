import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { failure, fieldCodes } from '../auth/errors'
import { useConnection } from '../hooks/useConnection'
import { languageName, regionName } from '../lib/format'
import { Button, ChoiceRow, Icon, TextField } from '../ui'
import { Page, StepHead, useServer } from './Page'
import { PickerSheet } from './PickerSheet'
import { laterKey, setFlag } from './step'
import { OfflineScreen } from './StatusScreen'

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

/** Step 3: one or more libraries, folders picked on the server's disks (F007 R7). */
export function Libraries() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
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

  const load = useCallback(async () => {
    try {
      const { data } = await api.GET('/libraries')
      if (data) {
        setAdded(data)
        if (data.length) setFormOpen(false)
      }
    } catch {
      lost()
    }
  }, [lost])

  useEffect(() => {
    api
      .GET('/libraries')
      .then(({ data }) => {
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
      setFormOpen(false)
      await load()
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

  return (
    <Page>
      <StepHead
        step={3}
        title={t('onboarding.libraries.title')}
        body={t('onboarding.libraries.body', { serverName })}
      />

      {added.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <ul className="ob__added" aria-label={t('onboarding.libraries.added')}>
            {added.map((l) => (
              <li key={l.id}>
                <div>
                  <p className="name">{l.name}</p>
                  <code>{l.folders.map((f) => f.path).join(' · ')}</code>
                </div>
                <span className="state">{shortState(l)}</span>
              </li>
            ))}
          </ul>
          {!formOpen && (
            <Button
              variant="secondary"
              style={{ alignSelf: 'flex-start' }}
              onClick={() => setFormOpen(true)}
            >
              {t('onboarding.libraries.another')}
            </Button>
          )}
        </div>
      )}

      {formOpen && (
        <div className="ob__form">
          <fieldset className="ob__fieldset">
            <legend className="ob__legend">{t('library.type.label')}</legend>
            <div className="ob__choices">
              {(['movies', 'shows'] as const).map((v) => (
                <ChoiceRow
                  key={v}
                  name="type"
                  label={t(v === 'movies' ? 'library.type.movies' : 'library.type.shows')}
                  checked={type === v}
                  onChange={() => setType(v)}
                />
              ))}
            </div>
          </fieldset>
          <TextField
            label={t('library.name')}
            value={shownName}
            onValue={(v) => {
              setNamed(true)
              setName(v)
            }}
            maxLength={64}
          />
          <div className="ob__folders">
            <div className="ob__folders-head">
              <span>{t('library.folders.label')}</span>
              <span>{t('library.folders.count', { count: folders.length, max: maxFolders })}</span>
            </div>
            {folders.map((p) => (
              <div key={p}>
                <div className="ob__folder">
                  <Icon name={folderError(p) ? 'error' : 'check'} size={18} />
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
                className="ob__add-folder pl-focusable"
                aria-expanded={picking}
                onClick={() => setPicking(true)}
              >
                {t('library.folders.add')}
              </button>
            )}
          </div>
          {changing ? (
            <div style={{ display: 'grid', gap: 12, gridTemplateColumns: '1fr 1fr' }}>
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
            <p className="ob__body" style={{ fontSize: 15, lineHeight: '22px' }}>
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
          <div className="ob__actions">
            <Button
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
              <button type="button" className="ob__link" onClick={() => setFormOpen(false)}>
                {t('common.cancel')}
              </button>
            )}
          </div>
        </div>
      )}

      {added.length > 0 && !formOpen && (
        <div className="ob__actions" style={{ marginTop: 'auto' }}>
          <Button onClick={() => void navigate('/setup/scan')}>
            {t('onboarding.libraries.next')}
          </Button>
        </div>
      )}

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
