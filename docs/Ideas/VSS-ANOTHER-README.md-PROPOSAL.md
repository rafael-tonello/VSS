## VSS — VarServerSHU variable server

VSS (binary name: `vss`) is the Variable Server for SHU-based systems. It exposes a variable storage and access layer via a VSTP socket API and an HTTP/HTTPS JSON API. VSS is intended to store and serve structured variables for home automation and related systems.

### Key features

- VSTP binary protocol server (default port 5032) for programmatic clients.
- HTTP JSON API (default ports 5024 HTTP / 5025 HTTPS) for integrations, dashboards and tools.
- Pluggable storage backends (default: RamCacheDB in-memory with periodic disk dump; optional file-based storage implementations available in packages).
- Portable mode: run entirely from the application folder (configs, logs and data kept next to the binary).
- TLS support for the HTTP API (use your certificate and key).
- Simple configuration system: command-line args, environment variables and conf file providers are supported and merged.

## Where to get it

- Build from source (recommended when developing): the project uses the `shu` meta-tool in the repository root. From the `VarServerSHU` folder run:

```bash
shu build         # production build
shu build --debug # debug build with symbols
```

The produced binary name is `vss` and will be available in `build/` after a successful build.

- Prebuilt binary: if you already have a `vss` binary (for example in `build/vss`), copy it to your target machine and run it directly.

## Prerequisites / System dependencies

The project expects the following commands/libraries to be present on the build machine (these are also listed in `shu.yaml`):

- `g++` or `clang++` (C++20/2a support)
- OpenSSL development headers (e.g. `libssl-dev` on Debian/Ubuntu)
- `curl` (used by some dev tools)

Install on Debian/Ubuntu:

```bash
sudo apt update
sudo apt install build-essential g++ libssl-dev curl
```

## Configuration and defaults

VSS loads configuration from three providers (merged, in order of precedence):

1. Command line arguments
2. Environment variables
3. Configuration file (`/etc/vss/confs.conf`), or `confs.conf` in the application directory when running in portable mode

Important defaults:

- Configuration file: `/etc/vss/confs.conf` (or `<app-dir>/confs.conf` in portable mode)
````markdown
## VSS — VarServerSHU variable server

VSS (binary name: `vss`) is the Variable Server: a small key-value database and runtime used by SHU-based systems. It stores variables (key → value) and also treats variables as observable streams — clients can subscribe and receive notifications when a variable changes. VSS exposes a VSTP socket API and an HTTP API (JSON and text/plain) for clients and integrations. It is intended for home automation and other systems that need a low-latency variable store with change notifications.

### Key features

- Key-value database: store and retrieve variables as simple key/value pairs.
- Observable variables: clients can subscribe to keys and receive notifications (variable values behave as streams).
- VSTP: a simple text-based protocol that uses a single connection per client (default port 5032).
- HTTP API: supports JSON and text/plain responses (default HTTP port 5024) for integrations, dashboards and scripts.
- Portable mode: run from the application folder to keep configs, logs and data next to the binary.
- Simple configuration system: configuration can be provided via command-line arguments, environment variables and a configuration file (see precedence below).

## Where to get it

- Build from source (recommended when developing): the repository includes helper scripts (in `shu/`) that automate build steps, but `shu` is only a helper tool and not part of `vss`. You can use the provided helpers or call your toolchain directly. After building the produced binary is `vss` and typically appears in `build/`.

- Prebuilt binary: if you already have a `vss` binary (for example in `build/vss`), copy it to your target machine and run it directly.

## Prerequisites / System dependencies

The project expects a C++ toolchain and necessary libraries on the build machine. See `shu.yaml` for a list of helpful system dependency checks. Typical dependencies:

- `g++` or `clang++` (C++20/2a support)
- `curl` (used by some dev tools)

On Debian/Ubuntu install common packages:

```bash
sudo apt update
sudo apt install build-essential g++ curl
```

## Configuration and defaults

VSS reads configuration from three sources which are merged with this precedence (highest → lowest):

1. Command-line arguments (highest priority)
2. Environment variables
3. Configuration file (lowest priority) — `/etc/vss/confs.conf` by default, or `confs.conf` in the application directory when running in portable mode

