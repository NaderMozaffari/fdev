The fdev release archive the installer carries, so it installs without the
internet: `fdev_darwin_<arch>.tar.gz` or `fdev_windows_amd64.zip`, the same
files a release holds. `scripts/installer.sh` puts the right one here before
each build and takes it away after; a build without one installs only what
it can download.
