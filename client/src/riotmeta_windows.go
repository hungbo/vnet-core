//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// riotMetaDest là C:\ProgramData\Riot Games. Hỏi hệ điều hành trước; hỏi không
// được mới rơi về %ProgramData%.
func riotMetaDest() string {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0); err == nil {
		return filepath.Join(dir, "Riot Games")
	}
	root := os.Getenv("ProgramData")
	if root == "" {
		root = `C:\ProgramData`
	}
	return filepath.Join(root, "Riot Games")
}
