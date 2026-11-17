# Build security and updates

Actions use full commit SHAs. Dockerfile bases, its BuildKit frontend, CI database
services and the Trivy scanner use image digests. Docker references retain their
tags for readability and use multi-platform index digests to preserve architecture
selection. These pins constrain upstream replacements; they do not make the entire
build reproducible byte for byte. Alpine repositories and vulnerability databases
remain live, and runtime stages apply `apk upgrade` to pick up security fixes.

CI runs on pushes, pull requests and Mondays at 09:00 UTC. It audits all npm
dependencies, including build tools, before `npm ci` installs the locked packages.
`setup-node` supplies Node/npm and caches downloads, not `node_modules`; installation
is still required on each runner. There is no project postinstall build. A single
`go generate ./...` step generates all sources and assets before testing/building.
It also runs existing Go tests and govulncheck, then scans both the nginx runtime
and standalone Go image. Trivy receives exported image archives and temporary
storage, without a Docker socket, repository credentials or privileged mode.
Its database is refreshed as needed and shared between the two scans in one job.

Both npm audit and Trivy fail CI on HIGH/CRITICAL advisories. Trivy does not ignore
unfixed findings. Network/database failures also fail the check. These tools cover
known advisories; npm build dependencies can affect generated assets even though
Node/npm are absent from the runtime. Image scans cover installed runtime packages
and the Go binary. Build/test permissions remain `contents: read`; only the
tag-push publication job can write, after the build job succeeds.

## Weekly update review

Dependabot proposes weekly updates for GitHub Actions and Dockerfile bases. Review
the upstream release and SHA/digest changes, then require CI before merging.
There is no automatic merge. GitHub Actions updates preserve same-line version
comments beside the SHA pins.

Also review the BuildKit frontend, CI PostgreSQL/MySQL services and the scanner
reference manually each week; do not assume Dependabot discovers images embedded
in workflow shell commands or the Dockerfile syntax directive. Resolve the tag's
index digest with `docker buildx imagetools inspect IMAGE:TAG` and check the
published platforms before updating it. For Actions, verify the commit against
the official repository's release/tag. Keep Go toolchain changes aligned with
`go.mod` and verify compatibility when changing base versions.

After updates, build both Dockerfile targets, run `nginx -t`, existing tests and
the runtime scans. Use a fresh build (`docker build --no-cache ...`) when checking
Alpine security updates: a cached `RUN apk upgrade` does not query repositories
again. Investigate findings and update packages/bases rather than disabling the
gate. Any necessary exception needs a documented scope, reason and expiry.

References: [GitHub pinning guidance](https://docs.github.com/en/actions/reference/security/secure-use),
[Dependabot ecosystem support](https://docs.github.com/en/code-security/reference/supply-chain-security/supported-ecosystems-and-repositories),
[Trivy image scanning](https://trivy.dev/docs/latest/guide/target/container_image/).
