package main

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestKittyTilesResendOnlyTheChangedRectangle(t *testing.T) {
	cell := CharSize{Width: 1, Height: 1} // tiles are kittyTileCols x kittyTileRows pixels
	first := image.NewRGBA(image.Rect(0, 0, 3*kittyTileCols, 3*kittyTileRows))
	for i := range first.Pix {
		first.Pix[i] = 200
	}
	var tiler kittyTiler
	a, err := tiler.prepare(first, nil, cell)
	if err != nil {
		t.Fatal(err)
	}
	if a.GridCols != 3 || len(a.Tiles) != 9 {
		t.Fatalf("grid = %d cols, %d tiles; want 3, 9", a.GridCols, len(a.Tiles))
	}

	second := image.NewRGBA(first.Bounds())
	copy(second.Pix, first.Pix)
	second.SetRGBA(2*kittyTileCols+1, kittyTileRows+1, color.RGBA{1, 2, 3, 255}) // grid row 1, col 2
	b, err := tiler.prepare(second, first, cell)
	if err != nil {
		t.Fatal(err)
	}
	for i := range b.Tiles {
		changed := a.Tiles[i].Serial != b.Tiles[i].Serial
		if changed != (i == 5) {
			t.Fatalf("tile %d changed=%v; only tile 5 should change", i, changed)
		}
	}

	shown := make([]uint64, len(a.Tiles))
	for i, tile := range a.Tiles {
		shown[i] = tile.Serial
	}
	var out bytes.Buffer
	next, err := writeKittyTiles(&out, b, shown, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Count(text, "a=T") != 1 || !strings.Contains(text, "i=1005,") {
		t.Fatalf("expected exactly tile 1005 to be sent, got %q", text)
	}
	if !strings.Contains(text, "\033[14;34H") { // row 10 + 1*rows, column 2 + 2*cols
		t.Fatalf("tile 5 was not positioned at its cell: %q", text)
	}
	if !strings.HasPrefix(text, "\033[?2026h") || !strings.HasSuffix(text, "\033[?2026l") {
		t.Fatal("tile update is not a synchronized update")
	}
	if next[5] != b.Tiles[5].Serial {
		t.Fatal("shown serials not updated")
	}

	out.Reset()
	if _, err := writeKittyTiles(&out, kittyTiles{GridCols: 3, Tiles: b.Tiles[:6]}, next, 10, 2); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"i=1006,", "i=1007,", "i=1008,"} {
		if !strings.Contains(out.String(), "a=d,d=I,"+id) {
			t.Fatalf("shrinking the grid did not delete %s: %q", id, out.String())
		}
	}
}
