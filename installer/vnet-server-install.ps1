# Cấu hình máy chủ VNET trên Windows — bộ cài vnet-server.nsi gọi sau khi chép tệp.
#
# Chạy được nhiều lần: lần đầu tạo database, sinh mật khẩu và tài khoản quản
# trị; những lần sau (cập nhật phiên bản) giữ nguyên dữ liệu và cấu hình, chỉ
# chạy lại dịch vụ với tệp mới và áp các bản sửa schema.
#
# Thông báo in ra KHÔNG dấu có chủ đích: NSIS đọc output qua bảng mã OEM, chữ
# có dấu hiện thành rác trong ô chi tiết. Bản đầy đủ nằm ở logs\install.log.
#
# Tệp phải lưu UTF-8 CÓ BOM — PowerShell 5.1 đọc tệp không BOM theo bảng mã
# ANSI và làm hỏng mọi chuỗi có dấu.

param([Parameter(Mandatory = $true)][string]$InstallDir)

$ErrorActionPreference = 'Stop'

$PgBin      = Join-Path $InstallDir 'pgsql\bin'
$DataDir    = Join-Path $InstallDir 'data'
$PgData     = Join-Path $DataDir 'pgdata'
$ConfigFile = Join-Path $DataDir 'config.env'
$LogDir     = Join-Path $InstallDir 'logs'

$PgService  = 'VNETPostgres'
$AppService = 'VNETServer'   # trùng serviceName trong backend/cmd/server/lifetime_windows.go

New-Item -ItemType Directory -Force -Path $DataDir, $LogDir | Out-Null

# Thư mục nằm ở gốc ổ C nên thừa hưởng quyền của C:\ — người dùng thường được
# TẠO tệp trong đó. Với pgsql\bin là đặt được DLL giả cho dịch vụ nạp; với data
# là đọc được config.env (mật khẩu database, JWT_SECRET). Cắt thừa hưởng trước
# khi ghi bất cứ bí mật nào. Dùng SID vì tên nhóm bị dịch theo ngôn ngữ Windows:
# S-1-5-18 SYSTEM, S-1-5-32-544 Administrators, S-1-5-32-545 Users (chỉ đọc/chạy).
icacls $InstallDir /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-32-545:(OI)(CI)RX' /Q | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Khong khoa duoc quyen thu muc $InstallDir" }
icacls $DataDir /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' /Q | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Khong khoa duoc quyen thu muc $DataDir" }

Start-Transcript -Path (Join-Path $LogDir 'install.log') -Append | Out-Null

function Step($msg) { Write-Host "==> $msg" }

function New-Secret([int]$Length) {
    $chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
    $bytes = New-Object byte[] $Length
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    -join ($bytes | ForEach-Object { $chars[$_ % $chars.Length] })
}

# Lệnh ngoài không ném lỗi khi thất bại; phải tự soi mã thoát.
#
# Và ngược lại: PowerShell 5.1 coi mỗi dòng stderr của lệnh ngoài là một lỗi
# khi output bị chuyển hướng (NSIS luôn chuyển hướng), nên với 'Stop' thì dòng
# log đầu tiên của vnet-migrate (Go ghi log ra stderr) đã làm hỏng cả bộ cài.
function Invoke-Checked([string]$What, [scriptblock]$Cmd) {
    $ErrorActionPreference = 'Continue'
    & $Cmd 2>&1 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { throw "$What that bai (ma thoat $LASTEXITCODE)" }
}

# Chạy một câu SQL trả về MỘT giá trị bằng psql, trả giá trị đó (đã cắt khoảng trắng). Dùng $cfg và $pgPort
# nên chỉ gọi được sau khi đã đọc config.env và bật PostgreSQL.
function Invoke-PsqlScalar([string]$Database, [string]$Sql) {
    $ErrorActionPreference = 'Continue'
    $out = & "$PgBin\psql.exe" -h localhost -p $pgPort -U $cfg['DB_USER'] -d $Database -X -q -t -A -c $Sql 2>&1
    if ($LASTEXITCODE -ne 0) { throw "psql that bai (ma thoat $LASTEXITCODE): $out" }
    ("$out").Trim()
}

