package main

import (
	"sync"
	"time"
)

// Settings.
const (
	LowBattery = 20               // % — notify below this
	LowReset   = 25               // % — re-arm the notification once back above this
	RSSIMin    = -75              // ignore weaker signals (other people's AirPods)
	PickWindow = 15 * time.Second // choose the strongest signal seen in this window
	StaleAfter = 60 * time.Second // no adverts for this long = show "last seen"
	LogEvery   = 5 * time.Minute  // history log interval
	Refresh    = 2 * time.Second  // tray refresh interval
)

// AppleCompanyID is Apple's Bluetooth SIG company identifier, used as the key
// for manufacturer data in BLE advertisements.
const AppleCompanyID = 0x004C

var models = map[uint16]string{
	0x0220: "AirPods (1st gen)",
	0x0F20: "AirPods (2nd gen)",
	0x1320: "AirPods (3rd gen)",
	0x0E20: "AirPods Pro",
	0x1420: "AirPods Pro (2nd gen)",
	0x2420: "AirPods Pro (2nd gen, USB-C)",
	0x0A20: "AirPods Max",
}

// Reading is one decoded AirPods status broadcast. Battery levels are
// percentages in 10% steps; -1 means the value isn't available (for example
// the case level while both buds are out).
type Reading struct {
	Model         string
	Left          int
	Right         int
	Case          int
	LeftCharging  bool
	RightCharging bool
	CaseCharging  bool
}

// Parse decodes Apple's "proximity pairing" advertisement (the manufacturer
// data that follows the 0x004C company ID). It returns false if the data
// isn't an AirPods status message.
func Parse(data []byte) (Reading, bool) {
	if len(data) != 27 || data[0] != 0x07 {
		return Reading{}, false
	}
	model := uint16(data[3])<<8 | uint16(data[4])
	flipped := data[5]&0x20 == 0 // which nibble holds the left bud
	hi, lo := int(data[6]>>4), int(data[6]&0x0F)
	left, right := lo, hi
	if flipped {
		left, right = hi, lo
	}
	charge := data[7] >> 4
	caseLevel := int(data[7] & 0x0F)

	leftBit, rightBit := byte(0b0001), byte(0b0010)
	if flipped {
		leftBit, rightBit = rightBit, leftBit
	}

	name, ok := models[model]
	if !ok {
		name = "AirPods"
	}
	return Reading{
		Model:         name,
		Left:          pct(left),
		Right:         pct(right),
		Case:          pct(caseLevel),
		LeftCharging:  charge&leftBit != 0,
		RightCharging: charge&rightBit != 0,
		CaseCharging:  charge&0b0100 != 0,
	}, true
}

func pct(v int) int {
	if v > 10 { // 15 = not available
		return -1
	}
	return v * 10
}

// HasBuds reports whether at least one bud's level is known.
func (r Reading) HasBuds() bool { return r.Left >= 0 || r.Right >= 0 }

// LowestBud returns the lower of the two buds' levels, or -1 if neither is known.
func (r Reading) LowestBud() int {
	switch {
	case r.Left < 0:
		return r.Right
	case r.Right < 0:
		return r.Left
	case r.Left < r.Right:
		return r.Left
	default:
		return r.Right
	}
}

// State keeps recent readings and picks the one with the strongest signal,
// which is almost always the user's own AirPods rather than someone nearby's.
type State struct {
	mu       sync.Mutex
	window   time.Duration
	recent   []sample
	last     *Reading
	lastTime time.Time
	err      error
}

type sample struct {
	at      time.Time
	rssi    int16
	reading Reading
}

func NewState(window time.Duration) *State { return &State{window: window} }

func (s *State) Add(r Reading, rssi int16, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, sample{now, rssi, r})
	cut := 0
	for cut < len(s.recent) && now.Sub(s.recent[cut].at) > s.window {
		cut++
	}
	s.recent = s.recent[cut:]
	if len(s.recent) > 200 {
		s.recent = s.recent[len(s.recent)-200:]
	}
	best := s.recent[0]
	for _, x := range s.recent[1:] {
		if x.rssi > best.rssi {
			best = x
		}
	}
	s.last = &best.reading
	s.lastTime = now
	s.err = nil
}

func (s *State) SetError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

// Snapshot returns the current best reading (nil if none yet), when it was
// last updated, and the latest scanner error.
func (s *State) Snapshot() (*Reading, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		return nil, s.lastTime, s.err
	}
	r := *s.last
	return &r, s.lastTime, s.err
}
