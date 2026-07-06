# One-command e2e harness: this machine acts as SteamBridge host, the
# configured homelab machine acts as guest (joins via --peer <HostSteamID>).
#
# Usage (elevated PowerShell):
#   .\scripts\e2e\run-e2e.ps1 [-Build] [-BuildRemote] [-Firewall]
#
# -Build        rebuild the local (host) binary before running
# -BuildRemote  git pull + rebuild on the homelab before running
# -Firewall     start the host with --firewall and run the firewall-block probe

param(
    [switch]$Build,
    [switch]$BuildRemote,
    [switch]$Firewall
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path "$PSScriptRoot\..\.."
Set-Location $RepoRoot

. "$PSScriptRoot\e2e.config.ps1"

# --- helpers -------------------------------------------------------------

function Test-IsAdmin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p = [Security.Principal.WindowsPrincipal]::new($id)
    return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-Remote {
    param([string]$Command)
    ssh $RemoteHost $Command
}

function Wait-JobOutputMatch {
    param($Job, [string]$Pattern, [int]$TimeoutSec)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $TimeoutSec) {
        $out = (Receive-Job -Job $Job -Keep) -join "`n"
        if ($out -match $Pattern) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

$Results = @()
function Add-Result {
    param([string]$Name, [bool]$Passed, [string]$Detail = "")
    $script:Results += [PSCustomObject]@{ Probe = $Name; Result = if ($Passed) { "PASS" } else { "FAIL" }; Detail = $Detail }
}

# --- preflight -------------------------------------------------------------

Write-Host "== Preflight =="

if (-not (Test-IsAdmin)) {
    throw "Run this script from an elevated PowerShell (Wintun adapter creation requires admin)."
}

if (-not (Get-Process steam -ErrorAction SilentlyContinue)) {
    throw "Steam is not running locally. Start Steam and retry."
}

ssh -o BatchMode=yes -o ConnectTimeout=5 $RemoteHost "true" 2>$null
if ($LASTEXITCODE -ne 0) {
    throw "Cannot ssh to $RemoteHost non-interactively. Check key auth (ssh -o BatchMode=yes $RemoteHost true)."
}

if ($Build) {
    Write-Host "Building locally..."
    & "$RepoRoot\scripts\build-windows.bat"
}

if ($BuildRemote) {
    Write-Host "Building on homelab..."
    Invoke-Remote "cd $RemoteRepoPath && git pull && (cd cbridge && bash build.sh) && go build ./cmd/steambridge"
    if ($LASTEXITCODE -ne 0) { throw "Remote build failed." }
}

$localHead = (git rev-parse HEAD).Trim()
$remoteHead = (Invoke-Remote "git -C $RemoteRepoPath rev-parse HEAD").Trim()
if ($localHead -ne $remoteHead) {
    throw "HEAD mismatch - local $localHead vs homelab $remoteHead. Build/pull one side or pass -BuildRemote/-Build."
}
Write-Host "HEAD parity confirmed: $localHead"

$localDirty = (git status --porcelain)
$remoteDirty = Invoke-Remote "git -C $RemoteRepoPath status --porcelain"
if ($localDirty) { Write-Warning "Local working tree is dirty - HEAD parity does not guarantee code parity." }
if ($remoteDirty) { Write-Warning "Homelab working tree is dirty - HEAD parity does not guarantee code parity." }

# --- run ---------------------------------------------------------------

$HostJob = $null
$GuestJob = $null

try {
    Write-Host "`n== Starting host =="
    $hostArgs = @("run", "./cmd/steambridge", "--ifaceName", $Iface)
    if ($Firewall) { $hostArgs += "--firewall" }
    $HostJob = Start-Job -ScriptBlock {
        param($repoRoot, $goArgs)
        Set-Location $repoRoot
        & go @goArgs 2>&1
    } -ArgumentList $RepoRoot, $hostArgs

    if (-not (Wait-JobOutputMatch -Job $HostJob -Pattern "SteamBridge is live" -TimeoutSec 20)) {
        throw "Host bridge did not report ready within 20s. Job output:`n$((Receive-Job -Job $HostJob -Keep) -join "`n")"
    }
    Write-Host "Host is live."

    Write-Host "Assigning host IP $HostIP..."
    netsh interface ip set address name="$Iface" static $HostIP 255.255.255.0 | Out-Null

    Write-Host "`n== Launching guest on $RemoteHost =="
    $guestCmd = "cd $RemoteRepoPath && bash scripts/e2e/remote-guest.sh $HostSteamID"
    $GuestJob = Start-Job -ScriptBlock {
        param($remoteHost, $cmd)
        ssh $remoteHost $cmd 2>&1
    } -ArgumentList $RemoteHost, $guestCmd

    if (-not (Wait-JobOutputMatch -Job $GuestJob -Pattern "E2E_GUEST_READY" -TimeoutSec 60)) {
        throw "Guest did not become ready within 60s. Job output:`n$((Receive-Job -Job $GuestJob -Keep) -join "`n")"
    }
    Write-Host "Guest is live at $GuestIP."

    # --- probe registry --------------------------------------------------
    # Add new scenarios here as one more entry; each Run block returns
    # $true/$false and has $GuestIP/$ProbePort/$BlockedPort/Invoke-Remote in scope.
    $Probes = @(
        @{
            Name = "icmp-ping"
            RequiresFirewall = $false
            Run = {
                $ok = Test-Connection -ComputerName $GuestIP -Count 3 -Quiet
                return $ok
            }
        }
        @{
            Name = "tcp-probe"
            RequiresFirewall = $false
            Run = {
                $r = Test-NetConnection -ComputerName $GuestIP -Port $ProbePort -WarningAction SilentlyContinue
                return $r.TcpTestSucceeded
            }
        }
        @{
            Name = "udp-probe"
            RequiresFirewall = $false
            # Sends a random token over UDP and reads it back from the
            # guest's listener log (remote-guest.sh redirects `nc -l -u` to
            # /tmp/e2e_udp.log) to confirm both delivery and content integrity.
            Run = {
                $UdpClient = New-Object System.Net.Sockets.UdpClient
                $UdpClient.Connect($GuestIP, $ProbePort)
                $Message = [guid]::NewGuid().ToString("N")
                $Bytes = [System.Text.Encoding]::UTF8.GetBytes($Message)
                [void]$UdpClient.Send($Bytes, $Bytes.Length)
                $UdpClient.Close()

                Start-Sleep -Milliseconds 500
                $RemoteLog = Invoke-Remote "cat /tmp/e2e_udp.log"

                return $RemoteLog -ceq $Message
            }
        }
        @{
            Name = "disconnect"
            RequiresFirewall = $false
            Run = {
                Invoke-Remote "pkill -f 'steambridge --ifaceName'" | Out-Null
                return Wait-JobOutputMatch -Job $HostJob -Pattern "disconnected; releasing IP" -TimeoutSec 15
            }
        }
        @{
            Name = "firewall-block"
            RequiresFirewall = $true
            Run = {
                $blocked = Test-NetConnection -ComputerName $GuestIP -Port $BlockedPort -WarningAction SilentlyContinue
                $allowed = Test-NetConnection -ComputerName $GuestIP -Port $ProbePort -WarningAction SilentlyContinue
                return (-not $blocked.TcpTestSucceeded) -and $allowed.TcpTestSucceeded
            }
        }
    )

    Write-Host "`n== Running probes =="
    foreach ($probe in $Probes) {
        if ($probe.RequiresFirewall -and -not $Firewall) {
            Add-Result -Name $probe.Name -Passed $true -Detail "skipped (pass -Firewall to run)"
            continue
        }
        try {
            $passed = & $probe.Run
            Add-Result -Name $probe.Name -Passed $passed
        } catch {
            Add-Result -Name $probe.Name -Passed $false -Detail $_.Exception.Message
        }
    }
}
finally {
    Write-Host "`n== Tearing down =="
    if ($GuestJob) {
        Invoke-Remote "pkill -f 'steambridge --ifaceName'" 2>$null | Out-Null
        Stop-Job -Job $GuestJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job -Job $GuestJob -Force -ErrorAction SilentlyContinue | Out-Null
    }
    if ($HostJob) {
        Stop-Job -Job $HostJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job -Job $HostJob -Force -ErrorAction SilentlyContinue | Out-Null
    }

    Start-Sleep -Seconds 2
    $remoteIfaceGone = Invoke-Remote "ip link show $Iface 2>/dev/null || echo GONE"
    if ($remoteIfaceGone -notmatch "GONE") {
        Write-Warning "Remote interface $Iface still present after teardown - next run may need manual cleanup."
    }
}

# --- report ----------------------------------------------------------------

Write-Host "`n== Results =="
$Results | Format-Table -AutoSize

if ($Results | Where-Object { $_.Result -eq "FAIL" }) {
    exit 1
}
exit 0
