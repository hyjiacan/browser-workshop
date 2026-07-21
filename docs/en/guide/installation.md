# Installation

This chapter introduces the installation methods for bws, including compiling from source and downloading pre-compiled binaries, as well as an explanation of portable mode.

## Compile from Source

If you have the Go environment installed, you can compile and install from source:

```bash
go build -o bws.exe .
```

After compilation, place the generated `bws.exe` in your desired directory to use it.

### Compilation Requirements

- Go 1.22 or higher
- Supports mainstream platforms including Windows, macOS, and Linux

## Download Pre-compiled Binaries

You can download pre-compiled binaries for the corresponding platform (Windows / macOS / Linux) from the Release pages below:

- **GitHub**: https://github.com/hyjiacan/browser-workshop/releases
- **Gitee**: https://gitee.com/hyjiacan/browser-workshop/releases

Pure Go implementation, no external tools required. Download the archive for your operating system and architecture, extract it, and place the binary in a suitable directory.

### Windows

Download `bws_windows_amd64.zip` or `bws_windows_386.zip`, and extract to obtain `bws.exe`.

### macOS

Download the version for your architecture, extract it, and grant execution permission:

```bash
chmod +x bws
```

### Linux

Download the version for your architecture, extract it, and grant execution permission:

```bash
chmod +x bws
```

## Portable Mode

bws defaults to portable mode, with all data stored in the `bws-data/` directory at the same level as the program.

### How It Works

Place `bws.exe` in any directory. The `bws-data/` folder will be automatically generated in the same directory on first run. All data (configuration, versions, cache, logs) is stored within it. The entire program along with its data can be copied to a USB drive or another computer for use.

### Directory Structure

```
bws/
├── bws.exe
└── bws-data/              # All data is here
    ├── config.json       # Configuration file
    ├── logs/             # Log directory
    ├── cache/            # Download cache
    ├── versions/         # Installed browser versions
    └── runtime/          # Runtime data (Profiles, etc.)
```

### Advantages of Portable Mode

- **Plug-and-play**: Copy the entire directory to use on another machine
- **Data isolation**: All data is under the program directory, no system pollution
- **Easy backup**: Simply back up the `bws-data/` directory to completely back up all configuration and data
- **USB-friendly**: Can be placed on a USB drive and carried around for use on different computers

### Custom Data Directory

If you do not want to use the `bws-data/` directory at the same level as the program, you can set a custom data directory via the configuration command:

```bash
bws cfg set data-dir D:\browser-data
```

After setting, all data (configuration, versions, cache, logs) will be stored in the specified directory. Clear this configuration to restore the default portable mode:

```bash
bws cfg set data-dir ""
```


