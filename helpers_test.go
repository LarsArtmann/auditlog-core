package auditlogcore_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	auditlogcore "github.com/larsartmann/auditlog-core"
)

func TestWriteToFile_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "output.json")

	err := auditlogcore.WriteToFile(t.Context(), path, func(w io.Writer) error {
		_, writeErr := w.Write([]byte(`{"status":"ok"}`))
		return writeErr
	})
	if err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if string(data) != `{"status":"ok"}` {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestWriteToFile_AtomicRename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.json")

	err := auditlogcore.WriteToFile(t.Context(), path, func(w io.Writer) error {
		_, writeErr := w.Write([]byte(`{"atomic":true}`))
		return writeErr
	})
	if err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	// No temp files should remain
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", entry.Name())
		}
	}
}

func TestWriteToFile_WriteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fail.json")

	errWrite := errors.New("write failed")

	err := auditlogcore.WriteToFile(t.Context(), path, func(w io.Writer) error {
		return errWrite
	})
	if err == nil {
		t.Fatal("expected error from WriteToFile")
	}

	if !errors.Is(err, errWrite) {
		t.Fatalf("expected write error, got: %v", err)
	}

	// No file should exist at the target path
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("file should not exist after write error")
	}
}

func TestCheckNoClobber_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "exists.json")

	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := auditlogcore.CheckNoClobber(path)
	if err == nil {
		t.Fatal("expected ErrFileExists")
	}

	if !errors.Is(err, auditlogcore.ErrFileExists) {
		t.Fatalf("expected ErrFileExists, got: %v", err)
	}
}

func TestCheckNoClobber_NotExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "not-exists.json")

	err := auditlogcore.CheckNoClobber(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
