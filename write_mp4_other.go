//go:build !ios && !android && !js

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strconv"
)

func videoExportAvailable() bool { return true }

func WriteMP4(frames []image.Image, delays []int, path string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "karmamanager_mp4_*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write each frame as a PNG.
	for i, img := range frames {
		framePath := fmt.Sprintf("%s/frame_%04d.png", tmpDir, i)
		f, err := os.Create(framePath)
		if err != nil {
			return err
		}
		encErr := png.Encode(f, img)
		f.Close()
		if encErr != nil {
			return encErr
		}
	}

	// Build an ffmpeg concat demuxer file for variable frame rate.
	concatPath := tmpDir + "/concat.txt"
	cf, err := os.Create(concatPath)
	if err != nil {
		return err
	}
	for i := range frames {
		delay := delays[i]
		if delay < 1 {
			delay = 1
		}
		durSec := float64(delay) / 100.0
		fmt.Fprintf(cf, "file 'frame_%04d.png'\n", i)
		fmt.Fprintf(cf, "duration %s\n", strconv.FormatFloat(durSec, 'f', 4, 64))
	}
	// The concat demuxer requires the last file entry repeated without a duration.
	if len(frames) > 0 {
		fmt.Fprintf(cf, "file 'frame_%04d.png'\n", len(frames)-1)
	}
	cf.Close()

	cmd := exec.Command(ffmpeg,
		"-f", "concat",
		"-safe", "0",
		"-i", concatPath,
		"-c:v", "libx264",
		"-vf", "pad=ceil(iw/2)*2:ceil(ih/2)*2",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-y",
		path,
	)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, output)
	}
	return nil
}
