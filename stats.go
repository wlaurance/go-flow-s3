package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
)

func getImageConfigFromJpegBytes(b []byte) image.Config {
	config, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return config
}

func getImageConfigFromPngBytes(b []byte) image.Config {
	config, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return config
}

func GetImageConfigFromBytesAndType(t string, b []byte) image.Config {
	if t == ".jpg" {
		return getImageConfigFromJpegBytes(b)
	} else if t == ".png" {
		return getImageConfigFromPngBytes(b)
	}
	return image.Config{}
}
