//go:build !windows

package main

// WebView2 runtime cài cho cả máy chỉ có trên máy trạm Windows.
func webView2Root() string { return "" }
