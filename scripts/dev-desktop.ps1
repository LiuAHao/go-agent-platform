$ProjectRoot = Split-Path -Parent $PSScriptRoot
$ConsoleDir = Join-Path $ProjectRoot 'web\console'
$DesktopDir = Join-Path $ProjectRoot 'desktop'

$viteJob = $null

try {
    if (-not (Test-Path (Join-Path $ConsoleDir 'node_modules'))) {
        throw 'Frontend dependencies are missing. Run npm install in web/console first.'
    }

    if (-not (Test-Path (Join-Path $DesktopDir 'node_modules'))) {
        throw 'Desktop dependencies are missing. Run npm install in desktop first.'
    }

    Write-Host 'Starting Vite dev server...'
    $viteJob = Start-Job -ScriptBlock {
        param($ConsoleDir)
        Set-Location $ConsoleDir
        & npm.cmd run dev -- --host 127.0.0.1
    } -ArgumentList $ConsoleDir

    Start-Sleep -Seconds 2

    Write-Host 'Starting Electron desktop client...'
    Set-Location $DesktopDir
    & npm.cmd run dev
}
finally {
    if ($viteJob) {
        Stop-Job $viteJob -ErrorAction SilentlyContinue | Out-Null
        Remove-Job $viteJob -Force -ErrorAction SilentlyContinue | Out-Null
    }
}
