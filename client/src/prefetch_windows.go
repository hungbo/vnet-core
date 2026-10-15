//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// webView2Root là nơi bộ cài Evergreen đặt WebView2 runtime cho cả máy:
// C:\Program Files (x86)\Microsoft\EdgeWebView\Application. Hỏi hệ điều hành
// trước; hỏi không được mới rơi về %ProgramFiles(x86)%.
func webView2Root() string {
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFilesX86, 0)
	if err != nil {
		root = os.Getenv("ProgramFiles(x86)")
	}
	if root == "" {
		root = `C:\Program Files (x86)`
	}
	return filepath.Join(root, "Microsoft", "EdgeWebView", "Application")
}
