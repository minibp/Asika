!define PRODUCT_NAME "Asika"
!define PRODUCT_VERSION "1.0.0"
!define INSTALL_DIR "$PROGRAMFILES\${PRODUCT_NAME}"

Outfile "Asika_Setup_${PRODUCT_VERSION}.exe"
InstallDir "${INSTALL_DIR}"
RequestExecutionLevel admin
SetCompressor lzma

Section "Install"
    SetOutPath "$INSTDIR"

    ; Copy binaries and config
    File /r "asikad.exe"
    File /r "asika.exe"
    File /r "asika_config.toml"

    ; Set service environment variable via registry
    WriteRegStr HKLM "SYSTEM\CurrentControlSet\Services\asikad\Environment" \
        "ASIKA_CONFIG" "$INSTDIR\asika_config.toml"
    WriteRegStr HKLM "SYSTEM\CurrentControlSet\Services\asikad\Environment" \
        "GOMEMLIMIT" "256MiB"

    ; Remove existing service if present
    nsExec::Exec 'sc query asikad >nul 2>&1'
    Pop $0
    ${If} $0 == 0
        nsExec::Exec 'sc stop asikad'
        nsExec::Exec 'sc delete asikad'
        Sleep 2000
    ${EndIf}

    ; Create service
    nsExec::Exec 'sc create asikad binPath= "\"$INSTDIR\asikad.exe\"" start= auto'
    Pop $0
    ${If} $0 != 0
        MessageBox MB_ICONSTOP "Failed to create service."
        Quit
    ${EndIf}

    ; Set description
    nsExec::Exec 'sc description asikad "Asika PR Manager — cross-platform PR manager and merge queue service"'

    ; Set failure recovery: restart after 10s, 20s, 30s; reset counter after 1 hour
    nsExec::Exec 'sc failure asikad actions= restart/10000/restart/20000/restart/30000 reset= 3600'

    ; Start service
    nsExec::Exec 'sc start asikad'

    ; Write uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"

    ; Start menu shortcuts
    CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
    CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\Asika Dashboard.lnk" \
        "$SYSDIR\cmd.exe" '/k "$INSTDIR\asikad.exe" --desktop' \
        "$SYSDIR\cmd.exe" "" SW_SHOWNORMAL "" \
        "Start Asika Dashboard in foreground mode"
    CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall Asika.lnk" "$INSTDIR\uninstall.exe"
SectionEnd

Section "Uninstall"
    ; Stop and delete service
    nsExec::Exec 'sc stop asikad'
    Sleep 2000
    nsExec::Exec 'sc delete asikad'

    ; Clean registry
    DeleteRegKey HKLM "SYSTEM\CurrentControlSet\Services\asikad\Environment"

    ; Clean files
    Delete "$INSTDIR\*.*"
    RMDir "$INSTDIR"
    Delete "$SMPROGRAMS\${PRODUCT_NAME}\*.*"
    RMDir "$SMPROGRAMS\${PRODUCT_NAME}"
SectionEnd