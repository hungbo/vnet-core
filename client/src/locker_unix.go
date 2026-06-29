//go:build darwin || linux

package main

type ScreenLocker struct{}

func NewScreenLocker() *ScreenLocker {
	return &ScreenLocker{}
}

func (l *ScreenLocker) Lock() error {
	return nil
}

func (l *ScreenLocker) Unlock() {
}

func (l *ScreenLocker) ShowMessage(title, message string) error {
	return nil
}
