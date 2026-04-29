$ProjectRoot = Split-Path -Parent $PSScriptRoot
$ConsoleDir = Join-Path $ProjectRoot 'web\console'
$DesktopDir = Join-Path $ProjectRoot 'desktop'

if (-not (Test-Path (Join-Path $ConsoleDir 'dist\index.html'))) {
    throw 'Frontend build output is missing. Run npm run build in web/console first.'
}

if (-not (Test-Path (Join-Path $DesktopDir 'node_modules'))) {
    throw 'Desktop dependencies are missing. Run npm install in desktop first.'
}

Set-Location $DesktopDir
& npm.cmd run start
