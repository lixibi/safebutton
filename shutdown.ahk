#Requires AutoHotkey v2.0
#SingleInstance Force

armed := false
armTimeoutMs := 10000

; Step 1: Press "Browser_Favorites" to disarm the safety lock
Browser_Favorites:: {
    global armed, armTimeoutMs

    armed := true
    SetTimer(Disarm, -armTimeoutMs) ; Starts the 10-second countdown

    TrayTip("Safety lock disarmed. Please press the second button within 10 seconds.", "Armed")
}

; Step 2: Press "Browser_Search" within 10 seconds to trigger shutdown
Browser_Search:: {
    global armed

    if armed {
        armed := false
        SetTimer(Disarm, 0) ; Turns off the timer

        ; Executes Windows Shutdown command
        ; 1 = Shutdown
        ; Note: If you want to force close running apps without saving prompts, use Shutdown(1 + 4) instead.
        Shutdown(1)
    } else {
        TrayTip("Please unlock the safety switch first (Step 1).", "Locked")
    }
}

; Timeout function to re-lock the system
Disarm() {
    global armed

    if armed {
        armed := false
        TrayTip("Operation timed out. System has been re-locked.", "Safety Lock Engaged")
    }
}