param(
  [string]$Repo = $env:SPEC_KITTY_ANALYZER_REPO,
  [string]$Version = $env:SPEC_KITTY_ANALYZER_VERSION,
  [string]$BinDir = $env:SPEC_KITTY_ANALYZER_BIN_DIR,
  [switch]$NoPathEdit
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Repo)) {
  $Repo = "Priivacy-ai/spec-kitty-analyzer"
}
if ([string]::IsNullOrWhiteSpace($Version)) {
  $Version = "latest"
}
if ([string]::IsNullOrWhiteSpace($BinDir)) {
  $BinDir = Join-Path $HOME ".local\bin"
}

$binName = "spec-kitty-analyzer.exe"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$workDir = $null

function Fail($Message) {
  Write-Error $Message
  exit 1
}

function Get-AssetArch {
  switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64"; break }
    "ARM64" { "arm64"; break }
    default { Fail "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
  }
}

function Get-AssetUrl($Arch) {
  $asset = "spec-kitty-analyzer_windows_$Arch.zip"
  if ($Version -eq "latest") {
    "https://github.com/$Repo/releases/latest/download/$asset"
  } else {
    "https://github.com/$Repo/releases/download/$Version/$asset"
  }
}

function Find-LocalBinary {
  $candidates = @(
    (Join-Path $scriptDir $binName),
    (Join-Path (Split-Path -Parent $scriptDir) $binName),
    (Join-Path $scriptDir "bin\$binName"),
    (Join-Path (Split-Path -Parent $scriptDir) "bin\$binName")
  )
  foreach ($candidate in $candidates) {
    if (Test-Path $candidate) {
      return $candidate
    }
  }
  return $null
}

function Find-SkillSource {
  $candidates = @(
    (Join-Path $scriptDir "skills\spec-kitty-analyzer\SKILL.md"),
    (Join-Path (Split-Path -Parent $scriptDir) "skills\spec-kitty-analyzer\SKILL.md")
  )
  if ($workDir) {
    $candidates += (Join-Path $workDir "skills\spec-kitty-analyzer\SKILL.md")
  }
  foreach ($candidate in $candidates) {
    if (Test-Path $candidate) {
      return $candidate
    }
  }
  return $null
}

function Add-PathIfNeeded($Dir) {
  $current = [Environment]::GetEnvironmentVariable("Path", "User")
  $parts = @()
  if ($current) {
    $parts = $current -split ";"
  }
  if ($parts -contains $Dir) {
    return
  }
  if ($NoPathEdit -or $env:SPEC_KITTY_ANALYZER_NO_PATH_EDIT -eq "1") {
    Write-Host "$Dir is not on PATH; path edit skipped."
    return
  }
  $newPath = if ($current) { "$Dir;$current" } else { $Dir }
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  $env:Path = "$Dir;$env:Path"
  Write-Host "Added $Dir to user PATH. Open a new shell to inherit it."
}

function Install-Skill($Root, $Src) {
  if (!(Test-Path $Root)) {
    return
  }
  $dest = Join-Path $Root "spec-kitty-analyzer"
  New-Item -ItemType Directory -Force -Path $dest | Out-Null
  Copy-Item $Src (Join-Path $dest "SKILL.md") -Force
  Write-Host "Installed skill: $(Join-Path $dest 'SKILL.md')"
}

try {
  New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

  $binary = Find-LocalBinary
  if (-not $binary) {
    $arch = Get-AssetArch
    $url = Get-AssetUrl $arch
    $workDir = Join-Path ([IO.Path]::GetTempPath()) ("spec-kitty-analyzer-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $workDir | Out-Null
    $archive = Join-Path $workDir "spec-kitty-analyzer.zip"
    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $archive
    Expand-Archive -Path $archive -DestinationPath $workDir -Force
    $binary = Join-Path $workDir $binName
  }

  if (!(Test-Path $binary)) {
    Fail "could not locate $binName binary"
  }

  $destBinary = Join-Path $BinDir $binName
  Copy-Item $binary $destBinary -Force
  Add-PathIfNeeded $BinDir
  Write-Host "Installed CLI: $destBinary"

  $skillSrc = Find-SkillSource
  if ($skillSrc) {
    Install-Skill (Join-Path $HOME ".agents\skills") $skillSrc
    Install-Skill (Join-Path $HOME ".claude\skills") $skillSrc
  } else {
    Write-Host "Skill source not found; CLI installed without agent skill."
  }

  & $destBinary --version
} finally {
  if ($workDir -and (Test-Path $workDir)) {
    Remove-Item -Recurse -Force $workDir
  }
}
