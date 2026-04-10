$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
$BinName = "parquet-rowcount"
$OutputDir = Join-Path $RootDir "dist"
$Targets = @(
    @{ GOOS = "darwin"; GOARCH = "amd64" },
    @{ GOOS = "darwin"; GOARCH = "arm64" },
    @{ GOOS = "linux"; GOARCH = "amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64" },
    @{ GOOS = "windows"; GOARCH = "amd64" },
    @{ GOOS = "windows"; GOARCH = "arm64" }
)

if (Test-Path $OutputDir) {
    Remove-Item -Recurse -Force $OutputDir
}
New-Item -ItemType Directory -Path $OutputDir | Out-Null

foreach ($Target in $Targets) {
    $goos = $Target.GOOS
    $goarch = $Target.GOARCH
    $ext = if ($goos -eq "windows") { ".exe" } else { "" }
    $output = Join-Path $OutputDir "$BinName-$goos-$goarch$ext"

    Write-Host "building $output"
    $env:CGO_ENABLED = "0"
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    go build -trimpath -ldflags="-s -w" -o $output ./cmd/parquet-rowcount
}

Write-Host "artifacts written to $OutputDir"
