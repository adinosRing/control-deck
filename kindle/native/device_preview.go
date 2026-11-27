//go:build !linux || !arm

package main

import "errors"

func runDevice(_ *dashboard, _ string) error {
	return errors.New("the Kindle app requires ARM Linux; use -preview to render a PNG on this computer")
}
