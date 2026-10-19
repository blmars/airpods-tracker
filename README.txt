AirPods Battery for Windows
===========================

A small system-tray app that shows your AirPods' battery levels (left, right
and case) on Windows.

How to run
----------
1. Unzip this folder somewhere permanent (e.g. Documents\AirPods Battery).
2. Double-click "Start AirPods Battery.bat".
   - If Python isn't installed, it opens the download page. Install Python 3,
     tick "Add python.exe to PATH", then double-click the .bat again.
3. Look for the coloured number in the system tray (you may need to click the
   ^ arrow and drag it onto the taskbar so it's always visible).

What you'll see
---------------
- Tray icon: the lower of your two buds' battery %, green / amber / red.
  Grey means it hasn't heard from your AirPods recently.
- Hover: left, right and case levels (a lightning bolt = charging).
- Right-click: details, "Open battery history", "Run at Windows startup", Quit.
- A Windows notification when a bud drops below 20%.
- History is logged every 5 minutes to %APPDATA%\AirPodsBattery\history.csv.

Good to know
------------
- Your PC needs Bluetooth (BLE) turned on. The AirPods don't need to be paired
  with the PC: the app just listens to the battery broadcast AirPods send out.
- AirPods report battery in 10% steps, the same as on an iPhone.
- The case level is only broadcast while at least one bud is in the case,
  and AirPods go quiet when they're in a closed case — the app then shows the
  last known level in grey.
- If someone else's AirPods are nearby, the app picks the strongest signal,
  which is almost always yours. Settings (thresholds, intervals) are at the
  top of airpods_battery.pyw.
