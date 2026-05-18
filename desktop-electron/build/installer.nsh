!macro customInit
  ; Backup .hermind before old uninstaller wipes it.
  ; $INSTDIR is already resolved to the previous install location by initMultiUser.
  ${If} ${FileExists} "$INSTDIR\.hermind"
    ${If} ${FileExists} "$TEMP\hermind-data-backup"
      RMDir /r "$TEMP\hermind-data-backup"
    ${EndIf}
    ExecWait 'cmd /c xcopy "$INSTDIR\.hermind" "$TEMP\hermind-data-backup\" /E /I /H /Y' $0
  ${EndIf}
!macroend

!macro customInstall
  ; Restore .hermind after installation completes
  ${If} ${FileExists} "$TEMP\hermind-data-backup"
    CreateDirectory "$INSTDIR"
    ExecWait 'cmd /c xcopy "$TEMP\hermind-data-backup" "$INSTDIR\.hermind\" /E /I /H /Y && rmdir /s /q "$TEMP\hermind-data-backup"' $0
  ${EndIf}
!macroend

!macro customRemoveFiles
  ; Clean up any stale backup first
  ${If} ${FileExists} "$TEMP\hermind-data-backup"
    RMDir /r "$TEMP\hermind-data-backup"
  ${EndIf}

  ; Backup .hermind before wiping the install directory
  ${If} ${FileExists} "$INSTDIR\.hermind"
    ExecWait 'cmd /c xcopy "$INSTDIR\.hermind" "$TEMP\hermind-data-backup\" /E /I /H /Y' $0
  ${EndIf}

  ; Delete the install directory (may fail for locked files like the uninstaller itself)
  RMDir /r "$INSTDIR"

  ; Recreate install dir and restore .hermind
  ${If} ${FileExists} "$TEMP\hermind-data-backup"
    CreateDirectory "$INSTDIR"
    ExecWait 'cmd /c xcopy "$TEMP\hermind-data-backup" "$INSTDIR\.hermind\" /E /I /H /Y && rmdir /s /q "$TEMP\hermind-data-backup"' $0
  ${EndIf}
!macroend
