package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// A 3x5 pixel font for the tray icon. Pixel fonts stay sharp at tray sizes
// (16–32 px), where anti-aliased text turns to mush.
var glyphs = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", "###", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	'-': {"...", "...", "###", "...", "..."},
}

var (
	colGreen = color.RGBA{52, 199, 89, 255}
	colAmber = color.RGBA{255, 159, 10, 255}
	colRed   = color.RGBA{255, 59, 48, 255}
	colGrey  = color.RGBA{142, 142, 147, 255}
)

// ColorFor picks the badge colour for a battery level.
func ColorFor(level int, stale bool) color.RGBA {
	switch {
	case level < 0 || stale:
		return colGrey
	case level < LowBattery:
		return colRed
	case level < 40:
		return colAmber
	default:
		return colGreen
	}
}

// renderBadge draws text in white on a rounded coloured square.
func renderBadge(text string, bg color.RGBA, size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	radius := size / 5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if insideRounded(x, y, size, radius) {
				img.SetRGBA(x, y, bg)
			}
		}
	}

	runes := []rune(text)
	cols := len(runes)*4 - 1
	pad := size / 8
	scale := (size - 2*pad) / cols
	if s := (size - 2*pad) / 5; s < scale {
		scale = s
	}
	if scale < 1 {
		scale = 1
	}
	x0 := (size - cols*scale) / 2
	y0 := (size - 5*scale) / 2
	white := color.RGBA{255, 255, 255, 255}
	for i, r := range runes {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		for gy, row := range g {
			for gx, c := range row {
				if c != '#' {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						img.SetRGBA(x0+(i*4+gx)*scale+dx, y0+gy*scale+dy, white)
					}
				}
			}
		}
	}
	return img
}

func insideRounded(x, y, size, r int) bool {
	cx, cy := x, y
	switch {
	case x < r:
		cx = r
	case x >= size-r:
		cx = size - r - 1
	}
	switch {
	case y < r:
		cy = r
	case y >= size-r:
		cy = size - r - 1
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// MakeIcon returns a Windows .ico file (PNG-compressed frames at several
// sizes) showing text on a coloured badge.
func MakeIcon(text string, bg color.RGBA) []byte {
	sizes := []int{16, 24, 32, 48}
	frames := make([][]byte, len(sizes))
	for i, s := range sizes {
		var b bytes.Buffer
		_ = png.Encode(&b, renderBadge(text, bg, s))
		frames[i] = b.Bytes()
	}

	var out bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&out, le, [3]uint16{0, 1, uint16(len(sizes))}) // ICONDIR
	offset := 6 + 16*len(sizes)
	for i, s := range sizes { // ICONDIRENTRY
		out.WriteByte(byte(s))
		out.WriteByte(byte(s))
		out.WriteByte(0)                       // palette colours
		out.WriteByte(0)                       // reserved
		_ = binary.Write(&out, le, uint16(1))  // planes
		_ = binary.Write(&out, le, uint16(32)) // bits per pixel
		_ = binary.Write(&out, le, uint32(len(frames[i])))
		_ = binary.Write(&out, le, uint32(offset))
		offset += len(frames[i])
	}
	for _, f := range frames {
		out.Write(f)
	}
	return out.Bytes()
}
