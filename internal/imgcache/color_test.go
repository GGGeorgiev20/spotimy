package imgcache

import (
	"image"
	"image/color"
	"testing"
)

func TestDominantPrefersColorOverWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			c := color.RGBA{255, 255, 255, 255} // 70% white
			if x < 30 {
				c = color.RGBA{200, 30, 30, 255}
			}
			img.Set(x, y, c)
		}
	}
	if got := dominant(img); got != "#c81e1e" {
		t.Fatalf("got %s", got)
	}
}

func TestDominantGrayscaleFallsBackToAverage(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	for i := range img.Pix {
		img.Pix[i] = 100
	}
	if got := dominant(img); got != "#646464" {
		t.Fatalf("got %s", got)
	}
}
