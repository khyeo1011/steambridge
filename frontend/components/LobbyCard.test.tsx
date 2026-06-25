import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LobbyCard } from './LobbyCard'

const noop = () => Promise.resolve()

function renderCard(
  props: Partial<{
    running: boolean
    connections: Record<string, 'pending' | 'connected' | 'failed'>
    onJoinLobby: (id: string) => Promise<void>
    onOpenOverlay: () => void
  }> = {}
) {
  const defaults = {
    running: true,
    connections: {},
    onJoinLobby: vi.fn().mockResolvedValue(undefined),
    onOpenOverlay: vi.fn(),
  }
  const merged = { ...defaults, ...props }
  render(<LobbyCard {...merged} />)
  return merged
}

describe('LobbyCard - input validation', () => {
  it('accepts digit-only Steam IDs', async () => {
    const onJoinLobby = vi.fn().mockResolvedValue(undefined)
    renderCard({ onJoinLobby })
    const input = screen.getByPlaceholderText(/steam id/i)
    await userEvent.type(input, '76561198012345678')
    await userEvent.click(screen.getByRole('button', { name: /join lobby/i }))
    expect(onJoinLobby).toHaveBeenCalledWith('76561198012345678')
  })

  it('rejects non-digit input', async () => {
    const onJoinLobby = vi.fn().mockResolvedValue(undefined)
    renderCard({ onJoinLobby })
    const input = screen.getByPlaceholderText(/steam id/i)
    await userEvent.type(input, 'not-a-number')
    await userEvent.click(screen.getByRole('button', { name: /join lobby/i }))
    expect(onJoinLobby).not.toHaveBeenCalled()
  })

  it('rejects empty input', async () => {
    const onJoinLobby = vi.fn().mockResolvedValue(undefined)
    renderCard({ onJoinLobby })
    await userEvent.click(screen.getByRole('button', { name: /join lobby/i }))
    expect(onJoinLobby).not.toHaveBeenCalled()
  })

  it('clears input after successful join', async () => {
    renderCard()
    const input = screen.getByPlaceholderText(/steam id/i) as HTMLInputElement
    await userEvent.type(input, '12345')
    await userEvent.click(screen.getByRole('button', { name: /join lobby/i }))
    expect(input.value).toBe('')
  })

  it('triggers join on Enter key', async () => {
    const onJoinLobby = vi.fn().mockResolvedValue(undefined)
    renderCard({ onJoinLobby })
    const input = screen.getByPlaceholderText(/steam id/i)
    await userEvent.type(input, '99999{Enter}')
    expect(onJoinLobby).toHaveBeenCalledWith('99999')
  })
})

describe('LobbyCard - button state', () => {
  it('disables Join Lobby button when not running', () => {
    renderCard({ running: false })
    expect(screen.getByRole('button', { name: /join lobby/i })).toBeDisabled()
  })

  it('disables Join Lobby button when input is empty', () => {
    renderCard({ running: true })
    expect(screen.getByRole('button', { name: /join lobby/i })).toBeDisabled()
  })

  it('enables Join Lobby button when running and input has text', async () => {
    renderCard({ running: true })
    await userEvent.type(screen.getByPlaceholderText(/steam id/i), '12345')
    expect(screen.getByRole('button', { name: /join lobby/i })).not.toBeDisabled()
  })

  it('disables Steam Friends button when not running', () => {
    renderCard({ running: false })
    expect(screen.getByRole('button', { name: /join via steam friends/i })).toBeDisabled()
  })
})

describe('LobbyCard - connection status badges', () => {
  it('renders pending/connected/failed badges', () => {
    renderCard({
      connections: {
        '111': 'pending',
        '222': 'connected',
        '333': 'failed',
      },
    })
    expect(screen.getByText('pending')).toBeInTheDocument()
    expect(screen.getByText('connected')).toBeInTheDocument()
    expect(screen.getByText('failed')).toBeInTheDocument()
    expect(screen.getByText('111')).toBeInTheDocument()
    expect(screen.getByText('222')).toBeInTheDocument()
    expect(screen.getByText('333')).toBeInTheDocument()
  })

  it('renders nothing when connections is empty', () => {
    renderCard({ connections: {} })
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })
})
