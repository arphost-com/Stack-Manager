#!/bin/sh
# Git's credential helper protocol passes the requested origin on stdin.
# Return the approved CI token only for the intended HTTPS GitHub origin.
[ "$1" = get ] || exit 0
protocol=
host=
while IFS= read -r field; do
  case "$field" in
    protocol=*) protocol=${field#protocol=} ;;
    host=*) host=${field#host=} ;;
  esac
done
[ "$protocol" = https ] && [ "$host" = github.com ] && [ -n "${GITHUB_PAT:-}" ] || exit 1
printf 'username=x-access-token\npassword=%s\n' "$GITHUB_PAT"
