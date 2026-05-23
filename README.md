# props

Show the Windows **Properties** dialog for any file, directory, drive, or shell object — from the CLI.

---

## What it does

Exactly like right-clicking something in Explorer and choosing **Properties** — but from your terminal.

- Prompt returns **immediately** (like running a GUI app)  
- The Properties dialog **keeps running** on its own  
- No blocking, no waiting, no hooks on the dialog  
- No PowerShell, no WMIC — pure Win32 `ShellExecuteEx` via Go syscall  
- Supports **multiple paths** in one call (opens one dialog per path)

---

## Build

```bat
build.bat
```

Or with Make:

```bash
make build
```

Cross-compile from Linux/macOS:

```bash
make cross
# produces props_amd64.exe and props_x86.exe
```

---

## Usage

```
props <path> [path2 path3 ...]
```

### Examples

```bat
props C:\Windows
props myfile.txt
props C:\
props . ..
props C:\pagefile.sys C:\hiberfil.sys
```

---

## Install

Copy `props.exe` anywhere on your `PATH`, for example:

```bat
copy props.exe C:\Windows\System32\props.exe
```

Or use the Makefile:

```bat
make install
```

---

## License

MIT

## 👤 Author
        
[Hadi Cahyadi](mailto:cumulus13@gmail.com)
    

[![Buy Me a Coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/cumulus13)

[![Donate via Ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/cumulus13)
 
[Support me on Patreon](https://www.patreon.com/cumulus13)