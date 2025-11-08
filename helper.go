package apng

import (
	"bytes"
	"image"
	"log"
	"os"
)

// Save writes an APNG file with the given images and uniform frame delay.
//
// Images obtained via image.SubImage() are not supported, If an image is a sub-image, copy it into a new image before encoding.
//
// The successive delay times, one per frame, in 100ths of a second (centiseconds).
func Save(filePath string, images []image.Image, delay uint16) {
	totalFrames := len(images)
	if totalFrames == 0 {
		log.Fatal("apng: no images provided")
	}

	delays := make([]uint16, totalFrames)
	for i := range delays {
		delays[i] = delay
	}

	animPng := APNG{
		Images: images,
		Delays: delays,
	}

	file, err := os.Create(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	encodeError := EncodeAll(file, &animPng)
	if encodeError != nil {
		log.Fatal(encodeError)
	}
}

// APNGBytes encodes a slice of images into an APNG byte stream with a consistent delay per frame.
//
// Images obtained via image.SubImage() are not supported, If an image is a sub-image, copy it into a new image before encoding.
//
// The successive delay times, one per frame, in 100ths of a second (centiseconds).
func APNGBytes(images []image.Image, delay uint16) []byte {
	totalFrames := len(images)
	if totalFrames == 0 {
		log.Fatal("apng: no images provided")
	}

	delays := make([]uint16, totalFrames)
	for i := range delays {
		delays[i] = delay
	}

	animPng := APNG{
		Images: images,
		Delays: delays,
	}

	// Create a buffer to store the bytes
	buf := new(bytes.Buffer)

	// Encode to buffer instead of file
	if err := EncodeAll(buf, &animPng); err != nil {
		log.Fatal(err)
	}

	return buf.Bytes()
}
