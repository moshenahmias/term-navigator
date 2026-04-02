```
████████╗███████╗██████╗ ███╗   ███╗                                     
╚══██╔══╝██╔════╝██╔══██╗████╗ ████║                                     
   ██║   █████╗  ██████╔╝██╔████╔██║                                     
   ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║                                     
   ██║   ███████╗██║  ██║██║ ╚═╝ ██║                                     
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝                                     
                                                                         
███╗   ██╗ █████╗ ██╗   ██╗██╗ ██████╗  █████╗ ████████╗ ██████╗ ██████╗ 
████╗  ██║██╔══██╗██║   ██║██║██╔════╝ ██╔══██╗╚══██╔══╝██╔═══██╗██╔══██╗
██╔██╗ ██║███████║██║   ██║██║██║  ███╗███████║   ██║   ██║   ██║██████╔╝
██║╚██╗██║██╔══██║╚██╗ ██╔╝██║██║   ██║██╔══██║   ██║   ██║   ██║██╔══██╗
██║ ╚████║██║  ██║ ╚████╔╝ ██║╚██████╔╝██║  ██║   ██║   ╚██████╔╝██║  ██║
╚═╝  ╚═══╝╚═╝  ╚═╝  ╚═══╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝
```

Term Navigator is a dual-pane terminal file manager for local filesystems and
S3-compatible storage. It provides fast navigation, viewing, editing, copying,
moving, deleting, and metadata inspection across multiple devices.

---

## Features

- Dual-pane interface (left and right)
- Local filesystem backend
- S3 backend (AWS or any S3-compatible service)
- Copy and move files across devices
- In-memory virtual filesystem

---

## Installation

Clone the repository:

```
git clone https://github.com/moshenahmias/term-navigator.git
cd term-navigator
```

Build:

```
go build ./cmd/termnav
```

Run:

```
./termnav
```

---

## Configuration

Term Navigator loads configuration from:

```
~/.termnav
```

Example configuration:

```json
{
  "devices": [
    { "name": "local", "type": "local", "path": "/home/user" },
    {
      "name": "minio",
      "type": "s3",
      "bucket": "mybucket",
      "region": "us-east-1",
      "endpoint": "http://localhost:9000",
      "key": "minioadmin",
      "secret": "minioadmin",
      "prefix": "projects/"
    }
  ],
  "left": "local",
  "right": "minio"
}
```

Supported device types:

- `local`
- `s3`

---

## Keybindings

```
F1     Help
F2     Rename
F3     View (less)
F4     Edit (vim)
F5     Copy to opposite pane
F6     Move to opposite pane
F7     Create directory
F8     Delete
F9     Metadata
F10    Change device
F12    Swap panes
Tab    Switch active pane
Enter  Open directory or file
Backspace  Go to parent directory
ESC    Cancel input mode
```

---

## Command Mode

Press `:` to enter command mode. Autocomplete is available.

Commands:

```
help
rename <old> <new>
view <file>
edit <file>
copy <src> <dest>
move <src> <dest>
mkdir <name>
delete <name>
info <file>
device <name>
swap
exit
config
exec <command>
refresh
cd <folder>
shell
```

Aliases:

```
cp   -> copy
mv   -> move
del  -> delete
dev  -> device
cfg  -> config
quit -> exit
```