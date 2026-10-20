package imgcache

import (
	"context"
	"fmt"
	"image"
	"os"
)

// DominantColor returns the most prominent colorful color of an image as
// #rrggbb, used for page header gradients. Grayscale images give their average.
func (c *Cache) DominantColor(ctx context.Context, rawURL string) (string, error) {
	if v, ok := c.color.Load(rawURL); ok {
		return v.(string), nil
	}
	p, err := c.file(ctx, rawURL)
	if err != nil {
		return "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	hex := dominant(img)
	c.color.Store(rawURL, hex)
	return hex, nil
}

// dominant buckets sampled pixels into a 4-bit-per-channel histogram weighted
// by saturation and returns the mean color of the heaviest bucket.
func dominant(img image.Image) string {
	b := img.Bounds()
	step := max(1, max(b.Dx(), b.Dy())/64)
	type acc struct{ w, r, g, b float64 }
	buckets := map[int]*acc{}
	var avg acc
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r16, g16, b16, _ := img.At(x, y).RGBA()
			r, g, bl := float64(r16>>8), float64(g16>>8), float64(b16>>8)
			avg.w++
			avg.r, avg.g, avg.b = avg.r+r, avg.g+g, avg.b+bl
			hi, lo := max(r, g, bl), min(r, g, bl)
			if hi < 25 || lo > 235 || hi == 0 {
				continue // near black or white
			}
			sat := (hi - lo) / hi
			if sat < 0.15 {
				continue
			}
			k := int(r)>>4<<8 | int(g)>>4<<4 | int(bl)>>4
			a := buckets[k]
			if a == nil {
				a = &acc{}
				buckets[k] = a
			}
			w := 1 + 3*sat
			a.w += w
			a.r, a.g, a.b = a.r+r*w, a.g+g*w, a.b+bl*w
		}
	}
	best := &avg
	for _, a := range buckets {
		if best == &avg || a.w > best.w {
			best = a
		}
	}
	if best.w == 0 {
		return "#535353"
	}
	return fmt.Sprintf("#%02x%02x%02x", int(best.r/best.w), int(best.g/best.w), int(best.b/best.w))
}
