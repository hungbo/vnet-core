; Bộ cài máy khách VNET.
;
; Biên dịch bằng Inno Setup 6 (iscc.exe). scripts/build-client.sh gọi nó nếu có
; trên máy, không có thì bỏ qua và vẫn xuất .exe + .zip như trước — Inno Setup
; chỉ chạy trên Windows.
;
;   iscc /DVersion=1.2.0 /DSourceExe=..\client\dist\vnet-client-amd64.exe vnet-client.iss

#ifndef Version
  #define Version "dev"
#endif
#ifndef SourceExe
  #define SourceExe "..\client\dist\vnet-client-amd64.exe"
#endif

[Setup]
AppName=VNET Client
AppVersion={#Version}
AppPublisher=VNET
DefaultDirName={autopf}\VNET Client
DefaultGroupName=VNET
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=vnet-client-setup-{#Version}
Compression=lzma2
SolidCompression=yes
; Đăng ký dịch vụ Windows cần quyền quản trị. Không có nó thì phần nền không cài
; được, và đó là cả điểm của bộ cài này.
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
WizardStyle=modern
; Icon của bộ cài và của mục gỡ cài đặt trong Settings → Apps. Tệp .exe đã
; mang sẵn icon (scripts/build-client.sh nhúng lúc build) nên lối tắt tự hiện đúng.
SetupIconFile=..\client\src\build\windows\icon.ico
UninstallDisplayIcon={app}\vnet-client.exe

[Languages]
Name: "vi"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "vnet-client.exe"; Flags: ignoreversion
; TightVNC cho remote desktop: tải bản MSI 64-bit từ tightvnc.com, đặt tại
; installer\tightvnc.msi. Dịch vụ nền tự cài nó khi nhận mật khẩu VNC từ máy
; chủ. Không có tệp thì bộ cài vẫn build, chỉ là máy đó không remote được.
Source: "tightvnc.msi"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist

[Icons]
Name: "{group}\VNET Client"; Filename: "{app}\vnet-client.exe"
; Lối tắt VNET trên Desktop chung (C:\Users\Public\Desktop\VNET.lnk): cửa sổ VNET
; không có nút tắt, nên đây là đường mở lại nó. Dịch vụ nền cũng tự dựng lại tệp
; này mỗi lần bật (shortcuts_windows.go) — máy diskless mất nó sau mỗi lần khởi
; động — nên tên PHẢI giữ là "VNET".
Name: "{commondesktop}\VNET"; Filename: "{app}\vnet-client.exe"; WorkingDir: "{app}"; Comment: "VNET"

[UninstallDelete]
; Inno tự xoá lối tắt nó đã tạo, nhưng bản do dịch vụ nền dựng thì nó không biết.
Type: files; Name: "{commondesktop}\VNET.lnk"

[Run]
; Ghi cấu hình TRƯỚC khi bật dịch vụ: dịch vụ chạy ở session 0 và không thừa
; hưởng biến môi trường của người cài, nên nó chỉ đọc được tệp config.json.
Filename: "{cmd}"; \
  Parameters: "/C echo {{""server_url"":""{code:GetServerURL}""} > ""{app}\config.json"""; \
  Flags: runhidden

; Đặt tài khoản quản trị máy trạm TRƯỚC khi bật dịch vụ, và bằng chính .exe chứ
; không ghi thẳng vào tệp: bộ cài không băm được, mà ghi mật khẩu trần thì khách
; đọc được — config.json nằm trong Program Files, chặn ghi chứ không chặn đọc.
Filename: "{app}\vnet-client.exe"; \
  Parameters: "--set-admin-user ""{code:GetAdminUser}"" --set-admin-pass ""{code:GetAdminPass}"""; \
  StatusMsg: "Đang đặt tài khoản quản trị máy trạm..."; Flags: runhidden

Filename: "{app}\vnet-client.exe"; Parameters: "--install-service"; \
  StatusMsg: "Đang đăng ký dịch vụ nền..."; Flags: runhidden

[UninstallRun]
; Gỡ dịch vụ trước khi xoá tệp, nếu không Windows giữ tệp .exe lại.
Filename: "{app}\vnet-client.exe"; Parameters: "--uninstall-service"; Flags: runhidden; RunOnceId: "RemoveService"

[Code]
var
  ConfigPage: TInputQueryWizardPage;

procedure InitializeWizard;
begin
  ConfigPage := CreateInputQueryPage(wpSelectDir,
    'Cấu hình máy trạm',
    'Địa chỉ máy chủ và tài khoản quản trị máy trạm. Mã máy là TÊN MÁY Windows, máy tự gửi lên.',
    'Máy chủ tự thêm máy mới ở lần nối đầu tiên; sau đó vào trang Máy để xếp nhóm ' +
    '(máy chưa có nhóm thì khách chưa đăng nhập được). Hệ diskless đặt tên máy ở máy chủ boot.' + #13#10#13#10 +
    'Tài khoản quản trị máy trạm: nhân viên kỹ thuật gõ vào ô đăng nhập trên màn hình khoá ' +
    'để mở máy sửa chữa, kể cả khi mất mạng. Đổi được sau này ở trang quản trị: ' +
    'Cài đặt → Máy trạm.');
  ConfigPage.Add('Địa chỉ máy chủ:', False);
  ConfigPage.Add('Tài khoản quản trị máy trạm:', False);
  ConfigPage.Add('Mật khẩu (ít nhất 6 ký tự):', True);
  ConfigPage.Add('Nhập lại mật khẩu:', True);
  ConfigPage.Values[0] := 'http://192.168.1.10:20800';
  ConfigPage.Values[1] := 'admin';
end;

function Loi(Msg: string): Boolean;
begin
  MsgBox(Msg, mbError, MB_OK);
  Result := False;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  User, Pass: string;
begin
  Result := True;
  if CurPageID <> ConfigPage.ID then Exit;

  User := Trim(ConfigPage.Values[1]);
  Pass := Trim(ConfigPage.Values[2]);
  if Trim(ConfigPage.Values[0]) = '' then
    Result := Loi('Chưa nhập địa chỉ máy chủ.')
  else if User = '' then
    Result := Loi('Chưa nhập tài khoản quản trị máy trạm.')
  else if (Pos(' ', User) > 0) or (Pos('"', User) > 0) then
    Result := Loi('Tên tài khoản không được có khoảng trắng hay dấu nháy kép.')
  else if Length(Pass) < 6 then
    Result := Loi('Mật khẩu quản trị máy trạm phải có ít nhất 6 ký tự.')
  else if Pos('"', Pass) > 0 then
    // Mật khẩu đi qua dòng lệnh trong dấu nháy kép; một dấu nháy bên trong sẽ cắt nó làm đôi.
    Result := Loi('Mật khẩu không được có dấu nháy kép (").')
  else if Pass <> Trim(ConfigPage.Values[3]) then
    Result := Loi('Hai lần nhập mật khẩu không khớp.');
end;

function GetServerURL(Param: string): string;
begin
  Result := Trim(ConfigPage.Values[0]);
end;

function GetAdminUser(Param: string): string;
begin
  Result := Trim(ConfigPage.Values[1]);
end;

function GetAdminPass(Param: string): string;
begin
  Result := Trim(ConfigPage.Values[2]);
end;
