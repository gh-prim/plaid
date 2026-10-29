// Development only (F007 PR A evidence): every ported component in its
// states, in both themes. Not routed in production builds; removed once
// the onboarding screens exist (PR B).
import { useEffect, useState } from 'react'
import { setfaclCommand } from '../lib/shellQuote'
import {
  AmbientBackdrop,
  Button,
  ChoiceRow,
  Chip,
  CommandBlock,
  EmptyState,
  FolderPicker,
  PosterSlot,
  ProgressBar,
  SegmentedControl,
  ServerFact,
  TextField,
  Toast,
} from '../ui'

/* eslint-disable i18next/no-literal-string -- sample data, never shipped */
export function Gallery() {
  const [theme, setTheme] = useState<'dark' | 'light'>('dark')
  const [lang, setLang] = useState('en')
  const [type, setType] = useState<'movies' | 'shows'>('movies')
  const [pw, setPw] = useState('')
  const [code, setCode] = useState('KQ7M')
  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])
  return (
    <main
      className="pl"
      style={{ padding: 32, display: 'grid', gap: 32, maxWidth: 1200, margin: '0 auto' }}
    >
      <div style={{ display: 'flex', gap: 12 }}>
        <Button variant="secondary" onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>
          Theme: {theme}
        </Button>
      </div>
      <section style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
        <Button>Add library</Button>
        <Button variant="play">Start watching</Button>
        <Button variant="secondary">Check again</Button>
        <Button variant="ghost">Cancel</Button>
        <Button variant="icon" icon="chevronLeft" label="Parent folder" />
        <Button loading>Creating account</Button>
        <Chip selected>/srv/media/movies</Chip>
        <Chip>/mnt/nas/Films</Chip>
      </section>
      <section style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 24 }}>
        <TextField label="Username" value="sa" onValue={() => {}} error="3 to 32 characters." />
        <TextField
          kind="password"
          label="Password"
          value={pw}
          onValue={setPw}
          help="8 characters or more."
        />
        <TextField kind="code" label="Setup code" value={code} onValue={setCode} />
      </section>
      <section style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
        <div style={{ display: 'grid', gap: 8 }}>
          <ChoiceRow
            name="lang"
            label="English"
            lang="en"
            checked={lang === 'en'}
            onChange={() => setLang('en')}
          />
          <ChoiceRow
            name="lang"
            label="Français"
            lang="fr"
            checked={lang === 'fr'}
            onChange={() => setLang('fr')}
          />
          <SegmentedControl
            label="Type"
            value={type}
            onChange={setType}
            options={[
              { value: 'movies', label: 'Movies' },
              { value: 'shows', label: 'Shows' },
            ]}
          />
        </div>
        <div style={{ display: 'grid', gap: 12 }}>
          <ProgressBar
            variant="scan"
            value={63.8}
            label="Reading files with ffprobe"
            valueLabel="806 of 1,263 analyzed"
          />
          <CommandBlock command={setfaclCommand('/media/pi/USB-4TB/Séries')} />
          <ServerFact tone="hardware">
            Video conversion uses the NVIDIA GPU (NVENC): up to 4 at once when a device can’t play a
            file as it is.
          </ServerFact>
          <ServerFact
            tone="warning"
            command="echo fs.inotify.max_user_watches=524288 | sudo tee /etc/sysctl.d/90-plaid.conf && sudo sysctl --system"
          >
            Linux’s file watch limit is reached, so /srv/media is checked every 15 minutes instead
            of live.
          </ServerFact>
        </div>
      </section>
      <section
        style={{ display: 'grid', gridTemplateColumns: '375px 1fr', gap: 24, alignItems: 'start' }}
      >
        <div style={{ height: 640 }}>
          <FolderPicker
            serverName="mediabox"
            path="/srv/media"
            entries={[
              { name: 'Concerts 2019', path: '/srv/media/Concerts 2019', status: 'ok' },
              { name: 'Kids', path: '/srv/media/Kids', status: 'ok' },
              { name: 'movies', path: '/srv/media/movies', status: 'noAccess', expanded: true },
              { name: 'Séries TV', path: '/srv/media/Séries TV', status: 'ok' },
            ]}
            onOpen={() => {}}
            onToggle={() => {}}
            onCheck={() => {}}
            onParent={() => {}}
            onUse={() => {}}
            onClose={() => {}}
            onTypePath={() => {}}
          />
        </div>
        <div style={{ position: 'relative', minHeight: 420, overflow: 'hidden', borderRadius: 8 }}>
          <AmbientBackdrop
            src="https://image.tmdb.org/t/p/w1280/eZ239CUp1d6OryZEBPnO2n87gMG.jpg"
            theme={theme}
          />
          <div
            style={{
              position: 'relative',
              display: 'grid',
              gridTemplateColumns: 'repeat(6, 1fr)',
              gap: 16,
              padding: 24,
            }}
          >
            <PosterSlot
              src="https://image.tmdb.org/t/p/w342/6izwz7rsy95ARzTR3poZ8H6c5pp.jpg"
              title="Dune: Part Two"
            />
            <PosterSlot
              src="https://image.tmdb.org/t/p/w342/8Gxv8gSFCU0XGDykEGv7zR1n2ua.jpg"
              title="Oppenheimer"
            />
            <PosterSlot title="Brittany holidays (VHS rip)" />
            <PosterSlot />
            <PosterSlot />
            <PosterSlot />
          </div>
        </div>
      </section>
      <section style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
        <Toast variant="success">Stopped Dune: Part Two for Sam.</Toast>
        <Toast variant="error" onClose={() => {}}>
          Can’t reach mediabox.
        </Toast>
        <EmptyState
          icon="folder"
          title="No libraries yet."
          actions={<Button>Add a library</Button>}
        >
          Add a folder and PLAID will find your movies and shows.
        </EmptyState>
      </section>
    </main>
  )
}
