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

[Languages]
Name: "vi"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "vnet-client.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\VNET Client"; Filename: "{app}\vnet-client.exe"

[Run]
; Ghi cấu hình TRƯỚC khi bật dịch vụ: dịch vụ chạy ở session 0 và không thừa
; hưởng biến môi trường của người cài, nên nó chỉ đọc được tệp config.json.
Filename: "{cmd}"; \
  Parameters: "/C echo {{""server_url"":""{code:GetServerURL}"",""machine_code"":""{code:GetMachineCode}""} > ""{app}\config.json"""; \
  Flags: runhidden

; Đặt PIN TRƯỚC khi bật dịch vụ, và bằng chính .exe chứ không ghi thẳng vào tệp:
; bộ cài không băm được, mà ghi PIN trần thì khách đọc được — config.json nằm
; trong Program Files, chặn ghi chứ không chặn đọc.
Filename: "{app}\vnet-client.exe"; Parameters: "--set-pin ""{code:GetPin}"""; \
  StatusMsg: "Đang đặt PIN kỹ thuật..."; Flags: runhidden; Check: CoPin

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
    'Ba thông tin này lấy từ trang Máy trong phần quản trị.',
    'Khoá máy là TUỲ CHỌN — để trống là được, máy cắm vào là chạy. ' +
    'Chỉ điền khi bạn đã bấm "Cấp khoá" cho máy này ở trang Máy; từ lúc đó máy chủ ' +
    'bắt buộc phải có khoá đúng.' + #13#10#13#10 +
    'PIN kỹ thuật là đường vào DUY NHẤT khi mất mạng: màn hình khoá phủ kín màn hình ' +
    'và mọi cách đăng nhập khác đều phải hỏi máy chủ. Đặt chung một PIN cho cả quán.');
  ConfigPage.Add('Địa chỉ máy chủ:', False);
  ConfigPage.Add('Mã máy:', False);
  ConfigPage.Add('PIN kỹ thuật:', False);
  ConfigPage.Values[0] := 'http://192.168.1.10:20800';
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = ConfigPage.ID then
  begin
    if (Trim(ConfigPage.Values[2]) <> '') and (Length(Trim(ConfigPage.Values[2])) < 4) then
    begin
      MsgBox('PIN kỹ thuật phải có ít nhất 4 ký tự.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if Trim(ConfigPage.Values[0]) = '' then
    begin
      MsgBox('Chưa nhập địa chỉ máy chủ.', mbError, MB_OK);
      Result := False;
    end
    else if Trim(ConfigPage.Values[1]) = '' then
    begin
      MsgBox('Chưa nhập mã máy.', mbError, MB_OK);
      Result := False;
    end;
  end;
end;

function GetServerURL(Param: string): string;
begin
  Result := Trim(ConfigPage.Values[0]);
end;

function GetMachineCode(Param: string): string;
begin
  Result := Trim(ConfigPage.Values[1]);
end;

function GetPin(Param: string): string;
begin
  Result := Trim(ConfigPage.Values[2]);
end;

function CoPin: Boolean;
begin
  Result := Trim(ConfigPage.Values[2]) <> '';
end;
