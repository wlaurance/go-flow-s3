package main

import (
	"bytes"
	"image"
	"image/jpeg"
)

func getImageConfigFromJpegBytes(b []byte) image.Config {
	config, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return config
}

func GetWidthFromJpegBytes(b []byte) int {
	return getImageConfigFromJpegBytes(b).Width
}

func GetHeightFromJpegBytes(b []byte) int {
	return getImageConfigFromJpegBytes(b).Height
}
