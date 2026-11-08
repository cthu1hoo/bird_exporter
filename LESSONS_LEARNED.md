# LESSONS_LEARNED

Environment and tooling findings for this workspace (air-gapped sandbox, uid `pi` = 1001,
primary group `daemon`). Recorded 2026-11-08 during the BIRD/OSPF metrics research.

## Host capabilities (measured, not assumed)

- Shell runs in `.../pi-agent-bird_exporter-*.service` (a scope created for the agent); never
  stop or restart units under `user.slice` that this session did not create.
- `unshare -U true` -> 0 and `unshare -Urn ip link add ...` works: user and network namespaces
  are available without root.
- `podman` (rootless) and `docker` binaries exist; rootless podman runs containers and creates
  bridge networks fine. Pre-existing `sd-gallery_*` containers/networks are other people's work,
  leave them alone.
- No root. `sudo` exists but is not usable for these tasks; everything below runs unprivileged.

## Getting software without internet

- `apt-get download <pkg>` works: apt points at `https://dev-nexus.fakecake.org/repository/debian`
  (and `debian-security`, `trixie-backports`). No root needed for `download`.
- Extract a .deb without root: `dpkg-deb -x <pkg>.deb ./x`, then run `./x/usr/sbin/<bin>`.
  Useful for reading a program's `.rodata`: `strings -n 2 ./x/usr/sbin/bird` reveals exact CLI
  output format strings, which is the fastest way to learn a CLI's real grammar.
- `apt-cache policy <pkg>` shows available versions (e.g. `bird2` 2.17.5, `bird3` 3.1.7 and
  3.3.2 in backports).

## Sandbox kernel limitation (important, cost hours)

- IPv4 **multicast** is not delivered to local raw sockets in this environment.
  - Repro: two veth ends in one `unshare -Urn` netns, `IP_ADD_MEMBERSHIP` + `SOCK_RAW 89` on one
    side, raw send to `224.0.0.5` on the other -> receiver times out; `tcpdump -Z root` proves the
    frames cross the wire, `IpInReceives` increments, the socket still gets nothing.
    `ip route get 224.0.0.5` initially returns "Network is unreachable" until
    `ip route add 224.0.0.0/4 dev <if>` is added; even then local delivery fails.
  - IPv4 **unicast** raw delivery works (`SOCK_RAW 89` received a handcrafted proto-89 packet).
  - Same result inside rootless podman across containers on a bridge network.
  - Consequence: protocol adjacencies that depend on multicast hello packets (OSPF in broadcast,
    ptp or NBMA-multicast mode) never form in local labs. Real fixtures must come from live
    systems. Unicast-session protocols are fine.
- `tcpdump` needs `-Z root` inside a namespace (the `tcpdump` system user does not exist here).

## BIRD specifics (2.17.5, checked against 3.1.7)

- CLI flags: `-p` means "parse config and exit" (takes **no** argument); `-P <pidfile>` is the
  pid file. `-f` runs in the foreground.
- Config grammar gotchas: every statement needs `;` (`router id 1.1.1.1;`); OSPF interface
  keyword is `interface` (singular); NBMA neighbor list is `neighbors { 10.1.0.2; };`
  (`neighbor <ip>;` is a syntax error); `passive` is not accepted as an interface option in
  2.17.5 in either `passive;` or `passive yes;` form.
- OSPF show commands: the instance name goes **after** the subcommand, and the LSDB command is
  `lsadb` (not `lsdb`):

  ```
  show ospf [<proto>]                  # area counters (what bird_exporter uses today)
  show ospf state [<proto>]            # SPF state; "state all" = all routers
  show ospf topology [<proto>]         # SPF tree;   "topology all" = all routers
  show ospf interface [<proto>]        # per-interface state, cost, DR/BDR
  show ospf neighbors [<proto>]        # Router ID, Pri, State, DTime, Interface, Router IP
  show ospf lsadb [<proto>]            # Type, LS ID, Router, Sequence, Age, Checksum
  ```

  `show ospf <proto> state`, `show ospf lsdb`, `show ospf interfaces`, `show ospf counters` are
  syntax errors. `birdc "commands"` does not exist; there is no `?` help from a one-shot call.
- An unnamed protocol block gets an automatic instance name (`protocol ospf v2 { ... }` ->
  `ospf1`), which is what shows up in exporter labels.

## Container lab notes

- Copying a host binary into a stock `debian:trixie` image: also copy everything `ldd` resolves
  **except** `libc.so.6` and `ld-linux-x86-64.so.2`, or the binary dies with "cannot open shared
  object file".
- Single-file bind mounts (`-v ./x.conf:/etc/bird/bird.conf:ro`) pin the inode. Editing the host
  file with `sed -i` or an editor that rewrites the file leaves the container reading the old
  content: recreate the container instead of `podman restart`.

## Go toolchain for this repository (go.mod requires go 1.26.10)

- Host has only Go 1.24 (`/usr/lib/go-1.24`); `go build`/`go test` in the repo tries to download
  toolchain `go1.26.10` and fails: `proxy.golang.org` is blocked and the Nexus mirror answers
  `400 Bad Request` for `golang.org/toolchain`. `GOTOOLCHAIN=local` does not help because 1.24 is
  older than the `go` directive.
- Working recipe (matches the CI image in `.gitlab-ci.yml`):

  ```bash
  podman pull registry.fakecake.org/docker.io/library/golang:1.26
  podman run --rm -v "$PWD":/src -w /src \
    -e GOPROXY=https://nexus.fakecake.org/repository/go,direct \
    -e GOMODCACHE=/tmp/gomod -e GOCACHE=/tmp/gocache -e GOTOOLCHAIN=local \
    registry.fakecake.org/docker.io/library/golang:1.26 go test ./...
  ```

  Keep the caches inside the container so no root-owned files land in the working tree. The Nexus
  Go proxy serves project modules fine (first run downloads them).

