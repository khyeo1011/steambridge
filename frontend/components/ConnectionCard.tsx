import React from 'react'
import styles from '../pages/index.module.css'

interface ConnectionCardProps {
  running: boolean
  localIP: string
  steamID: string
  peerCount: number
  busy: boolean
  onStart: () => void
  onStop: () => void
  onCopy: (text: string, type: 'ip' | 'steamid') => void
  copiedIP: boolean
  copiedSteamID: boolean
}

export const ConnectionCard: React.FC<ConnectionCardProps> = ({
  running,
  localIP,
  steamID,
  peerCount,
  busy,
  onStart,
  onStop,
  onCopy,
  copiedIP,
  copiedSteamID,
}) => {
  return (
    <div className={styles.card}>
      <h2>Connection Status</h2>
      <div className={styles.statsList}>
        <div className={styles.statRow}>
          <span className={styles.statLabel}>Local IP</span>
          <span className={styles.statValue}>
            {localIP}
            {localIP !== '—' && localIP !== '0.0.0.0' && (
              <button
                className={styles.copyBtn}
                onClick={() => onCopy(localIP, 'ip')}
                title="Copy IP"
              >
                {copiedIP ? (
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                ) : (
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                  </svg>
                )}
              </button>
            )}
          </span>
        </div>
        <div className={styles.statRow}>
          <span className={styles.statLabel}>Steam ID</span>
          <span className={styles.statValue}>
            {steamID}
            {steamID !== '—' && (
              <button
                className={styles.copyBtn}
                onClick={() => onCopy(steamID, 'steamid')}
                title="Copy Steam ID"
              >
                {copiedSteamID ? (
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                ) : (
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                  </svg>
                )}
              </button>
            )}
          </span>
        </div>
        <div className={styles.statRow}>
          <span className={styles.statLabel}>Active Peers</span>
          <span className={styles.statValue}>{peerCount}</span>
        </div>
      </div>

      <div className={styles.btnRow}>
        {running ? (
          <button
            className={`${styles.btn} ${styles.btnDanger}`}
            onClick={onStop}
            disabled={busy}
          >
            {busy ? 'Stopping...' : 'Stop Bridge'}
          </button>
        ) : (
          <button
            className={`${styles.btn} ${styles.btnPrimary}`}
            onClick={onStart}
            disabled={busy}
          >
            {busy ? 'Starting...' : 'Start Bridge'}
          </button>
        )}
      </div>
    </div>
  )
}
