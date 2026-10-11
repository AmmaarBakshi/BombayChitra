<#
.SYNOPSIS
  Builds the BombayChitra dataset from OpenStreetMap and Overture Maps (Windows).

.DESCRIPTION
  Produces data\build\bombaychitra.pmtiles (vector tiles) and data\build\bombaychitra.db (SQLite).
  Requires Java 21+ (JDK); Python 3 for the Overture Maps places and buildings (optional - without it the build uses
  OpenStreetMap only; $env:SKIP_OVERTURE = "1" skips them on purpose). Extra arguments are passed to the pipeline:
    .\scripts\build-data.ps1 --threads=4 --only_layers=roads,pois
    $env:JAVA_OPTS = "-Xmx8g"; .\scripts\build-data.ps1 --refresh_sources
#>
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$PlanetilerVersion = if ($env:PLANETILER_VERSION) { $env:PLANETILER_VERSION } else { '0.10.2' }
$PlanetilerSha256 = if ($env:PLANETILER_SHA256) { $env:PLANETILER_SHA256 } else { 'f310bd0413e2e4512b27f4046d418664e8e1d3bf31603c2a70e23de06c167e4d' }
$JavaOpts = if ($env:JAVA_OPTS) { $env:JAVA_OPTS -split ' ' } else { @('-Xmx4g') }
$OvertureCliVersion = if ($env:OVERTURE_CLI_VERSION) { $env:OVERTURE_CLI_VERSION } else { '1.0.2' }
$Jar = "data\tools\planetiler-$PlanetilerVersion.jar"
$Classes = 'pipeline\build\classes'

foreach ($tool in 'java', 'javac') {
  if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "'$tool' is required (install a JDK 21 or newer)" }
}

New-Item -ItemType Directory -Force 'data\tools', 'data\sources', 'data\build' | Out-Null
if (-not (Test-Path $Jar)) {
  Write-Host "-> downloading Planetiler $PlanetilerVersion"
  $part = "$Jar.part"
  Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/onthegomap/planetiler/releases/download/v$PlanetilerVersion/planetiler.jar" -OutFile $part
  $actual = (Get-FileHash $part -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actual -ne $PlanetilerSha256) {
    Remove-Item $part
    throw "checksum mismatch for planetiler.jar ($actual)"
  }
  Move-Item $part $Jar
}

Write-Host '-> compiling pipeline'
if (Test-Path $Classes) { Remove-Item -Recurse -Force $Classes }
New-Item -ItemType Directory -Force $Classes | Out-Null
$sources = Get-ChildItem -Recurse -Filter *.java pipeline\src | ForEach-Object { $_.FullName }
& javac -encoding UTF-8 -implicit:none -sourcepath pipeline\src -cp $Jar -d $Classes @sources
if ($LASTEXITCODE -ne 0) { throw 'javac failed' }

# Overture Maps extracts (places, machine-learned building footprints) in a private virtual environment.
if ($env:SKIP_OVERTURE -eq '1') {
  Write-Host '-> skipping Overture Maps (SKIP_OVERTURE=1)'
} else {
  $python = Get-Command python3, python -ErrorAction SilentlyContinue | Where-Object { $_.Source -notlike '*WindowsApps*' } | Select-Object -First 1
  if (-not $python) {
    Write-Warning 'Python 3 not found - building without Overture Maps places and buildings'
  } else {
    $venv = "data\tools\overture-$OvertureCliVersion"
    $venvPython = Join-Path $venv 'Scripts\python.exe'
    if (-not (Test-Path $venvPython)) {
      Write-Host "-> installing the Overture Maps downloader $OvertureCliVersion"
      & $python.Source -m venv $venv
      & $venvPython -m pip install --quiet --disable-pip-version-check "overturemaps==$OvertureCliVersion"
      if ($LASTEXITCODE -ne 0) { throw 'could not install the Overture Maps downloader' }
    }
    $fetchArgs = @()
    if ($args -contains '--refresh_sources') { $fetchArgs += '--refresh' }
    & $venvPython scripts\fetch-overture.py @fetchArgs
    if ($LASTEXITCODE -ne 0) { throw 'Overture Maps download failed' }
  }
}

Write-Host '-> building data (first run downloads the OpenStreetMap extract)'
& java @JavaOpts --enable-native-access=ALL-UNNAMED -cp "$Jar;$Classes" bombaychitra.pipeline.Main --download @args
if ($LASTEXITCODE -ne 0) { throw 'pipeline failed' }

Write-Host 'done:'
Get-Item data\build\bombaychitra.pmtiles, data\build\bombaychitra.db | Format-Table Name, @{ n = 'MB'; e = { [math]::Round($_.Length / 1MB, 1) } }
