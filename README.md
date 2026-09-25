# Holdotfiles

Back up and restore dotfiles as ZIP archives using Cloudflare R2, with a terminal UI and CLI.

**Project name: Holdotfiles. Command: `hdt`.** Run `hdt` without arguments to open the TUI.

## Quick install

Requirements: Linux, `curl`, `tar`, and Go 1.24.1 or later. The installer downloads
the source code and Go dependencies, so an internet connection is required. Git
and `sudo` are not required.

```sh
curl -fsSL https://raw.githubusercontent.com/lunebakami/holdotfiles-go/main/install.sh | sh
```

The installer builds the version published on GitHub and installs it to
`~/.local/bin/hdt`. It creates configuration templates without overwriting
existing files. Run the same command again to update. You can also clone the
repository and run `sh install.sh` to build local changes.

If `~/.local/bin` is not on your `PATH`, add this to `~/.zshrc`:

```zsh
export PATH="$HOME/.local/bin:$PATH"
```

Alternatives: `make install` uses the same installer; `sh install.sh --prefix
/absolute/path` installs to `/absolute/path/bin/hdt`. Go users can run
`go install ./cmd/hdt` from a checkout; this does not create configuration templates.

## Features

- Reads files and directories listed in `~/.hdtconfig`.
- Recursively includes directory contents.
- Creates a ZIP whose root represents the home directory.
- Uploads each backup as a separate object named
  `<computer>/backup-YYYY-MM-DDTHH-MM-SS.nanosZ.zip` in R2.
- Calculates a SHA-256 digest and verifies archive integrity during restore.
- Separates computers by a prefix (the hostname by default).
- Lets you cancel an active sync with `x`.

Restore is available in the TUI and CLI. It previews changes and saves replaced
files in a recovery folder before installing. In the TUI, press `r` to load
versions, use the arrow keys to select one, and press Enter to preview. In the
CLI, `hdt --list` displays each object key; use
`hdt --restore 'hostname/backup-DATE.zip'` to select a specific version, or
`hdt --restore hostname` to select the latest. Every upload creates a new
version, and older versions are not deleted automatically. The legacy
`hostname/backup.zip` object remains available for restore.

## Configuration

1. Create a Standard bucket in the Cloudflare R2 dashboard.
2. Create an API token limited to that bucket with Object Read & Write access.
3. Add your credentials to `~/.config/holdotfiles/.env` (created by the
   installer). With `XDG_CONFIG_HOME`, use
   `$XDG_CONFIG_HOME/holdotfiles/.env` instead.
4. Create `~/.hdtconfig` with one path per line:

```text
# Individual files
~/.zshrc
~/.gitconfig

# Directories are included recursively
~/.config/ghostty
```

Blank lines, comments starting with `#`, and duplicate paths are ignored.

Example credentials:

```dotenv
R2_ACCOUNT_ID=your_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret_key
R2_BUCKET=holdotfiles
# Optional; defaults to the computer's hostname:
R2_PREFIX=my-computer
```

Set `R2_ENDPOINT` to use a custom endpoint. Set `HOLDOTFILES_CONFIG` to use a
different paths file. Environment variables take precedence over the `.env` file
in the current directory, which takes precedence over the global `.env` file.
This lets you run `hdt` from any directory. The local `.env` remains useful for
development. The installer never copies project credentials.

To migrate from an earlier installation, move the values from the local `.env`
to the global file. Do not include credential files in your backup. The ZIP is
not encrypted; keep the R2 bucket private.

## Usage

```sh
make test
make build
./bin/hdt
```

In the TUI, use `Tab` to switch screens, `s` to sync, `x` to cancel, and `q` to
quit. Press `r` to open or refresh the **Restore backup** screen. It shows each
computer, the upload time in your local timezone, and archive size, with the
newest versions first. Use `↑`/`↓` to select a version and Enter to download,
verify, and preview it. Press `i` to install or `Esc` to go back. The arrow keys
scroll through preview entries. The recovery folder is shown when installation
finishes.

You can open the TUI and restore backups without `~/.hdtconfig`; uploading
remains unavailable until you configure the paths and restart the program. The
`--dest` argument also sets the restore destination in the TUI.

The version date comes from the R2 object's LastModified field. New object names
use UTC timestamps; the TUI displays local time. Versions are never deleted
automatically. The legacy `<computer>/backup.zip` object remains restorable.
`hdt --list` displays full object keys. Use
`hdt --restore 'computer/backup-DATE.zip'` for a specific version, or
`hdt --restore computer` for the latest. Keeping more versions uses more R2 storage.

## Archive layout

For the configured paths `~/.config/nvim` and `~/kitty.conf`, the ZIP contains:

```text
backup.zip
├── .config/
│   └── nvim/
│       └── init.lua
└── kitty.conf
```

The prefix is the hostname or `R2_PREFIX`. The ZIP root always represents `~`,
without the source username. Empty directories are preserved. Symlinks to files
are followed: the target contents are stored as a regular file at the link's
original path in the ZIP. This includes targets outside `~`, while the
configured link path itself must be inside `~`. Restore does not recreate the
symlink or track later changes to its target.

Paths outside `~`, broken symlinks, symlinks to directories, and special files
are rejected. Symlinks in the restore destination are also validated. If any
source path fails, no partial ZIP is uploaded.

## Backup and restore from the CLI

```sh
# Upload without opening the TUI
./bin/hdt --backup

# List available versions
./bin/hdt --list

# Download and preview without installing
./bin/hdt --restore hostname/backup-DATE.zip

# Install to the current home directory
./bin/hdt --restore hostname/backup-DATE.zip --apply

# Install to another directory
./bin/hdt --restore hostname/backup-DATE.zip --dest /tmp/dotfiles-demo --apply
```

Listing and restoring do not require `~/.hdtconfig` on the destination machine,
but R2 credentials are required. Restore verifies SHA-256, validates paths, and
extracts the entire ZIP to a staging area before installation. Entries that
escape the destination and destinations containing symlinks are rejected.
Replaced files are preserved under `.holdotfiles-recovery-*` inside the
destination, with the same directory layout. The recovery path is printed even
if installation only partially succeeds. Use `--recover` to restore files from
that copy. Newly created files have no previous copy. Installation is not
transactional: there is no automatic rollback of the whole set, and extra local
files are not removed. File permissions are preserved; new directories use
private permissions.

Downloads and uncompressed archive contents are limited to 1 GiB.

## Local recovery

```sh
# List recovery copies in the home directory
./bin/hdt --recoveries

# Preview a recovery
./bin/hdt --recover .holdotfiles-recovery-123

# Restore the previous files
./bin/hdt --recover .holdotfiles-recovery-123 --apply
```

Replace the example directory with a name shown by `--recoveries`. If the
original install used `--dest`, pass the same destination to both commands:

```sh
./bin/hdt --recoveries --dest /tmp/dotfiles-demo
./bin/hdt --recover .holdotfiles-recovery-123 --dest /tmp/dotfiles-demo --apply
```

This works offline and does not require `.env` or `~/.hdtconfig`. The selected
recovery copy remains intact. Current files that are replaced are saved to a
new recovery folder, so you can undo the recovery using the same command. Only
files present in the recovery copy are restored; new or extra files remain in
the destination. Empty recovery folders produce a message without changing files.

## Tests

```sh
go test -race ./...
go vet ./...
# Opt-in: uses .env, uploads synthetic data only, then deletes the remote object.
HOLDOTFILES_LIVE_TEST=1 go test ./internal/storage -run '^TestR2Live$' -v -count=1
```
