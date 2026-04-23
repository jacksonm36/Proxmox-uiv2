# Packaging templates

- **debian/** — copy the `debian` directory to the **repository root** (next to `go.mod`) as `debian/`, then fix paths in `debian/rules` so `go build` runs from the module root, and run `debuild` or `dpkg-buildpackage`.
- **rpm/cloudmanager.spec** — place the `cloudmanager-0.1.0` tarball layout so `%build` finds `cmd/`, `apps/web/`, and `deploy/` at the spec’s expected root, or adjust `cd` in `%build` / `%install`.

The shipped `debian/rules` in `packaging/debian/rules` is illustrative: adjust `$(cd …)` to your build tree.

Production packaging should add a non-root `cloudmanager` user, postinst to enable systemd units, and `debian/postinst` to run `systemd-tmpfiles --create` and `systemctl daemon-reload`.
