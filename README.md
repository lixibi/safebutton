# SafeButton — Two-Step Safety Shutdown Switch

A custom **AutoHotkey v2** project that turns a physical key switch and a push button into a two-step safety shutdown system for Windows. Designed for industrial or kiosk environments where an accidental shutdown must be prevented.

![Physical Button Panel](pic.jpg)

---

## Custom Specifications

This project is built to order for a client with the following requirements:

| # | Requirement | Implementation |
|---|------------|---------------|
| 1 | Key switch set to **Browser_Favorites** | The key switch sends the `Browser_Favorites` media key. |
| 2 | Red push button set to **Browser_Search** | The button sends the `Browser_Search` media key. |
| 3 | Both keys work as independent inputs | Each key is mapped to a separate AutoHotkey hotkey. |
| 4 | Settings survive USB disconnect / reconnect | The USB controller's key mapping is stored in its onboard memory — no software reconfiguration needed. |
| 5 | Keys **do not** revert to Enter | The custom USB firmware permanently maps the keys; they will not fall back to default behavior. |

---

## How It Works

The safety switch uses a **two-step arming sequence** to prevent accidental shutdown:

1. **Step 1 — Unlock (Key Switch)**  
   Turn the key switch. This sends `Browser_Favorites` to Windows.  
   - **Demo version**: A confirmation dialog asks "Are you sure?"  
   - **Production version**: The safety lock is silently disarmed.

2. **Step 2 — Execute (Red Button)**  
   Press the red button within **10 seconds**. This sends `Browser_Search` to Windows.  
   - **Demo version**: A test-success message is shown (no actual shutdown).  
   - **Production version**: Windows executes a clean shutdown via `Shutdown(1)`.

If the red button is not pressed within 10 seconds, the system **re-locks** itself automatically.

---

## Project Structure

```
safebutton/
├── README.md              # This documentation
├── pic.jpg                # Photo of the physical switch panel
├── demo.ahk               # Test / demo script (safe to run)
├── shutdown.ahk           # Production shutdown script
└── bin/                   # AutoHotkey v2 interpreters
    ├── AutoHotkey.exe         # 32-bit (recommended)
    ├── AutoHotkey64.exe       # 64-bit
    ├── AutoHotkey32_UIA.exe   # 32-bit with UI Access
    └── AutoHotkey64_UIA.exe   # 64-bit with UI Access
```

### File Descriptions

| File | Description |
|------|-------------|
| `demo.ahk` | **Test / demo script** — runs through the two-step flow with confirmation dialogs but does **not** shut down. Use this first to verify the physical buttons work correctly. |
| `shutdown.ahk` | **Production script** — silent arming + real Windows shutdown. Use this in the live environment. |
| `pic.jpg` | Photo of the physical switch panel for reference. |
| `bin/AutoHotkey.exe` | AutoHotkey v2 interpreter (32-bit). The recommended version for running the scripts. |
| `bin/AutoHotkey64.exe` | AutoHotkey v2 interpreter (64-bit). |
| `bin/AutoHotkey32_UIA.exe` | AutoHotkey v2 with UI Access (32-bit) — may help if the script interacts with elevated/admin windows. |
| `bin/AutoHotkey64_UIA.exe` | AutoHotkey v2 with UI Access (64-bit). |

---

## Usage

### 1. Run the Demo (Test Hardware)

```cmd
bin\AutoHotkey.exe demo.ahk
```

Or drag `demo.ahk` onto `bin\AutoHotkey.exe`.

**Expected behavior:**

1. Turn the **key switch** → a dialog appears: *"Are you sure you want to unlock the safety switch?"*
2. Click **Yes** → a tray tip says *"Please press the second button within 10 seconds."*
3. Press the **red button** → a message box confirms: *"Two-step verification passed!"*
4. Wait 10+ seconds and press the red button again → the tray tip says *"Please unlock the safety switch first."*

✅ If all steps work, your hardware is correctly configured.

### 2. Run the Production Script

Once verified:

```cmd
bin\AutoHotkey.exe shutdown.ahk
```

**Expected behavior:**

1. Turn the **key switch** → the safety lock disarms (tray tip: *"Safety lock disarmed"*).
2. Press the **red button** within 10 seconds → Windows begins a clean shutdown.
3. If you wait >10 seconds, the system re-locks automatically.

---

## Customization

### Change the shutdown action

In `shutdown.ahk`, the action is:

```autohotkey
Shutdown(1)
```

You can replace this with any command. For example:

| Action | Code |
|--------|------|
| Shutdown | `Shutdown(1)` |
| Shutdown (force) | `Shutdown(1 + 4)` |
| Restart | `Shutdown(2)` |
| Log off | `Shutdown(0)` |
| Run a custom program | `Run("C:\Path\to\program.exe")` |

### Adjust the timeout

Change the `armTimeoutMs` value at the top of the script:

```autohotkey
armTimeoutMs := 10000  ; 10 seconds; change to 5000 for 5 seconds, etc.
```

### Change the hotkeys

If your physical switch panel uses different keys, replace the hotkey names at the top of each script:

```autohotkey
Browser_Favorites::   ; Step 1
Browser_Search::      ; Step 2
```

Common alternatives: `F13`–`F24`, `Volume_Up`, `Launch_Media`, or any keyboard scancode.

---

## Requirements

- **Windows** (any version supported by AutoHotkey v2)
- **AutoHotkey v2** (included in the `bin/` folder)
- **USB Switch Panel** programmed to send `Browser_Favorites` (key switch) and `Browser_Search` (red button)

---

## License

This project is custom-built for a specific client. Free to adapt for your own use.
