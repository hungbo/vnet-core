; Bộ cài máy chủ VNET cho Windows (Server 2016+/10/11, 64-bit).
;
; NSIS chứ không phải Inno Setup như bộ cài máy khách: makensis chạy được trên
; macOS/Linux (brew install makensis), nên scripts/build-server-windows.sh xuất
; ra tệp setup ngay trên máy build, không cần Windows.
;
; Bộ cài chỉ chép tệp và tạo lối tắt. Toàn bộ phần cấu hình — database, dịch
; vụ, tường lửa — nằm trong vnet-server-install.ps1 để sửa và đọc được dễ hơn.
;
;   makensis -DVERSION=1.2.0 -DWINVER=1.2.0.0 -DSTAGE=<thư mục đã gom> \
;            -DICON=<icon.ico> -DOUTFILE=<tệp ra> vnet-server.nsi

Unicode true
ManifestDPIAware true

!include MUI2.nsh
!include x64.nsh
!include LogicLib.nsh

!define APP      "VNET Server"
!define REGKEY   "Software\VNET Server"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\VNETServer"
!define ADMINURL "http://localhost:20800"

Name "${APP}"
OutFile "${OUTFILE}"
; Ổ C gốc chứ không Program Files: chủ quán cần tìm thấy thư mục dữ liệu và
; bản sao lưu, và đường dẫn không có khoảng trắng.
InstallDir "C:\VNET"
InstallDirRegKey HKLM "${REGKEY}" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "${APP} ${VERSION}"
ShowInstDetails show
ShowUninstDetails show

VIProductVersion "${WINVER}"
VIAddVersionKey /LANG=0 "ProductName" "${APP}"
VIAddVersionKey /LANG=0 "CompanyName" "VNET"
VIAddVersionKey /LANG=0 "FileDescription" "Bộ cài ${APP}"
VIAddVersionKey /LANG=0 "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=0 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=0 "LegalCopyright" "VNET"

!define MUI_ICON   "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING

!define MUI_WELCOMEPAGE_TEXT "Bộ cài sẽ cài máy chủ VNET cùng cơ sở dữ liệu PostgreSQL và tự chạy chúng mỗi khi bật máy.$\r$\n$\r$\nĐã cài rồi thì chạy bộ cài mới để cập nhật — dữ liệu và cấu hình được giữ nguyên."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_TITLE "Đã cài xong ${APP}"
!define MUI_FINISHPAGE_TEXT "Mở trang quản trị tại ${ADMINURL} (có lối tắt VNET Quản lý trên màn hình).$\r$\n$\r$\nCài lần đầu: mật khẩu ngẫu nhiên của admin / manager / staff nằm trong $INSTDIR\data\initial-admin-password.txt (chỉ quản trị viên Windows mở được). Đăng nhập, đổi mật khẩu rồi xoá tệp đó. Cập nhật thì giữ nguyên mật khẩu cũ.$\r$\n$\r$\nMáy trạm trỏ về http://<IP máy này>:20800 — các địa chỉ IP nằm ở dòng cuối trong ô chi tiết."
; Đoạn chữ ở trên dài hơn mặc định; không có dòng này nó đè lên ô "Mở trang quản trị".
!define MUI_FINISHPAGE_TEXT_LARGE
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Mở trang quản trị"
!define MUI_FINISHPAGE_RUN_FUNCTION OpenAdmin
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Vietnamese"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "${APP} chỉ chạy trên Windows 64-bit."
    Abort
  ${EndIf}
  SetRegView 64
FunctionEnd

Function OpenAdmin
  ExecShell "open" "${ADMINURL}"
FunctionEnd

