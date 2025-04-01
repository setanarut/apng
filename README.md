[![GoDoc](https://godoc.org/github.com/setanarut/apng?status.svg)](https://pkg.go.dev/github.com/setanarut/apng)

# apng
APNG encoder

![animated_gopher](_example/res/animated_gopher.png)

## Installation

```sh
go get github.com/setanarut/apng
```

## Example

```Go
package main

import (
	"image"
	"image/color"
	"math/rand/v2"

	"github.com/setanarut/apng"
)

func main() {
	frames := make([]image.Image, 8)
	for i := range 8 {
		frames[i] = generateNoiseImage(600, 200)
	}
	apng.Save("out.png", frames, 7)
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
```


