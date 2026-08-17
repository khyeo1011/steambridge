import type { NextPage } from 'next'
import Head from 'next/head'
import { useState, useEffect, useCallback } from 'react'
import styles from './index.module.css'
import * as WailsApp from '../wailsjs/wailsjs/go/main/App'
import { mockApp, isWails } from '../lib/mock'
import { main } from '../wailsjs/wailsjs/go/models'
type StatusPayload = main.StatusPayload
type PeerInfo = main.PeerInfo
import { EventsOn } from '../wailsjs/wailsjs/runtime'

// Modular Components
import { Header } from '../components/Header'
import { ConnectionCard } from '../components/ConnectionCard'
import { FirewallCard } from '../components/FirewallCard'
import { LobbyCard } from '../components/LobbyCard'
import { PeersCard } from '../components/PeersCard'

const app = {
  StartBridge: () => isWails() ? WailsApp.StartBridge() : mockApp.StartBridge(),
  StopBridge: () => isWails() ? WailsApp.StopBridge() : mockApp.StopBridge(),
  GetStatus: () => isWails() ? WailsApp.GetStatus() : mockApp.GetStatus(),
  GetPeers: () => isWails() ? WailsApp.GetPeers() : mockApp.GetPeers(),
  GetFirewallState: () => isWails() ? WailsApp.GetFirewallState() : mockApp.GetFirewallState(),
  GetAllowedPorts: () => isWails() ? WailsApp.GetAllowedPorts() : mockApp.GetAllowedPorts(),
  AddPort: (p: number) => isWails() ? WailsApp.AddPort(p) : mockApp.AddPort(p),
  RemovePort: (p: number) => isWails() ? WailsApp.RemovePort(p) : mockApp.RemovePort(p),
  ToggleFirewall: (e: boolean) => isWails() ? WailsApp.ToggleFirewall(e) : mockApp.ToggleFirewall(e),
  JoinLobby: (id: string) => isWails() ? WailsApp.JoinLobby(id) : mockApp.JoinLobby(id),
  OpenFriendsOverlay: () => isWails() ? WailsApp.OpenFriendsOverlay() : mockApp.OpenFriendsOverlay(),
  RespondToJoin: (id: string, accept: boolean) => isWails() ? WailsApp.RespondToJoin(id, accept) : mockApp.RespondToJoin(id, accept),
}

const defaultStatus: StatusPayload = {
  running: false,
  localIP: '—',
  steamID: '—',
  peerCount: 0,
}

const errorMessage = (err: unknown): string => {
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  // Wails can reject with plain values; avoid rendering "[object Object]".
  try {
    return JSON.stringify(err) ?? String(err)
  } catch {
    return String(err)
  }
}

