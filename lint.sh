#!/bin/bash

set -e

exclude_files=$(cat <<EOF
consensus/qkchash/native/native.go
consensus/qkchash/native/wrapper.go
EOF
)

lint_temp=$(mktemp /tmp/golint.XXXXXXXXX)
echo "$exclude_files" > $lint_temp

warnings=$(
find . -type f -name "*.go" \
	| grep -v -f $lint_temp \
	| xargs -I {} golint {}
	)
echo "$warnings"
[ ! -z "${warnings}" ] && exit 1 || exit 0
