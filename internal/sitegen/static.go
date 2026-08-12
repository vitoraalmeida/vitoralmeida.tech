package sitegen

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyStatic copia recursivamente os arquivos estáticos (CSS, fontes, imagens)
// do diretório de origem para o destino, preservando a estrutura e permissões.
func CopyStatic(source, destination string) error {
	if err := copyDirectoryContents(source, destination); err != nil {
		return fmt.Errorf("copy static files from %q: %w", source, err)
	}
	return nil
}

// copyPostAssets copia os assets de cada post (imagens referenciadas no
// Markdown) para public/posts/{slug}/ no output, espelhando o caminho público.
func copyPostAssets(posts []Post, output string) error {
	for _, post := range posts {
		if post.AssetsDir == "" {
			continue
		}
		destination := filepath.Join(output, "public", "posts", post.Slug)
		if err := copyDirectoryContents(post.AssetsDir, destination); err != nil {
			return fmt.Errorf("copy assets for post %q: %w", post.Slug, err)
		}
	}
	return nil
}

// copyDirectoryContents percorre source recursivamente, recriando diretórios
// e copiando arquivos com suas permissões originais sob destination.
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

// copyFile copia o conteúdo de um arquivo para outro preservando as permissões
// e fechando ambos os handles mesmo em caso de erro. Arquivos CSS são
// minificados na cópia; os demais são copiados byte a byte.
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
	if strings.EqualFold(filepath.Ext(source), ".css") {
		content, err := io.ReadAll(input)
		if err != nil {
			_ = input.Close()
			_ = output.Close()
			return err
		}
		if _, err := output.Write(minifyCSS(content)); err != nil {
			_ = input.Close()
			_ = output.Close()
			return err
		}
	} else if _, err := io.Copy(output, input); err != nil {
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
