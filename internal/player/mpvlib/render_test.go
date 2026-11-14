package mpvlib

import "testing"

func TestOpaqueSetsAlpha(t *testing.T) {
	pix := []byte{1, 2, 3, 0, 4, 5, 6, 9}
	opaque(pix)
	if pix[0] != 1 || pix[1] != 2 || pix[2] != 3 || pix[3] != 0xff {
		t.Fatalf("first pixel %v", pix[:4])
	}
	if pix[4] != 4 || pix[7] != 0xff {
		t.Fatalf("second pixel %v", pix[4:])
	}
}
