# Runs a DOL or WAD in Dolphin with an SD card image built from a folder, waits
# for the program to power the console off, then prints the files the run left
# on the card (in particular shroombrew-channel.log). No windows, no screenshots.
#
#   run-dolphin.ps1 -Boot file.dol -SdFolder C:\path\sd [-Seconds 90]
#                   [-Image out.raw] [-Files /shroombrew-channel.log,/forwarder-test.txt]
#
# Dolphin's SD card file is swapped for the run and put back afterwards, because
# the per-run SD path setting was not reliably honoured.
param(
  [Parameter(Mandatory)][string]$Boot,
  [Parameter(Mandatory)][string]$SdFolder,
  [int]$Seconds = 90,
  [string]$Image = "$env:TEMP\sbtest-sd.raw",
  [string[]]$Files = @('/shroombrew-channel.log', '/forwarder-test.txt'),
  [string]$DolphinDir = 'C:\Users\marki\Documents\Dolphin-x64'
)
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$card = "$DolphinDir\User\Load\WiiSD.raw"
$blank = "$env:TEMP\blank-sd.raw"
$backup = "$card.bak-shroombrew"
$failed = $false

if (-not (Test-Path $blank)) { throw "missing clean SD template: $blank" }
if (-not (Test-Path $backup)) { Copy-Item $card $backup }

Get-Process Dolphin -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep 1
python -W ignore -I "$here\sdimage.py" make $Image $SdFolder $blank
Copy-Item $Image $card -Force
try {
  $p = Start-Process "$DolphinDir\Dolphin.exe" -ArgumentList @('-b', '-e', $Boot, '-v', 'Null') -PassThru -WindowStyle Hidden
  if (-not $p.WaitForExit($Seconds * 1000)) {
    Stop-Process -Id $p.Id -Force
    "TIMEOUT after $Seconds s (killed)"
    $failed = $true
  } else {
    "dolphin exited with code $($p.ExitCode)"
    if ($p.ExitCode -ne 0) { $failed = $true }
  }
} finally {
  Get-Process Dolphin -ErrorAction SilentlyContinue | Stop-Process -Force
  Start-Sleep 1
  Copy-Item $card $Image -Force
  Copy-Item $backup $card -Force
}

"--- files on the card"
python -W ignore -I "$here\sdimage.py" list $Image 2>&1 | Select-Object -Last 20
foreach ($f in $Files) {
  "--- $f"
  python -W ignore -I "$here\sdimage.py" read $Image $f 2>&1 | Select-Object -Last 40
}
if ($failed) { exit 1 }
