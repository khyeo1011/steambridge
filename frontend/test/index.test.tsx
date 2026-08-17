import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, waitFor, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Home from '../pages/index'
import { resetMock, mockApp } from '../lib/mock'

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
  vi.restoreAllMocks()
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

describe('Dashboard - start/stop error handling', () => {
  it('shows an error and re-enables the button when StartBridge fails', async () => {
    vi.spyOn(mockApp, 'StartBridge').mockRejectedValueOnce(new Error('steam not running'))
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to start bridge: steam not running')
    // busy must reset so the button is usable again
    expect(screen.getByRole('button', { name: /start bridge/i })).not.toBeDisabled()
  })

  it('shows an error and re-enables the button when StopBridge fails', async () => {
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    vi.spyOn(mockApp, 'StopBridge').mockRejectedValueOnce(new Error('bridge stuck'))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /stop bridge/i }))
      await flushPromises()
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to stop bridge: bridge stuck')
    expect(screen.getByRole('button', { name: /stop bridge/i })).not.toBeDisabled()
  })

  it('dismisses the error banner when the dismiss button is clicked', async () => {
    vi.spyOn(mockApp, 'StartBridge').mockRejectedValueOnce(new Error('boom'))
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })
    expect(screen.getByRole('alert')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /dismiss error/i }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('Dashboard - firewall toggle rollback', () => {
  it('rolls back the switch and shows an error when ToggleFirewall fails', async () => {
    vi.spyOn(mockApp, 'ToggleFirewall').mockRejectedValueOnce(new Error('permission denied'))
    await renderHome()

    const toggle = screen.getByRole('checkbox')
    expect(toggle).not.toBeChecked()

    await act(async () => {
      fireEvent.click(toggle)
      await flushPromises()
    })

    expect(toggle).not.toBeChecked()
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to enable firewall: permission denied')
  })

  it('keeps the switch state when ToggleFirewall succeeds', async () => {
    await renderHome()

    const toggle = screen.getByRole('checkbox')
    await act(async () => {
      fireEvent.click(toggle)
      await flushPromises()
    })

    expect(toggle).toBeChecked()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('Dashboard - join lobby error handling', () => {
  it('marks the connection failed and shows an error when JoinLobby rejects', async () => {
    vi.spyOn(mockApp, 'JoinLobby').mockRejectedValueOnce(new Error('lobby full'))
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    const input = screen.getByPlaceholderText(/steam id/i)
    await userEvent.type(input, '55555')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /join lobby/i }))
      await flushPromises()
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to join lobby 55555: lobby full')
    // Connection must not be stuck in 'pending' forever.
    expect(screen.queryByText('pending')).not.toBeInTheDocument()
    expect(screen.getByText('failed')).toBeInTheDocument()
  })
})

describe('Dashboard - port list error handling', () => {
  it('shows an error when AddPort rejects', async () => {
    vi.spyOn(mockApp, 'AddPort').mockRejectedValueOnce(new Error('nftables unavailable'))
    await renderHome()

    await userEvent.type(screen.getByPlaceholderText(/^port$/i), '8080')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /add port/i }))
      await flushPromises()
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to add port 8080: nftables unavailable')
  })

  it('shows an error when RemovePort rejects', async () => {
    vi.spyOn(mockApp, 'RemovePort').mockRejectedValueOnce(new Error('rule locked'))
    await renderHome()

    const removeButtons = screen.getAllByTitle(/remove port/i)
    await act(async () => {
      fireEvent.click(removeButtons[0])
      await flushPromises()
    })

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to remove port 80: rule locked')
  })
})

describe('Dashboard - error banner behavior', () => {
  it('renders string rejections verbatim, not "[object Object]"', async () => {
    // Wails rejects with plain strings, not Error instances.
    vi.spyOn(mockApp, 'StartBridge').mockRejectedValueOnce('steam is not running' as never)
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Failed to start bridge: steam is not running')
    expect(alert.textContent).not.toContain('[object Object]')
  })

  it('serializes plain-object rejections instead of "[object Object]"', async () => {
    vi.spyOn(mockApp, 'StartBridge').mockRejectedValueOnce({ code: 5 } as never)
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    const alert = screen.getByRole('alert')
    expect(alert.textContent).not.toContain('[object Object]')
    expect(alert).toHaveTextContent('{"code":5}')
  })

  it('clears the banner when the next action succeeds', async () => {
    vi.spyOn(mockApp, 'StartBridge').mockRejectedValueOnce(new Error('transient'))
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })
    expect(screen.getByRole('alert')).toBeInTheDocument()

    // Second click uses the real mock implementation and succeeds.
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /stop bridge/i })).toBeInTheDocument()
  })

  it('exposes the dismiss control as an accessible button', async () => {
    vi.spyOn(mockApp, 'StartBridge').mockRejectedValueOnce(new Error('boom'))
    await renderHome()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /start bridge/i }))
      await flushPromises()
    })

    const dismiss = screen.getByRole('button', { name: 'Dismiss error' })
    expect(dismiss).toBeInTheDocument()
  })
})

