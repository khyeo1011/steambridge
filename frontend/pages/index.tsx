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
  const [portInput, setPortInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [joinInput, setJoinInput] = useState('')
  const [joinNotice, setJoinNotice] = useState('')

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

  const handleJoin = async () => {
    const id = joinInput.trim()
    if (!/^\d+$/.test(id)) return
    await app.JoinLobby(id)
    setJoinInput('')
    setJoinNotice(`Joining ${id}…`)
    setTimeout(() => setJoinNotice(''), 5000)
    await refresh()
  }

  const handleToggleFirewall = async () => {
    const next = !firewallEnabled
    setFirewallEnabled(next)
    await app.ToggleFirewall(next)
  }

  const handleAddPort = async () => {
    const port = parseInt(portInput, 10)
    if (isNaN(port) || port < 1 || port > 65535) return
    await app.AddPort(port)
    setPortInput('')
    const updated = await app.GetAllowedPorts()
    setAllowedPorts(updated)
  }

  const handleRemovePort = async (port: number) => {
    await app.RemovePort(port)
    const updated = await app.GetAllowedPorts()
    setAllowedPorts(updated)
  }

  return (
    <div className={styles.page}>
      <Head><title>SteamBridge</title></Head>

      <header className={styles.header}>
        <span className={styles.title}>SteamBridge</span>
        <span className={`${styles.badge} ${status.running ? styles.badgeOn : styles.badgeOff}`}>
          {status.running ? '● Running' : '○ Stopped'}
        </span>
      </header>

      <div className={styles.row}>
        <div className={styles.card}>
          <h2>Network</h2>
          <p><strong>Local IP:</strong> {status.localIP}</p>
          <p><strong>Steam ID:</strong> {status.steamID}</p>
          <p><strong>Peers:</strong> {status.peerCount}</p>
        </div>

        <div className={styles.card}>
          <h2>Controls</h2>
          <div className={styles.btnRow}>
            <button
              className={styles.btn}
              onClick={handleStart}
              disabled={busy || status.running}
            >
              Start
            </button>
            <button
              className={styles.btn}
              onClick={handleStop}
              disabled={busy || !status.running}
            >
              Stop
            </button>
          </div>
        </div>
      </div>

      <div className={styles.card}>
        <h2>Friends</h2>
        {joinNotice && <p className={styles.empty}>{joinNotice}</p>}
        <div className={styles.btnRow}>
          <button
            className={styles.btn}
            onClick={() => app.OpenFriendsOverlay()}
            disabled={!status.running}
          >
            Join via Steam
          </button>
        </div>
        <div className={styles.portAdd}>
          <input
            className={styles.portInput}
            type="text"
            inputMode="numeric"
            placeholder="Steam ID"
            value={joinInput}
            onChange={e => setJoinInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleJoin() }}
          />
          <button
            className={styles.btn}
            onClick={handleJoin}
            disabled={!status.running}
          >
            Join by Steam ID
          </button>
        </div>
      </div>

      <div className={styles.card}>
        <h2>Connected Peers</h2>
        {peers.length === 0 ? (
          <p className={styles.empty}>No peers connected</p>
        ) : (
          <table className={styles.table}>
            <thead>
              <tr><th>Steam ID</th><th>IP Address</th></tr>
            </thead>
            <tbody>
              {peers.map(p => (
                <tr key={p.steamID}>
                  <td>{p.steamID}</td>
                  <td>{p.ip}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className={styles.card}>
        <h2>
          Firewall&nbsp;
          <button
            className={`${styles.toggle} ${firewallEnabled ? styles.toggleOn : styles.toggleOff}`}
            onClick={handleToggleFirewall}
          >
            {firewallEnabled ? 'ON' : 'OFF'}
          </button>
        </h2>
        <div className={styles.portList}>
          {allowedPorts.map(p => (
            <span key={p} className={styles.portTag}>
              {p}
              <button className={styles.portRemove} onClick={() => handleRemovePort(p)}>×</button>
            </span>
          ))}
        </div>
        <div className={styles.portAdd}>
          <input
            className={styles.portInput}
            type="number"
            min={1}
            max={65535}
            placeholder="port"
            value={portInput}
            onChange={e => setPortInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleAddPort() }}
          />
          <button className={styles.btn} onClick={handleAddPort}>Add Port</button>
        </div>
      </div>
    </div>
  )
}

export default Home
