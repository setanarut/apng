package main

import (
	"image"
	"image/color"
	"math/rand/v2"

	"github.com/setanarut/apng"
)

const FrameCount int = 40

func main() {

	frames := make([]image.Image, FrameCount)
	for i := range FrameCount {
		frames[i] = generateNoiseImage(600, 200)
	}
	apng.Save("out.png", frames, 3)
}

func generateNoiseImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			col := noisePalette[rand.IntN(4)]
			img.SetRGBA(x, y, col)
		}
	}
	return img
}

var noisePalette = []color.RGBA{
	{R: 0, G: 0, B: 0, A: 255},   // Black
	{R: 255, G: 0, B: 0, A: 255}, // Red
	{R: 0, G: 255, B: 0, A: 255}, // Green
	{R: 0, G: 0, B: 255, A: 255}, // Blue
}
