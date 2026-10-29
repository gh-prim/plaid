import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { renderUi } from '../test/renderUi'
import { CommandBlock } from './CommandBlock'
import { ProgressBar, Toast } from './feedback'
import { FolderPicker, type FolderEntry } from './FolderPicker'
import { FolderRow } from './FolderRow'
import { AmbientBackdrop, backdropEvery, PosterSlot } from './scan'
import { TextField } from './TextField'

describe('FolderRow', () => {
  it('opens a readable folder', async () => {
    const onOpen = vi.fn()
    await renderUi(<FolderRow name="movies" path="/srv/media/movies" onOpen={onOpen} />)
    await userEvent.click(screen.getByRole('button', { name: /movies/ }))
    expect(onOpen).toHaveBeenCalledOnce()
  })

  it('explains an unreadable folder with a quoted fix and Check again', async () => {
    const onCheck = vi.fn()
    await renderUi(
      <FolderRow
        name="Bob's films"
        path="/srv/Bob's films"
        status="noAccess"
        expanded
        onCheck={onCheck}
      />,
    )
    expect(screen.getByRole('button', { name: /Bob's films.*No access/ })).toHaveAttribute(
      'aria-expanded',
      'true',
    )
    expect(screen.getByText('PLAID can’t read this folder.')).toBeInTheDocument()
    expect(
      screen.getByText(`sudo setfacl -R -m u:plaid:rX '/srv/Bob'\\''s films'`),
    ).toBeInTheDocument()
    expect(screen.getByText(/On Docker/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Check again' }))
    expect(onCheck).toHaveBeenCalledOnce()
  })

  it('speaks French', async () => {
    await renderUi(<FolderRow name="films" path="/srv/films" status="noAccess" expanded />, 'fr-FR')
    expect(screen.getByText('PLAID ne peut pas lire ce dossier.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Vérifier à nouveau' })).toBeInTheDocument()
  })
})

describe('FolderPicker', () => {
  const entries: FolderEntry[] = [
    { name: 'Kids', path: '/srv/media/Kids', status: 'ok' },
    { name: 'movies', path: '/srv/media/movies', status: 'noAccess' },
  ]
  const handlers = {
    onOpen: vi.fn(),
    onToggle: vi.fn(),
    onCheck: vi.fn(),
    onUse: vi.fn(),
    onClose: vi.fn(),
  }

  it('lists a folder with its path and the parent button', async () => {
    const onParent = vi.fn()
    await renderUi(
      <FolderPicker
        serverName="nas"
        path="/srv/media"
        entries={entries}
        onParent={onParent}
        {...handlers}
      />,
    )
    expect(screen.getByRole('region', { name: 'Folders on nas' })).toBeInTheDocument()
    expect(screen.getByText('media')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Parent folder' }))
    expect(onParent).toHaveBeenCalledOnce()
    expect(screen.getByRole('button', { name: 'Use this folder' })).toBeEnabled()
  })

  it('at the roots: no parent, the roots note, nothing to use yet', async () => {
    await renderUi(
      <FolderPicker serverName="nas" path="" atRoots entries={entries} {...handlers} />,
    )
    expect(screen.getByRole('button', { name: 'Parent folder' })).toBeDisabled()
    expect(screen.getByText(/PLAID_BROWSE_ROOTS/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Use this folder' })).toBeDisabled()
  })

  it('at / shows a single slash, no parent, and lets you type a path on desktop too', async () => {
    const onTypePath = vi.fn()
    await renderUi(
      <FolderPicker
        serverName="nas"
        path="/"
        layout="panel"
        entries={entries}
        onTypePath={onTypePath}
        {...handlers}
      />,
    )
    expect(document.querySelector('.pl-picker__crumbs')).toHaveTextContent(/^\/$/)
    expect(screen.getByRole('button', { name: 'Parent folder' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Type a path' }))
    expect(onTypePath).toHaveBeenCalledOnce()
  })

  it('says an empty folder can still be the right one', async () => {
    await renderUi(<FolderPicker serverName="nas" path="/srv/empty" entries={[]} {...handlers} />)
    expect(screen.getByText('No sub-folders.')).toBeInTheDocument()
    expect(screen.getByText(/this can still be the right folder/)).toBeInTheDocument()
  })

  it('filters and says when nothing matches', async () => {
    function Filtered() {
      const [q, setQ] = useState('')
      const shown = entries.filter((e) => e.name.toLowerCase().includes(q.toLowerCase()))
      return (
        <FolderPicker
          serverName="nas"
          path="/srv/media"
          entries={shown}
          filter={{ value: q, total: entries.length, onChange: setQ }}
          {...handlers}
        />
      )
    }
    await renderUi(<Filtered />)
    await userEvent.type(screen.getByRole('textbox', { name: 'Filter 2 folders' }), 'zzz')
    expect(screen.getByText('No folder matches “zzz”.')).toBeInTheDocument()
    expect(screen.getByText('0 of 2')).toBeInTheDocument()
  })
})

describe('CommandBlock', () => {
  it('copies the exact command and says so for 2 s', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText }, languages: ['en-US'] })
    await renderUi(<CommandBlock command="docker logs plaid 2>&1 | grep first_run_setup_code" />)
    await userEvent.click(screen.getByRole('button', { name: 'Copy' }))
    expect(writeText).toHaveBeenCalledWith('docker logs plaid 2>&1 | grep first_run_setup_code')
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
    await act(() => vi.advanceTimersByTimeAsync(2100))
    expect(screen.getByRole('button', { name: 'Copy' })).toBeInTheDocument()
    vi.useRealTimers()
  })
})

describe('TextField', () => {
  it('shows and hides a password', async () => {
    function Pw() {
      const [v, setV] = useState('')
      return <TextField kind="password" label="Password" value={v} onValue={setV} />
    }
    await renderUi(<Pw />)
    const input = screen.getByLabelText('Password')
    expect(input).toHaveAttribute('type', 'password')
    await userEvent.click(screen.getByRole('button', { name: 'Show password' }))
    expect(input).toHaveAttribute('type', 'text')
  })

  it('links the error to the field', async () => {
    await renderUi(<TextField label="Username" value="ab" onValue={() => {}} error="Too short." />)
    expect(screen.getByLabelText('Username')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText('Too short.')).toBeInTheDocument()
  })
})

describe('feedback', () => {
  it('a scan progress bar exposes its value', async () => {
    await renderUi(
      <ProgressBar variant="scan" value={63.8} label="Files analyzed" valueLabel="806 of 1,263" />,
    )
    const bar = screen.getByRole('progressbar', { name: 'Files analyzed' })
    expect(bar).toHaveAttribute('aria-valuenow', '64')
    expect(bar).toHaveAttribute('aria-valuetext', '806 of 1,263')
  })

  it('an error toast is an alert', async () => {
    await renderUi(<Toast variant="error">Can’t reach nas.</Toast>)
    expect(screen.getByRole('alert')).toHaveTextContent('Can’t reach nas.')
  })
})

describe('scan pieces', () => {
  it('a slot is empty, missing or arrived', async () => {
    const { container } = await renderUi(
      <>
        <PosterSlot />
        <PosterSlot title="Brittany holidays (VHS rip)" />
        <PosterSlot src="/api/v1/images/1?w=342" title="Dune" />
      </>,
    )
    const slots = container.querySelectorAll('.pl-slot')
    expect(slots[0]).toHaveAttribute('aria-hidden', 'true')
    expect(slots[1]).toHaveTextContent('Brittany holidays (VHS rip)')
    expect(screen.getByRole('img', { name: 'Dune' })).toHaveAttribute(
      'src',
      '/api/v1/images/1?w=342',
    )
  })

  it('changes the backdrop at most every 8 s', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const { container, rerender } = await renderUi(<AmbientBackdrop src="/a.jpg" />)
    const srcs = () => [...container.querySelectorAll('img')].map((i) => i.getAttribute('src'))
    rerender(<AmbientBackdrop src="/b.jpg" />)
    await act(() => vi.advanceTimersByTimeAsync(10))
    expect(srcs()).toEqual(['/a.jpg', '/b.jpg'])
    rerender(<AmbientBackdrop src="/c.jpg" />)
    await act(() => vi.advanceTimersByTimeAsync(backdropEvery / 2))
    expect(srcs()).toEqual(['/a.jpg', '/b.jpg'])
    await act(() => vi.advanceTimersByTimeAsync(backdropEvery))
    expect(srcs()).toEqual(['/b.jpg', '/c.jpg'])
    vi.useRealTimers()
  })
})
