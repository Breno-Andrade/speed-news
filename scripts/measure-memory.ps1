# Memory used by a running Speed News, per process group (Windows).
#
#   powershell -ExecutionPolicy Bypass -File scripts\measure-memory.ps1
#
# Private bytes are what each process alone costs (WebView2 shares the rest with Edge).
# Measure with the popover open, 2 minutes after closing it (webview destroyed), and after a day.
$ErrorActionPreference = "Stop"

$shellProcess = Get-CimInstance Win32_Process -Filter "Name = 'speed-news.exe'" | Select-Object -First 1
if (-not $shellProcess) {
    Write-Error "Speed News is not running"
}

$allProcesses = Get-CimInstance Win32_Process
function Get-Descendants([uint32]$parentId) {
    foreach ($childProcess in $allProcesses | Where-Object { $_.ParentProcessId -eq $parentId }) {
        $childProcess
        Get-Descendants $childProcess.ProcessId
    }
}

$groupBytes = [ordered]@{ "shell (Rust/Tauri)" = 0; "webview (WebView2)" = 0; "sidecar (Go news-api)" = 0; "other children" = 0 }
$groupBytes["shell (Rust/Tauri)"] = (Get-Process -Id $shellProcess.ProcessId).PrivateMemorySize64
foreach ($descendant in Get-Descendants $shellProcess.ProcessId) {
    $runningProcess = Get-Process -Id $descendant.ProcessId -ErrorAction SilentlyContinue
    if (-not $runningProcess) { continue }
    $groupName = switch -Wildcard ($descendant.Name) {
        "msedgewebview2*" { "webview (WebView2)" }
        "news-api*" { "sidecar (Go news-api)" }
        default { "other children" }
    }
    $groupBytes[$groupName] += $runningProcess.PrivateMemorySize64
}

$totalBytes = 0
"{0,-26} {1,12}" -f "group", "private (MiB)"
foreach ($groupName in $groupBytes.Keys) {
    $totalBytes += $groupBytes[$groupName]
    "{0,-26} {1,12:N1}" -f $groupName, ($groupBytes[$groupName] / 1MB)
}
"{0,-26} {1,12:N1}" -f "total", ($totalBytes / 1MB)
