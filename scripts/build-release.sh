#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
VERSION="$(tr -d '\r\n ' < VERSION)"
TAG="v$VERSION"
SDK_NAME="LatchSDK-1.0-alpha2.6-windows.zip"
VSIX_NAME="latch-language-$VERSION.vsix"
rm -rf build dist
mkdir -p build/sdk/LatchSDK-1.0-alpha2.6/bin build/sdk/LatchSDK-1.0-alpha2.6/vscode-latch dist

# Compiler: native Windows x64, no external runtime required.
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/latch.exe ./compiler-src/main.go
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/latch-test ./compiler-src/main.go

# Gate the release with compiler behavior tests.
printf 'int main() { println("ok"); return 0; }\n' > build/good.lt
./build/latch-test check build/good.lt | grep -q 'Latch check passed.'
./build/latch-test run build/good.lt | grep -qx 'ok'
printf 'int main() { UnknownThing x; println("MUST_NOT_RUN"); return 0; }\n' > build/bad.lt
if ./build/latch-test run build/bad.lt >build/bad.out 2>build/bad.err; then echo 'negative semantic test unexpectedly passed'; exit 1; fi
if grep -q 'MUST_NOT_RUN' build/bad.out; then echo 'invalid program executed'; exit 1; fi

# Alpha 2.6 type API and Unicode char gates.
cat > build/type-api.lt <<'EOF'
int main() { char c = 'λ'; string s = "Latch"; println(c.isLetter()); println(s.length); println(s.toUpper()); println(s.substring(0, 3)); println(s.charAt(0)); return 0; }
EOF
./build/latch-test check build/type-api.lt | grep -q 'Latch check passed.'
printf 'true\n5\nLATCH\nLat\nL\n' > build/type-api.expected
./build/latch-test run build/type-api.lt > build/type-api.out
diff -u build/type-api.expected build/type-api.out
printf "int main() { int x = 'A'; return 0; }\n" > build/char-type-error.lt
if ./build/latch-test check build/char-type-error.lt >build/char-type.out 2>&1; then echo 'char/int mismatch unexpectedly passed'; exit 1; fi
grep -q 'cannot initialize int with char' build/char-type.out
printf "int main() { char x = 'AB'; return 0; }\n" > build/bad-char.lt
if ./build/latch-test check build/bad-char.lt >build/bad-char.out 2>&1; then echo 'multi-code-point char unexpectedly passed'; exit 1; fi
grep -q 'character literal must contain exactly one Unicode code point' build/bad-char.out

# Build extension payload. VSIX is an OPC zip with extension/ payload.
cp -r vscode-extension build/extension
cp build/latch.exe build/extension/latch.exe
node --check build/extension/extension.js
mkdir -p build/vsix/extension
cp -r build/extension/. build/vsix/extension/
cat > build/vsix/extension.vsixmanifest <<MANIFEST
<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011">
  <Metadata><Identity Language="en-US" Id="latch-language" Version="$VERSION" Publisher="latch-lang"/><DisplayName>Latch Language</DisplayName><Description xml:space="preserve">Language support for Latch</Description><Tags>latch,language</Tags><Categories>Programming Languages</Categories><Properties><Property Id="Microsoft.VisualStudio.Code.Engine" Value="^1.85.0"/></Properties></Metadata>
  <Installation><InstallationTarget Id="Microsoft.VisualStudio.Code"/></Installation>
  <Dependencies/><Assets><Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true"/></Assets>
</PackageManifest>
MANIFEST
cat > build/vsix/'[Content_Types].xml' <<'TYPES'
<?xml version="1.0" encoding="utf-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="json" ContentType="application/json"/><Default Extension="js" ContentType="application/javascript"/><Default Extension="png" ContentType="image/png"/><Default Extension="svg" ContentType="image/svg+xml"/><Default Extension="exe" ContentType="application/octet-stream"/><Default Extension="html" ContentType="text/html"/><Default Extension="css" ContentType="text/css"/><Default Extension="xml" ContentType="text/xml"/><Override PartName="/extension.vsixmanifest" ContentType="text/xml"/></Types>
TYPES
(cd build/vsix && zip -qr "$ROOT/dist/$VSIX_NAME" .)

# SDK.
cp build/latch.exe build/sdk/LatchSDK-1.0-alpha2.6/bin/latch.exe
cp -r compiler-src lib examples tests assets build/sdk/LatchSDK-1.0-alpha2.6/
cp -r docs build/sdk/LatchSDK-1.0-alpha2.6/ 2>/dev/null || true
cp "$ROOT/dist/$VSIX_NAME" build/sdk/LatchSDK-1.0-alpha2.6/vscode-latch/
cp README.md VERSION build/sdk/LatchSDK-1.0-alpha2.6/
(cd build/sdk && zip -qr "$ROOT/dist/$SDK_NAME" LatchSDK-1.0-alpha2.6)

# Manifest consumed by installed Latch extension.
cat > dist/release-manifest.json <<JSON
{
  "version": "$VERSION",
  "channel": "alpha",
  "downloadUrl": "https://github.com/AirzOverload/Latch/releases/download/$TAG/$VSIX_NAME",
  "sdkUrl": "https://github.com/AirzOverload/Latch/releases/download/$TAG/$SDK_NAME",
  "notesUrl": "https://github.com/AirzOverload/Latch/releases/tag/$TAG"
}
JSON
