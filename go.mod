module github.com/blmar/airpods-tracker

go 1.22

require (
	fyne.io/systray v1.11.0
	golang.org/x/sys v0.30.0
	tinygo.org/x/bluetooth v0.12.0
)

// The non-Windows entries here are never compiled into the app; they are
// listed so `go mod vendor` (run by the SLSA release builder) can resolve
// every platform's imports.
require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/saltosystems/winrt-go v0.0.0-20240509164145-4f7860a3bd2b // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/soypat/cyw43439 v0.0.0-20250505012923-830110c8f4af // indirect
	github.com/soypat/seqs v0.0.0-20250124201400-0d65bc7c1710 // indirect
	github.com/tinygo-org/cbgo v0.0.4 // indirect
	github.com/tinygo-org/pio v0.2.0 // indirect
	golang.org/x/exp v0.0.0-20240808152545-0cdaa3abc0fa // indirect
)

// Vanity import paths are pinned to their GitHub mirrors (same upstream
// sources) so the build doesn't depend on the vanity domains being reachable.
replace (
	fyne.io/systray => github.com/fyne-io/systray v1.11.0
	golang.org/x/crypto => github.com/golang/crypto v0.12.0
	golang.org/x/exp => github.com/golang/exp v0.0.0-20240808152545-0cdaa3abc0fa
	golang.org/x/sys => github.com/golang/sys v0.30.0
	golang.org/x/term => github.com/golang/term v0.11.0
	tinygo.org/x/bluetooth => github.com/tinygo-org/bluetooth v0.12.0
	tinygo.org/x/drivers => github.com/tinygo-org/drivers v0.28.1-0.20241028090715-76a4276b5dea
	tinygo.org/x/tinyfont => github.com/tinygo-org/tinyfont v0.4.0
	tinygo.org/x/tinyterm => github.com/tinygo-org/tinyterm v0.3.1-0.20241028084705-e36d93d72cca

	// Needed only by winrt-go's code generator, never compiled into this app.
	github.com/tdakkota/win32metadata => ./third_party/stub/win32metadata
	golang.org/x/mod => ./third_party/stub/mod
	golang.org/x/tools => ./third_party/stub/tools
	gopkg.in/yaml.v3 => ./third_party/stub/yaml.v3
)
