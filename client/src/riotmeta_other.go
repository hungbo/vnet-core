//go:build !windows

package main

// C:\ProgramData\Riot Games chỉ có trên máy trạm Windows.
func riotMetaDest() string { return "" }
