package sitegen

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func CopyStatic(source, destination string) error {
	if err := copyDirectoryContents(source, destination); err != nil {
		return fmt.Errorf("copy static files from %q: %w", source, err)
	}
	return nil
}

func copyDirectoryContents(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		_ = input.Close()
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = input.Close()
		_ = output.Close()
		return err
	}
	if err := input.Close(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}
