import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, waitFor, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Home from '../pages/index'
import { resetMock } from '../lib/mock'

// Mock Wails-only modules â€” never actually called since isWails() is false
// in jsdom (window.go is absent).
vi.mock('../wailsjs/wailsjs/go/main/App', () => ({
  StartBridge: vi.fn(),
  StopBridge: vi.fn(),
  GetStatus: vi.fn(),
  GetPeers: vi.fn(),
  GetFirewallState: vi.fn(),
  GetAllowedPorts: vi.fn(),
  AddPort: vi.fn(),
  RemovePort: vi.fn(),
  ToggleFirewall: vi.fn(),
  JoinLobby: vi.fn(),
  OpenFriendsOverlay: vi.fn(),
  RespondToJoin: vi.fn(),
}))

vi.mock('../wailsjs/wailsjs/runtime', () => ({
  EventsOn: vi.fn(() => () => {}),
}))

vi.mock('next/head', () => ({
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

// Flush pending promise callbacks and React state updates.
// Works without fake timers â€” uses a real 0ms timeout to yield to the microtask queue.
const flushPromises = () => act(async () => {
  await new Promise<void>(resolve => setTimeout(resolve, 0))
})

beforeEach(() => {
  resetMock()
})

afterEach(() => {
  cleanup()
})

async function renderHome() {
  render(<Home />)
  // Let useEffect fire and the initial refresh() chain resolve.
  await flushPromises()
}

describe('Dashboard - initial state', () => {
  it('shows Start Bridge button on mount', async () => {
    await renderHome()
    expect(screen.getByRole('button', { name: /start bridge/i })).toBeInTheDocument()
  })
})

describe('Dashboard - start â†’ running transition', () => {
  it('shows local IP and Stop Bridge button after starting', async () => {
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })
    expect(screen.getByText('10.8.0.2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /stop bridge/i })).toBeInTheDocument()
  })
})

describe('Dashboard - stop transition', () => {
  it('returns to Start Bridge button after stopping', async () => {
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /stop bridge/i }))
      await flushPromises()
    })
    expect(screen.getByRole('button', { name: /start bridge/i })).toBeInTheDocument()
  })
})

describe('Dashboard - manual join flow', () => {
  it('sets connection to pending immediately', async () => {
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    const input = screen.getByPlaceholderText(/steam id/i)
    await userEvent.type(input, '99999')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /join lobby/i }))
      await flushPromises()
    })

    expect(screen.getByText('pending')).toBeInTheDocument()
    expect(screen.getByText('99999')).toBeInTheDocument()
  })

  // The browser-mode join simulation resolves to 'connected' after 1500ms.
  // This test uses real timers so it takes ~1.5s but avoids fake-timer/waitFor interaction.
  it('transitions to connected after the 1500ms simulation delay', async () => {
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })
    const input = screen.getByPlaceholderText(/steam id/i)
    await userEvent.type(input, '11111')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /join lobby/i }))
      await flushPromises()
    })
    expect(screen.getByText('pending')).toBeInTheDocument()

    await waitFor(
      () => expect(screen.getByText('connected')).toBeInTheDocument(),
      { timeout: 3000 }
    )
  }, 5000)
})

