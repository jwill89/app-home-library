#!/usr/bin/env bash
#
# Auto-tag + GitHub Release when the current version has no release yet. Run by
# the CI "release" job on a push to the default branch, AFTER the gate jobs pass
# — so only green commits are ever released.
#
# The tag is v<version> (from web/package.json) and the release body is the
# matching "## [<version>]" section of CHANGELOG.md, so a green version bump
# auto-publishes a release linked to its notes. Idempotent: a version already
# released is skipped, so ordinary pushes are a no-op. When the git tag already
# exists (e.g. created by hand) but has no release, the release is attached to it;
# otherwise the tag is created at the pushed commit.
set -euo pipefail

version="$(jq -r '.version // empty' web/package.json || true)"
if [ -z "$version" ]; then
  echo "!! Could not read a version from web/package.json — nothing to release" >&2
  exit 0
fi
tag="v${version}"

if gh release view "$tag" >/dev/null 2>&1; then
  echo "== ${tag}: release already exists — skipping"
  exit 0
fi

# The CHANGELOG "## [<version>]" section, up to the next "## " heading. index()
# is a literal (non-regex) substring match anchored at column 1, so the version
# dots are literal and the date separator after the bracket doesn't matter.
notes_file="$(mktemp)"
awk -v ver="## [${version}]" '
  index($0, ver)==1 { cap=1; print; next }
  cap && /^## / { cap=0 }
  cap { print }
' CHANGELOG.md > "$notes_file"
if [ ! -s "$notes_file" ]; then
  printf '_No CHANGELOG entry found for %s._\n' "$version" > "$notes_file"
  echo "!! ${tag}: no CHANGELOG section found — releasing with a placeholder" >&2
fi

if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null 2>&1; then
  echo "++ ${tag}: tag exists, attaching a release"
  gh release create "$tag" --verify-tag --title "$tag" --notes-file "$notes_file"
else
  echo "++ ${tag}: creating tag + release at ${GITHUB_SHA}"
  gh release create "$tag" --target "$GITHUB_SHA" --title "$tag" --notes-file "$notes_file"
fi
