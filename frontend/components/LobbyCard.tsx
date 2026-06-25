import React, { useState } from 'react'
import styles from '../pages/index.module.css'

type ConnectionState = 'pending' | 'awaiting' | 'connected' | 'failed'

interface LobbyCardProps {
  running: boolean
  connections: Record<string, ConnectionState>
  onJoinLobby: (steamID: string) => Promise<void>
  onOpenOverlay: () => void
  onRespond: (steamID: string, accept: boolean) => void
}

export const LobbyCard: React.FC<LobbyCardProps> = ({
  running,
  connections,
  onJoinLobby,
  onOpenOverlay,
  onRespond,
}) => {
  const [joinInput, setJoinInput] = useState('')

  const handleJoin = async () => {
    const id = joinInput.trim()
    if (!/^\d+$/.test(id)) return
    await onJoinLobby(id)
    setJoinInput('')
  }

  const badgeClass = (state: ConnectionState) =>
    state === 'pending' ? styles.statusPending :
    state === 'awaiting' ? styles.statusPending :
    state === 'connected' ? styles.statusConnected :
    styles.statusFailed

  const entries = Object.entries(connections)

  return (
    <div className={styles.card}>
      <h2>Lobby Connections</h2>

      <div className={styles.friendsLayout}>
        <div className={styles.friendsActionRow}>
          <button
            className={`${styles.btn} ${styles.btnSecondary}`}
            onClick={onOpenOverlay}
            disabled={!running}
            style={{ width: '100%' }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginRight: '0.25rem' }}>
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
              <path d="M16 3.13a4 4 0 0 1 0 7.75" />
            </svg>
            Join via Steam Friends
          </button>
        </div>

        <div className={styles.friendsInputContainer}>
          <input
            className={styles.input}
            type="text"
            inputMode="numeric"
            placeholder="Enter Steam ID to join lobby..."
            value={joinInput}
            onChange={e => setJoinInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleJoin() }}
          />
          <button
            className={`${styles.btn} ${styles.btnPrimary}`}
            onClick={handleJoin}
            disabled={!running || !joinInput.trim()}
            style={{ flex: 'none' }}
          >
            Join Lobby
          </button>
        </div>

        {entries.length > 0 && (
          <ul className={styles.connectionList}>
            {entries.map(([steamID, state]) => (
              <li key={steamID} className={styles.connectionRow}>
                <span className={styles.connectionSteamID}>{steamID}</span>
                {state === 'awaiting' ? (
                  <>
                    <button
                      className={`${styles.btn} ${styles.btnPrimary}`}
                      style={{ padding: '0.1rem 0.5rem', fontSize: '0.75rem' }}
                      onClick={() => onRespond(steamID, true)}
                    >Accept</button>
                    <button
                      className={`${styles.btn} ${styles.btnSecondary}`}
                      style={{ padding: '0.1rem 0.5rem', fontSize: '0.75rem' }}
                      onClick={() => onRespond(steamID, false)}
                    >Reject</button>
                  </>
                ) : (
                  <span className={badgeClass(state)}>{state}</span>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
