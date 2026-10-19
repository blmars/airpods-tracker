"""
AirPods Battery — a Windows system-tray app.

AirPods continuously broadcast a small Bluetooth LE advertisement (Apple's
"proximity pairing" message) that contains the battery level of each bud and
the case. Windows doesn't show it, but any app listening for BLE adverts can
read it. This app does that and shows the result in the system tray.

- Tray icon shows the lowest bud's battery, colour-coded.
- Hover for left / right / case; click for the full menu.
- Windows notification when a bud drops below the low-battery threshold.
- Logs readings to %APPDATA%\\AirPodsBattery\\history.csv every few minutes.
"""

import asyncio
import csv
import datetime
import os
import sys
import threading
import time
from collections import deque

# ---------------------------------------------------------------- settings
LOW_BATTERY = 20          # % — notify below this
LOW_RESET = 25            # % — re-arm notification once back above this
RSSI_MIN = -75            # ignore weaker signals (other people's AirPods)
PICK_WINDOW = 15          # s — choose the strongest signal seen in this window
STALE_AFTER = 60          # s — no adverts for this long = "last seen"
LOG_EVERY = 5 * 60        # s — history log interval
REFRESH = 2               # s — tray refresh interval

APPLE_COMPANY_ID = 0x004C
MODELS = {
    0x0220: "AirPods (1st gen)",
    0x0F20: "AirPods (2nd gen)",
    0x1320: "AirPods (3rd gen)",
    0x0E20: "AirPods Pro",
    0x1420: "AirPods Pro (2nd gen)",
    0x2420: "AirPods Pro (2nd gen, USB-C)",
    0x0A20: "AirPods Max",
}

DATA_DIR = os.path.join(os.environ.get("APPDATA", os.path.expanduser("~")), "AirPodsBattery")
HISTORY = os.path.join(DATA_DIR, "history.csv")
STARTUP_FILE = os.path.join(
    os.environ.get("APPDATA", ""),
    r"Microsoft\Windows\Start Menu\Programs\Startup",
    "AirPods Battery.vbs",
)


# ---------------------------------------------------------------- parsing
def parse(data: bytes):
    """Decode Apple's proximity-pairing advert (manufacturer data after the
    0x004C company ID). Returns a dict or None if it isn't an AirPods status."""
    if len(data) != 27 or data[0] != 0x07:
        return None
    model = (data[3] << 8) | data[4]
    flipped = (data[5] & 0x20) == 0          # which nibble is the left bud
    hi, lo = data[6] >> 4, data[6] & 0x0F
    left, right = (hi, lo) if flipped else (lo, hi)
    charge = data[7] >> 4
    case = data[7] & 0x0F

    def pct(v):
        return min(v * 10, 100) if v <= 10 else None   # 15 = not available

    return {
        "model": MODELS.get(model, "AirPods"),
        "left": pct(left),
        "right": pct(right),
        "case": pct(case),
        "left_charging": bool(charge & (0b0010 if flipped else 0b0001)),
        "right_charging": bool(charge & (0b0001 if flipped else 0b0010)),
        "case_charging": bool(charge & 0b0100),
    }


# ---------------------------------------------------------------- state
class State:
    def __init__(self):
        self.lock = threading.Lock()
        self.recent = deque(maxlen=200)   # (time, rssi, reading)
        self.last = None                  # last chosen reading
        self.last_time = 0.0
        self.error = None

    def add(self, reading, rssi):
        now = time.time()
        with self.lock:
            self.recent.append((now, rssi, reading))
            while self.recent and now - self.recent[0][0] > PICK_WINDOW:
                self.recent.popleft()
            self.last = max(self.recent, key=lambda r: r[1])[2]
            self.last_time = now
            self.error = None

    def snapshot(self):
        with self.lock:
            return self.last, self.last_time, self.error


# ---------------------------------------------------------------- scanning
def scanner_thread(state: State):
    from bleak import BleakScanner

    def on_advert(_device, adv):
        data = adv.manufacturer_data.get(APPLE_COMPANY_ID)
        if not data or adv.rssi is None or adv.rssi < RSSI_MIN:
            return
        reading = parse(bytes(data))
        if reading and (reading["left"] is not None or reading["right"] is not None):
            state.add(reading, adv.rssi)

    async def run():
        while True:
            try:
                async with BleakScanner(detection_callback=on_advert):
                    while True:
                        await asyncio.sleep(1)
            except Exception as e:  # Bluetooth off, adapter busy, etc.
                with state.lock:
                    state.error = str(e) or type(e).__name__
                await asyncio.sleep(10)

    asyncio.run(run())


# ---------------------------------------------------------------- helpers
def fmt(value, charging):
    if value is None:
        return "—"
    return f"{value}%" + (" ⚡" if charging else "")


def lowest_bud(r):
    vals = [v for v in (r["left"], r["right"]) if v is not None]
    return min(vals) if vals else None


def ago(seconds):
    if seconds < 90:
        return "just now"
    if seconds < 3600:
        return f"{int(seconds // 60)} min ago"
    return f"{int(seconds // 3600)} h ago"


