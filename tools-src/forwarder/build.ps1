# Builds forwarder.dol with devkitPPC. Usage: powershell -File build.ps1
$ErrorActionPreference = 'Stop'
$dk = if ($env:DEVKITPRO -and (Test-Path "$env:DEVKITPRO\devkitPPC")) { $env:DEVKITPRO } else { 'C:\devkitPro' }
$env:PATH = "$dk\devkitPPC\bin;$dk\tools\bin;$env:PATH"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$out = Join-Path $here 'build'
New-Item -ItemType Directory -Force $out | Out-Null

$common = @('-O2', '-Wall', '-mrvl', '-mcpu=750', '-meabi', '-mhard-float', '-I', "$dk\libogc\include", '-I', "$dk\portlibs\ppc\include")
# dol.c holds the relocated stub: no builtins, no loop-to-memcpy rewriting, keep function order.
& powerpc-eabi-gcc @($common + @('-ffreestanding', '-fno-builtin', '-fno-tree-loop-distribute-patterns', '-fno-toplevel-reorder', '-c', "$here\source\dol.c", '-o', "$out\dol.o"))
if ($LASTEXITCODE) { throw 'dol.c failed' }
& powerpc-eabi-gcc @common -c "$here\source\main.c" -o "$out\main.o"
if ($LASTEXITCODE) { throw 'main.c failed' }
& powerpc-eabi-gcc @common -c "$here\source\payload.c" -o "$out\payload.o"
if ($LASTEXITCODE) { throw 'payload.c failed' }
& powerpc-eabi-gcc -mrvl -mcpu=750 -meabi -mhard-float "$out\main.o" "$out\payload.o" "$out\dol.o" -L "$dk\libogc\lib\wii" -L "$dk\portlibs\ppc\lib" -lfat -lz -logc -lm -o "$out\forwarder.elf"
if ($LASTEXITCODE) { throw 'link failed' }
& elf2dol "$out\forwarder.elf" "$out\forwarder.dol"
if ($LASTEXITCODE) { throw 'elf2dol failed' }
"built $out\forwarder.dol ($((Get-Item "$out\forwarder.dol").Length) bytes)"