try {
    # ------------------------------------------------------------ cấu hình
    # Sinh một lần, giữ mãi: đổi JWT_SECRET là đăng xuất mọi người, đổi
    # DB_PASSWORD thì database cũ không mở được nữa.
    if (-not (Test-Path $ConfigFile)) {
        Step 'Tao cau hinh (data\config.env)'
        @(
            '# Cấu hình máy chủ VNET. Sửa xong khởi động lại dịch vụ VNETServer để áp dụng'
            '# (đổi SERVER_PORT thì chạy lại bộ cài để mở cổng tường lửa mới).'
            '# KHÔNG đổi DB_PASSWORD hay DB_PORT: database đã tạo theo hai giá trị này.'
            'GIN_MODE=release'
            'SERVER_PORT=20800'
            'DB_HOST=localhost'
            'DB_PORT=25432'
            'DB_USER=vnet'
            'DB_NAME=vnet'
            "DB_PASSWORD=$(New-Secret 32)"
            "JWT_SECRET=$(New-Secret 64)"
            '# Trang quản trị chạy cùng địa chỉ với API nên không cần liệt kê IP LAN ở đây.'
            'ALLOWED_ORIGINS=http://localhost:20800'
            'HARDWARE_HISTORY_DAYS=7'
        ) | Set-Content -Path $ConfigFile -Encoding UTF8
    }

    $cfg = [ordered]@{}
    foreach ($line in Get-Content $ConfigFile -Encoding UTF8) {
        if ($line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=(.*)$') { $cfg[$Matches[1]] = $Matches[2].Trim() }
    }
    # Cho vnet-migrate/vnet-seed chạy từ đây. Bản thân dịch vụ tự tính các
    # đường dẫn này (loadServiceConfig trong backend/cmd/server/lifetime_windows.go).
    $cfg['UPLOAD_DIR'] = Join-Path $DataDir 'uploads'
    $cfg['BACKUP_DIR'] = Join-Path $DataDir 'backups'

    $port    = $cfg['SERVER_PORT']
    $pgPort  = $cfg['DB_PORT']
    $env:PGPASSWORD = $cfg['DB_PASSWORD']

    # ----------------------------------------------------------- PostgreSQL
    $bundledMajor = ((& "$PgBin\postgres.exe" -V) -replace '^\D*(\d+).*$', '$1')
    $fresh = -not (Test-Path (Join-Path $PgData 'PG_VERSION'))

    if (-not $fresh) {
        # Database tạo bằng bản chính khác thì PostgreSQL từ chối mở; chặn ở
        # đây với lời nhắn rõ ràng thay vì để dịch vụ chết không lý do.
        $dataMajor = (Get-Content (Join-Path $PgData 'PG_VERSION')).Trim()
        if ($dataMajor -ne $bundledMajor) {
            throw "Database tao bang PostgreSQL $dataMajor, bo cai mang PostgreSQL $bundledMajor. Can nang cap database (pg_upgrade) truoc."
        }
    } else {
        Step "Khoi tao database (PostgreSQL $bundledMajor)"
        # initdb tự bỏ quyền Administrators trước khi chạy (PostgreSQL không
        # chịu chạy với quyền quản trị), mà data\ chỉ cho SYSTEM và
        # Administrators — nó không thấy được thư mục cha và báo "File exists".
        # Tạo sẵn pgdata (initdb nhận thư mục rỗng) và cấp quyền cho chính tài
        # khoản đang cài, thứ duy nhất còn lại trong token đã bị cắt quyền.
        $me = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
        New-Item -ItemType Directory -Force -Path $PgData | Out-Null
        Invoke-Checked 'Cap quyen pgdata cho initdb' { icacls $PgData /grant "*${me}:(OI)(CI)F" '*S-1-5-20:(OI)(CI)F' /Q | Out-Null }
        $pwFile = Join-Path $env:TEMP "vnet-pw-$(New-Secret 8).txt"
        [IO.File]::WriteAllText($pwFile, $cfg['DB_PASSWORD'])
        try {
            Invoke-Checked 'initdb' { & "$PgBin\initdb.exe" -D $PgData -U $cfg['DB_USER'] "--pwfile=$pwFile" -E UTF8 --locale=C -A scram-sha-256 }
        } finally {
            Remove-Item $pwFile -Force -ErrorAction SilentlyContinue
        }
        # Chỉ nghe trên máy này: database không có lý do gì mở ra mạng quán.
        # Cổng riêng để không đụng một PostgreSQL khác có sẵn trên máy.
        Add-Content -Path (Join-Path $PgData 'postgresql.conf') -Encoding ASCII -Value @(
            ''
            '# --- VNET ---'
            "port = $pgPort"
            "listen_addresses = 'localhost'"
            'logging_collector = on'
        )
    }

    # Dịch vụ chạy dưới NETWORK SERVICE như bộ cài chính thức của PostgreSQL.
    # Dùng SID chứ không dùng tên: tên tài khoản hệ thống bị dịch theo ngôn ngữ Windows.
    $netSvc = (New-Object Security.Principal.SecurityIdentifier 'S-1-5-20').Translate([Security.Principal.NTAccount]).Value
    Invoke-Checked 'Cap quyen thu muc database' { icacls $PgData /grant '*S-1-5-20:(OI)(CI)F' /T /Q | Out-Null }

    if (-not (Get-Service $PgService -ErrorAction SilentlyContinue)) {
        Step 'Dang ky dich vu VNETPostgres'
        Invoke-Checked 'pg_ctl register' { & "$PgBin\pg_ctl.exe" register -N $PgService -U $netSvc -D $PgData -S auto -w }
    }
    sc.exe failure $PgService reset= 86400 actions= restart/10000/restart/10000/restart/30000 | Out-Null

    Step 'Bat PostgreSQL'
    Start-Service $PgService
    $ready = $false
    for ($i = 0; $i -lt 30; $i++) {
        & "$PgBin\pg_isready.exe" -q -h localhost -p $pgPort
        if ($LASTEXITCODE -eq 0) { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw "PostgreSQL khong len sau 30 giay. Xem $PgData\log" }

    # Hỏi database THẬT chứ không dựa vào $fresh: initdb tạo PG_VERSION rất sớm, nên lần cài hỏng giữa initdb
    # và createdb (PostgreSQL lên chậm, antivirus quét ổ...) để lại thư mục "không còn mới" mà chưa có
    # database. Dựa vào $fresh thì lần chạy lại bỏ qua createdb và hỏng y như cũ, mãi tới khi ai đó xoá tay pgdata.
    $dbName = $cfg['DB_NAME']
    $dbNameSql = $dbName.Replace("'", "''")
    if ((Invoke-PsqlScalar 'postgres' "SELECT 1 FROM pg_database WHERE datname = '$dbNameSql'") -ne '1') {
        Step "Tao database $dbName"
        Invoke-Checked 'createdb' { & "$PgBin\createdb.exe" -h localhost -p $pgPort -U $cfg['DB_USER'] $dbName }
    }

    # --------------------------------------------------------------- VNET
    $exe = Join-Path $InstallDir 'vnet-server.exe'
    if (-not (Get-Service $AppService -ErrorAction SilentlyContinue)) {
        Step 'Dang ky dich vu VNETServer'
        New-Service -Name $AppService -BinaryPathName "`"$exe`"" -DisplayName 'VNET Server' `
            -Description 'Máy chủ quản lý phòng máy VNET (API + trang quản trị).' `
            -StartupType Automatic -DependsOn $PgService | Out-Null
    }
    # Cấu hình KHÔNG đi qua registry: dịch vụ tự đọc data\config.env (đã khoá
    # quyền), còn khoá Services trong registry thì mọi tài khoản Users đọc được.
    # Server từ chối khởi động khi database chưa sẵn sàng (log.Fatalf) — SCM bật lại sau 10 giây.
    sc.exe failure $AppService reset= 86400 actions= restart/10000/restart/10000/restart/30000 | Out-Null

    Step 'Bat VNET Server'
    Restart-Service $AppService -Force
    $healthy = $false
    for ($i = 0; $i -lt 60; $i++) {
        try {
            Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 "http://localhost:$port/api/health" | Out-Null
            $healthy = $true; break
        } catch { Start-Sleep -Seconds 1 }
    }
    if (-not $healthy) {
        $tail = Get-Content (Join-Path $LogDir 'server.log') -Tail 20 -ErrorAction SilentlyContinue
        throw "VNET Server khong tra loi sau 60 giay. Cuoi logs\server.log:`n$($tail -join "`n")"
    }

    # Sửa schema sau khi server đã tạo bảng — đúng thứ tự như docker-compose.
    foreach ($k in $cfg.Keys) { [Environment]::SetEnvironmentVariable($k, $cfg[$k], 'Process') }
    Step 'Cap nhat schema'
    Invoke-Checked 'vnet-migrate' { & (Join-Path $InstallDir 'vnet-migrate.exe') }
    # Chỉ tạo tài khoản khi bảng users còn TRỐNG chứ không dựa vào $fresh: lần cài đầu hỏng sau initdb mà
    # trước vnet-seed (cổng bị chiếm, antivirus giữ vnet-server.exe...) để lại $fresh = false, và lần chạy
    # lại sẽ không bao giờ tạo tài khoản — trang quản trị trống trơn, không có tệp mật khẩu. Cập nhật thì
    # users đã có dòng nên vẫn không đụng tới tài khoản nào.
    if ((Invoke-PsqlScalar $dbName 'SELECT count(*) FROM users') -eq '0') {
        Step 'Tao tai khoan quan tri'
        # Mật khẩu 'admin123' mặc định của vnet-seed nằm sẵn trong tài liệu: ai mở
        # trang quản trị từ máy khách cũng thử được. Chỉ khi chưa có tài khoản nào mới sinh mật khẩu
        # ngẫu nhiên; cập nhật thì không đụng tới tài khoản nào (vnet-seed không chạy).
        # Mỗi tài khoản một mật khẩu riêng: giao tài khoản staff cho thu ngân mà
        # chung mật khẩu thì cũng là giao luôn chìa khoá admin.
        $accounts = 'admin', 'manager', 'staff'
        $initial = [ordered]@{}
        foreach ($a in $accounts) { $initial[$a] = New-Secret 16 }

        # Tạo tệp rỗng và khoá quyền TRƯỚC khi ghi mật khẩu vào. data\ đã chỉ cho
        # SYSTEM và Administrators; icacls lần nữa để quyền không phụ thuộc thư mục cha.
        $pwFile = Join-Path $DataDir 'initial-admin-password.txt'
        New-Item -ItemType File -Force -Path $pwFile | Out-Null
        icacls $pwFile /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' /Q | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Remove-Item $pwFile -Force -ErrorAction SilentlyContinue
            throw "Khong khoa duoc quyen tep $pwFile"
        }
        @(
            'VNET - mat khau ban dau (sinh ngau nhien luc cai dat lan dau)'
            "Dang nhap tai http://localhost:$port roi DOI MAT KHAU, sau do XOA tep nay."
            ''
            ($accounts | ForEach-Object { '{0,-8} : {1}' -f $_, $initial[$_] })
        ) | Set-Content -Path $pwFile -Encoding ASCII

        # Ghi tệp TRƯỚC khi seed: tài khoản đã tạo mà tệp chưa có thì chủ quán bị
        # khoá ngoài, còn tệp có sẵn mà seed hỏng thì chỉ thừa một tệp.
        foreach ($a in $accounts) { [Environment]::SetEnvironmentVariable("SEED_PASSWORD_$($a.ToUpper())", $initial[$a], 'Process') }
        try {
            Invoke-Checked 'vnet-seed' { & (Join-Path $InstallDir 'vnet-seed.exe') }
        } finally {
            foreach ($a in $accounts) { [Environment]::SetEnvironmentVariable("SEED_PASSWORD_$($a.ToUpper())", $null, 'Process') }
        }
        Step "Mat khau ban dau cua admin / manager / staff nam trong: $pwFile"
    }

    # Máy trạm trong quán gọi vào cổng này. Tạo lại mỗi lần để theo SERVER_PORT.
    Step "Mo cong $port tren tuong lua"
    Get-NetFirewallRule -Name 'VNET-Server' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
    New-NetFirewallRule -Name 'VNET-Server' -DisplayName 'VNET Server' -Direction Inbound `
        -Protocol TCP -LocalPort $port -Action Allow -Profile Any | Out-Null

    $ips = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
        ForEach-Object { "http://$($_.IPAddress):$port" }
    Step "Xong. Dia chi cho may tram: $($ips -join ', ')"
    $exitCode = 0
} catch {
    Write-Host "LOI: $($_.Exception.Message)"
    $exitCode = 1
} finally {
    Stop-Transcript | Out-Null
}
exit $exitCode
