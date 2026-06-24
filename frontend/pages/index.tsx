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
}

const defaultStatus: StatusPayload = {
  running: false,
  localIP: '—',
  steamID: '—',
  peerCount: 0,
}

const Home: NextPage = () => {
  const [status, setStatus] = useState<StatusPayload>(defaultStatus)
  const [peers, setPeers] = useState<PeerInfo[]>([])
  const [firewallEnabled, setFirewallEnabled] = useState(false)
  const [allowedPorts, setAllowedPorts] = useState<number[]>([])
  const [busy, setBusy] = useState(false)
  const [joinNotice, setJoinNotice] = useState('')
  const [copiedIP, setCopiedIP] = useState(false)
  const [copiedSteamID, setCopiedSteamID] = useState(false)

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
    const unlisten = EventsOn('joinRequest', (steamID: string) => {
      setJoinNotice(`Joining ${steamID}…`)
      setTimeout(() => setJoinNotice(''), 5000)
    })
    return () => unlisten()
  }, [])

  const handleStart = async () => {
    setBusy(true)
    await app.StartBridge()
    await refresh()
    setBusy(false)
  }

  const handleStop = async () => {
    setBusy(true)
    await app.StopBridge()
    await refresh()
    setBusy(false)
  }

  const handleJoin = async (steamID: string) => {
    await app.JoinLobby(steamID)
    setJoinNotice(`Joining ${steamID}…`)
    setTimeout(() => setJoinNotice(''), 5000)
    await refresh()
  }

  const handleToggleFirewall = async () => {
    const next = !firewallEnabled
    setFirewallEnabled(next)
    await app.ToggleFirewall(next)
  }

  const handleAddPort = async (port: number) => {
    await app.AddPort(port)
    const updated = await app.GetAllowedPorts()
    setAllowedPorts(updated)
  }

  const handleRemovePort = async (port: number) => {
    await app.RemovePort(port)
    const updated = await app.GetAllowedPorts()
    setAllowedPorts(updated)
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
            joinNotice={joinNotice}
            onJoinLobby={handleJoin}
            onOpenOverlay={() => app.OpenFriendsOverlay()}
          />

          <PeersCard peers={peers} />
        </div>
      </div>
    </div>
  )
}

export default Home


