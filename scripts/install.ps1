# Installs fdev on Windows, or updates it: the newest stable release for
# this CPU, checked against its checksums, into %LOCALAPPDATA%\Programs\fdev,
# which goes on your PATH. Needs nothing but PowerShell, which Windows has.
#
#   irm https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.ps1 | iex
#
# From cmd.exe:
#
#   powershell -ExecutionPolicy Bypass -c "irm https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.ps1 | iex"
#
# $env:FDEV_CHANNEL = 'beta' takes the newest beta too, $env:FDEV_VERSION =
# 'v0.2.0' a version, $env:FDEV_INSTALL_DIR the folder. Until there is a
# stable release, it installs the newest beta, and says so. A token
# ($env:GH_TOKEN, or the GitHub CLI logged in) raises GitHub's rate limit.

$DefaultRepo = 'NaderMozaffari/fdev'

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # Windows PowerShell downloads slowly with it
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = if ($env:FDEV_REPO) { $env:FDEV_REPO } else { $DefaultRepo }
    $dir = if ($env:FDEV_INSTALL_DIR) { $env:FDEV_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\fdev' }
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }
    $asset = "fdev_windows_$arch.zip"

    $token = @($env:FDEV_GITHUB_TOKEN, $env:GH_TOKEN, $env:GITHUB_TOKEN) | Where-Object { $_ } | Select-Object -First 1
    if (-not $token -and (Get-Command gh -ErrorAction SilentlyContinue)) {
        $token = (& gh auth token 2>$null | Out-String).Trim()
    }
    $headers = @{ 'X-GitHub-Api-Version' = '2022-11-28' }
    if ($token) { $headers['Authorization'] = "Bearer $token" }

    $api = "https://api.github.com/repos/$repo/releases"
    $beta = $env:FDEV_CHANNEL -eq 'beta'
    $tag = $env:FDEV_VERSION
    if ($tag -and -not $tag.StartsWith('v')) { $tag = "v$tag" }
    try {
        if ($tag) {
            $release = Invoke-RestMethod -UseBasicParsing -Headers $headers "$api/tags/$tag"
        } else {
            # Not in @(): Windows PowerShell would make the list one item.
            $list = Invoke-RestMethod -UseBasicParsing -Headers $headers "$api`?per_page=100"
        }
    } catch {
        $what = if ($tag) { "release $tag" } else { 'releases' }
        throw "fdev: can't read the $what of $repo ($($_.Exception.Message)). Is github.com reachable?"
    }
    if (-not $tag) {
        # GitHub lists releases newest first; a beta is a pre-release (and
        # its version has a -beta.1 or the like).
        $all = @($list | Where-Object { -not $_.draft })
        $release = if ($beta) { $all | Select-Object -First 1 } else {
            $all | Where-Object { -not $_.prerelease -and $_.tag_name -notmatch '-' } | Select-Object -First 1
        }
        if (-not $release -and -not $beta) {
            $release = $all | Select-Object -First 1
            if ($release) { Write-Host "fdev: there is no stable release yet, so this is the newest beta: $($release.tag_name)" -ForegroundColor Yellow }
        }
        if (-not $release) { throw "fdev: $repo has no releases yet" }
    }
    $zip = $release.assets | Where-Object name -eq $asset | Select-Object -First 1
    $sums = $release.assets | Where-Object name -eq 'checksums.txt' | Select-Object -First 1
    if (-not $zip -or -not $sums) { throw "fdev: release $($release.tag_name) has no $asset" }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("fdev-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        $note = if ($release.tag_name -match '-') { ', a beta: a pre-release, it may have bugs' } else { '' }
        Write-Host "downloading fdev $($release.tag_name) ($asset)$note"
        $get = $headers.Clone()
        $get['Accept'] = 'application/octet-stream'
        Invoke-WebRequest -UseBasicParsing -Headers $get $zip.url -OutFile (Join-Path $tmp $asset)
        Invoke-WebRequest -UseBasicParsing -Headers $get $sums.url -OutFile (Join-Path $tmp 'checksums.txt')

        $want = Get-Content (Join-Path $tmp 'checksums.txt') |
            ForEach-Object { $f = $_ -split '\s+'; if ($f[1] -eq $asset -or $f[1] -eq "*$asset") { $f[0] } } |
            Select-Object -First 1
        $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash
        if (-not $want -or $want -ne $got) { throw "fdev: checksum mismatch for $asset" }

        Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath (Join-Path $tmp 'x') -Force
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        $exe = Join-Path $dir 'fdev.exe'
        if (Test-Path $exe) {
            # A running fdev.exe can't be overwritten, but it can be moved aside.
            Remove-Item "$exe.old" -Force -ErrorAction SilentlyContinue
            Move-Item $exe "$exe.old" -Force
        }
        Copy-Item (Join-Path $tmp 'x\fdev.exe') $exe -Force
        Remove-Item "$exe.old" -Force -ErrorAction SilentlyContinue
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not (($userPath -split ';') -contains $dir)) {
        [Environment]::SetEnvironmentVariable('Path', (@($userPath, $dir) | Where-Object { $_ }) -join ';', 'User')
        Write-Host "added $dir to your PATH (open a new terminal to use it)"
    }
    if (-not (($env:Path -split ';') -contains $dir)) { $env:Path += ";$dir" }

    Write-Host "installed $(& $exe version) in $dir"
}
