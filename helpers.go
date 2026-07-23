package auditlogcore

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrExportWriteFailed wraps errors encountered while writing exported output
// to a file or io.Writer (file creation, buffer flush, atomic rename, direct
// writes). Classified as Infrastructure — these are system-level failures not
// retryable by the caller. Consumers can match on it with [errors.Is].
var ErrExportWriteFailed = errors.New("export write failed")

// ErrFileExists is returned when a file already exists at the target path
// and the caller requested no-clobber behavior. Classified as Rejection —
// the caller asked for an impossible operation (writing to an existing file
// without overwrite). Consumers can match on it with [errors.Is].
var ErrFileExists = fmt.Errorf("%w: file already exists", ErrExportWriteFailed)

// fileWriteBufferSize is the bufio buffer size used for atomic file exports.
const fileWriteBufferSize = 65536

// CheckNoClobber returns ErrFileExists if a file already exists at path.
// Call this before Export* methods to prevent accidental overwrites:
//
//	if err := auditlogcore.CheckNoClobber(path); err != nil { return err }
//	report.ExportJSON(path)
func CheckNoClobber(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return fmt.Errorf("%w: %q", ErrFileExists, path)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("%w: stat %q: %w", ErrExportWriteFailed, path, err)
	}

	return nil
}

// WriteToFile creates a file at path and calls fn with a buffered writer.
// The bufio.Writer batches small writes into 64KB blocks, reducing syscall count
// by 10-100x compared to writing directly to os.File.
//
// Writes are atomic: data is written to a temporary file in the same directory,
// then atomically renamed to the final path. A crash during write leaves the
// previous file (if any) intact rather than a partial file.
//
// The context is checked before the write begins and before the atomic rename.
// If cancelled, the temp file is cleaned up and ctx.Err() is returned.
func WriteToFile(ctx context.Context, path string, fn func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: cancelled before write: %w", ErrExportWriteFailed, err)
	}

	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, ".tmp-auditlog-*")
	if err != nil {
		return fmt.Errorf("%w: create temp file in %q: %w", ErrExportWriteFailed, dir, err)
	}

	tmpPath := tmpFile.Name()
	cleanup := true

	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	bw := bufio.NewWriterSize(tmpFile, fileWriteBufferSize)

	writeErr := fn(bw)

	flushErr := bw.Flush()

	closeErr := tmpFile.Close()

	if writeErr != nil {
		return writeErr
	}

	if flushErr != nil {
		return fmt.Errorf("%w: flush temp file %q: %w", ErrExportWriteFailed, tmpPath, flushErr)
	}

	if closeErr != nil {
		return fmt.Errorf("%w: close temp file %q: %w", ErrExportWriteFailed, tmpPath, closeErr)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: cancelled before rename: %w", ErrExportWriteFailed, err)
	}

	renameErr := os.Rename(tmpPath, path)
	if renameErr != nil {
		return fmt.Errorf("%w: rename %q -> %q: %w", ErrExportWriteFailed, tmpPath, path, renameErr)
	}

	cleanup = false

	return nil
}
