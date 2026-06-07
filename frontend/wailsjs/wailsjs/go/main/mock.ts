import type { StatusPayload, PeerInfo } from './App'

// Simulated state for browser-only development (no Go backend)
let mockRunning = false

export const mockApp = {
  StartBridge: async (): Promise<void> => { mockRunning = true },
  StopBridge: async (): Promise<void> => { mockRunning = false },

  GetStatus: async (): Promise<StatusPayload> => ({
    running: mockRunning,
    localIP: mockRunning ? '10.8.0.2' : '0.0.0.0',
    steamID: '76561198012345678',
    peerCount: mockRunning ? 1 : 0,
  }),

  GetPeers: async (): Promise<PeerInfo[]> =>
    mockRunning ? [{ steamID: '76561198099999999', ip: '10.8.0.3' }] : [],

  GetFirewallState: async (): Promise<boolean> => false,

  GetAllowedPorts: async (): Promise<number[]> => [80, 443],

  AddPort: async (_port: number): Promise<void> => {},
  RemovePort: async (_port: number): Promise<void> => {},
  ToggleFirewall: async (_enabled: boolean): Promise<void> => {},
}

// Returns true when running inside the Wails runtime
export function isWails(): boolean {
  return typeof window !== 'undefined' && !!(window as any)['go']
}
