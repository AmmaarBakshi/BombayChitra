<#
.SYNOPSIS
  Builds the BombayChitra dataset from OpenStreetMap (Windows).

.DESCRIPTION
  Produces data\build\bombaychitra.pmtiles (vector tiles) and data\build\bombaychitra.db (SQLite).
  Requires Java 21+ (JDK). Extra arguments are passed to the pipeline, for example:
    .\scripts\build-data.ps1 --threads=4 --only_layers=roads,pois
    $env:JAVA_OPTS = "-Xmx8g"; .\scripts\build-data.ps1 --refresh_sources
#>
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$PlanetilerVersion = if ($env:PLANETILER_VERSION) { $env:PLANETILER_VERSION } else { '0.10.2' }
$PlanetilerSha256 = if ($env:PLANETILER_SHA256) { $env:PLANETILER_SHA256 } else { 'f310bd0413e2e4512b27f4046d418664e8e1d3bf31603c2a70e23de06c167e4d' }
$JavaOpts = if ($env:JAVA_OPTS) { $env:JAVA_OPTS -split ' ' } else { @('-Xmx4g') }
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

Write-Host '-> building data (first run downloads the OpenStreetMap extract)'
& java @JavaOpts --enable-native-access=ALL-UNNAMED -cp "$Jar;$Classes" bombaychitra.pipeline.Main --download @args
if ($LASTEXITCODE -ne 0) { throw 'pipeline failed' }

Write-Host 'done:'
Get-Item data\build\bombaychitra.pmtiles, data\build\bombaychitra.db | Format-Table Name, @{ n = 'MB'; e = { [math]::Round($_.Length / 1MB, 1) } }
