# AirPods Battery for Windows

A small system-tray app that shows your AirPods' battery levels (left, right
and case) on Windows. Written in Go, it builds to a single `.exe` with no
runtime to install.

## Run it

Double-click `AirPodsBattery.exe`. Look for the coloured number in the system
tray (you may need to click the `^` arrow and drag it onto the taskbar so it's
always visible).

## Build it

Install [Go](https://go.dev/dl/) 1.22 or newer, then double-click `build.bat`,
or run:

```
go build -trimpath -ldflags "-H windowsgui -s -w" -o AirPodsBattery.exe .
```

`-H windowsgui` stops a console window from opening alongside the tray icon.
Run the tests with `go test ./...`.

## What you'll see

- **Tray icon:** the lower of your two buds' battery %, green / amber / red.
  Grey means it hasn't heard from your AirPods recently.
- **Hover:** left, right and case levels (⚡ = charging).
- **Right-click:** details, *Open battery history*, *Run at Windows startup*,
  *Quit*.
- A Windows notification when a bud drops below 20%.
- History is logged every 5 minutes to `%APPDATA%\AirPodsBattery\history.csv`.

## How it works

AirPods continuously broadcast a Bluetooth LE advertisement (Apple's
"proximity pairing" message) carrying the battery of each bud and the case.
The app listens for these with Windows' own Bluetooth stack, so the AirPods
don't need to be paired with the PC Bluetooth just has to be on.

- AirPods report battery in 10% steps.
- The case level is only broadcast while at least one bud is in the case, and
  AirPods go quiet in a closed case; the app then shows the last known level
  in grey.
- If someone else's AirPods are nearby, the app picks the strongest signal,
  which is almost always yours.

Thresholds and intervals are constants at the top of `airpods.go`.

## Files

| File | What it is |
| --- | --- |
| `main.go` | Tray UI, Bluetooth scanning, notifications, history log, startup setting (Windows only) |
| `airpods.go` | Advert decoding and signal-strength selection |
| `icon.go` | Draws the tray badge and packs it as an `.ico` |
| `airpods_test.go` | Tests for decoding, selection and icon output |
| `third_party/stub` | Empty placeholders for modules only a dependency's code generator uses |
