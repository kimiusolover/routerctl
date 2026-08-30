package artifactverify

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/example/routerctl/internal/backend"
	"github.com/example/routerctl/internal/manifest"
)

func TestVerifyChecksProducerManifestAgainstArtifact(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "ax23v-v1.bin")
	if err := os.WriteFile(output, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("image"))
	data := fmt.Sprintf(`{"schema":2,"device":"ax23v-v1","flashable":true,"artifacts":[{"name":"ax23v-v1.bin","sha256":"%x","size":5,"format":"tplink-safeloader","board_id":"ARCHER-AX23-V1"}]}`, hash)
	if err := os.WriteFile(filepath.Join(dir, "ax23v-v1.manifest.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Verify(output, backend.Artifact{Path: output, Digest: fmt.Sprintf("sha256:%x", hash)}, manifest.ArtifactExpected{Device: "ax23v-v1", Format: "tplink-safeloader", BoardID: "ARCHER-AX23-V1", MaxSizeBytes: 5})
	if err != nil {
		t.Fatal(err)
	}
}
