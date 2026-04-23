Name:           cloudmanager
Version:        0.1.0
Release:        1%{?dist}
Summary:        Proxmox control plane (API, worker, web)

License:        ASL 2.0
URL:            https://example.com/cloudmanager
Source0:        %{name}-%{version}.tar.gz

BuildRequires:   golang >= 1.22
BuildRequires:  systemd-rpm-macros
Requires:       postgresql
Requires:       systemd

%description
Horizon-style web console and Terraform execution pipeline for Proxmox VE.
MSP-style multi-tenancy and audit; native systemd units.

%prep
%autosetup

%build
cd %{_builddir}/%{name}-%{version}
go build -trimpath -ldflags "-s -w" -o cloudmanager-api ./cmd/api
go build -trimpath -ldflags "-s -w" -o cloudmanager-worker ./cmd/worker
cd apps/web && (command -v npm && npm install && npm run build) || true

%install
install -d %{buildroot}%{_bindir} %{buildroot}%{_unitdir} %{buildroot}%{_prefix}/lib/tmpfiles.d
install -m755 cloudmanager-api cloudmanager-worker %{buildroot}%{_bindir}/
install -D -m644 deploy/systemd/cloudmanager-api.service %{buildroot}%{_unitdir}/
install -D -m644 deploy/systemd/cloudmanager-worker.service %{buildroot}%{_unitdir}/
install -D -m644 deploy/systemd/cloudmanager.tmpfiles.conf %{buildroot}%{_prefix}/lib/tmpfiles.d/cloudmanager.conf
install -d %{buildroot}%{_datadir}/cloudmanager/web
-cp -r apps/web/dist/* %{buildroot}%{_datadir}/cloudmanager/web/ 2>/dev/null || true

%files
%{_bindir}/cloudmanager-api
%{_bindir}/cloudmanager-worker
%{_unitdir}/cloudmanager-api.service
%{_unitdir}/cloudmanager-worker.service
%{_prefix}/lib/tmpfiles.d/cloudmanager.conf
%dir %{_datadir}/cloudmanager
%dir %{_datadir}/cloudmanager/web
%{_datadir}/cloudmanager/web/*

%changelog
* Wed Apr 22 2026  Cloudmanager 0.1.0-1
- Initial spec template
