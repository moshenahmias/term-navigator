package explorer

import (
	"fmt"
	"path"
)

// CopyBetween copies a file from one FileExplorer to another.
// srcPath and dstPath are full paths (or keys) within their explorers.
func CopyBetween(src FileExplorer, srcPath string, dst FileExplorer, dstPath string) error {
	// 1. Stat source
	info, err := src.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("copy: cannot stat source %q: %w", srcPath, err)
	}
	if info.IsDir {
		return fmt.Errorf("copy: directories not supported yet: %q", srcPath)
	}

	// 2. If destination is a directory, append filename
	dstInfo, err := dst.Stat(dstPath)
	if err == nil && dstInfo.IsDir {
		dstPath = path.Join(dstPath, info.Name)
	}

	// 3. Check if destination exists
	if dst.Exists(dstPath) {
		// You can later hook this into a UI overwrite dialog
		return fmt.Errorf("copy: destination already exists: %q", dstPath)
	}

	// 4. Open source stream
	r, err := src.Read(srcPath)
	if err != nil {
		return fmt.Errorf("copy: cannot read source %q: %w", srcPath, err)
	}
	defer r.Close()

	// 5. Write to destination
	if err := dst.Write(dstPath, r); err != nil {
		return fmt.Errorf("copy: cannot write destination %q: %w", dstPath, err)
	}

	return nil
}
