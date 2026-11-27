# Control Deck one-click setup for Windows.
# Installs the Python packages, pairs this PC, starts the PC companion and,
# if the Kindle is plugged in by USB, copies the dashboard onto it.
# Run it with Setup.bat. It is safe to run again (for example after your PC's IP changes).

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root
$kindleFolder = Join-Path $root "dist\KindleDashboardNative"
$launcher = Join-Path $root "kindle\documents\Personal Dashboard Offline.sh"

function Step($text) { Write-Host ""; Write-Host "== $text" -ForegroundColor Cyan }
function Done($text) { Write-Host "   OK  $text" -ForegroundColor Green }
function Warn($text) { Write-Host "   !!  $text" -ForegroundColor Yellow }

function Find-Python {
    foreach ($candidate in @(@("py", "-3"), @("python"))) {
        $exe = $candidate[0]
        if (-not (Get-Command $exe -ErrorAction SilentlyContinue)) { continue }
        $extra = @($candidate | Select-Object -Skip 1)
        try {
            $version = & $exe @extra -c "import sys; print('%d.%d' % sys.version_info[:2])" 2>$null
        } catch { continue }
        if ($LASTEXITCODE -eq 0 -and $version -and [version]$version -ge [version]"3.10") {
            return ,@($candidate)
        }
    }
    return $null
}

function Refresh-Path {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = "$machine;$user"
}

# 1. Python
Step "Checking Python"
$python = Find-Python
if (-not $python) {
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Warn "Python 3.10+ not found. Installing Python 3.12 with winget (Windows may ask you to confirm)..."
        winget install -e --id Python.Python.3.12 --scope user --accept-source-agreements --accept-package-agreements
        Refresh-Path
        $python = Find-Python
    }
}
if (-not $python) {
    Warn "Python 3.10 or newer is needed. Install it from python.org (tick 'Add Python to PATH'), then run Setup.bat again."
    Start-Process "https://www.python.org/downloads/windows/"
    exit 1
}
$pyExe = $python[0]
$pyArgs = @($python | Select-Object -Skip 1)
Done ("Using " + (& $pyExe @pyArgs --version))

# 2. Packages
Step "Installing Python packages (psutil, websockets, cryptography)"
& $pyExe @pyArgs -m pip install --disable-pip-version-check -q -r companion\requirements.txt
if ($LASTEXITCODE -ne 0) { Warn "pip could not install the packages. Check your internet connection and try again."; exit 1 }
Done "Packages installed"

# 3. This PC's address on the local network
Step "Finding this PC's local IP address"
$addresses = @()
try {
    $addresses = @(Get-NetIPConfiguration |
        Where-Object { $_.IPv4DefaultGateway -and $_.NetAdapter.Status -eq "Up" } |
        ForEach-Object { $_.IPv4Address.IPAddress })
} catch { }
if ($addresses.Count -eq 0) {
    $addresses = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { $_.IPAddress -notlike "127.*" -and $_.IPAddress -notlike "169.254.*" } |
        ForEach-Object { $_.IPAddress })
}
# A Kindle with USBNetLite turned on is a USB network adapter on 192.168.15.x (no gateway), so add that too.
$usbNet = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -like "192.168.15.*" -and $_.IPAddress -ne "192.168.15.244" } |
    ForEach-Object { $_.IPAddress })
if ($usbNet.Count -gt 0) { Write-Host "   Found the Kindle USB network at $($usbNet -join ', ')" }
$addresses = @($usbNet) + @($addresses)
$addresses = @($addresses | Where-Object { $_ -match '^(10\.|192\.168\.|172\.(1[6-9]|2[0-9]|3[01])\.)' } | Select-Object -Unique)
if ($addresses.Count -eq 0) {
    Warn "No local network address found. Connect this PC to Wi-Fi (the same network as the Kindle) and run Setup.bat again."
    exit 1
}
Done ("PC address: " + ($addresses -join ", "))

# 4. Pairing
Step "Pairing this PC with the Kindle"
$pairArgs = @("companion\pair.py")
foreach ($address in $addresses) { $pairArgs += @("--pc-address", $address) }
& $pyExe @pyArgs @pairArgs
if ($LASTEXITCODE -ne 0) { Warn "Pairing failed."; exit 1 }
Done "Pairing files are in dist\KindleDashboardNative"

# 5. Copy to the Kindle
Step "Looking for your Kindle on USB"
$copied = $false
$volume = Get-Volume -ErrorAction SilentlyContinue |
    Where-Object { $_.FileSystemLabel -match "Kindle" -and $_.DriveLetter } | Select-Object -First 1
if ($volume) {
    # Older firmware: the Kindle is a normal drive such as E:
    $drive = "$($volume.DriveLetter):\"
    Copy-Item -Path $kindleFolder -Destination $drive -Recurse -Force
    New-Item -ItemType Directory -Force -Path (Join-Path $drive "documents") | Out-Null
    Copy-Item -Path $launcher -Destination (Join-Path $drive "documents") -Force
    Done "Copied to the Kindle at $drive"
    $copied = $true
} else {
    # Newer firmware: the Kindle is a portable (MTP) device under This PC.
    try {
        $shell = New-Object -ComObject Shell.Application
        $thisPC = $shell.Namespace(17)
        $device = $thisPC.Items() | Where-Object { $_.Name -match "Kindle" } | Select-Object -First 1
        if ($device) {
            $storage = $device.GetFolder.Items() | Select-Object -First 1
            $target = $storage.GetFolder
            $documents = $target.Items() | Where-Object { $_.Name -eq "documents" } | Select-Object -First 1
            # 16 = answer Yes to all, 4 = no progress dialog.
            $target.CopyHere($kindleFolder, 20)
            if ($documents) { $documents.GetFolder.CopyHere($launcher, 20) }
            $deadline = (Get-Date).AddSeconds(90)
            while ((Get-Date) -lt $deadline) {
                Start-Sleep -Seconds 2
                $copiedFolder = $target.Items() | Where-Object { $_.Name -eq "KindleDashboardNative" }
                if ($copiedFolder -and @($copiedFolder.GetFolder.Items()).Count -ge 3) { break }
            }
            if ($copiedFolder -and $documents) {
                Done "Copied to the Kindle ($($device.Name))"
                $copied = $true
            }
        }
    } catch { }
}
if (-not $copied) {
    Warn "Kindle not found (or it could not be written). Copy these by hand:"
    Write-Host "      1. The folder  dist\KindleDashboardNative  -> the Kindle's main folder"
    Write-Host "      2. The file    kindle\documents\Personal Dashboard Offline.sh  -> the Kindle's documents folder"
    Start-Process explorer.exe (Join-Path $root "dist")
    Start-Process explorer.exe (Join-Path $root "kindle\documents")
}

# 6. Start the PC companion
Step "Starting the PC companion"
$bridge = Join-Path $root "companion\bridge.py"
Start-Process -FilePath $pyExe -ArgumentList ($pyArgs + @("`"$bridge`"")) -WorkingDirectory $root
Start-Sleep -Seconds 3
Done "Running in its own window. If Windows Firewall asks, choose 'Private networks' and Allow."

Step "Almost done"
Write-Host "   - Play a song on YouTube Music in your browser to see its album art on the Kindle."
Write-Host "   - Eject the Kindle, join the same Wi-Fi as this PC, and open 'Personal Dashboard Offline' in the Library."
Write-Host "   - Next time, just double-click 'Start Control Deck.bat'."
Write-Host "   - YouTube controls (optional): load companion\youtube-extension as an unpacked extension in Brave or Chrome."
Write-Host ""
Read-Host "Press Enter to close"
