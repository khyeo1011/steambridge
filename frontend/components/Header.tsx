import React from 'react'
import styles from '../pages/index.module.css'

interface HeaderProps {
  running: boolean
}

export const Header: React.FC<HeaderProps> = ({ running }) => {
  return (
    <header className={styles.header}>
      <div className={styles.headerContainer}>
        <div className={styles.logoIcon}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M16 18l6-6-6-6M8 6l-6 6 6 6" />
            <line x1="12" y1="2" x2="12" y2="22" />
          </svg>
        </div>
        <span className={styles.title}>SteamBridge</span>
      </div>
      <div className={`${styles.badge} ${running ? styles.badgeOn : styles.badgeOff}`}>
        <span className={`${styles.badgeDot} ${running ? styles.badgeDotOn : styles.badgeDotOff}`}></span>
        {running ? 'Running' : 'Stopped'}
      </div>
    </header>
  )
}
