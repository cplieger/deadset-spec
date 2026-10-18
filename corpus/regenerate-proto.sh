#!/bin/sh
# Regenerates the generated code of the corpus fixtures that bind schema
# declarations: protoc 36.2 must be on PATH; every plugin is installed at the
# version proto_generate_test.go names into a scratch directory.
set -eu
protoc --version | grep -qx 'libprotoc 36.2' || { echo "regenerate-proto.sh: want protoc 36.2 on PATH" >&2; exit 1; }
plugins=$(mktemp -d)
trap 'rm -rf "$plugins"' EXIT
gobin() { # module path, version, binary
	GOBIN="$plugins/$2" GOWORK=off GOFLAGS='' go install "$1@$2"
	mv "$plugins/$2/$3" "$plugins/$3@$2"
}
npmbin() { # package, version, binary
	mkdir -p "$plugins/npm-$3-$2"
	(cd "$plugins/npm-$3-$2" && npm init -y >/dev/null && npm install --no-audit --no-fund --save-exact "$1@$2" >/dev/null)
	ln -s "$plugins/npm-$3-$2/node_modules/.bin/$3" "$plugins/$3@v$2"
}
for v in v1.36.3 v1.36.5 v1.36.11; do gobin google.golang.org/protobuf/cmd/protoc-gen-go "$v" protoc-gen-go; done
for v in v1.0.1 v1.6.2; do gobin google.golang.org/grpc/cmd/protoc-gen-go-grpc "$v" protoc-gen-go-grpc; done
gobin connectrpc.com/connect/cmd/protoc-gen-connect-go v1.21.1 protoc-gen-connect-go
gobin github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway v2.32.0 protoc-gen-grpc-gateway
for v in 0.5.0 2.16.0; do npmbin @bufbuild/protoc-gen-es "$v" protoc-gen-es; done
npmbin ts-proto 2.15.0 protoc-gen-ts_proto
cd "$(dirname "$0")"
PROTO_PLUGINS="$plugins" go test -count=1 -run TestRegenerateProtoFixtures .
