import React, { useState } from 'react'
import styles from '../pages/index.module.css'

interface FirewallCardProps {
  firewallEnabled: boolean
  allowedPorts: number[]
  onToggleFirewall: () => void
  onAddPort: (port: number) => Promise<void>
  onRemovePort: (port: number) => Promise<void>
}

export const FirewallCard: React.FC<FirewallCardProps> = ({
  firewallEnabled,
  allowedPorts,
  onToggleFirewall,
  onAddPort,
  onRemovePort,
}) => {
  const [portInput, setPortInput] = useState('')

  const handleAdd = async () => {
    const port = parseInt(portInput, 10)
    if (isNaN(port) || port < 1 || port > 65535) return
    await onAddPort(port)
    setPortInput('')
  }

  return (
    <div className={styles.card}>
      <div className={styles.firewallHeader}>
        <h2>Firewall Settings</h2>
        <label className={styles.switch}>
          <input
            type="checkbox"
            checked={firewallEnabled}
            onChange={onToggleFirewall}
          />
          <span className={styles.slider}></span>
        </label>
      </div>
      
      <p className={styles.statLabel} style={{ marginBottom: '0.5rem', marginTop: '0.25rem' }}>
        Allow connections only on these ports:
      </p>

      <div className={styles.portList}>
        {allowedPorts.length === 0 ? (
          <span className={styles.empty} style={{ padding: '0.25rem 0', fontSize: '0.75rem' }}>No ports allowed</span>
        ) : (
          allowedPorts.map(p => (
            <span key={p} className={styles.portTag}>
              {p}
              <button className={styles.portRemove} onClick={() => onRemovePort(p)} title="Remove port">×</button>
            </span>
          ))
        )}
      </div>

      <div className={styles.portAddForm}>
        <input
          className={`${styles.input} ${styles.portInput}`}
          type="number"
          min={1}
          max={65535}
          placeholder="Port"
          value={portInput}
          onChange={e => setPortInput(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
        />
        <button
          className={`${styles.btn} ${styles.btnSecondary}`}
          onClick={handleAdd}
          style={{ flex: 'none' }}
        >
          Add Port
        </button>
      </div>
    </div>
  )
}
