package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

var (
	OK      = icon(green, drawCheck)
	Locked  = icon(red, drawLock)
	Warning = icon(yellow, drawBang)
	Offline = icon(gray, drawDash)
)

var (
	green  = color.RGBA{R: 0x20, G: 0xB2, B: 0x6B, A: 0xFF}
	red    = color.RGBA{R: 0xD9, G: 0x3A, B: 0x3A, A: 0xFF}
	yellow = color.RGBA{R: 0xE6, G: 0xA7, B: 0x00, A: 0xFF}
	gray   = color.RGBA{R: 0x7B, G: 0x84, B: 0x91, A: 0xFF}
	white  = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	dark   = color.RGBA{R: 0x1F, G: 0x24, B: 0x2A, A: 0xFF}
)

func icon(fill color.RGBA, symbol func(*image.RGBA)) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fillCircle(img, 16, 16, 14, dark)
	fillCircle(img, 16, 16, 12, fill)
	symbol(img)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func drawCheck(img *image.RGBA) {
	thickLine(img, 9, 16, 14, 21, 4, white)
	thickLine(img, 14, 21, 24, 10, 4, white)
}

func drawLock(img *image.RGBA) {
	fillRect(img, 10, 15, 22, 24, white)
	thickLine(img, 12, 15, 12, 12, 3, white)
	thickLine(img, 20, 15, 20, 12, 3, white)
	thickLine(img, 12, 12, 20, 12, 3, white)
	fillRect(img, 15, 18, 17, 22, red)
}

func drawBang(img *image.RGBA) {
	thickLine(img, 16, 8, 16, 19, 4, white)
	fillCircle(img, 16, 24, 2, white)
}

func drawDash(img *image.RGBA) {
	thickLine(img, 10, 16, 22, 16, 4, white)
}

func fillCircle(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	r2 := radius * radius
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillRect(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func thickLine(img *image.RGBA, x1, y1, x2, y2, width int, c color.RGBA) {
	dx := abs(x2 - x1)
	dy := -abs(y2 - y1)
	sx := -1
	if x1 < x2 {
		sx = 1
	}
	sy := -1
	if y1 < y2 {
		sy = 1
	}
	err := dx + dy
	for {
		fillCircle(img, x1, y1, width/2, c)
		if x1 == x2 && y1 == y2 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x1 += sx
		}
		if e2 <= dx {
			err += dx
			y1 += sy
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
