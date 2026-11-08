#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p .build
compiler="$(command -v swiftc)"
toolchain_compiler="$(xcrun --find swiftc)"
args=(-swift-version 5)
include_dir="$(dirname "$toolchain_compiler")/../include/swift"
# Older CLT remnants can declare SwiftBridging twice. Hide only that duplicate
# through a temporary compiler filesystem view; never modify installed tools.
if [[ -f "$include_dir/module.modulemap" && -f "$include_dir/bridging.modulemap" ]] &&
   /usr/bin/grep -q 'module SwiftBridging' "$include_dir/module.modulemap" &&
   /usr/bin/grep -q 'module SwiftBridging' "$include_dir/bridging.modulemap"; then
  overlay_dir="$(mktemp -d /private/tmp/kferp-swift.XXXXXX)"
  trap 'rm -rf "$overlay_dir"' EXIT
  python3 - "$include_dir" "$overlay_dir" <<'PY'
import json,sys
from pathlib import Path
import os
inc=Path(os.path.abspath(sys.argv[1]));temp=Path(sys.argv[2]);empty=temp/'empty.modulemap';empty.write_text('')
(temp/'overlay.json').write_text(json.dumps({'version':0,'roots':[{'type':'file','name':str(inc/'module.modulemap'),'external-contents':str(empty)}]}))
PY
  args+=(-vfsoverlay "$overlay_dir/overlay.json" -Xcc -ivfsoverlay -Xcc "$overlay_dir/overlay.json")
fi
"$compiler" -swift-version 5 Sources/NotifierCore/Guard.swift Tests/NotifierCoreTests/GuardTests.swift -o .build/GuardTests
.build/GuardTests
"$compiler" "${args[@]}" Sources/NotifierCore/Guard.swift Sources/WeChatNotifier/main.swift -o .build/WeChatNotifier
