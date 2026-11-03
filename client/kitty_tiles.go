package main

import (
	"bytes"
	"fmt"
	"image"
	"io"
)

// Kitty frames are split into a grid of tiles aligned to terminal cells, each
// its own image. Only tiles whose pixels changed are retransmitted, so a caret
// blink or a typed character replaces one small rectangle instead of the page.
// Multiplexers that composite pane images (herdr) then swap small images.
//
// shortcut: fixed tile size; tune kittyTileRows/kittyTileCols if many small
// placements cost more than they save in a given multiplexer.
const (
	kittyTileRows   = 4
	kittyTileCols   = 16
	kittyTileBaseID = 1000
)

// A tile's Serial changes whenever its pixels do. Payloads are heap-owned and
// immutable, so frames that keep a tile unchanged share the same bytes.
type kittyTile struct {
	Serial  uint64
	Payload []byte
}

// kittyTiles is one frame's grid, row-major, GridCols tiles per grid row.
type kittyTiles struct {
	GridCols int
	Tiles    []kittyTile
}

// Owned by the preparation worker. last mirrors the last prepared image.
type kittyTiler struct {
	encoder kittyEncoder
	last    kittyTiles
	tileW   int
	tileH   int
	serial  uint64
}

func (t *kittyTiler) prepare(img, previous *image.RGBA, cell CharSize) (kittyTiles, error) {
	bounds := img.Bounds()
	tileW, tileH := bounds.Dx(), bounds.Dy()
	if cell.Width > 0 && cell.Height > 0 {
		tileW, tileH = kittyTileCols*cell.Width, kittyTileRows*cell.Height
	}
	cols := (bounds.Dx() + tileW - 1) / tileW
	rows := (bounds.Dy() + tileH - 1) / tileH
	reuse := previous != nil && previous.Bounds() == bounds && t.tileW == tileW && t.tileH == tileH && len(t.last.Tiles) == cols*rows
	next := kittyTiles{GridCols: cols, Tiles: make([]kittyTile, cols*rows)}
	for i := range next.Tiles {
		x := bounds.Min.X + (i%cols)*tileW
		y := bounds.Min.Y + (i/cols)*tileH
		rect := image.Rect(x, y, min(x+tileW, bounds.Max.X), min(y+tileH, bounds.Max.Y))
		if reuse && !rectChanged(img, previous, rect) {
			next.Tiles[i] = t.last.Tiles[i]
			continue
		}
		tile := img.SubImage(rect).(*image.RGBA)
		payload, err := t.encoder.encodeRGBControl(tile, fmt.Sprintf("a=T,i=%d,p=1,C=1,q=2", kittyTileBaseID+i), nil)
		if err != nil {
			return kittyTiles{}, err
		}
		t.serial++
		next.Tiles[i] = kittyTile{Serial: t.serial, Payload: payload}
	}
	t.last, t.tileW, t.tileH = next, tileW, tileH
	return kittyTiles{GridCols: cols, Tiles: append([]kittyTile(nil), next.Tiles...)}, nil
}

// rectChanged compares one rectangle of two same-sized images row by row.
func rectChanged(img, previous *image.RGBA, rect image.Rectangle) bool {
	width := rect.Dx() * 4
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		a := img.PixOffset(rect.Min.X, y)
		b := previous.PixOffset(rect.Min.X, y)
		if !bytes.Equal(img.Pix[a:a+width], previous.Pix[b:b+width]) {
			return true
		}
	}
	return false
}

// Serials of the tiles currently on screen, by tile index. UI owner only.
var kittyShown []uint64

// writeKittyTiles sends only tiles whose serial differs from what is shown, in
// one synchronized update, and deletes tiles beyond the new count.
func writeKittyTiles(w io.Writer, frame kittyTiles, shown []uint64, row, column int) ([]uint64, error) {
	var out bytes.Buffer
	out.WriteString("\033[?2026h\033[s")
	for i, tile := range frame.Tiles {
		if i < len(shown) && shown[i] == tile.Serial {
			continue
		}
		r := row + (i/frame.GridCols)*kittyTileRows
		c := column + (i%frame.GridCols)*kittyTileCols
		fmt.Fprintf(&out, "\033[%d;%dH", r, c)
		out.Write(tile.Payload)
	}
	for i := len(frame.Tiles); i < len(shown); i++ {
		fmt.Fprintf(&out, "\033_Ga=d,d=I,i=%d,q=2\033\\", kittyTileBaseID+i)
	}
	out.WriteString("\033[u\033[?2026l")
	n, err := w.Write(out.Bytes())
	if err == nil && n != out.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return nil, err
	}
	next := make([]uint64, len(frame.Tiles))
	for i, tile := range frame.Tiles {
		next[i] = tile.Serial
	}
	return next, nil
}

// kittyDeleteAll removes the splash placement and every tile on screen.
func kittyDeleteAll() string {
	var out bytes.Buffer
	out.WriteString(kittyDelete)
	for i := range kittyShown {
		fmt.Fprintf(&out, "\033_Ga=d,d=I,i=%d,q=2\033\\", kittyTileBaseID+i)
	}
	kittyShown = nil
	return out.String()
}
