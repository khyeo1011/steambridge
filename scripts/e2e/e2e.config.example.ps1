# Copy this file to e2e.config.ps1 and fill in your values.
# e2e.config.ps1 is gitignored because it contains a hostname/username.

# ssh target for the homelab guest (must support non-interactive key auth)
$RemoteHost = "user@homelab"

# Path to this repo on the homelab machine
$RemoteRepoPath = "/home/user/steambridge"

# SteamID64 of the account running on this (host) machine
$HostSteamID = "0"

# SteamID64 of the account running on the homelab (guest) machine
$GuestSteamID = "0"

# Port already in the CLI's default firewall allowlist (see cmd/steambridge/main.go)
$ProbePort = 5000

# Port NOT in the allowlist, used for the firewall-block negative test
$BlockedPort = 9999

$Iface = "steambridge0"
$HostIP = "10.8.0.1"
$GuestIP = "10.8.0.2"
