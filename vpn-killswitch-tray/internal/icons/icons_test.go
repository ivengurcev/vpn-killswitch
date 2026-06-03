package icons

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIconsAreValidPNG(t *testing.T) {
	for name, data := range map[string][]byte{
		"ok":      OK,
		"locked":  Locked,
		"warning": Warning,
		"offline": Offline,
	} {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s icon is not valid png: %v", name, err)
		}
		if got := img.Bounds().Dx(); got != 32 {
			t.Fatalf("%s icon width = %d", name, got)
		}
		if got := img.Bounds().Dy(); got != 32 {
			t.Fatalf("%s icon height = %d", name, got)
		}
	}
}
