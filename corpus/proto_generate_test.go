package corpus

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/txtar"
)

// protocRun is one protoc invocation over a rendering's schema files, which sit
// below target/proto: plugin is the name after protoc-gen-, out the directory
// below the rendering the plugin writes into.
type protocRun struct {
	fixture, language, plugin, version, opt, out string
	files                                        []string
}

// protoGenerations is every generated file of the corpus, as the plugin and the
// version that wrote it.
var protoGenerations = []protocRun{
	{fixture: "proto-go-generated-fact", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-go-raw-descriptor-shapes", language: "go", plugin: "go", version: "v1.36.3", opt: "module=example.com/target", out: "target", files: []string{"v1/a.proto"}},
	{fixture: "proto-go-raw-descriptor-shapes", language: "go", plugin: "go", version: "v1.36.5", opt: "module=example.com/target", out: "target", files: []string{"v1/b.proto"}},
	{fixture: "proto-go-raw-descriptor-shapes", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/c.proto"}},
	{fixture: "proto-go-group-field-by-number", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/search.proto", "v1/query.proto"}},
	{fixture: "proto-go-grpc-no-type-fallback", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-go-grpc-no-type-fallback", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-go-grpc-no-type-fallback", language: "go", plugin: "connect-go", version: "v1.21.1", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-protobuf-es-v2-tags", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-ts-proto-default-rpc-only", language: "ts", plugin: "ts_proto", version: "v2.15.0", opt: "", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-facts-leave-report-unchanged-go", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-facts-leave-report-unchanged-go", language: "go", plugin: "go-grpc", version: "v1.0.1", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-facts-leave-report-unchanged-ts", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-facts-leave-report-unchanged-ts", language: "ts", plugin: "es", version: "v0.5.0", opt: "target=ts", out: "target/src/legacy", files: []string{"v1/legacy.proto"}},
	{fixture: "proto-facts-unevaluated-before-serializers", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-facts-unevaluated-before-serializers", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-rpc-called-from-ts", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-ts-create-literal-writes", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-oneof-protobuf-es", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/outcome.proto"}},
	{fixture: "proto-ts-serialization-contract", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-ts-equals-reads", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-ts-proto-message-fns", language: "ts", plugin: "ts_proto", version: "v2.15.0", opt: "outputSchema=true", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-field-read-in-one-language", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-field-read-in-one-language", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-field-written-only", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-field-written-only", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-transport-reads-nothing", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-transport-reads-nothing", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-transport-reads-nothing", language: "go", plugin: "connect-go", version: "v1.21.1", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-transport-reads-nothing", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-hand-serializer-reads", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/envelope.proto"}},
	{fixture: "proto-hand-serializer-reads", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/envelope.proto"}},
	{fixture: "proto-protojson-unmarshal-caps", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-protojson-unmarshal-caps", language: "ts", plugin: "es", version: "v2.16.0", opt: "target=ts", out: "target/src/gen", files: []string{"v1/service.proto"}},
	{fixture: "proto-field-read-by-getter", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-oneof-go-wrapper", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/outcome.proto"}},
	{fixture: "proto-handler-read-request-field", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-handler-read-request-field", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-handler-calls-handler", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/config.proto"}},
	{fixture: "proto-handler-calls-handler", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/config.proto"}},
	{fixture: "proto-delegating-handler", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/config.proto"}},
	{fixture: "proto-delegating-handler", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/config.proto"}},
	{fixture: "proto-service-registered-only", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-service-registered-only", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-grpc-server-reflection-caps", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-grpc-server-reflection-caps", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-gateway-server-mode", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-gateway-server-mode", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-gateway-server-mode", language: "go", plugin: "grpc-gateway", version: "v2.32.0", opt: "module=example.com/target,generate_unbound_methods=true", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-rpc-uncalled-with-go-handler", language: "go", plugin: "go", version: "v1.36.11", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-rpc-uncalled-with-go-handler", language: "go", plugin: "go-grpc", version: "v1.6.2", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
	{fixture: "proto-rpc-uncalled-with-go-handler", language: "go", plugin: "connect-go", version: "v1.21.1", opt: "module=example.com/target", out: "target", files: []string{"v1/service.proto"}},
}

// TestRegenerateProtoFixtures rewrites the generated code of the fixtures that
// bind schema declarations, with the plugins regenerate-proto.sh installs into
// PROTO_PLUGINS; without it the test is skipped.
func TestRegenerateProtoFixtures(t *testing.T) {
	plugins := os.Getenv("PROTO_PLUGINS")
	if plugins == "" {
		t.Skip("PROTO_PLUGINS is unset; corpus/regenerate-proto.sh sets it")
	}
	for _, run := range protoGenerations {
		dir := t.TempDir()
		rendering := filepath.Join("..", FixturesDir, run.fixture, run.language)
		if run.language == "go" {
			rendering = filepath.Join(dir, "rendering")
			if err := extractArchive(filepath.Join("..", FixturesDir, run.fixture, "go.txtar"), rendering); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(dir, "out")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		args := []string{
			"-I", filepath.Join(rendering, "target", "proto"),
			"--plugin=protoc-gen-" + run.plugin + "=" + filepath.Join(plugins, "protoc-gen-"+run.plugin+"@"+run.version),
			"--" + run.plugin + "_out=" + out,
		}
		if run.opt != "" {
			args = append(args, "--"+run.plugin+"_opt="+run.opt)
		}
		if output, err := exec.Command("protoc", append(args, run.files...)...).CombinedOutput(); err != nil {
			t.Fatalf("%s: protoc: %v: %s", run.fixture, err, output)
		}
		if err := placeGenerated(run, out); err != nil {
			t.Fatal(err)
		}
	}
}

func extractArchive(name, dir string) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	for _, file := range txtar.Parse(data).Files {
		target := filepath.Join(dir, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, file.Data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// placeGenerated writes every file the run generated below its out directory,
// into the directory rendering or as a section of the archive one.
func placeGenerated(run protocRun, out string) error {
	archiveName := filepath.Join("..", FixturesDir, run.fixture, "go.txtar")
	var archive *txtar.Archive
	if run.language == "go" {
		data, err := os.ReadFile(archiveName)
		if err != nil {
			return err
		}
		archive = txtar.Parse(data)
	}
	err := filepath.WalkDir(out, func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(out, name)
		at := path.Join(run.out, filepath.ToSlash(rel))
		if archive == nil {
			target := filepath.Join("..", FixturesDir, run.fixture, run.language, filepath.FromSlash(at))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o600)
		}
		if i := slices.IndexFunc(archive.Files, func(f txtar.File) bool { return f.Name == at }); i >= 0 {
			archive.Files[i].Data = data
		} else {
			archive.Files = append(archive.Files, txtar.File{Name: at, Data: data})
		}
		return nil
	})
	if err != nil || archive == nil {
		return err
	}
	return os.WriteFile(archiveName, txtar.Format(archive), 0o600)
}
