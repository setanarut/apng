package main

import (
	"image"
	"image/color"
	"math/rand/v2"

	"github.com/setanarut/apng"
)

const FrameCount int = 30

func main() {
	frames := make([]image.Image, FrameCount)
	// frames2 := make([]image.Image, FrameCount)

	for i := range FrameCount {
		frames[i] = generatePalettedFrames(600, 200)
		// frames2[i] = generateRGBAFrames(600, 200)
	}
	apng.Save("outPaletted.png", frames, 7)
	// apng.Save("outRGBA.png", frames2, 7)
}

func generateRGBAFrames(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			c := plt[rand.IntN(3)]
			img.SetRGBA(x, y, c.(color.RGBA))
		}
	}
	return img
}
func generatePalettedFrames(width, height int) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, width, height), plt)
	for y := range height {
		for x := range width {
			img.SetColorIndex(x, y, uint8(rand.IntN(3)))
		}
	}
	return img
}

var plt = []color.Color{
	color.RGBA{R: 0, G: 0, B: 0, A: 255},       // Black
	color.RGBA{R: 255, G: 255, B: 255, A: 255}, // White
	color.RGBA{R: 255, G: 0, B: 255, A: 255},   // White
}
