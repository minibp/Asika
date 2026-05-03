package templates

import (
	"io/fs"
	"testing"
)

func TestFSEmbedded(t *testing.T) {
	// Verify FS is not empty (at least one file)
	files, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatalf("failed to read embed.FS: %v", err)
	}

	if len(files) == 0 {
		t.Error("embed.FS should contain at least one template file")
	}
}

func TestFSHasHTMLFiles(t *testing.T) {
	err := fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && len(path) > 5 && path[len(path)-5:] == ".html" {
			t.Logf("found template: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}
}
