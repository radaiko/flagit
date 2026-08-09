#!/bin/sh
# Resolves the git revision to stamp into the Flagit binary at image build time.
#
# Usage: resolve-commit.sh [build-context-dir]
#
# It prints the revision on stdout with no trailing newline, or nothing at all
# when it cannot honestly name one — the binary then reports "unknown" rather
# than a guess. Sources, in order:
#
#   1. GIT_COMMIT   an explicit --build-arg; `make docker` passes it.
#   2. SOURCE_COMMIT  what Coolify calls the deployed revision, when it reaches
#      the build at all (see 3).
#   3. the checked-out revision read straight out of .git in the build context.
#      This is the one that needs no configuration anywhere: raw Docker Compose
#      deployments do not reliably hand SOURCE_COMMIT to the build, but the
#      checkout being built is right there.
#
# Deliberately reads .git by hand instead of shelling out to git: the resolver
# runs in a minimal image, and HEAD plus refs is all the question needs. Only
# these files are exempted from .dockerignore, so nothing else of the history
# enters the build context.
set -eu

context="${1:-.}"
git_dir="$context/.git"

# trim strips surrounding whitespace, including the newline git leaves on HEAD.
trim() {
	printf '%s' "$1" | tr -d '\n\r\t' | sed -e 's/^ *//' -e 's/ *$//'
}

# is_revision accepts only a hex object name. Everything downstream substitutes
# this value into an -ldflags argument, so a ref name, a shell fragment or an
# empty variable has to be rejected here rather than baked into the binary.
is_revision() {
	case "$1" in
	'' | *[!0-9a-fA-F]*) return 1 ;;
	esac
	[ "${#1}" -ge 7 ] && [ "${#1}" -le 64 ]
}

emit() {
	printf '%s' "$1"
	exit 0
}

for candidate in "${GIT_COMMIT:-}" "${SOURCE_COMMIT:-}"; do
	candidate=$(trim "$candidate")
	if is_revision "$candidate"; then
		emit "$candidate"
	fi
done

if [ ! -f "$git_dir/HEAD" ]; then
	emit ""
fi

head=$(trim "$(cat "$git_dir/HEAD")")
case "$head" in
ref:*)
	# On a branch. A fresh clone has its refs packed, so the loose file that
	# HEAD names may not exist.
	ref=$(trim "${head#ref:}")
	if [ -f "$git_dir/$ref" ]; then
		candidate=$(trim "$(cat "$git_dir/$ref")")
	elif [ -f "$git_dir/packed-refs" ]; then
		candidate=$(trim "$(awk -v ref="$ref" \
			'$1 !~ /^[#^]/ && $2 == ref { print $1; exit }' "$git_dir/packed-refs")")
	else
		candidate=""
	fi
	;;
*)
	# Detached HEAD: how a deployment checkout of one commit looks.
	candidate="$head"
	;;
esac

if is_revision "$candidate"; then
	emit "$candidate"
fi
emit ""