Section "Install"
  SetRegView 64
  ; NSIS là chương trình 32-bit: không tắt chuyển hướng thì $SYSDIR trỏ vào
  ; SysWOW64 và PowerShell chạy bản 32-bit.
  ${DisableX64FSRedirection}

  ; Cập nhật: dừng dịch vụ trước, nếu không Windows khoá tệp .exe đang chạy và
  ; không chép đè được. Chưa cài thì lệnh báo lỗi — bỏ qua.
  DetailPrint "Dừng dịch vụ đang chạy (nếu có)..."
  nsExec::ExecToLog '"$SYSDIR\net.exe" stop VNETServer'
  Pop $0
  nsExec::ExecToLog '"$SYSDIR\net.exe" stop VNETPostgres'
  Pop $0

  SetOutPath "$INSTDIR"
  File /r "${STAGE}\*"

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "${REGKEY}" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayName" "${APP}"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINSTKEY}" "Publisher" "VNET"
  WriteRegStr HKLM "${UNINSTKEY}" "DisplayIcon" "$INSTDIR\vnet-server.exe"
  WriteRegStr HKLM "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNINSTKEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINSTKEY}" "NoRepair" 1

  ; PostgreSQL cần bộ thư viện Visual C++ (msvcp140.dll), Windows Server cài
  ; mới không có sẵn. Máy đã có bản mới hơn thì trả 1638 — không phải lỗi.
  DetailPrint "Cài Visual C++ Runtime..."
  ExecWait '"$INSTDIR\setup\vc_redist.x64.exe" /install /quiet /norestart' $0
  ${If} $0 != 0
  ${AndIf} $0 != 1638
  ${AndIf} $0 != 3010
    MessageBox MB_ICONSTOP "Cài Visual C++ Runtime thất bại (mã $0)."
    Abort
  ${EndIf}

  DetailPrint "Cấu hình database và dịch vụ..."
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\setup\vnet-server-install.ps1" -InstallDir "$INSTDIR"'
  Pop $0
  ${If} $0 != 0
    MessageBox MB_ICONSTOP "Cấu hình máy chủ thất bại. Xem chi tiết trong:$\r$\n$INSTDIR\logs\install.log"
    Abort
  ${EndIf}

  ; Lối tắt mở trang quản trị trong trình duyệt mặc định, mang icon VNET.
  SetShellVarContext all
  CreateShortCut "$DESKTOP\VNET Quản lý.lnk" "$WINDIR\explorer.exe" "${ADMINURL}" "$INSTDIR\vnet-server.exe" 0
  CreateDirectory "$SMPROGRAMS\VNET Server"
  CreateShortCut "$SMPROGRAMS\VNET Server\VNET Quản lý.lnk" "$WINDIR\explorer.exe" "${ADMINURL}" "$INSTDIR\vnet-server.exe" 0
  CreateShortCut "$SMPROGRAMS\VNET Server\Thư mục dữ liệu.lnk" "$INSTDIR\data"
  CreateShortCut "$SMPROGRAMS\VNET Server\Nhật ký.lnk" "$INSTDIR\logs"
  CreateShortCut "$SMPROGRAMS\VNET Server\Gỡ cài đặt.lnk" "$INSTDIR\uninstall.exe"
SectionEnd

Section "Uninstall"
  SetRegView 64
  ${DisableX64FSRedirection}

  nsExec::ExecToLog '"$SYSDIR\net.exe" stop VNETServer'
  Pop $0
  nsExec::ExecToLog '"$SYSDIR\sc.exe" delete VNETServer'
  Pop $0
  nsExec::ExecToLog '"$SYSDIR\net.exe" stop VNETPostgres'
  Pop $0
  nsExec::ExecToLog '"$SYSDIR\sc.exe" delete VNETPostgres'
  Pop $0
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -Command "Remove-NetFirewallRule -Name VNET-Server -ErrorAction SilentlyContinue"'
  Pop $0

  ; Dữ liệu quán (database, ảnh, bản sao lưu) mặc định GIỮ LẠI: gỡ để cài lại
  ; là chuyện thường, mất sạch dữ liệu vì bấm nhầm thì không lấy lại được.
  ; Gỡ im lặng (/S) cũng giữ.
  MessageBox MB_YESNO|MB_ICONEXCLAMATION|MB_DEFBUTTON2 "Xoá luôn DỮ LIỆU của quán (database, ảnh, bản sao lưu) trong $INSTDIR\data?$\r$\n$\r$\nChọn Không để giữ lại — cài lại sau sẽ dùng tiếp dữ liệu cũ." /SD IDNO IDYES removeData IDNO keepData
  removeData:
    RMDir /r "$INSTDIR\data"
  keepData:

  Delete "$INSTDIR\vnet-server.exe"
  Delete "$INSTDIR\vnet-migrate.exe"
  Delete "$INSTDIR\vnet-seed.exe"
  RMDir /r "$INSTDIR\pgsql"
  RMDir /r "$INSTDIR\setup"
  RMDir /r "$INSTDIR\logs"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  SetShellVarContext all
  Delete "$DESKTOP\VNET Quản lý.lnk"
  RMDir /r "$SMPROGRAMS\VNET Server"

  DeleteRegKey HKLM "${UNINSTKEY}"
  DeleteRegKey HKLM "${REGKEY}"
SectionEnd
