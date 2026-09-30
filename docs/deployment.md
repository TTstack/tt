# Deployment

Build the Go binaries with `make release`. The CLI retains the Rust default
release directory (`./target/release`) for compatibility; pass `--release-dir
./bin` for a Go build. Deployment installs binaries and services. It does not
install hypervisors, prepare images, provision ZFS or register agents.

## Local Linux with systemd

```sh
sudo ./bin/tt deploy all --release-dir ./bin
export TT_API_KEY="$(sudo cat /opt/ttstack/etc/api-key)"
/opt/ttstack/bin/tt config 127.0.0.1:9200
unset TT_API_KEY
/opt/ttstack/bin/tt host add 127.0.0.1:9100
```

When automating, wait for `tt status` to succeed after deployment and for the
prepared image to appear in `tt image list` before creating an environment.
Service activation and the periodic image catalog are separate readiness checks.

Use `deploy agent` or `deploy ctl` to install one role. The agent runs as root;
the controller runs as `ttstack`. Binaries go to `/opt/ttstack/bin`, state and
images to `/home/ttstack`, units to `/etc/systemd/system`. Existing role keys are
reused and persisted in `/opt/ttstack/etc/api-key` (0600). Conflicting role keys
cause an error; they are never silently replaced. Service environment files are
0600. Keep administrative keys out of commands, logs and version control.

The agent unit uses `KillMode=process` so restarting the service does not kill
its guests, and `TimeoutStopSec=360`. Deployment preserves existing
`RequiresMountsFor` and mountpoint `ExecStartPre` guards in
`tt-agent.service.d/90-ttstack-mounts.conf`. Put operator overrides in drop-ins.
A restart reconciles retained inventory; it does not automatically boot stopped
guests. `stop` retains disks but loses guest memory, and `delete` destroys disks.

## Distributed Linux deployment

Copy [deploy.toml.example](../tools/deploy.toml.example) into an untracked
`deploy.toml`, set addresses and resource budgets, then run:

```sh
./bin/tt deploy dist deploy.toml
```

`general.release_dir` and each agent's optional `release_dir` point to local
binaries built for that target. SSH must work with noninteractive `sudo`, including
root logins. The CLI checks paths, users, storage and budgets before remote work,
copies each binary to a private staging directory, and verifies SHA-256 before
installation. Setup scripts go through SSH stdin. Systemd, OpenRC and the Rust
manual fallback are retained. A timeout leaves the outcome unknown: inspect the
services before retrying.

The shared key is reused from `deploy.toml.api-key`, imported from an existing
service, or generated. An explicit `general.api_key` replaces that sidecar; keep
it private. `tt config ADDRESS` saves address-scoped credentials in `~/.ttconfig`
using an atomic 0600 replacement. `--server` only reuses the saved key when the
address exactly matches. `--api-key` or `TT_API_KEY` supplies an explicit key.

The API is plaintext HTTP with one shared bearer key. Expose it only through the
intended trusted network or an SSH tunnel. Agent CPU/memory defaults expose all
host capacity to scheduling; set explicit budgets on shared hosts. Fixed engine
paths and networking require full namespace/root isolation for parallel native
test fleets, as described in [roadmap.md](roadmap.md).

## Experimental FreeBSD

The deployment commands intentionally reject FreeBSD. Cross-build with
`CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o tt-agent ./cmd/tt-agent`, copy
binaries and start services manually with private data directories and the same
shared `TT_API_KEY`. Prepare `/home/ttstack/run`, authorized PF/networking and
eligible bhyve/Jail prerequisites first. These engines remain experimental;
consult the [validation records](validation/) for actual native coverage.