const Home: NextPage = () => {
  const [status, setStatus] = useState<StatusPayload>(defaultStatus)
  const [peers, setPeers] = useState<PeerInfo[]>([])
  const [firewallEnabled, setFirewallEnabled] = useState(false)
  const [allowedPorts, setAllowedPorts] = useState<number[]>([])
  const [busy, setBusy] = useState(false)
  const [connections, setConnections] = useState<Record<string, 'pending' | 'awaiting' | 'connected' | 'failed'>>({})
  const [copiedIP, setCopiedIP] = useState(false)
  const [copiedSteamID, setCopiedSteamID] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    const [s, p, fw, ports] = await Promise.all([
      app.GetStatus(),
      app.GetPeers(),
      app.GetFirewallState(),
      app.GetAllowedPorts(),
    ])
    setStatus(s)
    setPeers(p)
    setFirewallEnabled(fw)
    setAllowedPorts(ports)
  }, [])

  useEffect(() => {
    refresh()
    const id = setInterval(refresh, 2000)
    return () => clearInterval(id)
  }, [refresh])

  useEffect(() => {
    if (!isWails()) return
    // Track unlisteners for cleanup
    let unsubscribeRequest: () => void;
    let unsubscribeResult: () => void;

    unsubscribeRequest = EventsOn('joinRequest', (steamID: string) => {
      setConnections(connections => ({
        ...connections,
        [steamID]: 'pending'
      }));
    })

    unsubscribeResult = EventsOn('joinResult', (steamID: string, state: 'connected' | 'failed') => {
      setConnections(connections => {
        const next = { ...connections }
        if (state === 'connected') {
          delete next[steamID]  // peer moves to Connected Peers card once data flows
        } else {
          next[steamID] = state
        }
        return next
      })
    })

    const unsubscribeConfirm = EventsOn('joinConfirmRequest', (steamID: string) => {
      setConnections(connections => ({
        ...connections,
        [steamID]: 'awaiting'
      }))
    })

    return () => {
      if (unsubscribeRequest) unsubscribeRequest();
      if (unsubscribeResult) unsubscribeResult();
      unsubscribeConfirm();
    }
  }, [])

  const handleStart = async () => {
    setBusy(true)
    setError(null)
    try {
      await app.StartBridge()
      await refresh()
    } catch (err) {
      setError(`Failed to start bridge: ${errorMessage(err)}`)
    } finally {
      setBusy(false)
    }
  }

  const handleStop = async () => {
    setBusy(true)
    setError(null)
    try {
      await app.StopBridge()
      setConnections({})
      await refresh()
    } catch (err) {
      setError(`Failed to stop bridge: ${errorMessage(err)}`)
    } finally {
      setBusy(false)
    }
  }

  const handleRespond = async (steamID: string, accept: boolean) => {
    setError(null)
    try {
      await app.RespondToJoin(steamID, accept)
      setConnections(prev => {
        const next = { ...prev }
        if (accept) {
          delete next[steamID]  // peer moves to Connected Peers card once data flows
        } else {
          next[steamID] = 'failed'
        }
        return next
      })
    } catch (err) {
      setError(`Failed to ${accept ? 'accept' : 'reject'} join request: ${errorMessage(err)}`)
    }
  }

  const handleJoin = async (steamID: string) => {
    setConnections(prev => ({ ...prev, [steamID]: 'pending' }))
    setError(null)
    try {
      await app.JoinLobby(steamID)
      // In browser mode, simulate the result after a short delay so the UI is testable.
      if (!isWails()) {
        setTimeout(() => setConnections(prev => ({ ...prev, [steamID]: 'connected' })), 1500)
      }
      await refresh()
    } catch (err) {
      // Don't leave the connection stuck in 'pending' forever.
      setConnections(prev => ({ ...prev, [steamID]: 'failed' }))
      setError(`Failed to join lobby ${steamID}: ${errorMessage(err)}`)
    }
  }

  const handleToggleFirewall = async () => {
    const prev = firewallEnabled
    const next = !prev
    setFirewallEnabled(next)
    setError(null)
    try {
      await app.ToggleFirewall(next)
    } catch (err) {
      // Roll back the optimistic update so the switch matches the bridge.
      setFirewallEnabled(prev)
      setError(`Failed to ${next ? 'enable' : 'disable'} firewall: ${errorMessage(err)}`)
    }
  }

  const handleAddPort = async (port: number) => {
    setError(null)
    try {
      await app.AddPort(port)
      const updated = await app.GetAllowedPorts()
      setAllowedPorts(updated)
    } catch (err) {
      setError(`Failed to add port ${port}: ${errorMessage(err)}`)
    }
  }

  const handleRemovePort = async (port: number) => {
    setError(null)
    try {
      await app.RemovePort(port)
      const updated = await app.GetAllowedPorts()
      setAllowedPorts(updated)
    } catch (err) {
      setError(`Failed to remove port ${port}: ${errorMessage(err)}`)
    }
  }

  const handleCopy = (text: string, type: 'ip' | 'steamid') => {
    if (!text || text === '—') return
    navigator.clipboard.writeText(text)
    if (type === 'ip') {
      setCopiedIP(true)
      setTimeout(() => setCopiedIP(false), 2000)
    } else {
      setCopiedSteamID(true)
      setTimeout(() => setCopiedSteamID(false), 2000)
    }
  }

  return (
    <div className={styles.page}>
      <Head>
        <title>SteamBridge</title>
      </Head>

      <Header running={status.running} />

      {error && (
        <div className={styles.errorBanner} role="alert">
          <span>{error}</span>
          <button
            className={styles.errorDismiss}
            onClick={() => setError(null)}
            aria-label="Dismiss error"
            title="Dismiss"
          >
            ✕
          </button>
        </div>
      )}

      <div className={styles.dashboard}>
        <div className={styles.leftCol}>
          <ConnectionCard
            running={status.running}
            localIP={status.localIP}
            steamID={status.steamID}
            peerCount={status.peerCount}
            busy={busy}
            onStart={handleStart}
            onStop={handleStop}
            onCopy={handleCopy}
            copiedIP={copiedIP}
            copiedSteamID={copiedSteamID}
          />

          <FirewallCard
            firewallEnabled={firewallEnabled}
            allowedPorts={allowedPorts}
            onToggleFirewall={handleToggleFirewall}
            onAddPort={handleAddPort}
            onRemovePort={handleRemovePort}
          />
        </div>

        <div className={styles.rightCol}>
          <LobbyCard
            running={status.running}
            connections={connections}
            onJoinLobby={handleJoin}
            onOpenOverlay={() => app.OpenFriendsOverlay()}
            onRespond={handleRespond}
          />

          <PeersCard peers={peers} />
        </div>
      </div>
    </div>
  )
}

export default Home


