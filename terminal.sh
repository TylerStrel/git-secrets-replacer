#!/bin/bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"$DIR/git-secrets-replacer-linux-${ARCH}-${VERSION_TAG}"
