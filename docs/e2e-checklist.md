# Manual E2E Checklist — GUI-only flows

Run `scripts/e2e/run-e2e.ps1` first for the scriptable connectivity checks.
This checklist covers the flows that require the Wails GUI or the Steam
overlay and can't be driven headlessly.

## Dashboard lifecycle

- [ ] Launch the Wails app, click **Start Bridge** — status card shows Running, local SteamID, `0.0.0.0` local IP (host has no self-assigned IP - expected, see HANDOFF known limitations)
- [ ] Click **Stop Bridge** — status card returns to stopped, no crash/panic in console

## Peer table

- [ ] With host running, have the guest join (script or manual) — Peers card shows the guest's SteamID and assigned IP within a few seconds
- [ ] Guest disconnects — peer disappears from the table

## Firewall controls

- [ ] Toggle firewall on — GetFirewallState reflects it in the UI
- [ ] Add a port via the port editor, confirm traffic on that port passes; remove it, confirm it's blocked again

## Steam social integration

- [ ] Host sets rich presence (bridge running) — friend sees "Join Game" in the Steam friends list (requires a real shared app ID, not test ID 480 - see HANDOFF Phase 2.3 notes)
- [ ] Friend clicks "Join Game" — guest auto-joins without manual SteamID entry
- [ ] Non-friend submits a SteamID via "Join by Steam ID" — host sees `joinConfirmRequest` banner; Accept lets the join proceed, Reject blocks it

## Disconnect / error handling

- [ ] Kill the guest process abruptly (not graceful stop) — host eventually reflects the disconnect (or note if it hangs - known gap, HANDOFF 2.5)
- [ ] `bridgeDisconnected` event (abnormal engine exit) surfaces visibly in the UI, not just the console log
