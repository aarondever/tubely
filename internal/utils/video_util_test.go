package utils

import (
	"testing"
)

func TestGetVideoAspectRatio(t *testing.T) {
	filePath := "/home/ubuntu/Developer/workspace/github.com/aarondever/tubely/samples/boots-video-vertical.mp4"
	actual, err := GetVideoAspectRatio(filePath)
	if err != nil {
		t.Errorf(`GetVideoAspectRatio(%s) = %q, %v`, actual, filePath, err)
		return
	}

	expected := "9:16"
	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestProcessVideoForFastStart(t *testing.T) {
	filePath := "/home/ubuntu/Developer/workspace/github.com/aarondever/tubely/samples/boots-video-vertical.mp4"
	tempFilePath, err := ProcessVideoForFastStart(filePath)
	if err != nil {
		t.Errorf(`ProcessVideoForFastStart(%s) = %q, %v`, tempFilePath, filePath, err)
		return
	}
}
