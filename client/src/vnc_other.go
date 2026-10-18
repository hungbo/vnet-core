//go:build !windows

package main

import "errors"

func applyVNC(password, remoteIP string) error {
	return errors.New("TightVNC chỉ có trên Windows")
}
