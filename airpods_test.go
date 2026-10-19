package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
	"time"
)

func packet(model uint16, status, pods, chargeCase byte) []byte {
	p := make([]byte, 27)
	p[0], p[1], p[2] = 0x07, 0x19, 0x01
	p[3], p[4] = byte(model>>8), byte(model)
	p[5], p[6], p[7] = status, pods, chargeCase
	return p
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Reading
	}{
		{
			// Not flipped: left = low nibble. Case charging at 50%.
			"normal", packet(0x0E20, 0x2B, 0x87, 0x45),
			Reading{Model: "AirPods Pro", Left: 70, Right: 80, Case: 50, CaseCharging: true},
		},
		{
			// Flipped: left = high nibble; left charging; case unavailable.
			"flipped", packet(0x1420, 0x0B, 0x87, 0x2F),
			Reading{Model: "AirPods Pro (2nd gen)", Left: 80, Right: 70, Case: -1, LeftCharging: true},
		},
		{
			"unknown model, full", packet(0xBEEF, 0x20, 0xAA, 0x0A),
			Reading{Model: "AirPods", Left: 100, Right: 100, Case: 100},
		},
	}
	for _, tt := range tests {
		got, ok := Parse(tt.data)
		if !ok || got != tt.want {
			t.Errorf("%s: got %+v (ok=%v), want %+v", tt.name, got, ok, tt.want)
		}
	}
	if _, ok := Parse([]byte{0x10, 0x05}); ok {
		t.Error("non-AirPods advert should not parse")
	}
}

func TestLowestBud(t *testing.T) {
	for _, c := range []struct{ l, r, want int }{{70, 80, 70}, {-1, 40, 40}, {30, -1, 30}, {-1, -1, -1}} {
		if got := (Reading{Left: c.l, Right: c.r}).LowestBud(); got != c.want {
			t.Errorf("LowestBud(%d,%d) = %d, want %d", c.l, c.r, got, c.want)
		}
	}
}

func TestStatePicksStrongestSignal(t *testing.T) {
	s := NewState(PickWindow)
	now := time.Now()
	s.Add(Reading{Model: "mine", Left: 50}, -45, now)
	s.Add(Reading{Model: "neighbour", Left: 90}, -70, now.Add(time.Second))
	if r, _, _ := s.Snapshot(); r.Model != "mine" {
		t.Errorf("picked %q, want the stronger signal", r.Model)
	}
	// Once the strong reading ages out of the window, the other one wins.
	s.Add(Reading{Model: "neighbour", Left: 90}, -70, now.Add(PickWindow+2*time.Second))
	if r, _, _ := s.Snapshot(); r.Model != "neighbour" {
		t.Errorf("picked %q after window expired", r.Model)
	}
}

func TestMakeIcon(t *testing.T) {
	for _, text := range []string{"-", "80", "100"} {
		ico := MakeIcon(text, colGreen)
		var hdr [3]uint16
		_ = binary.Read(bytes.NewReader(ico), binary.LittleEndian, &hdr)
		if hdr[0] != 0 || hdr[1] != 1 || hdr[2] != 4 {
			t.Fatalf("bad ICO header %v", hdr)
		}
		off := binary.LittleEndian.Uint32(ico[6+12 : 6+16])
		size := binary.LittleEndian.Uint32(ico[6+8 : 6+12])
		img, err := png.Decode(bytes.NewReader(ico[off : off+size]))
		if err != nil || img.Bounds().Dx() != 16 {
			t.Fatalf("first frame: %v, %v", err, img)
		}
	}
}
