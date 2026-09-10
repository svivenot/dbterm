package db

import "testing"

func TestFormatMSSQLGuid(t *testing.T) {
	// Canonical SQL Server byte order: the first three groups are little-endian.
	raw := []byte{
		0xff, 0x19, 0x96, 0x6f,
		0x86, 0x8b,
		0x11, 0xd0,
		0xb4, 0x2d,
		0x00, 0xcf, 0x4f, 0xc9, 0x64, 0xff,
	}
	want := "6F9619FF-8B86-D011-B42D-00CF4FC964FF"
	if got := formatMSSQLGuid(raw); got != want {
		t.Fatalf("formatMSSQLGuid = %q, want %q", got, want)
	}

	// A non-16-byte slice must be returned unchanged (defensive).
	if got := formatMSSQLGuid([]byte("abc")); got != "abc" {
		t.Errorf("expected passthrough for non-16-byte input, got %q", got)
	}
}
