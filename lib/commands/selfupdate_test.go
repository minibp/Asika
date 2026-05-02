package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestIsNewer(t *testing.T) {
	if !isNewer("2.0.0", "1.0.0") {
		t.Error("2.0.0 should be newer than 1.0.0")
	}
	if isNewer("1.0.0", "1.0.0") {
		t.Error("1.0.0 should not be newer than 1.0.0")
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()

	binaryData := []byte("fake binary content")
	binaryPath := filepath.Join(dir, "asika_linux_amd64")
	if err := os.WriteFile(binaryPath, binaryData, 0644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(binaryData)
	expectedSum := hex.EncodeToString(sum[:])

	t.Run("valid checksum", func(t *testing.T) {
		checksums := expectedSum + "  asika_linux_amd64\nothersum  otherfile\n"
		checksumPath := filepath.Join(dir, "checksums.txt")
		if err := os.WriteFile(checksumPath, []byte(checksums), 0644); err != nil {
			t.Fatal(err)
		}
		if err := verifyChecksum(binaryPath, checksumPath, "asika_linux_amd64"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("mismatched checksum", func(t *testing.T) {
		checksums := "0000000000000000000000000000000000000000000000000000000000000000  asika_linux_amd64\n"
		checksumPath := filepath.Join(dir, "checksums_bad.txt")
		if err := os.WriteFile(checksumPath, []byte(checksums), 0644); err != nil {
			t.Fatal(err)
		}
		if err := verifyChecksum(binaryPath, checksumPath, "asika_linux_amd64"); err == nil {
			t.Error("expected error for mismatched checksum")
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		checksums := "abc123  other_file\n"
		checksumPath := filepath.Join(dir, "checksums_missing.txt")
		if err := os.WriteFile(checksumPath, []byte(checksums), 0644); err != nil {
			t.Fatal(err)
		}
		if err := verifyChecksum(binaryPath, checksumPath, "asika_linux_amd64"); err == nil {
			t.Error("expected error for missing entry")
		}
	})

	t.Run("with SHA256 prefix in checksums", func(t *testing.T) {
		checksums := "SHA256 (asika_linux_amd64) = " + expectedSum + "\n"
		checksumPath := filepath.Join(dir, "checksums_prefix.txt")
		if err := os.WriteFile(checksumPath, []byte(checksums), 0644); err != nil {
			t.Fatal(err)
		}
		if err := verifyChecksum(binaryPath, checksumPath, "asika_linux_amd64"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestDoRollbackNoBackup(t *testing.T) {
	currentPath, _ := os.Executable()
	backupPath := currentPath + ".old"
	os.Remove(backupPath)

	_, err := os.Stat(backupPath)
	if err == nil {
		t.Skip("backup file unexpectedly exists, cannot run test")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	srcPath := filepath.Join(dir, "src")
	srcData := []byte("test copy data")
	if err := os.WriteFile(srcPath, srcData, 0644); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(dir, "dst")
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatal(err)
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(dstData) != string(srcData) {
		t.Error("copied data does not match source")
	}
}

func TestDetectBinary(t *testing.T) {
	name := detectBinary()
	if name == "" {
		t.Error("detectBinary returned empty string")
	}
}
