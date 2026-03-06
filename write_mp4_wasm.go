//go:build js

package main

import (
	"fmt"
	"image"
)

func videoExportAvailable() bool { return false }

func WriteMP4(_ []image.Image, _ []int, _ string) error {
	return fmt.Errorf("video export is not supported in the browser")
}
