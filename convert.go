package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
)

func ConvertToJpegFromPng(b []byte) []byte {
	nr := bytes.NewReader(b)
	buff := new(bytes.Buffer)
	img, err := png.Decode(nr)
	if err != nil {
		panic(err)
	}
	var rgba *image.RGBA
	if nrgba, ok := img.(*image.NRGBA); ok {
		if nrgba.Opaque() {
			rgba = &image.RGBA{
				Pix:    nrgba.Pix,
				Stride: nrgba.Stride,
				Rect:   nrgba.Rect,
			}
		}
	}
	if rgba != nil {
		err = jpeg.Encode(buff, rgba, &jpeg.Options{Quality: 95})
	} else {
		err = jpeg.Encode(buff, img, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		panic(err)
	}
	return buff.Bytes()
}
