#!/usr/bin/env bash
set -euo pipefail

pipeline="${1:-.gitlab-ci.yml}"

# A credential in the remote URL is copied into Git process arguments and can
# surface in diagnostics. This guard intentionally checks the actual CI job.
if grep -E 'https?://[^[:space:]]*(GITHUB_PAT|x-access-token)' "$pipeline" >/dev/null; then
  echo 'GitHub publish credentials must not appear in a remote URL.' >&2
  exit 1
fi

grep -Fq 'export GITHUB_PAT' "$pipeline" || {
  echo 'GitHub publish must export the approved CI variable to Git.' >&2
  exit 1
}

grep -Fq 'helper="!${CI_PROJECT_DIR}/scripts/github-credential.sh"' "$pipeline" || {
  echo 'GitHub publish must use the scoped credential helper.' >&2
  exit 1
}

grep -Fq 'GH="https://github.com/arphost-com/Stack-Manager.git"' "$pipeline" || {
  echo 'GitHub publish must use the fixed, credential-free remote.' >&2
  exit 1
}

# Exercise Git's credential protocol with a harmless local fixture.
# Keep the credential output inside this process and check only the expected
# username/password fields; no network access or real secret is used.
response="$(GITHUB_PAT=nonsecret-fixture GIT_ASKPASS=/bin/false \
  GIT_TERMINAL_PROMPT=0 git -c credential.helper= \
  -c credential.helper="!$PWD/scripts/github-credential.sh" credential fill <<<'protocol=https
host=github.com
')"
[[ "$response" == *$'username=x-access-token'* ]]
[[ "$response" == *$'password=nonsecret-fixture'* ]]

if GITHUB_PAT=nonsecret-fixture scripts/github-credential.sh get <<<'protocol=http
host=github.com
' >/dev/null 2>&1; then
  echo 'Credential helper accepted an untrusted origin.' >&2
  exit 1
fi
