; Custom NSIS macros for Hermind installer
; Backs up and restores the .hermind user data directory across reinstalls.

!macro customInit
  ; If an existing installation has a .hermind directory, back it up
  ; before the installer/uninstaller wipes the installation directory.
  IfFileExists "$INSTDIR\.hermind" 0 customInit_done
    CreateDirectory "$TEMP\HermindBackup"
    ; Remove any stale backup from a previous interrupted install
    RMDir /r "$TEMP\HermindBackup\.hermind"
    Rename "$INSTDIR\.hermind" "$TEMP\HermindBackup\.hermind"
  customInit_done:
!macroend

!macro customInstall
  ; After installation is complete, restore the backed-up .hermind
  ; into the (possibly new) installation directory.
  IfFileExists "$TEMP\HermindBackup\.hermind" 0 customInstall_done
    CreateDirectory "$INSTDIR"
    Rename "$TEMP\HermindBackup\.hermind" "$INSTDIR\.hermind"
    RMDir "$TEMP\HermindBackup"
  customInstall_done:
!macroend