def make_icon(text, color):
    from PIL import Image, ImageDraw, ImageFont
    img = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.rounded_rectangle((1, 1, 63, 63), radius=14, fill=color)
    size = 40 if len(text) <= 2 else 30
    font = None
    for name in ("segoeuib.ttf", "arialbd.ttf", "arial.ttf"):
        try:
            font = ImageFont.truetype(name, size)
            break
        except OSError:
            pass
    if font is None:
        font = ImageFont.load_default()
    box = d.textbbox((0, 0), text, font=font)
    w, h = box[2] - box[0], box[3] - box[1]
    d.text(((64 - w) / 2 - box[0], (64 - h) / 2 - box[1]), text, font=font, fill="white")
    return img


def color_for(pct, stale):
    if pct is None or stale:
        return (142, 142, 147)
    if pct < LOW_BATTERY:
        return (255, 59, 48)
    if pct < 40:
        return (255, 159, 10)
    return (52, 199, 89)


def log_reading(r):
    os.makedirs(DATA_DIR, exist_ok=True)
    new = not os.path.exists(HISTORY)
    with open(HISTORY, "a", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        if new:
            w.writerow(["timestamp", "model", "left", "right", "case",
                        "left_charging", "right_charging", "case_charging"])
        w.writerow([datetime.datetime.now().isoformat(timespec="seconds"), r["model"],
                    r["left"], r["right"], r["case"],
                    r["left_charging"], r["right_charging"], r["case_charging"]])


def startup_enabled(_item=None):
    return os.path.exists(STARTUP_FILE)


def toggle_startup(icon, _item):
    if startup_enabled():
        os.remove(STARTUP_FILE)
    else:
        pyw = os.path.join(os.path.dirname(sys.executable), "pythonw.exe")
        if not os.path.exists(pyw):
            pyw = sys.executable
        script = os.path.abspath(__file__)
        with open(STARTUP_FILE, "w", encoding="utf-8") as f:
            f.write(f'CreateObject("WScript.Shell").Run """{pyw}"" ""{script}""", 0, False\r\n')
    icon.update_menu()


def single_instance():
    try:
        import ctypes
        ctypes.windll.kernel32.CreateMutexW(None, False, "AirPodsBatteryTrayApp")
        return ctypes.windll.kernel32.GetLastError() != 183  # ERROR_ALREADY_EXISTS
    except Exception:
        return True


# ---------------------------------------------------------------- main
def main():
    if not single_instance():
        return
    import pystray
    from pystray import MenuItem as Item

    state = State()
    threading.Thread(target=scanner_thread, args=(state,), daemon=True).start()

    def line(key, label):
        def text(_item):
            r, _, _ = state.snapshot()
            if not r:
                return f"{label}: —"
            return f"{label}: {fmt(r[key], r[key + '_charging'])}"
        return text

    def header(_item):
        r, t, err = state.snapshot()
        if err and not r:
            return f"Bluetooth error: {err[:60]}"
        if not r:
            return "Searching for AirPods…"
        return f"{r['model']} · {ago(time.time() - t)}"

    def open_history(icon, _item):
        if os.path.exists(HISTORY):
            os.startfile(HISTORY)
        else:
            icon.notify("No readings logged yet.", "AirPods Battery")

    def quit_app(icon, _item):
        icon.stop()

    icon = pystray.Icon(
        "AirPodsBattery",
        make_icon("–", color_for(None, True)),
        "AirPods Battery — searching…",
        menu=pystray.Menu(
            Item(header, None, enabled=False),
            Item(line("left", "Left"), None, enabled=False),
            Item(line("right", "Right"), None, enabled=False),
            Item(line("case", "Case"), None, enabled=False),
            pystray.Menu.SEPARATOR,
            Item("Open battery history", open_history),
            Item("Run at Windows startup", toggle_startup, checked=startup_enabled),
            Item("Quit", quit_app),
        ),
    )

    def refresher():
        armed = {"left": True, "right": True}
        last_log = 0.0
        last_key = None
        while True:
            time.sleep(REFRESH)
            r, t, err = state.snapshot()
            now = time.time()
            stale = r is None or now - t > STALE_AFTER

            if r:
                low = lowest_bud(r)
                text = str(low) if low is not None else "–"
                tip = (f"{r['model']}\nL {fmt(r['left'], r['left_charging'])}  "
                       f"R {fmt(r['right'], r['right_charging'])}  "
                       f"Case {fmt(r['case'], r['case_charging'])}")
                if stale:
                    tip += f"\nLast seen {ago(now - t)}"
            else:
                low, text = None, "–"
                tip = "AirPods Battery — " + ("Bluetooth error" if err else "searching…")

            key = (text, color_for(low, stale), tip)
            if key != last_key:
                icon.icon = make_icon(text, key[1])
                icon.title = tip[:127]
                icon.update_menu()
                last_key = key

            if r and not stale:
                for side in ("left", "right"):
                    v, chg = r[side], r[side + "_charging"]
                    if v is None:
                        continue
                    if armed[side] and v < LOW_BATTERY and not chg:
                        icon.notify(f"{side.capitalize()} AirPod is at {v}%.", "AirPods battery low")
                        armed[side] = False
                    elif v >= LOW_RESET or chg:
                        armed[side] = True
                if now - last_log >= LOG_EVERY:
                    try:
                        log_reading(r)
                    except OSError:
                        pass
                    last_log = now

    def setup(icon):
        icon.visible = True
        threading.Thread(target=refresher, daemon=True).start()

    icon.run(setup=setup)


if __name__ == "__main__":
    main()