Command-line arguments override environment variables, and both override values in the configuration file.

Important defaults (examples):

- Configuration file: `/etc/vss/confs.conf` (or `<app-dir>/confs.conf` in portable mode)
- Data directory: `/var/vss/data` (or `<app-dir>/data` in portable mode)
  - Database folder: `/var/vss/data/database`
  - HTTP data folder: `/var/vss/data/http_data`
- Log file: `/var/log/vss.log` (or `<app-dir>/vss.log` in portable mode)
- HTTP API port: 5024 (env: `VSS_HTTP_API_PORT` or `--httpApiPort`)
- VSTP port: 5032 (env: `VSS_VSTP_API_PORT` or `--vstpApiPort`)
- Max log file size before rotation: 52428800 bytes (50 MiB) (`--maxLogFileSize`)

See `sources/main.cpp` for the full set of configuration aliases, defaults and CLI options.

## Command-line usage

Run `vss --help` to list all supported CLI flags and environment variable equivalents. A few common examples:

```bash
./vss --version
./vss --httpApiPort 8080 --vstpApiPort 9000
```

Environment variable examples:

```bash
export VSS_HTTP_API_PORT=8080
./vss
```

Portable mode

If a `confs.conf` file is present next to the `vss` binary, VSS will run in portable mode and use the application directory for logs/data/configs. This is useful for testing or running from an unpacked archive.

## Running as a service (systemd example)

Below is a minimal `systemd` unit that runs `vss` as a service. Adjust paths and user/group as needed.

```ini
[Unit]
Description=VSS - VarServerSHU variable server
After=network.target

[Service]
Type=simple
User=vss
Group=vss
WorkingDirectory=/opt/vss
ExecStart=/opt/vss/vss
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

After creating the unit file:

```bash
sudo systemctl daemon-reload
sudo systemctl enable vss
sudo systemctl start vss
sudo journalctl -u vss -f
```

## Protocols and client access

- VSTP: a simple text-based protocol which uses a single TCP connection per client. It is intended for programmatic clients and supports commands to read/write keys and to subscribe/unsubscribe to changes. The default port is 5032.
- HTTP: the HTTP API supports JSON responses for structured data and `text/plain` for simple endpoints or payloads; choose the representation that suits your integration. The default HTTP port is 5024.

## Storage

VSS is a key-value database and its primary behavior is to store keys and values and to allow clients to subscribe to updates. The default development storage is an in-memory cache with optional periodic disk persistence. Configuration controls storage paths and persistence behavior; consult `sources/` for implementation details.

## Logs and debugging

- Log file: `/var/log/vss.log` (or `<app-dir>/vss.log` in portable mode).
- Logging rotates when it exceeds the configured `maxLogFileSize`.

Debugging tips:

- Use `./vss --help` to validate runtime flags.
- Check the built `vss` binary is present in `build/` after building.
- Run under `valgrind` if you suspect memory issues:

```bash
valgrind ./vss
```

- Start `vss` in the foreground to see console logs.

## Troubleshooting checklist

- Permission errors: verify `vss` can write to `/var/vss/data` and `/var/log/vss.log`, or run in portable mode.
- Port conflicts: ensure configured ports are free or change them via CLI/env/file.
- Missing dependencies: ensure a C++ compiler/toolchain is installed on build machine.

## Testing

There is a `tests` folder in the repo. To build tests use the project's test build command or your build steps:

```bash
shu build-tests
# or run the test binary if present
./tests/build/tests
```

## Contributing

- Follow the existing coding style and build using the provided helpers if you like.
- Add tests under `tests/` for new features or bug fixes.
- Update `CHANGELOG.md` and the top-level `README.md` when adding new user-visible features.

## Where to read more

- Project docs folder: `docs/`
- Main sources: `sources/` (entry point: `sources/main.cpp`)
- Build and helper scripts: `shu/` and `shu/pcommands/`

## Contact and license

Check the repository root for license information (`LICENSE` or `README.md`). For questions, open an issue in the repository.

---

This document summarizes how to obtain, configure and run the `vss` binary and where to look for sources and configuration. If you'd like more examples (API endpoint examples, sample `confs.conf` or an init script) tell me which you'd prefer and I will add them.

````
