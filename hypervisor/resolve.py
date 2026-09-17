#!/usr/libexec/platform-python
"""Resolve Rocky-only packages without a host RPM transaction or host config."""
import json
import os
import pathlib
import sys
import dnf

root = pathlib.Path(sys.argv[1]).resolve()
source = pathlib.Path(__file__).resolve().parent
for name in ('cache', 'persist', 'logs', 'installroot'):
    (root / name).mkdir(parents=True, exist_ok=True)
with dnf.Base() as base:
    base.conf.installroot = str(root / 'installroot')
    base.conf.cachedir = str(root / 'cache')
    base.conf.persistdir = str(root / 'persist')
    base.conf.logdir = str(root / 'logs')
    base.conf.reposdir = []
    base.conf.install_weak_deps = False
    base.conf.module_platform_id = 'platform:el9'
    base.conf.substitutions['releasever'] = '9.8'
    base.conf.substitutions['basearch'] = 'x86_64'
    for name in ('BaseOS', 'AppStream'):
        repo = dnf.repo.Repo('rocky98-' + name.lower(), base.conf)
        repo.baseurl = ['https://download.rockylinux.org/pub/rocky/9.8/' + name + '/x86_64/os/']
        repo.gpgcheck = True
        repo.enable()
        base.repos.add(repo)
    base.fill_sack(load_system_repo=False)
    base.read_comps()
    base.group_install('core', ['mandatory', 'default'])
    requested = source.joinpath('packages.txt').read_text().split()
    for package in requested:
        base.install(package, strict=True)
    base.resolve()
    result = []
    for pkg in sorted(base.transaction.install_set, key=lambda p: (p.name, p.arch)):
        if pkg.arch not in ('x86_64', 'noarch'):
            raise RuntimeError('Unexpected architecture: ' + str(pkg))
        result.append({'name': pkg.name, 'nevra': str(pkg), 'repo': pkg.reponame,
                       'download_bytes': pkg.downloadsize, 'location': pkg.remote_location()})
    root.joinpath('resolved-packages.json').write_text(json.dumps(result, indent=2) + '\n')
    root.joinpath('resolved-packages.txt').write_text('\n'.join(p['nevra'] for p in result) + '\n')
    print('Resolved', len(result), 'Rocky 9.8 packages; download bytes:', sum(p['download_bytes'] for p in result))
    print('Requested:', ', '.join(requested))
    print('No RPM transaction executed; host RPM database was not loaded.')
