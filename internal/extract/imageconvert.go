package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func isPPMFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".ppm" || ext == ".pbm" || ext == ".pgm"
}

func convertToJPEG(srcPath string) (string, func(), error) {
	if _, err := exec.LookPath("convert"); err != nil {
		return "", nil, fmt.Errorf("ImageMagick not found: %w", err)
	}

	ext := filepath.Ext(srcPath)
	dstPath := strings.TrimSuffix(srcPath, ext) + ".jpg"

	cmd := exec.Command("convert", srcPath, "-quality", "90", dstPath)
	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("convert failed: %w", err)
	}

	cleanup := func() { os.Remove(dstPath) }
	return dstPath, cleanup, nil
}

func convertPPMFiles(dir string) error {
	if _, err := exec.LookPath("convert"); err != nil {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !isPPMFormat("." + ext) {
			continue
		}

		src := filepath.Join(dir, entry.Name())
		dst := strings.TrimSuffix(src, ext) + ".jpg"

		cmd := exec.Command("convert", src, "-quality", "90", dst)
		if err := cmd.Run(); err == nil {
			os.Remove(src)
		}
	}

	return nil
}
