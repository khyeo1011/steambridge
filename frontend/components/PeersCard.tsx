import React from 'react'
import styles from '../pages/index.module.css'
import { main } from '../wailsjs/wailsjs/go/models'
type PeerInfo = main.PeerInfo

interface PeersCardProps {
  peers: PeerInfo[]
}

// Generate gradient color based on hashing steamID
function getAvatarStyle(steamID: string) {
  let hash = 0
  for (let i = 0; i < steamID.length; i++) {
    hash = steamID.charCodeAt(i) + ((hash << 5) - hash)
  }
  const colors = [
    ['#3b82f6', '#1e40af'], // Blue
    ['#10b981', '#065f46'], // Emerald
    ['#8b5cf6', '#5b21b6'], // Violet
    ['#f59e0b', '#92400e'], // Amber
    ['#06b6d4', '#075985'], // Cyan
    ['#ec4899', '#9d174d'], // Pink
  ]
  const choice = colors[Math.abs(hash) % colors.length]
  return {
    background: `linear-gradient(135deg, ${choice[0]} 0%, ${choice[1]} 100%)`,
    boxShadow: `0 2px 8px rgba(0, 0, 0, 0.2)`
  }
}

export const PeersCard: React.FC<PeersCardProps> = ({ peers }) => {
  return (
    <div className={`${styles.card} ${styles.peersCard}`}>
      <h2>Connected Peers ({peers.length})</h2>
      
      <div className={styles.peersContainer}>
        {peers.length === 0 ? (
          <div className={styles.empty}>No active peer connections</div>
        ) : (
          peers.map(p => {
            const avatarStyle = getAvatarStyle(p.steamID)
            const initials = p.steamID.length > 2 ? p.steamID.slice(-2) : 'P'
            return (
              <div key={p.steamID} className={styles.peerRow}>
                <div className={styles.peerInfo}>
                  <div className={styles.peerAvatar} style={avatarStyle}>
                    {initials}
                  </div>
                  <div className={styles.peerMeta}>
                    <span className={styles.peerSteamID}>{p.steamID}</span>
                    <span className={styles.peerIP}>{p.ip}</span>
                  </div>
                </div>
                <span className={styles.peerStatusBadge}>Active</span>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
