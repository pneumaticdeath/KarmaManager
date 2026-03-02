package main

import (
	"image/gif"
	"os"
)

func WriteGIF(g *gif.GIF, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, g)
}
