param(
  [string]$ConfigDir = (Join-Path $env:TEMP "proxy-hub-dev"),
  [string]$HostName = "localhost",
  [int]$BackendPort = 8787,
  [int]$FrontendPort = 7878,
  [switch]$SkipInstall
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$web = Join-Path $root "web"
$configPath = Join-Path $ConfigDir "config.yaml"
$logDir = Join-Path $ConfigDir "logs"
$binDir = Join-Path $ConfigDir "bin"
$backendExe = Join-Path $binDir "proxy-hub-dev.exe"

function Get-RequiredCommand {
  param(
    [string]$Name,
    [string]$InstallHint
  )

  $command = Get-Command $Name -ErrorAction SilentlyContinue
  if ($null -eq $command) {
    throw "找不到命令：$Name。$InstallHint"
  }
  return $command.Source
}

function New-LogFile {
  param([string]$Path)

  New-Item -ItemType File -Force $Path | Out-Null
  Clear-Content -Path $Path
}

function Start-DevProcess {
  param(
    [string]$Name,
    [string]$FilePath,
    [string[]]$Arguments,
    [string]$WorkingDirectory,
    [string]$OutFile,
    [string]$ErrFile
  )

  New-LogFile $OutFile
  New-LogFile $ErrFile

  $proc = Start-Process `
    -FilePath $FilePath `
    -ArgumentList $Arguments `
    -WorkingDirectory $WorkingDirectory `
    -RedirectStandardOutput $OutFile `
    -RedirectStandardError $ErrFile `
    -WindowStyle Hidden `
    -PassThru

  return [pscustomobject]@{
    Name = $Name
    Process = $proc
    OutFile = $OutFile
    ErrFile = $ErrFile
  }
}

function Stop-ProcessTree {
  param([int]$ProcessId)

  $taskkill = Get-Command "taskkill.exe" -ErrorAction SilentlyContinue
  if ($null -ne $taskkill) {
    & $taskkill.Source /PID $ProcessId /T /F | Out-Null
    return
  }

  try {
    $children = Get-CimInstance Win32_Process -Filter "ParentProcessId = $ProcessId" -ErrorAction Stop
  } catch {
    $children = @()
  }

  foreach ($child in $children) {
    Stop-ProcessTree -ProcessId $child.ProcessId
  }
  Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
}

function Write-NewLogLines {
  param(
    [string]$Path,
    [string]$Prefix
  )

  if (-not (Test-Path $Path)) {
    return
  }

  if (-not $script:logPositions.ContainsKey($Path)) {
    $script:logPositions[$Path] = [int64]0
  }

  $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
  try {
    [void]$stream.Seek([int64]$script:logPositions[$Path], [System.IO.SeekOrigin]::Begin)
    $reader = [System.IO.StreamReader]::new($stream)
    try {
      while (-not $reader.EndOfStream) {
        $line = $reader.ReadLine()
        if ($line -and $line.Trim().Length -gt 0) {
          Write-Host "[$Prefix] $line"
        }
      }
      $script:logPositions[$Path] = $stream.Position
    }
    finally {
      $reader.Dispose()
    }
  }
  finally {
    $stream.Dispose()
  }
}

$go = Get-RequiredCommand "go.exe" "请安装 Go，并确认 go.exe 已加入 PATH。"
$pnpm = Get-RequiredCommand "pnpm.cmd" "请安装 Node.js/Corepack，并确认 pnpm.cmd 已加入 PATH。"

New-Item -ItemType Directory -Force $ConfigDir | Out-Null
New-Item -ItemType Directory -Force $logDir | Out-Null
New-Item -ItemType Directory -Force $binDir | Out-Null

if (-not $SkipInstall -and -not (Test-Path (Join-Path $web "node_modules"))) {
  Write-Host "正在安装前端依赖..."
  Push-Location $web
  try {
    & $pnpm install
  }
  finally {
    Pop-Location
  }
}

$proxyTarget = "http://${HostName}:${BackendPort}"
$env:VITE_PROXY_TARGET = $proxyTarget
$env:VITE_DEV_PORT = [string]$FrontendPort
$env:VITE_DEV_HOST = $HostName

Write-Host "正在构建后端..."
Push-Location $root
try {
  & $go build -o $backendExe ./cmd/proxy-hub
}
finally {
  Pop-Location
}

$backend = Start-DevProcess `
  -Name "backend" `
  -FilePath $backendExe `
  -Arguments @("--config", $configPath, "--host", $HostName, "--port", [string]$BackendPort, "--no-browser") `
  -WorkingDirectory $root `
  -OutFile (Join-Path $logDir "backend.out.log") `
  -ErrFile (Join-Path $logDir "backend.err.log")

$frontend = Start-DevProcess `
  -Name "frontend" `
  -FilePath $pnpm `
  -Arguments @("dev") `
  -WorkingDirectory $web `
  -OutFile (Join-Path $logDir "frontend.out.log") `
  -ErrFile (Join-Path $logDir "frontend.err.log")

$processes = @($backend, $frontend)
$script:logPositions = @{}

Write-Host "本地联调模式已启动。"
Write-Host "控制台：http://${HostName}:${FrontendPort}"
Write-Host "后端 API：http://${HostName}:${BackendPort}"
Write-Host "配置文件：$configPath"
Write-Host "后端 PID：$($backend.Process.Id)"
Write-Host "前端 PID：$($frontend.Process.Id)"
Write-Host "按 Ctrl+C 停止后端和前端进程。"

try {
  while ($true) {
    foreach ($item in $processes) {
      Write-NewLogLines -Path $item.OutFile -Prefix $item.Name
      Write-NewLogLines -Path $item.ErrFile -Prefix "$($item.Name):err"

      if ($item.Process.HasExited) {
        $exitCode = $item.Process.ExitCode
        throw "$($item.Name) 进程已退出，退出码：$exitCode"
      }
    }
    Start-Sleep -Milliseconds 500
  }
}
finally {
  foreach ($item in $processes) {
    Write-NewLogLines -Path $item.OutFile -Prefix $item.Name
    Write-NewLogLines -Path $item.ErrFile -Prefix "$($item.Name):err"
    if (-not $item.Process.HasExited) {
      Stop-ProcessTree -ProcessId $item.Process.Id
    }
  }
  Write-Host "本地联调模式已停止。"
}
