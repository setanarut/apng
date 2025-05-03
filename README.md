[![GoDoc](https://godoc.org/github.com/setanarut/apng?status.svg)](https://pkg.go.dev/github.com/setanarut/apng)

# apng
Fast APNG encoding with concurrent frame encoding.

## Installation

```sh
go get github.com/setanarut/apng
```

## 20 frames animation encoding benchmark

| Image Size | [kettek](https://github.com/kettek/apng) | [setanarut](https://github.com/setanarut/apng) |
| ---------- | ---------------------------------------- | ---------------------------------------------- |
| 125x125    | 173 ms                                   | 43 ms                                          |
| 250x250    | 655 ms                                   | 153 ms                                         |
| 500x500    | 2542 ms                                  | 565 ms                                         |
| 1000x1000  | 10174 ms                                 | 2213 ms                                        |
| 2000x2000  | 40745 ms                                 | 8831 ms                                        |

<img width="1175" alt="bench" src="https://github.com/user-attachments/assets/54558177-8e13-42cb-9eb0-5c00a414ef9d" />

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

![out](https://github.com/user-attachments/assets/b9c5d4f9-7479-479b-a2c3-a4642f5ccde3)

