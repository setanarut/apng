package apng

import (
	"bytes"
	"errors"
	"image"
	"os"
)

// Save writes an APNG file with the given images and uniform frame delay.
//
// The successive delay times, one per frame, in 100ths of a second (centiseconds).
//
// Note: For 30 FPS, each frame lasts 1/30 second ≈ 3.33 centiseconds.
// When using integer delays, you might use 3 centiseconds per frame.
func Save(filePath string, images []image.Image, delay uint16) error {
	totalFrames := len(images)
	if totalFrames == 0 {
		return errors.New("apng: no images provided")
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
		return err
	}
	defer file.Close()

	return EncodeAll(file, &animPng)
}

// APNGBytes encodes a slice of images into an APNG byte stream with a consistent delay per frame.
//
// The successive delay times, one per frame, in 100ths of a second (centiseconds).
//
// Note: For 30 FPS, each frame lasts 1/30 second ≈ 3.33 centiseconds.
// When using integer delays, you might use 3 centiseconds per frame.
func APNGBytes(images []image.Image, delay uint16) ([]byte, error) {
	totalFrames := len(images)
	if totalFrames == 0 {
		return nil, errors.New("apng: no images provided")
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
		return nil, err
	}

	return buf.Bytes(), nil
}
