# Ele File Lister

A lightbar file lister door for [EleBBS](https://github.com/mbek/elebbs) (and other RemoteAccess 2.5x-style boards). It shows the caller's current file area in two panes, lets them tag files for download, view archives, and scan for new files, keywords, or wildcards.

It talks to the caller directly over the inherited socket from `door32.sys`, with full CP437 / ANSI color. No FOSSIL driver is needed for the lister itself.

## Screen

- **Left pane (30 columns):** lightbar list of filenames. Tagged files show a `√` check. Names too long to fit end with `>`.
- **Right pane (45 columns):** the selected file's description.
- **Bottom right:** file size, download count, file date, and last download date, plus a running count and total size of tagged files.

The screen is 79×24.

## Keys

| Key | Action |
|---|---|
| Space | Tag the file and move to the next one, or untag it |
| Tab | Show up to 75 characters of the filename; any key restores the list |
| Enter | View the selected file with the command in `filelist.ini` |
| N | List files from the last *n* days |
| K | Search names and descriptions for a keyword |
| W | Search filenames with `*` and `?` wildcards |
| Up / Down | Previous / next file |
| Left / Right, PgUp / PgDn | Previous / next page |
| Home / End | First / last file |
| Q / Esc | Quit, or return from a scan to the area list |
| ? | Help |

After N, K, or W, choose the scope: **A** = this area, **G** = this group, **L** = all groups. Only areas the caller has list access to are searched.

## Install

1. Download a binary from [`releases/`](releases/):

   | File | Target |
   |---|---|
   | `filelist-win32.exe` | Windows 32-bit |
   | `filelist-linux-386` | Linux 32-bit |
   | `filelist-linux-amd64` | Linux 64-bit |
   | `filelist-linux-arm64` | Linux ARM 64-bit |

2. Put it and `filelist.ini` in a directory of your choice.
3. Add a menu entry that runs it as a door from the **node directory**, with `door32.sys` written there. No parameters are needed.

The door finds the system directory from the `RA` environment variable (or `RABBS`, `ELE`, `ELEBBS`), then the node directory and its parents, then its own directory.

## filelist.ini

The first non-comment line is the view command, run when the caller presses **V**:

```
c:\ele\xview\xview @ *N
```

| Token | Replaced with |
|---|---|
| `@` | Full path and filename of the selected file (no quotes) |
| `*N` | Node number |
| `*H` | Socket handle from `door32.sys` |

The command runs with the node directory as its working directory.

For a DOS viewer under NetFoss, point the command at `nf.bat`. The door adds `/N<node> /H<handle>` and opens a real console for it:

```
c:\ele\nf\nf.bat c:\ele\xview\xview.exe @ /N*N
```

Optional `key=value` lines after the first line:

| Key | Meaning |
|---|---|
| `syspath` | EleBBS system directory, if `RA` isn't set |

Lines starting with `#` or `;` are comments.

## Files used

**Read:**
- From the node directory: `door32.sys`, `EXITINFO.BBS`, and `DOOR.SYS`.
- From the system directory: `USERS.BBS`, `CONFIG.RA`, `FILES.RA`, and `FGROUPS.RA`.
- From the file base: `hdr\fdbN.hdr` and `txt\fdbN.txt` for each area.

The current area and group come from `EXITINFO.BBS`, falling back to `USERS.BBS`.

**Written:** `taglist.ra` in the node directory, in the EleBBS `TagFileRecord` format.

Each record holds the 12-character FDB name plus the area and record number. EleBBS uses those to look up the long filename at download time.

## Command line

| Flag | Meaning |
|---|---|
| `-local` | Use the local console instead of the socket (for testing) |
| `-sys <path>` | EleBBS system directory |

## Building

Requires Go 1.24 or later.

```
build.bat
```

That builds `filelist.exe` (native), `filelist32.exe` (Win32), and all four `releases/` binaries. To build one target by hand:

```
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o releases\filelist-linux-arm64 .
```

Run the tests with `go test ./...`.

## Notes

- The inactivity limit is taken from `UserTimeOut` in `CONFIG.RA`, the same setting EleBBS uses. Callers are warned 30 seconds before the limit and hung up when it runs out. A limit of 0, or `-local` mode, turns this off. Time spent in the viewer doesn't count.
- The Windows build is the one in use on a live board. The Linux builds compile and pass tests but haven't been run on a real board yet.
- On Linux, the `door32.sys` socket handle is treated as an inherited file descriptor, and the view command runs through `/bin/sh`. NetFoss doesn't apply.
- The door never closes the caller's socket, so the BBS keeps the connection when it exits.

## License

Copyright (C) 2026 Martin Kazmaier. Distributed under the [Q Public License version 1.0](LICENSE), the same license as EleBBS.

If you distribute the binaries, include the `LICENSE` file and point recipients to the source at <https://github.com/martykazmaier/filelist>.
