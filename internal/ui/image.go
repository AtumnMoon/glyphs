package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/sixel"
)

// Two ways to show a picture in the preview:
//
//   - sixel: a real bitmap, sized in pixels to fill the preview area. Used when
//     the terminal answers the startup queries (sixel support + cell size in
//     pixels), e.g. foot, WezTerm, mlterm, xterm -ti vt340.
//   - basic: "▀" half blocks with 24-bit colour (two image pixels per cell), a
//     coarse picture that works in any true-colour terminal.

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// fitSize scales (iw,ih) to fit inside (maxW,maxH) keeping the aspect ratio.
// Small images are never enlarged.
func fitSize(iw, ih, maxW, maxH int) (int, int) {
	if iw <= 0 || ih <= 0 || maxW <= 0 || maxH <= 0 {
		return 0, 0
	}
	scale := min(float64(maxW)/float64(iw), float64(maxH)/float64(ih), 1)
	return max(int(float64(iw)*scale), 1), max(int(float64(ih)*scale), 1)
}

// scale resamples src to w×h by averaging every source pixel that falls in a
// destination pixel (a box filter), which keeps text in screenshots legible
// when shrinking a lot. Transparent pixels are composited over bg.
func scale(src image.Image, w, h int, bg color.Color) *image.RGBA {
	b := src.Bounds()
	iw, ih := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	br, bgc, bb, _ := bg.RGBA()
	for y := 0; y < h; y++ {
		y0, y1 := y*ih/h, max((y+1)*ih/h, y*ih/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*iw/w, max((x+1)*iw/w, x*iw/w+1)
			var rs, gs, bs, n uint64
			for yy := y0; yy < y1 && yy < ih; yy++ {
				for xx := x0; xx < x1 && xx < iw; xx++ {
					r, g, bl, a := src.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					// composite over the background (premultiplied alpha)
					r += (0xffff - a) * br / 0xffff
					g += (0xffff - a) * bgc / 0xffff
					bl += (0xffff - a) * bb / 0xffff
					rs += uint64(r >> 8)
					gs += uint64(g >> 8)
					bs += uint64(bl >> 8)
					n++
				}
			}
			if n == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(rs / n), uint8(gs / n), uint8(bs / n), 255})
		}
	}
	return dst
}

// ---- basic (half-block) ----------------------------------------------------

// renderBasic draws path with half blocks into at most w×h cells.
func renderBasic(path string, w, h int) string {
	img, err := loadImage(path)
	if err != nil {
		return "(cannot read image)"
	}
	b := img.Bounds()
	tw, th := fitSize(b.Dx(), b.Dy(), w, h*2)
	if tw == 0 {
		return ""
	}
	th += th % 2 // whole cells
	px := scale(img, tw, th, color.Black)

	var sb strings.Builder
	for y := 0; y < th; y += 2 {
		for x := 0; x < tw; x++ {
			top, bot := px.RGBAAt(x, y), px.RGBAAt(x, min(y+1, th-1))
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top.R, top.G, top.B, bot.R, bot.G, bot.B)
		}
		sb.WriteString("\x1b[0m")
		if y+2 < th {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// ---- sixel ------------------------------------------------------------------

// sixelPaint returns the escape sequence that draws path as a sixel image
// whose top-left corner is cell (col,row) (1-based), fitting inside
// areaCols×areaRows cells of cellW×cellH pixels. The area is blanked first so
// a previous, larger image never shows through. The cursor is saved/restored
// so Bubble Tea's renderer is undisturbed.
func sixelPaint(path string, col, row, areaCols, areaRows, cellW, cellH int) (string, error) {
	img, err := loadImage(path)
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	pw, ph := fitSize(b.Dx(), b.Dy(), areaCols*cellW, areaRows*cellH)
	if pw == 0 {
		return "", fmt.Errorf("empty image")
	}
	// Terminals draw sixel in 6-pixel bands; trim so the bitmap never spills
	// below the area.
	if ph > 6 {
		ph -= ph % 6
	}
	px := image.NewRGBA(image.Rect(0, 0, pw, ph))
	draw.Draw(px, px.Bounds(), scale(img, pw, ph, color.Black), image.Point{}, draw.Src)

	var payload bytes.Buffer
	var enc sixel.Encoder
	if err := enc.Encode(&payload, px); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\x1b7") // save cursor
	blank := strings.Repeat(" ", areaCols)
	for r := 0; r < areaRows; r++ {
		sb.WriteString(ansi.CursorPosition(col, row+r))
		sb.WriteString(blank)
	}
	sb.WriteString(ansi.CursorPosition(col, row))
	sb.WriteString(ansi.SixelGraphics(0, 1, 0, payload.Bytes()))
	sb.WriteString("\x1b8") // restore cursor
	return sb.String(), nil
}
