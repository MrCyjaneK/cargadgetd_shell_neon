#!/bin/sh
# ftuna publishes the current Go toolchain as the unversioned golang RPM.
# This hook only adds the repo. The spec BuildRequires golang with no
# version, and sfosbuild installs BuildRequires on every build.
set -eu

repo="https://repo-oss.ftuna.eu/${SFOS_VERSION}/${SFOS_ARCH}/"
# repodata/ ships repomd.xml.asc and no repomd.xml.key, so auto-import
# has nothing to fetch until the repo file names the key.
key="https://repo-oss.ftuna.eu/ftuna-oss.key"
zypper ar -f "$repo" ftuna-oss
printf 'gpgkey=%s\n' "$key" >> /etc/zypp/repos.d/ftuna-oss.repo
zypper --non-interactive --gpg-auto-import-keys refresh ftuna-oss
