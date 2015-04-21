package main

import (
	"bytes"
	"image"
	"image/jpeg"
)

func getImageConfigFromBytes(b []byte) image.Config {
	config, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return config
}

func GetWidthFromJpegBytes(b []byte) int {
	return getImageConfigFromBytes(b).Width
}

func GetHeightFromJpegBytes(b []byte) int {
	return getImageConfigFromBytes(b).Height
}
