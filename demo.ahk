#Requires AutoHotkey v2.0
#SingleInstance Force

armed := false
armTimeoutMs := 10000

; Step 1: Press "Browser_Favorites", prompts a confirmation box
Browser_Favorites:: {
    global armed, armTimeoutMs
    
    ; 4 = Yes/No buttons, 32 = Question icon
    Result := MsgBox("Are you sure you want to unlock the safety switch?", "Safety Confirmation", 4 + 32)
    
    if (Result = "Yes") {
        armed := true
        SetTimer(Disarm, -armTimeoutMs) ; Starts the 10-second countdown
        TrayTip("Please press the second button within 10 seconds.", "Safety Lock Disarmed")
    } else {
        TrayTip("Operation cancelled.", "Locked")
    }
}

; Step 2: Press "Browser_Search" within 10 seconds (Test Version: prompt only, no shutdown)
Browser_Search:: {
    global armed

    if armed {
        armed := false
        SetTimer(Disarm, 0) ; Turns off the timer

        ; Test Notice: Displaying a message box instead of triggering actual shutdown
        MsgBox("[Test Success] Two-step verification passed! If this were the production version, the system would shut down now.", "Test Result")
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