## Git / project context

- The local checkout's `origin` is upstream `https://github.com/czerwonk/bird_exporter`, but an
  internal mirror exists on the GitLab instance: `git/bird_exporter` (project id **362**,
  default branch `master`, created 2026-11-08, currently at the same commit as local `main`,
  no `dev` branch, no open merge requests). Work should go to the mirror: plain HTTPS remote
  `https://gitlab.fakecake.org/git/bird_exporter.git`, `dev` branch plus merge request.
- An untracked `.gitlab-ci.yml` (GOPROXY `nexus.fakecake.org/repository/go`, golangci-lint,
  golang:1.26 image) appeared in the working tree during this session; it was not written by this
  agent. It is a copy of another project's pipeline, so the build stage was adapted to this repo:
  there is no `Makefile`, upstream releases use goreleaser. The build stage now runs
  `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=git-$CI_COMMIT_SHORT_SHA" -o bin/bird_exporter .`
  and the deploy stage uploads `bird_exporter-<sha>.tgz` to the Nexus raw repo.
- Useful read-only checks: `glab api "projects?search=bird_exporter"`,
  `glab api "projects/362/repository/branches"`, `glab mr list -R git/bird_exporter`.

## CI and Nexus deployment (verified 2026-11-08)

- The committed `.gitlab-ci.yml` works on the fork: `golangci-lint` and `gotest` in the lint stage,
  `build` producing `bin/bird_exporter`, `deploy` uploading to Nexus. MR !1 merged as `af8be80`,
  pipeline 72663 green, deploy job printed
  `uploading bird_exporter-af8be804.tgz to https://nexus.fakecake.org repo [MASKED]`.
- `INFRA_IMAGE`, `NEXUS_HOST`, `NEXUS_REPO`, `NEXUS_USER`, `NEXUS_PASSWORD` are inherited group CI/CD
  variables. The agent service account cannot read them (`GET /projects/362/variables` is 403) and
  `NEXUS_REPO` is masked in job traces, so the target repository is not visible from a job log.
- `deploy` runs only when `$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH`. A branch pipeline gives lint and
  build only, so a router-installable artifact needs a merge to `master`.
- The upload is `curl --upload-file` without `--fail`, so a rejected upload still exits 0: a green
  deploy job does not prove the asset exists. From this host the Nexus REST API and the browse
  endpoints are not reachable anonymously (all 404), so the upload could not be confirmed. Use
  `curl --fail --upload-file` when a silent failure is unacceptable.
- The binary is built with `CGO_ENABLED=0`, so it is static and can be copied straight onto the
  Debian routers; `bird_exporter -version` prints `git-<short-sha>`.

## Implementation lessons (OSPF topology metrics)

- Metric names: the exporter prefix already ends with an underscore. `prefix+"_ospf_link_up"`
  produces `bird__ospf_link_up`, which the Prometheus registry accepts silently, so the metric
  simply never shows up under the expected name. Follow the existing OSPF exporter and use
  `prefix+"ospf_link_up"`. Symptom to look for: the collector runs without errors but no series
  appear in the scrape output.
- `birdsocket.Query` returns the raw reply, reply codes included (`1002-ospf1:`, `1004-\t\t...`,
  terminal `0000`). Every new parser goes through `stripReplyCode` / `trimmedLine` in
  `parser/ospf_helpers.go`. One-shot `birdc "cmd"` pads columns with tabs, the interactive shell
  pads with spaces, so parsers split on whitespace runs and never on a fixed width.
- `show ospf interface` prints `Designated router (ID): 0.0.0.0` on a broadcast segment with no DR
  elected. `electedRouterID()` turns that into an empty string, so `bird_ospf_interface_dr_info`
  is absent until a DR exists instead of reporting `0.0.0.0`.
- Area labels differ by format and must not be joined blindly: the new metrics use the dotted quad
  (`area="0.0.0.0"`, exactly as BIRD prints it), the older `bird_ospf_*_count` metrics use `area="0"`.
- Cardinality is known and pinned by a test: 37 series per scrape at default OSPF flags, 221 with
  every group enabled, measured on the anonymised production capture (7 interfaces, 5 neighbors,
  30 LSAs, 30 routes).
- Pre-existing test trap: `go test ./parser/ -run IsoMsec` fails on unmodified upstream code, because
  `TestOSPFIsoMsecTimeFormat` depends on `time.Local = time.UTC` set by a test that runs earlier in
  the same binary. Run the whole package (`go test ./...`); do not chase this as a regression.
- Lint has to be installed and run inside one container run, `/tmp` does not survive between
  `podman run` calls:

  ```bash
  podman run --rm -v "$PWD":/src -w /src \
    -e GOPROXY=https://nexus.fakecake.org/repository/go,direct -e GOSUMDB=off \
    -e GOMODCACHE=/tmp/gomod -e GOCACHE=/tmp/gocache -e GOTOOLCHAIN=local -e GOBIN=/tmp/bin \
    registry.fakecake.org/docker.io/library/golang:1.26 \
    sh -c 'go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 && /tmp/bin/golangci-lint run --timeout=5m ./...'
  ```

- End to end smoke test that is worth repeating before any release: build to `.build/bird_exporter`,
  then start sandbox BIRD and the exporter inside one `unshare -Urn` namespace and scrape it
  (`/tmp/birdlab/e2e.sh`). This is the only check that exercises real socket bytes, since the unit
  tests work on recorded text. Files written by the container into the bind mount are owned by the
  `pi` user, so the build directory has to exist before the container starts.
