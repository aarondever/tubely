package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=display_aspect_ratio",
		"-of", "json",
		filePath,
	)

	var out, stderr bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %v, stderr: %s", err, stderr.String())
	}

	data := struct {
		Streams []struct {
			DisplayAspectRatio string `json:"display_aspect_ratio"`
		} `json:"streams"`
	}{}

	err = json.Unmarshal(out.Bytes(), &data)
	if err != nil {
		return "", err
	}

	ratio := data.Streams[0].DisplayAspectRatio

	switch ratio {
	case "16:9", "9:16":
		return ratio, nil
	default:
		return "other", nil
	}
}
