#!/usr/bin/env python3
"""Kiểm chứng đầu-cuối cho VNET Core.

Chạy hệ thống thật và xác nhận từng tính năng, thay vì suy luận từ code. Bộ
unit test dùng SQL giả không bắt được lỗi ở tầng database — ví dụ cột jsonb
nhận chuỗi Go trần — nên kịch bản này gọi API thật rồi soi lại database.

    docker compose -p vnetverify up -d --build
    docker compose -p vnetverify --profile tools run --rm seed
    python3 scripts/verify.py

Biến môi trường:
    VNET_URL   địa chỉ API          (mặc định http://localhost:8099)
    VNET_PG    tên container Postgres (mặc định vnetverify-postgres-1)
    VNET_DB    tên database          (mặc định vnet)
    VNET_USER  user database         (mặc định vnet)

Mã thoát: 0 nếu không có mục HỎNG, 1 nếu có.

KHÔNG phủ được ở đây: năm tính năng gọi thẳng API Windows (khoá màn hình + chặn
phím, in ra máy in nhiệt, ghi tệp hosts, cài bản cập nhật, chụp màn hình GDI).
Xem scripts/KIEM-CHUNG-WINDOWS.md — phải thử tay trên máy Windows thật.
"""

from __future__ import annotations

import base64
import json
import os
import socket
import subprocess
import threading
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone

BASE = os.environ.get("VNET_URL", "http://localhost:8099").rstrip("/")
PG_CONTAINER = os.environ.get("VNET_PG", "vnetverify-postgres-1")
# Tệp .sql nằm trong container CHẠY SERVER, không phải container database.
APP_CONTAINER = os.environ.get("VNET_APP", "vnetverify-app-1")
DB_NAME = os.environ.get("VNET_DB", "vnet")
DB_USER = os.environ.get("VNET_USER", "vnet")

VN_TZ = timezone(timedelta(hours=7))

# Trạng thái kết quả
OK = "OK"        # đúng hợp đồng
WRONG = "SAI"    # gọi được nhưng dữ liệu/hành vi sai
BROKEN = "HỎNG"  # 5xx, hoặc 4xx ở nơi lẽ ra thành công
SKIP = "BỎ QUA"  # tác dụng phụ nguy hiểm


@dataclass
class Result:
    area: str
    name: str
    status: str
    detail: str = ""


@dataclass
class Report:
    results: list[Result] = field(default_factory=list)

    def add(self, area: str, name: str, status: str, detail: str = "") -> None:
        self.results.append(Result(area, name, status, detail))
        mark = {OK: "  ok  ", WRONG: " SAI  ", BROKEN: " HỎNG ", SKIP: " bỏ   "}[status]
        line = f"[{mark}] {area:22} {name}"
        if detail:
            line += f"  — {detail}"
        print(line, flush=True)

    def counts(self) -> dict[str, int]:
        out = {OK: 0, WRONG: 0, BROKEN: 0, SKIP: 0}
        for r in self.results:
            out[r.status] += 1
        return out


report = Report()


# --------------------------------------------------------------------------- #
# Tiện ích
# --------------------------------------------------------------------------- #

def call(method: str, path: str, body=None, token: str | None = None, timeout: int = 30,
         headers: dict | None = None):
    """Gọi API. Trả về (mã HTTP, body đã parse). Không ném ngoại lệ khi 4xx/5xx."""
    hdrs = {"Content-Type": "application/json"}
    if token:
        hdrs["Authorization"] = "Bearer " + token
    if headers:
        hdrs.update(headers)
    headers = hdrs

    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, method=method, headers=headers, data=data)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode()
            return resp.status, (json.loads(raw) if raw else {})
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            return e.code, {"message": raw[:200]}
    except Exception as e:  # timeout, connection refused…
        return 0, {"message": f"{type(e).__name__}: {e}"}


def sql(query: str) -> str:
    """Truy vấn database trực tiếp — dùng để xác nhận trạng thái cuối, không chỉ mã HTTP."""
    out = subprocess.run(
        ["docker", "exec", PG_CONTAINER, "psql", "-U", DB_USER, "-d", DB_NAME, "-tAc", query],
        capture_output=True, text=True,
    )
    return out.stdout.strip()


def file_exists_in_app(path: str) -> bool:
    """Tệp sao lưu do server ghi ra nằm trong hệ tệp của container server."""
    out = subprocess.run(["docker", "exec", APP_CONTAINER, "test", "-f", path],
                         capture_output=True, text=True)
    return out.returncode == 0


def agent_hb(code: str, uptime: int) -> int:
    """Gửi một heartbeat như máy trạm thật, có kèm uptime.

    Máy chủ suy ra mốc khởi động từ uptime (booted_at = bây giờ - uptime), nên
    đây là cách giả lập reboot: uptime rơi từ cao xuống gần 0 = máy vừa bật lại.
    """
    body = {
        "machine_code": code, "cpu_temp": 45, "gpu_temp": 55,
        "cpu_usage": 10, "ram_usage": 40, "disk_usage": 50, "uptime": uptime,
    }
    return call("POST", f"/api/machines/by-code/{code}/heartbeat", body)[0]


def data_of(res) -> object:
    return res.get("data") if isinstance(res, dict) else None


def items_of(res) -> list:
    """Bóc danh sách khỏi cả hai kiểu phân trang mà backend dùng song song."""
    d = data_of(res)
    if isinstance(d, list):
        return d
    if isinstance(d, dict):
        for key in ("items", "records"):
            if isinstance(d.get(key), list):
                return d[key]
    return []


def expect(area: str, name: str, cond: bool, detail_ok: str = "", detail_bad: str = "") -> bool:
    report.add(area, name, OK if cond else WRONG, detail_ok if cond else detail_bad)
    return cond


def probe(area: str, name: str, method: str, path: str, body=None, token=None,
          want=(200, 201), timeout: int = 30):
    """Gọi một endpoint và chấm điểm theo mã trạng thái mong đợi."""
    st, res = call(method, path, body, token, timeout)
    if st in want:
        report.add(area, name, OK, f"{method} {path} → {st}")
    elif st == 0 or st >= 500:
        report.add(area, name, BROKEN, f"{method} {path} → {st} {res.get('message', '')}"[:160])
    else:
        report.add(area, name, BROKEN, f"{method} {path} → {st} {res.get('message', '')}"[:160])
    return st, res


# --------------------------------------------------------------------------- #
# Giai đoạn 0-1: smoke + auth
# --------------------------------------------------------------------------- #

def phase_auth() -> dict:
    area = "0. Smoke & Auth"
    probe(area, "health", "GET", "/api/health")
    probe(area, "route công khai", "GET", "/api/route/getConstantRoutes")

    tokens: dict = {}
    for user in ("admin", "manager", "staff"):
        st, res = call("POST", "/api/auth/login", {"username": user, "password": "admin123"})
        d = data_of(res) or {}
        if st == 200 and d.get("access_token"):
            tokens[user] = d["access_token"]
            tokens[user + "_refresh"] = d.get("refresh_token", "")
            report.add(area, f"đăng nhập {user}", OK)
        else:
            report.add(area, f"đăng nhập {user}", BROKEN, f"{st} {res.get('message','')}")

    if "admin" in tokens:
        probe(area, "auth/me", "GET", "/api/auth/me", token=tokens["admin"])
        probe(area, "menu động", "GET", "/api/route/getUserRoutes", token=tokens["admin"])
    return tokens


# --------------------------------------------------------------------------- #
# Giai đoạn 2-4: dữ liệu nền
# --------------------------------------------------------------------------- #

def phase_setup(t: str) -> dict:
    """Tạo dữ liệu nền. Trả về các id để các giai đoạn sau dùng."""
    area = "1. Dữ liệu nền"
    ids: dict = {}

    st, res = probe(area, "danh sách đơn vị tính", "GET", "/api/units", token=t)
    units = data_of(res) or []
    ids["unit"] = units[0]["id"] if units else ""

    st, res = probe(area, "tạo nhóm hội viên", "POST", "/api/member-groups",
                    {"name": "KT-Vang", "min_spent": 100000, "discount_percent": 10.0}, t)
    ids["member_group"] = (data_of(res) or {}).get("id", "")

    # price_per_hour là nguồn giá DUY NHẤT của hệ thống — phải khác 0.
    st, res = probe(area, "tạo nhóm máy (giá 20.000₫/giờ)", "POST", "/api/machine-groups",
                    {"name": "KT-Nhom", "price_per_hour": 20000, "color": "#f00"}, t)
    grp = data_of(res) or {}
    ids["machine_group"] = grp.get("id", "")
    expect(area, "giá nhóm máy được lưu", grp.get("price_per_hour") == 20000,
           "20000", f"nhận {grp.get('price_per_hour')} — trường bị DTO bỏ qua")

    st, res = probe(area, "tạo nhà cung cấp", "POST", "/api/suppliers",
                    {"name": "KT-NCC", "phone": "0900000000"}, t)
    ids["supplier"] = (data_of(res) or {}).get("id", "")

    # IP thuộc dải TEST-NET: không định tuyến, không thể gõ vào máy in thật.
    st, res = probe(area, "tạo máy in", "POST", "/api/printers",
                    {"name": "KT-May-in", "printer_type": "receipt",
                     "ip_address": "192.0.2.1", "port": 9100}, t)
    ids["printer"] = (data_of(res) or {}).get("id", "")

    st, res = probe(area, "tạo hội viên", "POST", "/api/members",
                    {"username": "kt_hoivien", "password": "test1234",
                     "full_name": "Khach kiem thu", "group_id": ids["member_group"]}, t)
    mem = data_of(res) or {}
    ids["member"] = mem.get("id", "")
    expect(area, "hội viên được gán nhóm", mem.get("group_id") == ids["member_group"],
           detail_bad=f"group_id={mem.get('group_id')}")

    # Ba máy: hai cho các luồng chính, máy thứ ba dành riêng cho phép kiểm tác
    # vụ định kỳ (tự đóng phiên, cưỡng chế giới nghiêm) để không giành máy với
    # các giai đoạn khác đang chạy song song trên cùng database.
    for n in (1, 2, 3):
        st, res = probe(area, f"tạo máy {n}", "POST", "/api/machines",
                        {"machine_code": f"KT-{n:02d}", "group_id": ids["machine_group"]}, t)
        ids[f"machine{n}"] = (data_of(res) or {}).get("id", "")

    st, res = probe(area, "tạo tài sản máy", "POST", "/api/machine-assets",
                    {"machine_id": ids["machine1"], "asset_type": "monitor",
                     "brand": "KT", "serial": "SN-001"}, t)
    ids["asset"] = (data_of(res) or {}).get("id", "")

    st, res = probe(area, "tạo danh mục", "POST", "/api/categories",
                    {"name": "KT-Danh-muc", "printer_id": ids["printer"]}, t)
    ids["category"] = (data_of(res) or {}).get("id", "")

    st, res = probe(area, "tạo nguyên liệu", "POST", "/api/products",
                    {"name": "KT-Nguyen-lieu", "price": 1000, "category_id": ids["category"],
                     "supplier_id": ids["supplier"], "unit_id": ids["unit"],
                     "has_stock": True, "current_stock": 0}, t)
    ids["ingredient"] = (data_of(res) or {}).get("id", "")

    st, res = probe(area, "tạo sản phẩm bán", "POST", "/api/products",
                    {"name": "KT-San-pham", "price": 25000, "category_id": ids["category"],
                     "supplier_id": ids["supplier"], "unit_id": ids["unit"], "is_retail": True}, t)
    ids["product"] = (data_of(res) or {}).get("id", "")

    probe(area, "gắn công thức (BOM)", "POST", f"/api/products/{ids['product']}/ingredients",
          {"ingredient_id": ids["ingredient"], "quantity": 2.0, "unit_id": ids["unit"]}, t)

    probe(area, "nhập kho nguyên liệu", "POST", "/api/stock-transactions",
          {"transaction_type": "inbound", "product_id": ids["ingredient"],
           "quantity": 100.0, "unit_price": 1000, "total_price": 100000,
           "supplier_id": ids["supplier"]}, t)

    return ids


# --------------------------------------------------------------------------- #
# Lớp lỗi: binding required trên trường số từ chối giá trị 0
# --------------------------------------------------------------------------- #

def phase_zero_values(t: str, ids: dict) -> None:
    """`binding:"required"` coi 0 là thiếu, nên các giá trị 0 hợp lệ bị từ chối."""
    area = "2. Giá trị 0"

    st, _ = call("POST", "/api/curfew",
                 {"day_of_week": 0, "curfew_start": "22:00:00", "curfew_end": "06:00:00"}, t)
    report.add(area, "tạo lịch giới nghiêm Chủ nhật (day_of_week=0)",
               OK if st in (200, 201) else WRONG,
               f"{st} — không đặt được lịch Chủ nhật" if st not in (200, 201) else "")

    # Điều chỉnh kho đặt tồn kho về giá trị tuyệt đối, nên phải dùng sản phẩm
    # riêng — nếu không sẽ làm cạn nguyên liệu mà các giai đoạn sau cần.
    st, res = call("POST", "/api/products",
                   {"name": "KT-San-pham-tam", "price": 1000,
                    "category_id": ids["category"], "has_stock": True,
                    "current_stock": 50}, t)
    scratch = (data_of(res) or {}).get("id", "")
    if scratch:
        st, _ = call("POST", "/api/stock-transactions",
                     {"transaction_type": "adjustment", "product_id": scratch,
                      "quantity": 0.0}, t)
        report.add(area, "điều chỉnh kho về 0", OK if st in (200, 201) else WRONG,
                   f"{st} — không ghi nhận được tồn kho bằng 0" if st not in (200, 201) else "")
        call("DELETE", f"/api/products/{scratch}", token=t)

    # Định lượng 0 trong công thức là vô nghĩa nên bị từ chối là đúng; ở đây
    # kiểm rằng lý do từ chối là quy tắc nghiệp vụ chứ không phải "thiếu dữ liệu".
    st, _ = call("POST", f"/api/products/{ids['product']}/ingredients",
                 {"ingredient_id": ids["ingredient"], "quantity": 0.0}, t)
    report.add(area, "công thức từ chối định lượng 0 (đúng)",
               OK if st >= 400 else WRONG,
               "" if st >= 400 else "chấp nhận định lượng 0")


# --------------------------------------------------------------------------- #
# Luồng: phiên chơi, tính tiền, ghi nợ
# --------------------------------------------------------------------------- #

def flow_session(t: str, ids: dict) -> None:
    area = "3. Phiên chơi"

    st, res = call("GET", f"/api/sessions/calculate-cost?machine_id={ids['machine1']}"
                          f"&member_id={ids['member']}&duration_minutes=60", token=t)
    cost = data_of(res) or {}
    # 60 phút × 20.000₫/giờ = 20.000₫, giảm 10% theo hạng ⇒ còn 18.000₫
    expect(area, "tính tiền ra số khác 0", cost.get("gross_cost") == 20000,
           "20.000₫", f"gross_cost={cost.get('gross_cost')} (công cụ tính tiền hỏng)")
    expect(area, "chiết khấu theo hạng được áp dụng", cost.get("discount_amount") == 2000,
           "giảm 2.000₫", f"discount_amount={cost.get('discount_amount')}")
    expect(area, "thành tiền sau giảm", cost.get("final_cost") == 18000,
           "18.000₫", f"final_cost={cost.get('final_cost')}")

    probe(area, "nạp tiền cho hội viên", "POST", f"/api/members/{ids['member']}/topup",
          {"amount": 500000, "payment_method": "cash"}, t)

    st, res = probe(area, "mở máy", "POST", "/api/sessions/start",
                    {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    sid = (data_of(res) or {}).get("id", "")
    ids["session"] = sid

    if sid:
        st, res = probe(area, "danh sách phiên đang chạy", "GET", "/api/sessions/active", token=t)
        rows = data_of(res) if isinstance(data_of(res), list) else items_of(res)
        row = next((r for r in rows if r.get("id") == sid), rows[0] if rows else {})
        # duration_minutes chỉ được ghi lúc TRẢ máy, nên phiên đang chạy trả null:
        # cột "Thời gian" ở Bảng điều khiển và trang Phiên bỏ trống đúng lúc cần nhất.
        expect(area, "phiên đang chạy có số phút đã chơi",
               row.get("duration_minutes") is not None,
               f"{row.get('duration_minutes')} phút",
               "duration_minutes = null — giao diện không biết khách ngồi bao lâu")
        expect(area, "phiên đang chạy có mã máy", bool(row.get("machine_code")),
               row.get("machine_code"), "machine_code rỗng")
        probe(area, "đổi máy", "POST", f"/api/sessions/{sid}/switch-machine",
              {"new_machine_id": ids["machine2"]}, t)

        # Lùi giờ bắt đầu để phát sinh phí thật.
        new_sid = sql("select id from machine_sessions where is_active = true "
                      f"and member_id = '{ids['member']}' limit 1") or sid
        sql(f"update machine_sessions set started_at = started_at - interval '60 minutes' "
            f"where id = '{new_sid}'")

        st, res = probe(area, "trả máy", "POST", f"/api/sessions/{new_sid}/end", token=t)
        end = data_of(res) or {}
        expect(area, "phí phiên khác 0", (end.get("total_cost") or 0) > 0,
               f"{end.get('total_cost')}₫", "tính ra 0₫")
        expect(area, "máy được giải phóng",
               sql(f"select status from machines where id = '{ids['machine2']}'") == "available",
               detail_bad="máy vẫn kẹt ở trạng thái đang dùng")

    # Hết tiền vẫn phải trả được máy, và bị chặn khi mở lại.
    sql(f"update members set balance = 1000, bonus_balance = 0 where id = '{ids['member']}'")
    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    sid2 = (data_of(res) or {}).get("id", "")
    if sid2:
        sql(f"update machine_sessions set started_at = started_at - interval '120 minutes' "
            f"where id = '{sid2}'")
        st, res = call("POST", f"/api/sessions/{sid2}/end", token=t)
        end = data_of(res) or {}
        expect(area, "hết tiền vẫn trả được máy (ghi nợ)", st == 200,
               f"nợ {end.get('amount_unpaid')}₫", f"{st} — máy bị kẹt")
        st, _ = call("POST", "/api/sessions/start",
                     {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
        expect(area, "đang nợ thì không mở máy được", st >= 400,
               detail_bad="vẫn mở được máy dù đang nợ")
        sql(f"update members set balance = 500000 where id = '{ids['member']}'")


# --------------------------------------------------------------------------- #
# Luồng: bảng giá 3 tầng (hạng hội viên > khung giờ > giá cơ bản)
# --------------------------------------------------------------------------- #

def flow_pricing(t: str, ids: dict) -> None:
    """Kiểm ba tầng giá qua ĐÚNG đường mà nhân viên dùng: REST, không chèn tay.

    Trước đây hai bảng giá đầu chỉ được ĐỌC lúc tính tiền — không có CRUD, không
    route, seed không chèn. Máy tính tiền 3 tầng đã kiểm chứng đúng nhưng không
    ai đặt được giá cho nó. Giai đoạn này dọn sạch dấu vết ở cuối để những luồng
    chạy sau vẫn thấy giá cơ bản 20.000₫/giờ.
    """
    area = "3b. Bảng giá"
    now = datetime.now(VN_TZ)
    go_dow = (now.weekday() + 1) % 7

    def cost(minutes: int, member: str | None) -> dict:
        q = f"machine_id={ids['machine1']}&duration_minutes={minutes}"
        if member:
            q += f"&member_id={member}"
        _, r = call("GET", f"/api/sessions/calculate-cost?{q}", token=t)
        return data_of(r) or {}

    base = cost(60, None)
    expect(area, "khởi điểm dùng giá cơ bản", base.get("gross_cost") == 20000,
           "20.000₫", f"nhận {base.get('gross_cost')} — dữ liệu nền không sạch")

    # --- tầng 1: giá theo hạng hội viên ------------------------------------
    st, res = probe(area, "tạo giá theo hạng (6.000₫/giờ, tối thiểu 30 phút)",
                    "POST", "/api/machine-prices",
                    {"machine_group_id": ids["machine_group"],
                     "member_group_id": ids["member_group"],
                     "price_per_hour": 6000, "min_duration": 30}, t)
    tier_id = (data_of(res) or {}).get("id", "")

    c = cost(60, ids["member"])
    expect(area, "khách có hạng được áp giá hạng",
           c.get("price_per_hour") == 6000 and c.get("gross_cost") == 6000,
           "6.000₫/giờ", f"nhận {c.get('price_per_hour')}₫/giờ, thành tiền {c.get('gross_cost')}")

    c = cost(60, None)
    expect(area, "khách vãng lai vẫn giá cơ bản", c.get("gross_cost") == 20000,
           "20.000₫", f"nhận {c.get('gross_cost')} — giá hạng rò sang khách không hạng")

    # MinDuration: ngồi 10 phút ở bảng giá tối thiểu 30 phút vẫn trả tiền 30 phút.
    c = cost(10, ids["member"])
    expect(area, "số phút tối thiểu được thực thi",
           c.get("billed_minutes") == 30 and c.get("gross_cost") == 3000,
           "10 phút → tính 30 phút = 3.000₫",
           f"tính {c.get('billed_minutes')} phút = {c.get('gross_cost')}₫ — min_duration bị bỏ qua")

    # Mở rồi trả ngay: 0 phút không được biến thành 30 phút tiền. Phải đi qua
    # đường thật (mở/trả máy) chứ không qua ô xem trước — ô xem trước chặn
    # duration_minutes=0 ở tầng handler, còn tiền thật thì tính ở tầng service.
    _, res = call("POST", "/api/sessions/start",
                  {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    zero_sid = (data_of(res) or {}).get("id", "")
    if zero_sid:
        _, res = call("POST", f"/api/sessions/{zero_sid}/end", token=t)
        end = data_of(res) or {}
        expect(area, "mở rồi trả ngay không bị tối thiểu sinh tiền",
               end.get("total_cost") == 0,
               "0₫", f"nhận {end.get('total_cost')}₫ — min_duration tính tiền cả phiên 0 phút")
    else:
        report.add(area, "mở rồi trả ngay không bị tối thiểu sinh tiền", BROKEN,
                   "không mở được máy để kiểm")

    st, res = call("GET", f"/api/machine-prices?machine_group_id={ids['machine_group']}", token=t)
    rows = items_of(res) or (data_of(res) if isinstance(data_of(res), list) else [])
    cur = [r for r in rows if r.get("is_current")]
    expect(area, "danh sách đánh dấu dòng đang hiệu lực", len(cur) == 1,
           f"1 dòng: {cur[0].get('price_per_hour') if cur else '-'}₫",
           f"{len(cur)} dòng được đánh dấu — nhân viên không biết giá nào đang chạy")

    # --- tầng 2: giá theo khung giờ ----------------------------------------
    start = (now - timedelta(hours=1)).strftime("%H:%M")
    end = (now + timedelta(hours=1)).strftime("%H:%M")
    st, res = probe(area, "tạo khung giờ cao điểm phủ thời điểm hiện tại",
                    "POST", "/api/time-pricing",
                    {"machine_group_id": ids["machine_group"], "day_of_week": go_dow,
                     "start_time": start, "end_time": end, "price_per_hour": 30000}, t)
    win_id = (data_of(res) or {}).get("id", "")

    c = cost(60, None)
    expect(area, "khung giờ thắng giá cơ bản", c.get("gross_cost") == 30000,
           "30.000₫", f"nhận {c.get('gross_cost')} — khung giờ không được áp")

    c = cost(60, ids["member"])
    expect(area, "giá hạng thắng giá khung giờ", c.get("gross_cost") == 6000,
           "6.000₫", f"nhận {c.get('gross_cost')} — thứ tự ưu tiên 3 tầng sai")

    # --- khung chồng nhau bị từ chối ---------------------------------------
    st, res = call("POST", "/api/time-pricing",
                   {"machine_group_id": ids["machine_group"], "day_of_week": go_dow,
                    "start_time": start, "end_time": end, "price_per_hour": 40000}, t)
    expect(area, "khung giờ chồng nhau bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:70],
           f"nhận {st} — hai khung chồng nhau, giá nào thắng là tuỳ database")

    # Khung vắt qua nửa đêm phải tạo được: quán net chạy xuyên đêm.
    other_dow = (go_dow + 3) % 7
    st, res = probe(area, "tạo khung vắt qua nửa đêm (22:00–02:00)",
                    "POST", "/api/time-pricing",
                    {"machine_group_id": ids["machine_group"], "day_of_week": other_dow,
                     "start_time": "22:00", "end_time": "02:00", "price_per_hour": 25000}, t)
    night_id = (data_of(res) or {}).get("id", "")

    st, res = call("POST", "/api/time-pricing",
                   {"machine_group_id": ids["machine_group"], "day_of_week": other_dow,
                    "start_time": "23:00", "end_time": "23:30", "price_per_hour": 26000}, t)
    expect(area, "khung nằm trong khung vắt nửa đêm cũng bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:70],
           f"nhận {st} — phép so khung không xử lý vắt qua nửa đêm")

    # --- dọn sạch để luồng sau vẫn thấy giá cơ bản -------------------------
    for pid in (win_id, night_id):
        if pid:
            call("DELETE", f"/api/time-pricing/{pid}", token=t)
    if tier_id:
        call("DELETE", f"/api/machine-prices/{tier_id}", token=t)

    c = cost(60, ids["member"])
    expect(area, "xoá bảng giá thì quay về giá cơ bản", c.get("gross_cost") == 20000,
           "20.000₫", f"nhận {c.get('gross_cost')} — dòng giá không thực sự bị xoá")


# --------------------------------------------------------------------------- #
# Luồng: bán hàng và nạp tiền
# --------------------------------------------------------------------------- #

def flow_orders(t: str, ids: dict) -> None:
    area = "4. Bán hàng"

    st, res = probe(area, "tạo đơn", "POST", "/api/orders",
                    {"member_id": ids["member"], "machine_id": ids["machine1"],
                     "items": [{"product_id": ids["product"], "quantity": 2}]}, t)
    order = data_of(res) or {}
    oid = order.get("id", "")
    expect(area, "tổng tiền đơn đúng", order.get("final_amount") == 50000,
           "50.000₫", f"final_amount={order.get('final_amount')}")

    if oid:
        stock_before = sql(f"select current_stock from products where id = '{ids['ingredient']}'")
        probe(area, "xác nhận đơn", "POST", f"/api/orders/{oid}/status",
              {"status": "confirmed"}, t)
        stock_after = sql(f"select current_stock from products where id = '{ids['ingredient']}'")
        expect(area, "xác nhận đơn có trừ kho theo công thức",
               stock_before != stock_after,
               f"{stock_before} → {stock_after}", f"tồn kho không đổi ({stock_before})")

        st, _ = call("POST", f"/api/orders/{oid}/pay",
                     {"payment_method": "cash", "amount": 1}, t)
        expect(area, "từ chối thanh toán sai số tiền", st >= 400,
               detail_bad="chấp nhận trả 1₫ cho đơn 50.000₫")

        bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        st, _ = probe(area, "thanh toán bằng số dư", "POST", f"/api/orders/{oid}/pay",
                      {"payment_method": "balance", "amount": 50000}, t)
        bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        expect(area, "thanh toán bằng số dư có trừ tiền thật",
               bal_before - bal_after == 50000,
               f"{bal_before} → {bal_after}", f"số dư không đổi ({bal_before})")

    # Sửa đơn và tách đơn: hai endpoint đã có từ lâu nhưng trang Đơn hàng không
    # có nút nào gọi tới.
    st, res = call("POST", "/api/orders",
                   {"member_id": ids["member"], "machine_id": ids["machine1"],
                    "items": [{"product_id": ids["product"], "quantity": 4}]}, t)
    split_src = data_of(res) or {}
    sid = split_src.get("id", "")
    if sid:
        st, _ = call("PUT", f"/api/orders/{sid}", {"table_number": "B12", "note": "khach ngoi ban 12"}, t)
        table = sql(f"select table_number from orders where id = '{sid}'")
        expect(area, "sửa đơn ghi được vào database", st == 200 and table == "B12",
               table, f"HTTP {st}, database là {table!r}")
        # Bảng orders không có cột updated_at; service từng gán vào đó nên mọi
        # lần sửa đều 42703. Ghi người sửa vào cột updated_by vốn đã có sẵn.
        expect(area, "lưu lại ai là người sửa đơn",
               sql(f"select updated_by is not null from orders where id = '{sid}'") == "t",
               detail_bad="updated_by rỗng — không truy được ai sửa đơn")

        item_id = sql(f"select id from order_items where order_id = '{sid}' limit 1")
        # Tách nhiều hơn số đang có là vô nghĩa và phải bị chặn.
        st, _ = call("POST", f"/api/orders/{sid}/split",
                     {"items": [{"order_item_id": item_id, "new_quantity": 9}]}, t)
        expect(area, "tách quá số lượng đang có bị từ chối", st >= 400,
               detail_bad="tách được 9 trên đơn chỉ có 4")

        st, res = call("POST", f"/api/orders/{sid}/split",
                       {"items": [{"order_item_id": item_id, "new_quantity": 1}]}, t)
        new_order = data_of(res) or {}
        expect(area, "tách đơn sinh đơn mới", st in (200, 201) and new_order.get("id"),
               f"đơn mới {new_order.get('order_code')}", f"HTTP {st} — {res.get('message', '')[:60]}")
        left = sql(f"select quantity from order_items where id = '{item_id}'")
        expect(area, "đơn gốc bị trừ đúng số đã tách", left == "3",
               f"còn {left}", f"còn {left}, mong 3")
        moved = sql("select coalesce(sum(quantity), 0) from order_items where order_id = "
                    f"'{new_order.get('id', '')}'")
        expect(area, "đơn mới nhận đúng số đã tách", moved == "1",
               detail_bad=f"đơn mới có {moved} sản phẩm")

    # Nạp tiền: hội viên yêu cầu, nhân viên duyệt.
    area = "5. Nạp tiền"
    st, res = probe(area, "hội viên gửi yêu cầu nạp", "POST", "/api/orders/topup-request",
                    {"member_id": ids["member"], "amount": 100000}, t)
    tid = (data_of(res) or {}).get("id", "")
    if tid:
        bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        probe(area, "nhân viên duyệt", "POST", f"/api/orders/{tid}/status",
              {"status": "completed"}, t)
        bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        expect(area, "số dư tăng đúng số tiền", bal_after - bal_before == 100000,
               f"+{bal_after - bal_before}₫", f"{bal_before} → {bal_after}")
        expect(area, "có bút toán trong sổ",
               sql("select count(*) from member_transactions where transaction_type = 'topup' "
                   f"and member_id = '{ids['member']}'") != "0")


# --------------------------------------------------------------------------- #
# Luồng: bộ máy khuyến mãi
# --------------------------------------------------------------------------- #

def flow_promotions(t: str, ids: dict) -> None:
    """Điều kiện khuyến mãi phải được đánh giá thật và biến thành tiền thật.

    Mọi khuyến mãi tạo ở đây đều được tắt trước khi thoát: các giai đoạn sau còn
    tạo đơn, và một khuyến mãi bỏ quên sẽ làm lệch số tiền chúng kỳ vọng.
    """
    area = "4b. Khuyến mãi"
    created: list[str] = []

    def mk(name: str, priority: int, conds: list, rewards: list) -> str:
        st, res = call("POST", "/api/promotions", {
            "name": name, "type": "order", "priority": priority, "is_active": True,
            "conditions": conds, "rewards": rewards,
        }, t)
        pid = (data_of(res) or {}).get("id", "") if st < 300 else ""
        if pid:
            created.append(pid)
        return pid

    def order_discount(qty: int, member: str | None = None) -> tuple:
        """Tạo đơn và trả về (số tiền giảm, tên khuyến mãi đã áp)."""
        body = {"items": [{"product_id": ids["product"], "quantity": qty}]}
        if member:
            body["member_id"] = member
        st, res = call("POST", "/api/orders", body, t)
        o = data_of(res) or {}
        return o.get("discount_amount", 0), (o.get("promotion_name") or ""), o

    # Giá sản phẩm do phase_setup đặt; lấy lại để tính ngưỡng cho đúng.
    _, res = call("GET", f"/api/products/{ids['product']}", None, t)
    price = (data_of(res) or {}).get("price", 0)
    if not price:
        report.add(area, "lấy giá sản phẩm", BROKEN, "không đọc được giá — bỏ qua cả nhóm")
        return

    # Khoá gõ sai phải bị chặn ngay lúc lưu, không lưu thành khuyến mãi câm.
    st, res = call("POST", "/api/promotions", {
        "name": "Khoá sai", "type": "order",
        "conditions": [{"condition_key": "min_amout", "condition_value": 1}],
        "rewards": [{"reward_type": "discount_percent", "reward_value": {"percent": 10}}],
    }, t)
    expect(area, "từ chối khoá điều kiện gõ sai", 400 <= st < 500,
           f"{st} — {res.get('message', '')[:80]}", f"chấp nhận khoá lạ (HTTP {st})")

    # Ngưỡng tiền: dưới ngưỡng không áp, từ ngưỡng trở lên thì áp.
    p_amount = mk("KM ngưỡng tiền", 10,
                  [{"condition_key": "min_amount", "condition_value": price * 2}],
                  [{"reward_type": "discount_percent", "reward_value": {"percent": 10}}])
    if p_amount:
        d, _, _ = order_discount(1)
        expect(area, "dưới ngưỡng tiền thì không giảm", d == 0, detail_bad=f"vẫn giảm {d}₫")
        d, name, _ = order_discount(2)
        want = price * 2 // 10
        expect(area, "đạt ngưỡng thì giảm đúng phần trăm", d == want,
               f"-{d}₫ ({name})", f"giảm {d}₫, mong {want}₫")
        call("PUT", f"/api/promotions/{p_amount}", {"is_active": False}, t)

    # Mức trần chặn phần trăm.
    p_cap = mk("KM có trần", 10, [],
               [{"reward_type": "discount_percent",
                 "reward_value": {"percent": 50, "max_discount": 1000}}])
    if p_cap:
        d, _, _ = order_discount(4)
        expect(area, "mức trần chặn được phần trăm", d == 1000,
               "-1.000₫", f"giảm {d}₫, mong 1.000₫")
        call("PUT", f"/api/promotions/{p_cap}", {"is_active": False}, t)

    # Ưu tiên: chỉ một khuyến mãi được áp, cái priority cao hơn thắng.
    p_low = mk("KM ưu tiên thấp", 1, [],
               [{"reward_type": "discount_amount", "reward_value": {"amount": 5000}}])
    p_high = mk("KM ưu tiên cao", 99, [],
                [{"reward_type": "discount_amount", "reward_value": {"amount": 1000}}])
    if p_low and p_high:
        d, name, _ = order_discount(3)
        expect(area, "ưu tiên cao thắng dù giảm ít hơn",
               d == 1000 and name == "KM ưu tiên cao",
               f"-{d}₫ ({name})", f"áp {name!r} giảm {d}₫, mong 'KM ưu tiên cao' 1.000₫")
        for pid in (p_low, p_high):
            call("PUT", f"/api/promotions/{pid}", {"is_active": False}, t)

    # Khung giờ và hiệu lực.
    p_time = mk("KM khung giờ", 10,
                [{"condition_key": "time_range",
                  "condition_value": {"from": "03:00", "to": "03:01"}}],
                [{"reward_type": "discount_amount", "reward_value": {"amount": 3000}}])
    if p_time:
        d, _, _ = order_discount(1)
        expect(area, "ngoài khung giờ thì không áp", d == 0, detail_bad=f"vẫn giảm {d}₫")
        call("PUT", f"/api/promotions/{p_time}", {
            "conditions": [{"condition_key": "time_range",
                            "condition_value": {"from": "00:00", "to": "23:59"}}],
            "rewards": [{"reward_type": "discount_amount", "reward_value": {"amount": 3000}}],
        }, t)
        d, _, _ = order_discount(1)
        expect(area, "trong khung giờ thì áp ngay", d == 3000,
               "-3.000₫", f"giảm {d}₫, mong 3.000₫")
        call("PUT", f"/api/promotions/{p_time}",
             {"valid_to": "2020-01-01T00:00:00Z"}, t)
        d, _, _ = order_discount(1)
        expect(area, "hết hiệu lực thì không áp", d == 0, detail_bad=f"vẫn giảm {d}₫")
        call("PUT", f"/api/promotions/{p_time}", {"is_active": False}, t)

    # Giảm không bao giờ vượt tổng đơn — không tạo ra đơn âm tiền.
    p_huge = mk("KM quá tay", 10, [],
                [{"reward_type": "discount_amount", "reward_value": {"amount": price * 100}}])
    if p_huge:
        d, _, o = order_discount(1)
        expect(area, "giảm bị kẹp bằng tổng đơn, không âm tiền",
               d == price and o.get("final_amount") == 0,
               f"tổng={price} giảm={d} phải_trả=0",
               f"giảm={d} phải_trả={o.get('final_amount')}")
        call("PUT", f"/api/promotions/{p_huge}", {"is_active": False}, t)

    # Tiền giảm phải là tiền thật: hội viên chỉ bị trừ đúng final_amount.
    p_pay = mk("KM thanh toán", 10, [],
               [{"reward_type": "discount_amount", "reward_value": {"amount": 2000}}])
    if p_pay:
        _, _, o = order_discount(2, ids["member"])
        oid, final = o.get("id", ""), o.get("final_amount", 0)
        if oid:
            call("POST", f"/api/orders/{oid}/status", {"status": "confirmed"}, t)
            bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
            st, _ = call("POST", f"/api/orders/{oid}/pay",
                         {"payment_method": "balance", "amount": final}, t)
            bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
            expect(area, "chỉ trừ số tiền sau giảm, không trừ tổng gốc",
                   st < 300 and bal_before - bal_after == final,
                   f"trừ {bal_before - bal_after}₫ = phải trả {final}₫ (tổng {o.get('total_amount')}₫)",
                   f"trừ {bal_before - bal_after}₫, mong {final}₫")
            expect(area, "đơn ghi lại khuyến mãi đã áp",
                   sql(f"select promotion_id from orders where id = '{oid}'") == p_pay,
                   detail_bad="promotion_id không khớp khuyến mãi đã áp")
        call("PUT", f"/api/promotions/{p_pay}", {"is_active": False}, t)

    # Báo cáo sử dụng khuyến mãi — trước đây luôn rỗng vì không gì ghi discount_amount.
    st, res = call("GET", "/api/reports/promotion-usage", None, t)
    rows = data_of(res) or []
    expect(area, "báo cáo sử dụng khuyến mãi có số liệu",
           st < 300 and len(rows) > 0,
           f"{len(rows)} dòng", "báo cáo rỗng dù đã có đơn được giảm")

    # Tắt sạch để không ảnh hưởng các giai đoạn sau.
    for pid in created:
        call("PUT", f"/api/promotions/{pid}", {"is_active": False}, t)
    still_on = sql("select count(*) from promotions where is_active = true and deleted_at is null")
    expect(area, "đã tắt hết khuyến mãi của kịch bản", still_on == "0",
           detail_bad=f"còn {still_on} khuyến mãi đang bật — các phép kiểm sau có thể lệch tiền")


# --------------------------------------------------------------------------- #
# Máy trạm giả — WebSocket tối giản
# --------------------------------------------------------------------------- #

class FakeMachine:
    """Giả làm một máy trạm nối vào /api/ws/client để nhận lệnh điều khiển.

    Tự cài đặt WebSocket thay vì thêm phụ thuộc: kịch bản này phải chạy được
    bằng Python trần trên bất kỳ máy nào, không cần pip install.

    Chỉ đọc, không gửi — nên bỏ qua toàn bộ phần đóng khung có mask.
    """

    def __init__(self, machine_code: str, token: str = ""):
        self.code = machine_code
        self.sock: socket.socket | None = None
        self.buf = b""
        self._connect(token)

    def _connect(self, token: str) -> None:
        u = urllib.parse.urlparse(BASE)
        host = u.hostname or "localhost"
        port = u.port or (443 if u.scheme == "https" else 80)

        self.sock = socket.create_connection((host, port), timeout=10)
        key = base64.b64encode(os.urandom(16)).decode()
        # Tiến trình nền nối chỉ bằng mã máy; giao diện kèm token người dùng.
        if token:
            path = f"/api/ws/client?machine_code={self.code}&token={urllib.parse.quote(token)}"
        else:
            path = f"/api/ws/client?machine_code={self.code}"
        req = (
            f"GET {path} HTTP/1.1\r\n"
            f"Host: {host}:{port}\r\n"
            "Upgrade: websocket\r\n"
            "Connection: Upgrade\r\n"
            f"Sec-WebSocket-Key: {key}\r\n"
            "Sec-WebSocket-Version: 13\r\n\r\n"
        )
        self.sock.sendall(req.encode())

        while b"\r\n\r\n" not in self.buf:
            chunk = self.sock.recv(4096)
            if not chunk:
                raise RuntimeError("máy chủ đóng kết nối khi bắt tay")
            self.buf += chunk
        head, self.buf = self.buf.split(b"\r\n\r\n", 1)
        if b"101" not in head.split(b"\r\n")[0]:
            raise RuntimeError("bắt tay thất bại: " + head.split(b"\r\n")[0].decode())

    def recv(self, timeout: float = 5.0) -> dict | None:
        """Đọc một sự kiện, hoặc None nếu hết thời gian chờ."""
        deadline = time.time() + timeout
        while True:
            frame = self._take_frame()
            if frame is not None:
                try:
                    return json.loads(frame)
                except json.JSONDecodeError:
                    continue
            remaining = deadline - time.time()
            if remaining <= 0:
                return None
            self.sock.settimeout(remaining)
            try:
                chunk = self.sock.recv(4096)
            except (socket.timeout, TimeoutError):
                return None
            if not chunk:
                return None
            self.buf += chunk

    def _take_frame(self) -> bytes | None:
        b = self.buf
        if len(b) < 2:
            return None
        opcode = b[0] & 0x0F
        masked = bool(b[1] & 0x80)
        length = b[1] & 0x7F
        i = 2
        if length == 126:
            if len(b) < 4:
                return None
            length = int.from_bytes(b[2:4], "big")
            i = 4
        elif length == 127:
            if len(b) < 10:
                return None
            length = int.from_bytes(b[2:10], "big")
            i = 10
        if masked:
            i += 4  # khung từ máy chủ không mask; phòng thân
        if len(b) < i + length:
            return None
        payload = b[i:i + length]
        self.buf = b[i + length:]
        if opcode != 1:  # chỉ quan tâm khung text
            return None
        return payload

    def close(self) -> None:
        if self.sock:
            try:
                self.sock.close()
            except OSError:
                pass


# --------------------------------------------------------------------------- #
# Luồng: vòng quay may mắn
# --------------------------------------------------------------------------- #

def flow_lucky_spin(t: str, ids: dict) -> None:
    """Ô thưởng trước nay chỉ được ĐỌC — không CRUD, seed không chèn, nên mọi
    lượt quay đều rơi vào tập rỗng. Ở đây đặt ô thưởng qua REST rồi quay thật và
    soi lại số dư hội viên."""
    area = "4k. Vòng quay may mắn"

    st, res = call("GET", "/api/lucky-spin/rewards", token=t)
    start_rows = items_of(res) or (data_of(res) if isinstance(data_of(res), list) else [])
    expect(area, "bàn quay khởi điểm rỗng", len(start_rows) == 0,
           detail_bad=f"{len(start_rows)} ô có sẵn — dữ liệu nền không sạch")

    # Loại chưa cấp được phần thưởng phải bị chặn ngay lúc tạo.
    st, res = call("POST", "/api/lucky-spin/rewards",
                   {"name": "KT-Phut-mien-phi", "reward_type": "free_minutes",
                    "amount": 30, "probability": 0.1}, t)
    expect(area, "loại phần thưởng chưa hỗ trợ bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:70],
           f"nhận {st} — quán cấu hình được ô thưởng không bao giờ trả gì")

    st, res = probe(area, "tạo ô thưởng cộng số dư 100%", "POST", "/api/lucky-spin/rewards",
                    {"name": "KT-Thuong-50k", "reward_type": "balance",
                     "amount": 50000, "probability": 1.0, "max_per_day": 5}, t)
    reward_id = (data_of(res) or {}).get("id", "")
    expect(area, "giá trị thưởng được bóc sẵn khỏi jsonb",
           (data_of(res) or {}).get("amount") == 50000,
           "50.000", f"amount={(data_of(res) or {}).get('amount')} — giao diện sẽ thấy ô trống")

    # Tổng xác suất vượt 100% bị chặn: nếu không, ô hiếm sẽ trúng nhiều hơn con
    # số quán đã đặt vì phép chọn phải chuẩn hoá lại.
    st, res = call("POST", "/api/lucky-spin/rewards",
                   {"name": "KT-Thua-xac-suat", "reward_type": "bonus_points",
                    "amount": 1000, "probability": 0.5}, t)
    expect(area, "tổng xác suất vượt 100% bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:70],
           f"nhận {st} — tổng xác suất không được kiểm")
    expect(area, "lý do từ chối nói bằng phần trăm như trên màn hình",
           "%" in res.get("message", ""),
           res.get("message", "")[:60],
           f"báo bằng phân số: {res.get('message','')[:60]}")

    # Quay thật: xác suất 1.0 nên chắc chắn trúng, và số dư phải tăng đúng 50k.
    before = sql(f"select balance from members where id = '{ids['member']}'")
    st, res = call("POST", "/api/lucky-spin/spin", {"member_id": ids["member"]}, t)
    spin = data_of(res) or {}
    expect(area, "quay trúng ô đã cấu hình", st == 200 and spin.get("is_win") is True,
           f"trúng {(spin.get('reward') or {}).get('name')}",
           f"{st} is_win={spin.get('is_win')} — quay vào tập rỗng")
    after = sql(f"select balance from members where id = '{ids['member']}'")
    expect(area, "phần thưởng được cộng vào số dư",
           before.isdigit() and after.isdigit() and int(after) - int(before) == 50000,
           f"{before} → {after}",
           f"{before} → {after} — phần thưởng không tới tay khách")

    # Hạ xuống 1% rồi quay nhiều lượt: phải có lượt trượt. Bản cũ chuẩn hoá theo
    # tổng xác suất nên lượt nào cũng trúng, cột is_win thành vô nghĩa.
    st, _ = call("PUT", f"/api/lucky-spin/rewards/{reward_id}",
                 {"name": "KT-Thuong-50k", "reward_type": "balance",
                  "amount": 1, "probability": 0.01, "max_per_day": 200}, t)
    report.add(area, "sửa ô thưởng", OK if st == 200 else BROKEN, "" if st == 200 else str(st))
    wins = 0
    for _ in range(40):
        _, r = call("POST", "/api/lucky-spin/spin", {"member_id": ids["member"]}, t)
        if (data_of(r) or {}).get("is_win"):
            wins += 1
    expect(area, "xác suất 1% thì phần lớn lượt phải trượt", wins < 20,
           f"{wins}/40 lượt trúng",
           f"{wins}/40 lượt trúng — xác suất bị chuẩn hoá, ô hiếm hoá ra chắc trúng")

    st, res = call("GET", "/api/lucky-spin/rewards?include_inactive=true", token=t)
    rows = items_of(res) or (data_of(res) if isinstance(data_of(res), list) else [])
    expect(area, "màn quản trị thấy được cả ô đã tắt", len(rows) >= 1,
           f"{len(rows)} ô", "không liệt kê được ô thưởng để sửa")

    st, _ = call("DELETE", f"/api/lucky-spin/rewards/{reward_id}", token=t)
    report.add(area, "xoá ô thưởng", OK if st == 200 else BROKEN, "" if st == 200 else str(st))


# --------------------------------------------------------------------------- #
# Luồng: khoá máy và điều khiển từ xa
# --------------------------------------------------------------------------- #

def flow_remote_control(t: str, ids: dict) -> None:
    """Lệnh điều khiển phải tới đúng máy, và phải báo lỗi khi không tới được."""
    area = "4c. Điều khiển từ xa"
    mid = ids.get("machine2") or ids.get("machine1")
    if not mid:
        report.add(area, "chuẩn bị máy", BROKEN, "không có máy nào để kiểm")
        return

    st, res = call("GET", f"/api/machines/{mid}", None, t)
    code = (data_of(res) or {}).get("machine_code", "")
    if not code:
        report.add(area, "đọc mã máy", BROKEN, f"GET /api/machines/{mid} → {st}")
        return

    def remote(action: str, body=None):
        return call("POST", f"/api/machines/{mid}/remote/{action}", body or {}, t)

    # Máy chưa kết nối: phải 409, tuyệt đối không phải 200.
    st, res = remote("lock", {"reason": "kiểm chứng"})
    expect(area, "máy chưa kết nối thì báo lỗi, không báo thành công",
           st == 409, f"409 — {res.get('message', '')}",
           f"HTTP {st} — bản cũ trả 200 dù không máy nào nhận")

    # Lệnh ngoài danh sách cho phép.
    st, res = remote("format_c")
    expect(area, "từ chối lệnh lạ", 400 <= st < 500,
           f"{st} — {res.get('message', '')[:90]}", f"chấp nhận lệnh lạ (HTTP {st})")

    # Không có lệnh chạy câu lệnh tuỳ ý — đây là ranh giới an ninh, không phải
    # thiếu tính năng: máy khách có sẵn ExecuteCommand chạy `cmd /C`.
    for danger in ("execute", "exec", "cmd", "shell"):
        st, _ = remote(danger, {"command": "whoami"})
        if not expect(area, f"không có lệnh chạy câu lệnh tuỳ ý ({danger})",
                      400 <= st < 500, detail_bad=f"lệnh {danger!r} được chấp nhận (HTTP {st})"):
            break

    # Nối một máy trạm giả rồi gửi thật.
    try:
        fm = FakeMachine(code, t)
    except Exception as e:  # noqa: BLE001 — ghi nhận rồi đi tiếp
        report.add(area, "nối máy trạm giả", BROKEN, f"{type(e).__name__}: {e}")
        return

    try:
        time.sleep(0.3)  # hub cần một nhịp để ghi máy vào sổ đăng ký
        st, _ = remote("lock", {"reason": "Hết giờ chơi"})
        expect(area, "máy đã kết nối thì lệnh được chấp nhận", st == 200,
               detail_bad=f"HTTP {st}")

        evt = fm.recv(timeout=5)
        expect(area, "lệnh tới được máy trạm",
               evt is not None and evt.get("type") == "remote:lock",
               f"nhận {evt.get('type') if evt else None}",
               "máy trạm không nhận được gì — chuỗi điều khiển đứt")

        # Lý do khoá phải tới nguyên vẹn: máy khách đọc data.payload.reason.
        reason = ""
        if evt:
            reason = ((evt.get("data") or {}).get("payload") or {}).get("reason", "")
        expect(area, "nội dung lệnh tới nguyên vẹn", reason == "Hết giờ chơi",
               f"reason={reason!r}", f"reason={reason!r}, mong 'Hết giờ chơi'")

        st, _ = remote("unlock")
        evt = fm.recv(timeout=5)
        expect(area, "lệnh mở khoá tới được máy trạm",
               st == 200 and evt is not None and evt.get("type") == "remote:unlock",
               detail_bad=f"HTTP {st}, nhận {evt.get('type') if evt else None}")
    finally:
        fm.close()

    # Nhật ký phải ghi cả lần gửi được lẫn lần không gửi được.
    expect(area, "nhật ký ghi lệnh không gửi được",
           sql("select count(*) from audit_logs where action like 'remote_%' "
               "and metadata->>'delivered' = 'false'") != "0",
           detail_bad="không có bản ghi nào cho lệnh thất bại")
    expect(area, "nhật ký ghi lệnh gửi được",
           sql("select count(*) from audit_logs where action like 'remote_%' "
               "and metadata->>'delivered' = 'true'") != "0",
           detail_bad="không có bản ghi nào cho lệnh thành công")
    expect(area, "nhật ký ghi ai đã ra lệnh",
           sql("select count(*) from audit_logs where action like 'remote_%' "
               "and user_id is not null") != "0",
           detail_bad="không ghi lại người thực hiện")


# --------------------------------------------------------------------------- #
# Máy in giả — máy chủ TCP bắt lại byte ESC/POS
# --------------------------------------------------------------------------- #

class FakePrinter:
    """Nghe TCP như một máy in nhiệt và giữ lại nguyên văn byte nhận được.

    Máy in nhiệt nói ESC/POS trên TCP cổng 9100, không bắt tay gì cả — nên giả
    lập chỉ cần một socket nghe và đọc tới khi phía kia đóng.
    """

    def __init__(self):
        self.jobs: list[bytes] = []
        self._srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._srv.bind(("0.0.0.0", 0))
        self.port = self._srv.getsockname()[1]
        self._srv.listen(8)
        self._stop = False
        self._thread = threading.Thread(target=self._serve, daemon=True)
        self._thread.start()

    def _serve(self) -> None:
        self._srv.settimeout(0.5)
        while not self._stop:
            try:
                conn, _ = self._srv.accept()
            except (socket.timeout, TimeoutError):
                continue
            except OSError:
                return
            with conn:
                conn.settimeout(3)
                data = b""
                try:
                    while True:
                        chunk = conn.recv(4096)
                        if not chunk:
                            break
                        data += chunk
                except (socket.timeout, TimeoutError, OSError):
                    pass
            if data:
                self.jobs.append(data)

    def wait(self, count: int = 1, timeout: float = 6.0) -> bool:
        deadline = time.time() + timeout
        while time.time() < deadline:
            if len(self.jobs) >= count:
                return True
            time.sleep(0.1)
        return len(self.jobs) >= count

    def close(self) -> None:
        self._stop = True
        try:
            self._srv.close()
        except OSError:
            pass


# --------------------------------------------------------------------------- #
# Luồng: in hoá đơn và phân luồng máy in
# --------------------------------------------------------------------------- #

def flow_printing(t: str, ids: dict) -> None:
    """Hoá đơn phải ra đúng byte, đúng máy in, và báo lỗi khi không in được."""
    area = "4d. In hoá đơn"

    # Máy chủ chạy trong Docker nên phải gọi ngược ra host bằng tên riêng của
    # Docker Desktop. Chạy máy chủ trực tiếp trên máy thì đặt VNET_PRINTER_HOST.
    host = os.environ.get("VNET_PRINTER_HOST", "host.docker.internal")

    # Đơn để in: dùng lại sản phẩm của phase_setup.
    st, res = call("POST", "/api/orders", {
        "table_number": "IN-1",
        "items": [{"product_id": ids["product"], "quantity": 2}],
    }, t)
    order = data_of(res) or {}
    oid = order.get("id", "")
    if not oid:
        report.add(area, "tạo đơn để in", BROKEN, f"POST /api/orders → {st}")
        return
    code = order.get("order_code", "")

    # Chưa có máy in nào khai IP hợp lệ → phải báo lỗi, không báo in xong.
    counter = FakePrinter()
    kitchen = FakePrinter()
    try:
        st, res = call("POST", "/api/printers", {
            "name": "Kiem-Quay", "printer_type": "thermal",
            "ip_address": host, "port": counter.port,
            "is_default": True, "chars_per_line": 32, "encoding": "ascii",
        }, t)
        pc = (data_of(res) or {}).get("id", "")
        st, res = call("POST", "/api/printers", {
            "name": "Kiem-Bep", "printer_type": "thermal",
            "ip_address": host, "port": kitchen.port,
            "chars_per_line": 32, "encoding": "ascii",
        }, t)
        pk = (data_of(res) or {}).get("id", "")
        if not pc or not pk:
            report.add(area, "khai máy in", BROKEN, f"POST /api/printers → {st}")
            return

        # --- hoá đơn khách ---
        st, res = call("POST", f"/api/orders/{oid}/print", {}, t)
        if st != 200:
            msg = str(res.get("message", ""))
            if "connect" in msg or "dial" in msg:
                report.add(area, "in hoá đơn", SKIP,
                           f"máy chủ không gọi ngược ra host được ({host}) — đặt VNET_PRINTER_HOST")
                return
            report.add(area, "in hoá đơn", BROKEN, f"{st} — {msg[:120]}")
            return
        report.add(area, "in hoá đơn được chấp nhận", OK, f"POST /api/orders/{oid}/print → 200")

        got = counter.wait(1)
        expect(area, "máy in nhận được dữ liệu", got,
               f"{len(counter.jobs[0]) if counter.jobs else 0} byte",
               "không byte nào tới máy in")
        job = counter.jobs[0] if counter.jobs else b""

        expect(area, "có lệnh khởi tạo ESC @", job.startswith(b"\x1b@"),
               detail_bad="thiếu ESC @ — hoá đơn trước để lại định dạng gì thì giữ nguyên")
        expect(area, "có lệnh cắt giấy GS V", b"\x1dV\x00" in job,
               detail_bad="thiếu lệnh cắt — giấy không rời máy")
        expect(area, "hoá đơn có mã đơn", code.encode() in job,
               detail_bad=f"không thấy {code} trong dữ liệu gửi máy in")

        # --- bản xem trước phải khớp giấy ---
        st, res = call("GET", f"/api/orders/{oid}/receipt-preview", None, t)
        preview = (data_of(res) or {}).get("text", "")
        expect(area, "xem trước khớp với giấy in",
               bool(preview) and code in preview
               and all(line.encode() in job for line in preview.splitlines() if line.strip()),
               f"{len(preview.splitlines())} dòng",
               "bản xem trước lệch với dữ liệu gửi máy in")

        # --- phân luồng ---
        st, res = call("POST", f"/api/orders/{oid}/print-stations", {}, t)
        d = data_of(res) or {}
        expect(area, "món chưa gán máy in thì báo ra, không nuốt",
               st == 200 and not d.get("jobs") and d.get("unrouted"),
               f"unrouted={d.get('unrouted')}",
               f"jobs={d.get('jobs')} unrouted={d.get('unrouted')}")

        st, res = call("PUT", f"/api/printers/{pk}/products",
                       {"product_ids": [ids["product"]]}, t)
        expect(area, "gán món cho máy in", st == 200, detail_bad=f"HTTP {st}")

        st, res = call("POST", f"/api/orders/{oid}/print-stations", {}, t)
        d = data_of(res) or {}
        expect(area, "phiếu chế biến ra đúng máy được gán",
               st == 200 and len(d.get("jobs") or []) == 1
               and (d["jobs"][0].get("printer_name") == "Kiem-Bep"),
               f"{d.get('jobs')}", f"HTTP {st} jobs={d.get('jobs')}")

        expect(area, "phiếu tới máy bếp", kitchen.wait(1),
               detail_bad="máy bếp không nhận được gì")
        ticket = kitchen.jobs[0] if kitchen.jobs else b""

        # Phiếu bếp KHÔNG được có giá: phiếu lọt ra ngoài cũng không lộ doanh thu.
        price = formatMoneyPy(order.get("total_amount", 0))
        expect(area, "phiếu chế biến không in giá", price.encode() not in ticket,
               detail_bad=f"phiếu bếp có chứa số tiền {price}")

        expect(area, "hoá đơn khách KHÔNG ra máy bếp", len(kitchen.jobs) == 1,
               detail_bad=f"máy bếp nhận {len(kitchen.jobs)} bản in")

        # --- tiếng Việt có dấu ---
        # Lỗi từng có: chế độ cp1258 bỏ dấu để căn cột rồi mới mã hoá, nên máy
        # in hỗ trợ tiếng Việt vẫn nhận chữ không dấu.
        #
        # Tên cửa hàng phải có dấu thì phép kiểm mới nói lên điều gì — sản phẩm
        # do phase_setup tạo có tên không dấu, tìm byte có dấu trên đó thì bao
        # giờ cũng "không thấy" dù code đúng hay sai.
        call("PUT", "/api/settings/general", {"store_name": "Quán Cà Phê Sữa Đá"}, t)
        call("PUT", f"/api/printers/{pc}", {"encoding": "cp1258", "code_page": 30}, t)
        before = len(counter.jobs)
        st, _ = call("POST", f"/api/orders/{oid}/print", {}, t)
        if st == 200 and counter.wait(before + 1):
            vi = counter.jobs[before]
            expect(area, "chế độ có dấu gửi lệnh chọn bảng mã", b"\x1bt\x1e" in vi,
                   detail_bad="thiếu ESC t — máy in không biết dùng bảng mã nào")
            expect(area, "chế độ có dấu KHÔNG bỏ dấu trước khi mã hoá",
                   any(bytes([b]) in vi for b in (0xCC, 0xEC, 0xDE, 0xD2, 0xF2, 0xEA, 0xF0)),
                   detail_bad="không byte có dấu nào — dấu bị bỏ trước khi mã hoá")
        else:
            report.add(area, "chế độ tiếng Việt có dấu", BROKEN, f"in lại → {st}")

        # --- máy in không nối được ---
        counter.close()
        time.sleep(0.3)
        st, res = call("POST", f"/api/orders/{oid}/print", {}, t)
        expect(area, "máy in tắt thì báo lỗi, không báo in xong", st == 502,
               f"502 — {(data_of(res) or {}).get('jobs')}",
               f"HTTP {st} — bản cũ sẽ báo thành công")
    finally:
        counter.close()
        kitchen.close()


def formatMoneyPy(v: int) -> str:
    """Định dạng tiền giống hàm formatMoney bên Go, để dò trên giấy in."""
    s = f"{abs(v)}"
    parts = []
    while len(s) > 3:
        parts.insert(0, s[-3:])
        s = s[:-3]
    parts.insert(0, s)
    return ".".join(parts)


# --------------------------------------------------------------------------- #
# Luồng: thẻ nạp và thẻ quà tặng
# --------------------------------------------------------------------------- #

def flow_cards(t: str, ids: dict) -> None:
    """Thẻ là tiền. Ba thứ phải đúng: mã không lưu thô, không dùng lại được,
    và hai người tiêu cùng lúc không ra hai lần tiền."""
    area = "4e. Thẻ nạp & quà tặng"

    # --- thẻ nạp ---
    st, res = call("POST", "/api/topup-cards/generate",
                   {"count": 3, "face_value": 100000, "bonus_value": 20000}, t)
    cards = (data_of(res) or {}).get("cards") or []
    if st != 200 or len(cards) != 3:
        report.add(area, "sinh lô thẻ nạp", BROKEN, f"POST /api/topup-cards/generate → {st}")
        return
    report.add(area, "sinh lô thẻ nạp", OK, "3 thẻ, mỗi thẻ 100.000 + 20.000")

    expect(area, "mã bí mật chỉ trả về lúc sinh thẻ",
           all(c.get("secret") for c in cards),
           detail_bad="phản hồi sinh thẻ không kèm mã — không in thẻ ra được")

    # Danh sách KHÔNG được kèm mã bí mật.
    st, res = call("GET", "/api/topup-cards", None, t)
    rows = items_of(res)
    expect(area, "danh sách không lộ mã bí mật",
           bool(rows) and all("pin" not in r and "secret" not in r for r in rows),
           detail_bad="mã bí mật lọt ra API danh sách")

    # Database phải lưu băm, không lưu mã thô.
    raw = cards[0]["secret"]
    expect(area, "database lưu băm chứ không lưu mã thô",
           sql(f"select count(*) from topup_cards where pin = '{raw}'") == "0"
           and sql("select min(length(pin)) from topup_cards") == "64",
           detail_bad="cột pin chứa mã thô — đọc được database là tiêu được thẻ")

    bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
    bonus_before = int(sql(f"select bonus_balance from members where id = '{ids['member']}'") or 0)
    st, res = call("POST", "/api/topup-cards/redeem",
                   {"serial": cards[0]["serial"], "secret": cards[0]["secret"],
                    "member_id": ids["member"]}, t)
    bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
    bonus_after = int(sql(f"select bonus_balance from members where id = '{ids['member']}'") or 0)
    expect(area, "nạp thẻ cộng đúng mệnh giá và khuyến mãi",
           st == 200 and bal_after - bal_before == 100000 and bonus_after - bonus_before == 20000,
           f"số dư +{bal_after - bal_before}, khuyến mãi +{bonus_after - bonus_before}",
           f"HTTP {st}, số dư +{bal_after - bal_before}, khuyến mãi +{bonus_after - bonus_before}")

    st, res = call("POST", "/api/topup-cards/redeem",
                   {"serial": cards[0]["serial"], "secret": cards[0]["secret"],
                    "member_id": ids["member"]}, t)
    expect(area, "thẻ đã nạp không nạp lại được", st >= 400,
           detail_bad="nạp lại được cùng một thẻ — nhân đôi tiền")

    # Sai seri và sai mã phải trả CÙNG một thông báo: phân biệt được là dò ra
    # seri nào có thật rồi mới đánh phần bí mật.
    _, r1 = call("POST", "/api/topup-cards/redeem",
                 {"serial": cards[1]["serial"], "secret": "SAISAISAISAISAI2",
                  "member_id": ids["member"]}, t)
    _, r2 = call("POST", "/api/topup-cards/redeem",
                 {"serial": "TCKHONGCOTHAT", "secret": "SAISAISAISAISAI2",
                  "member_id": ids["member"]}, t)
    expect(area, "sai seri và sai mã báo lỗi giống hệt nhau",
           r1.get("message") == r2.get("message") and bool(r1.get("message")),
           f"{r1.get('message')!r}",
           f"sai mã: {r1.get('message')!r} · sai seri: {r2.get('message')!r}")

    expect(area, "nạp hụt có vào nhật ký",
           sql("select count(*) from audit_logs where action = 'redeem_failed'") != "0",
           detail_bad="không ghi lại lần nạp hụt nào — không phát hiện được người dò mã")

    # Nhiều người nạp cùng một thẻ cùng lúc: đúng một người được tiền.
    third = cards[2]
    bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
    codes = parallel_post("/api/topup-cards/redeem",
                          {"serial": third["serial"], "secret": third["secret"],
                           "member_id": ids["member"]}, t, times=8)
    bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
    wins = sum(1 for c in codes if c == 200)
    expect(area, "8 người nạp cùng lúc thì đúng 1 người được tiền",
           wins == 1 and bal_after - bal_before == 100000,
           f"{wins}/8 thành công, số dư +{bal_after - bal_before}",
           f"{wins}/8 thành công, số dư +{bal_after - bal_before} (mong 1 và +100.000)")

    # --- thẻ quà tặng ---
    st, res = call("POST", "/api/gift-cards/generate", {"count": 1, "value": 100000}, t)
    gift = ((data_of(res) or {}).get("cards") or [{}])[0]
    if not gift.get("serial"):
        report.add(area, "sinh thẻ quà tặng", BROKEN, f"POST /api/gift-cards/generate → {st}")
        return
    report.add(area, "sinh thẻ quà tặng", OK, "1 thẻ 100.000")

    st, res = call("POST", "/api/gift-cards/check",
                   {"serial": gift["serial"], "secret": gift["secret"]}, t)
    expect(area, "tra được số dư thẻ quà tặng",
           st == 200 and (data_of(res) or {}).get("balance") == 100000,
           detail_bad=f"HTTP {st} — {data_of(res)}")

    st, res = call("POST", "/api/gift-cards/check",
                   {"serial": gift["serial"], "secret": "SAISAISAISAISAI2"}, t)
    expect(area, "tra số dư bằng mã sai bị từ chối", st >= 400,
           detail_bad="chỉ cần seri là xem được số dư")

    # Tiêu dần qua nhiều đơn.
    def new_order(qty: int) -> dict:
        """Tạo và xác nhận một đơn. Trả về dict rỗng nếu không tạo được —
        chỗ gọi phải tự kiểm, đừng để KeyError làm sập cả bộ kiểm."""
        st, r = call("POST", "/api/orders",
                     {"items": [{"product_id": ids["product"], "quantity": qty}]}, t)
        o = data_of(r) or {}
        if not o.get("id"):
            report.add(area, f"tạo đơn {qty} món", BROKEN,
                       f"POST /api/orders → {st} {r.get('message', '')}"[:140])
            return {}
        call("POST", f"/api/orders/{o['id']}/status", {"status": "confirmed"}, t)
        return o

    o = new_order(1)
    if not o:
        return
    price = o.get("final_amount", 0)
    st, _ = call("POST", f"/api/orders/{o['id']}/pay",
                 {"payment_method": "gift_card", "amount": price,
                  "card_serial": gift["serial"], "card_secret": gift["secret"]}, t)
    left = sql(f"select balance from gift_cards where serial = '{gift['serial']}'")
    expect(area, "thanh toán bằng thẻ quà tặng trừ đúng số tiền",
           st == 200 and int(left or 0) == 100000 - price,
           f"còn {left}", f"HTTP {st}, còn {left}, mong {100000 - price}")

    expect(area, "có bút toán trong sổ cái thẻ",
           sql(f"select count(*) from gift_card_transactions t join gift_cards g "
               f"on g.id = t.gift_card_id where g.serial = '{gift['serial']}' "
               f"and t.order_id is not null") != "0",
           detail_bad="không ghi sổ lần tiêu thẻ")

    # Đơn quá số dư thẻ: phải từ chối VÀ không được chốt đơn.
    big = new_order(20)
    if not big:
        return
    st, res = call("POST", f"/api/orders/{big['id']}/pay",
                   {"payment_method": "gift_card", "amount": big.get("final_amount", 0),
                    "card_serial": gift["serial"], "card_secret": gift["secret"]}, t)
    status_after = sql(f"select status from orders where id = '{big['id']}'")
    expect(area, "thẻ không đủ tiền thì đơn KHÔNG bị chốt",
           st >= 400 and status_after != "completed",
           f"{st}, đơn vẫn {status_after}",
           f"HTTP {st}, đơn thành {status_after}")

    # Nhiều đơn tiêu cùng một thẻ cùng lúc: không được tiêu quá số dư.
    st, res = call("POST", "/api/gift-cards/generate", {"count": 1, "value": price * 3}, t)
    g2 = ((data_of(res) or {}).get("cards") or [{}])[0]
    orders = [o for o in (new_order(1) for _ in range(8)) if o]
    if len(orders) < 8:
        report.add(area, "chuẩn bị 8 đơn để tiêu thẻ", BROKEN,
                   f"chỉ tạo được {len(orders)}/8 đơn")
        return
    results = []

    def pay(o):
        code, _ = call("POST", f"/api/orders/{o['id']}/pay",
                       {"payment_method": "gift_card", "amount": o.get("final_amount", 0),
                        "card_serial": g2["serial"], "card_secret": g2["secret"]}, t)
        results.append(code)

    threads = [threading.Thread(target=pay, args=(o,)) for o in orders]
    for th in threads:
        th.start()
    for th in threads:
        th.join()
    left = int(sql(f"select balance from gift_cards where serial = '{g2['serial']}'") or -1)
    wins = sum(1 for c in results if c == 200)
    expect(area, "8 đơn tiêu cùng lúc không vượt quá số dư thẻ",
           wins == 3 and left == 0,
           f"{wins}/8 đơn qua, thẻ còn {left}",
           f"{wins}/8 đơn qua, thẻ còn {left} (mong 3 đơn và còn 0)")

    expect(area, "tiêu hết thì thẻ tự chuyển sang đã dùng",
           sql(f"select status from gift_cards where serial = '{g2['serial']}'") == "used",
           detail_bad="thẻ hết tiền vẫn ở trạng thái còn dùng")

    # Huỷ thẻ.
    st, res = call("POST", "/api/topup-cards/generate",
                   {"count": 1, "face_value": 50000}, t)
    spare = ((data_of(res) or {}).get("cards") or [{}])[0]
    st, _ = call("POST", f"/api/topup-cards/{spare.get('id')}/cancel", {}, t)
    expect(area, "huỷ được thẻ chưa dùng", st == 200, detail_bad=f"HTTP {st}")
    st, _ = call("POST", "/api/topup-cards/redeem",
                 {"serial": spare["serial"], "secret": spare["secret"],
                  "member_id": ids["member"]}, t)
    expect(area, "thẻ đã huỷ không nạp được", st >= 400,
           detail_bad="thẻ đã huỷ vẫn nạp được")

    st, _ = call("POST", f"/api/topup-cards/{cards[0]['id']}/cancel", {}, t)
    expect(area, "không huỷ được thẻ đã dùng", st >= 400,
           detail_bad="huỷ được thẻ đã dùng — sổ sách sai")


def parallel_post(path: str, body, token: str, times: int) -> list:
    """Bắn cùng một request nhiều lần đồng thời, trả về danh sách mã trạng thái."""
    out: list = []
    lock = threading.Lock()

    def one():
        code, _ = call("POST", path, body, token)
        with lock:
            out.append(code)

    threads = [threading.Thread(target=one) for _ in range(times)]
    for th in threads:
        th.start()
    for th in threads:
        th.join()
    return out


# --------------------------------------------------------------------------- #
# Luồng: kiểm kê kho
# --------------------------------------------------------------------------- #

def flow_inventory_count(t: str, ids: dict) -> None:
    """Kiểm kê phải khớp tồn kho với thực tế mà KHÔNG xoá mất hàng bán ra giữa
    lúc đếm và lúc chốt."""
    area = "4f. Kiểm kê kho"

    # Mặt hàng riêng cho nhóm này: đụng vào hàng của giai đoạn khác sẽ làm lệch
    # số tiền chúng kỳ vọng.
    st, res = call("POST", "/api/products", {
        "name": "KK-Bia", "category_id": ids["category"], "price": 20000,
        "is_retail": True, "has_stock": True, "current_stock": 12,
    }, t)
    pid = (data_of(res) or {}).get("id", "")
    if not pid:
        report.add(area, "tạo mặt hàng để kiểm kê", BROKEN, f"POST /api/products → {st}")
        return

    def stock() -> float:
        _, r = call("GET", f"/api/products/{pid}", None, t)
        return float((data_of(r) or {}).get("current_stock", 0))

    st, res = call("POST", "/api/inventory-counts", {"note": "kiểm chứng"}, t)
    sess = data_of(res) or {}
    sid = sess.get("id", "")
    expect(area, "mở được phiên kiểm kê", st in (200, 201) and bool(sid),
           sess.get("code", ""), f"HTTP {st}")
    if not sid:
        return

    st, res = call("POST", f"/api/inventory-counts/{sid}/lines",
                   {"product_id": pid, "actual_qty": 10}, t)
    line = data_of(res) or {}
    expect(area, "ghi số đếm và tính đúng chênh lệch",
           st == 200 and line.get("expected_qty") == 12 and line.get("difference_qty") == -2,
           f"sổ {line.get('expected_qty')}, đếm {line.get('actual_qty')}, lệch {line.get('difference_qty')}",
           f"HTTP {st} — {line}")

    # Đếm lại cùng mặt hàng phải GHI ĐÈ, không tạo dòng thứ hai mâu thuẫn.
    call("POST", f"/api/inventory-counts/{sid}/lines",
         {"product_id": pid, "actual_qty": 10}, t)
    _, res = call("GET", f"/api/inventory-counts/{sid}", None, t)
    expect(area, "đếm lại thì ghi đè dòng cũ",
           (data_of(res) or {}).get("line_count") == 1,
           detail_bad=f"có {(data_of(res) or {}).get('line_count')} dòng cho cùng một mặt hàng")

    # Bán một cái GIỮA lúc đếm và lúc chốt.
    st, res = call("POST", "/api/orders",
                   {"items": [{"product_id": pid, "quantity": 1}]}, t)
    oid = (data_of(res) or {}).get("id", "")
    if not oid:
        report.add(area, "bán hàng giữa phiên", BROKEN, f"POST /api/orders → {st}")
        return
    call("POST", f"/api/orders/{oid}/status", {"status": "confirmed"}, t)
    mid = stock()
    expect(area, "bán hàng giữa phiên có trừ kho", mid == 11,
           f"sổ còn {mid}", f"sổ còn {mid}, mong 11")

    st, res = call("POST", f"/api/inventory-counts/{sid}/commit", {}, t)
    final = stock()
    # Áp chênh lệch: 11 + (-2) = 9. Đặt tuyệt đối sẽ ra 10 và xoá mất lần bán.
    expect(area, "chốt phiên áp CHÊNH LỆCH, không đặt tồn bằng số đã đếm",
           st == 200 and final == 9,
           f"11 + (-2) = {final}",
           f"tồn kho thành {final}; 10 nghĩa là đặt tuyệt đối và đã xoá mất lần bán")

    expect(area, "sinh phiếu điều chỉnh kho",
           sql(f"select count(*) from stock_transactions where transaction_type = 'adjustment' "
               f"and reference_id = '{sid}'") != "0",
           detail_bad="không có phiếu điều chỉnh nào tham chiếu phiên này")

    st, _ = call("POST", f"/api/inventory-counts/{sid}/commit", {}, t)
    expect(area, "không chốt lại được phiên đã chốt", st >= 400,
           detail_bad="chốt được hai lần — điều chỉnh kho nhân đôi")

    st, _ = call("POST", f"/api/inventory-counts/{sid}/lines",
                 {"product_id": pid, "actual_qty": 5}, t)
    expect(area, "không sửa được phiên đã chốt", st >= 400, detail_bad=f"HTTP {st}")

    # Phiên rỗng không chốt được.
    _, res = call("POST", "/api/inventory-counts", {"note": "rỗng"}, t)
    empty = (data_of(res) or {}).get("id", "")
    st, _ = call("POST", f"/api/inventory-counts/{empty}/commit", {}, t)
    expect(area, "phiên chưa đếm gì thì không chốt được", st >= 400, detail_bad=f"HTTP {st}")

    # Huỷ phiên không được đụng tồn kho.
    before = stock()
    _, res = call("POST", "/api/inventory-counts", {"note": "sẽ huỷ"}, t)
    doomed = (data_of(res) or {}).get("id", "")
    call("POST", f"/api/inventory-counts/{doomed}/lines",
         {"product_id": pid, "actual_qty": 999}, t)
    st, _ = call("POST", f"/api/inventory-counts/{doomed}/cancel", {}, t)
    expect(area, "huỷ phiên không đụng tồn kho", st == 200 and stock() == before,
           f"tồn kho vẫn {before}", f"tồn kho đổi từ {before} thành {stock()}")

    # Mặt hàng không theo dõi tồn kho thì không kiểm kê được.
    _, res = call("POST", "/api/products", {
        "name": "KK-Dich-vu", "category_id": ids["category"], "price": 5000,
        "is_retail": True, "has_stock": False,
    }, t)
    nostock = (data_of(res) or {}).get("id", "")
    _, res = call("POST", "/api/inventory-counts", {"note": "hàng không kho"}, t)
    sid2 = (data_of(res) or {}).get("id", "")
    st, _ = call("POST", f"/api/inventory-counts/{sid2}/lines",
                 {"product_id": nostock, "actual_qty": 1}, t)
    expect(area, "mặt hàng không theo dõi kho thì từ chối", st >= 400, detail_bad=f"HTTP {st}")
    call("POST", f"/api/inventory-counts/{sid2}/cancel", {}, t)


# --------------------------------------------------------------------------- #
# Luồng: điểm danh hằng ngày
# --------------------------------------------------------------------------- #

def flow_attendance(t: str, ids: dict) -> None:
    """Một hội viên điểm danh đúng một lần mỗi ngày, kể cả khi bấm dồn dập."""
    area = "4g. Điểm danh"

    call("PUT", "/api/settings/attendance",
         {"daily_bonus": "2000", "streak_bonus": "20000", "streak_every": "7"}, t)

    st, res = call("POST", "/api/members", {
        "username": "kt_diemdanh", "password": "test1234",
        "full_name": "KT Diem danh", "phone": "0900111222",
    }, t)
    mid = (data_of(res) or {}).get("id", "")
    if not mid:
        report.add(area, "tạo hội viên để điểm danh", BROKEN, f"POST /api/members → {st}")
        return

    def bonus() -> int:
        return int(sql(f"select bonus_balance from members where id = '{mid}'") or 0)

    st, res = call("GET", f"/api/attendance/status?member_id={mid}", None, t)
    d = data_of(res) or {}
    expect(area, "chưa điểm danh thì trạng thái nói rõ",
           st == 200 and d.get("checked_in_today") is False and d.get("streak_days") == 0,
           detail_bad=f"HTTP {st} — {d}")

    before = bonus()
    st, res = call("POST", "/api/attendance/checkin", {"member_id": mid}, t)
    d = data_of(res) or {}
    expect(area, "điểm danh cộng thưởng vào số dư khuyến mãi",
           st == 200 and d.get("streak_days") == 1 and bonus() - before == 2000,
           f"chuỗi {d.get('streak_days')}, thưởng +{bonus() - before}",
           f"HTTP {st}, thưởng +{bonus() - before}")

    # Thưởng phải vào bonus_balance, KHÔNG vào balance: tiền quán tặng khác
    # tiền khách nạp, trộn vào nhau sẽ làm sai sổ khi hoàn tiền.
    expect(area, "thưởng không lẫn vào số dư nạp",
           int(sql(f"select balance from members where id = '{mid}'") or -1) == 0,
           detail_bad="tiền thưởng chui vào balance thay vì bonus_balance")

    after_first = bonus()
    st, res = call("POST", "/api/attendance/checkin", {"member_id": mid}, t)
    expect(area, "điểm danh lần hai trong ngày bị từ chối", st == 409,
           f"409 — {res.get('message', '')}", f"HTTP {st}")
    expect(area, "lần bị từ chối không cộng thêm tiền", bonus() == after_first,
           detail_bad=f"số dư khuyến mãi đổi từ {after_first} thành {bonus()}")

    # Bấm dồn dập: ràng buộc duy nhất ở database phải chặn, không phải câu lệnh
    # SELECT trước INSERT.
    st, res = call("POST", "/api/members", {
        "username": "kt_dd_race", "password": "test1234",
        "full_name": "KT Race", "phone": "0900111333",
    }, t)
    rid = (data_of(res) or {}).get("id", "")
    codes = parallel_post("/api/attendance/checkin", {"member_id": rid}, t, times=8)
    rbonus = int(sql(f"select bonus_balance from members where id = '{rid}'") or -1)
    rows = sql(f"select count(*) from member_attendances where member_id = '{rid}'")
    wins = sum(1 for c in codes if c == 200)
    expect(area, "8 lần bấm cùng lúc chỉ ghi được 1 lượt",
           wins == 1 and rows == "1" and rbonus == 2000,
           f"{wins}/8 thành công, {rows} bản ghi, thưởng {rbonus}",
           f"{wins}/8 thành công, {rows} bản ghi, thưởng {rbonus}")

    # Chuỗi ngày: chèn lịch sử lùi ngày vì không chờ được 7 hôm.
    st, res = call("POST", "/api/members", {
        "username": "kt_dd_streak", "password": "test1234",
        "full_name": "KT Streak", "phone": "0900111444",
    }, t)
    sid = (data_of(res) or {}).get("id", "")
    sql(f"insert into member_attendances (member_id, checkin_date, checkin_at, streak_days, "
        f"reward_amount, reward_claimed) values ('{sid}', CURRENT_DATE - 1, now(), 6, 2000, true)")

    # Cột date của PostgreSQL về Go là nửa đêm UTC, còn "hôm nay" ở máy chủ là
    # nửa đêm giờ địa phương: so bằng instant sẽ báo chuỗi về 0 dù hôm qua vẫn
    # điểm danh. Đây là chốt chặn cho lỗi đó.
    st, res = call("GET", f"/api/attendance/status?member_id={sid}", None, t)
    d = data_of(res) or {}
    expect(area, "chuỗi ngày không bị lệch múi giờ",
           d.get("streak_days") == 6,
           f"chuỗi {d.get('streak_days')}",
           f"chuỗi báo {d.get('streak_days')}, mong 6 — nhiều khả năng so ngày bằng instant")

    st, res = call("POST", "/api/attendance/checkin", {"member_id": sid}, t)
    d = data_of(res) or {}
    expect(area, "chạm mốc chuỗi thì cộng thêm thưởng mốc",
           st == 200 and d.get("streak_days") == 7 and d.get("reward_amount") == 22000,
           f"chuỗi {d.get('streak_days')}, thưởng {d.get('reward_amount')}",
           f"chuỗi {d.get('streak_days')}, thưởng {d.get('reward_amount')}, mong 7 và 22.000")

    # Đứt chuỗi thì về 1.
    st, res = call("POST", "/api/members", {
        "username": "kt_dd_broken", "password": "test1234",
        "full_name": "KT Broken", "phone": "0900111555",
    }, t)
    bid = (data_of(res) or {}).get("id", "")
    sql(f"insert into member_attendances (member_id, checkin_date, checkin_at, streak_days, "
        f"reward_amount, reward_claimed) values ('{bid}', CURRENT_DATE - 10, now(), 5, 2000, true)")
    st, res = call("POST", "/api/attendance/checkin", {"member_id": bid}, t)
    expect(area, "bỏ nhiều ngày thì chuỗi về 1",
           (data_of(res) or {}).get("streak_days") == 1,
           detail_bad=f"chuỗi {(data_of(res) or {}).get('streak_days')}, mong 1")

    st, res = call("GET", "/api/attendance?page_size=50", None, t)
    expect(area, "danh sách điểm danh kèm tên hội viên",
           st == 200 and any(r.get("member_username") for r in items_of(res)),
           detail_bad="danh sách trống hoặc thiếu tên hội viên")


# --------------------------------------------------------------------------- #
# Luồng: chặn website
# --------------------------------------------------------------------------- #

def flow_website_block(t: str, ids: dict) -> None:
    """Máy trạm phải nhận đúng danh sách tên miền cần chặn NGAY LÚC NÀY: lọc
    theo luật đang bật, lịch áp dụng và nhóm máy."""
    area = "4h. Chặn website"

    # Hai nhóm máy riêng để kiểm phạm vi áp dụng.
    _, res = call("POST", "/api/machine-groups",
                  {"name": "WB-NhomA", "price_per_hour": 10000}, t)
    ga = (data_of(res) or {}).get("id", "")
    _, res = call("POST", "/api/machine-groups",
                  {"name": "WB-NhomB", "price_per_hour": 10000}, t)
    gb = (data_of(res) or {}).get("id", "")

    _, res = call("POST", "/api/machines", {"machine_code": "WB-A1", "group_id": ga}, t)
    ma = data_of(res) or {}
    _, res = call("POST", "/api/machines", {"machine_code": "WB-B1", "group_id": gb}, t)
    mb = data_of(res) or {}
    if not ma.get("id") or not mb.get("id"):
        report.add(area, "chuẩn bị máy để kiểm", BROKEN, "không tạo được máy")
        return

    # Route by-code mở: danh sách chặn lấy được chỉ bằng mã máy.
    def blocklist(code: str):
        st, r = call("GET", f"/api/machines/by-code/{code}/blocklist")
        return st, (data_of(r) or {}).get("domains", [])

    # Chuẩn hoá: mọi cách viết cùng một tên miền phải quy về một luật.
    forms = {
        "https://www.Facebook.com/abc?x=1": "facebook.com",
        "*.tiktok.com": "tiktok.com",
        "youtube.com:443": "youtube.com",
    }
    rule_ids = {}
    ok_norm = True
    for raw, want in forms.items():
        st, r = call("POST", "/api/website-rules", {"pattern": raw, "category": "kt"}, t)
        d = data_of(r) or {}
        if st >= 300 or d.get("pattern") != want:
            ok_norm = False
        rule_ids[want] = d.get("id", "")
    expect(area, "chuẩn hoá tên miền về một dạng", ok_norm,
           "giao thức, www, đường dẫn, cổng và '*.' đều bị gỡ",
           "có dạng viết không quy về tên miền gốc")

    # Không gán nhóm nào → áp cho mọi máy.
    _, da = blocklist("WB-A1")
    _, db_ = blocklist("WB-B1")
    expect(area, "luật không gán nhóm thì áp cho mọi máy",
           "facebook.com" in da and "facebook.com" in db_,
           f"A={len(da)} luật, B={len(db_)} luật",
           f"A={da} B={db_}")

    # Gán riêng cho nhóm A.
    st, _ = call("PUT", f"/api/website-rules/{rule_ids['facebook.com']}/groups",
                 {"machine_group_ids": [ga]}, t)
    _, da = blocklist("WB-A1")
    _, db_ = blocklist("WB-B1")
    expect(area, "gán nhóm thì chỉ máy trong nhóm bị chặn",
           st == 200 and "facebook.com" in da and "facebook.com" not in db_,
           f"A có facebook, B không",
           f"A={da} B={db_}")

    # Lịch ngoài giờ hiện tại → không áp.
    tt = rule_ids["tiktok.com"]
    st, _ = call("PUT", f"/api/website-rules/{tt}/schedules",
                 {"schedules": [{"day_of_week": [0, 1, 2, 3, 4, 5, 6],
                                 "start_time": "03:00", "end_time": "03:01"}]}, t)
    _, da = blocklist("WB-A1")
    expect(area, "ngoài khung giờ của lịch thì không chặn",
           st == 200 and "tiktok.com" not in da,
           detail_bad=f"HTTP {st}, danh sách {da}")

    # Mảng integer[] phải đọc lại nguyên vẹn. Khai []int với type:integer[] thì
    # driver mã hoá thành record và PostgreSQL từ chối — lỗi này cũng làm hỏng
    # khung ngày áp dụng của gói cước.
    st, res = call("GET", f"/api/website-rules/{tt}", None, t)
    scheds = (data_of(res) or {}).get("schedules") or []
    expect(area, "mảng thứ-trong-tuần lưu và đọc lại nguyên vẹn",
           len(scheds) == 1 and scheds[0].get("day_of_week") == [0, 1, 2, 3, 4, 5, 6],
           f"{scheds[0].get('day_of_week') if scheds else None}",
           f"đọc lại được {scheds[0].get('day_of_week') if scheds else None}")

    # Lịch phủ cả ngày → áp trở lại.
    call("PUT", f"/api/website-rules/{tt}/schedules",
         {"schedules": [{"day_of_week": [0, 1, 2, 3, 4, 5, 6],
                         "start_time": "00:00", "end_time": "23:59"}]}, t)
    _, da = blocklist("WB-A1")
    expect(area, "trong khung giờ thì chặn trở lại", "tiktok.com" in da,
           detail_bad=f"danh sách {da}")

    # Chỉ áp vào thứ khác hôm nay → không chặn.
    other = [d for d in range(7) if d != int(time.strftime("%w"))]
    call("PUT", f"/api/website-rules/{tt}/schedules",
         {"schedules": [{"day_of_week": other, "start_time": "00:00", "end_time": "23:59"}]}, t)
    _, da = blocklist("WB-A1")
    expect(area, "lịch không có hôm nay thì không chặn", "tiktok.com" not in da,
           detail_bad=f"danh sách {da}")

    # Luật allow trừ khỏi danh sách chặn.
    call("PUT", f"/api/website-rules/{tt}/schedules", {"schedules": []}, t)
    st, res = call("POST", "/api/website-rules",
                   {"pattern": "youtube.com", "rule_type": "allow"}, t)
    _, da = blocklist("WB-A1")
    expect(area, "luật cho phép trừ khỏi danh sách chặn",
           st in (200, 201) and "youtube.com" not in da,
           detail_bad=f"HTTP {st}, danh sách {da}")

    # Tắt luật thì biến khỏi danh sách.
    call("PUT", f"/api/website-rules/{rule_ids['facebook.com']}", {"is_active": False}, t)
    _, da = blocklist("WB-A1")
    expect(area, "tắt luật thì máy trạm không nhận nữa", "facebook.com" not in da,
           detail_bad=f"danh sách {da}")

    # Khoá máy trạm đã bỏ: route by-code mở, nhận diện bằng mã máy, không cần khoá.
    st, _ = call("GET", "/api/machines/by-code/WB-A1/blocklist")
    expect(area, "route by-code mở: lấy danh sách chặn không cần khoá", st == 200,
           f"{st}", f"HTTP {st}")

    # Báo vi phạm.
    st, _ = call("POST", "/api/machines/by-code/WB-A1/blocklist/violations",
                 {"domain": "https://www.tiktok.com/xyz", "process_name": "chrome.exe"})
    expect(area, "máy trạm báo được lần truy cập bị chặn", st == 200, detail_bad=f"HTTP {st}")

    st, res = call("GET", "/api/website-violations?page_size=20", None, t)
    rows = items_of(res)
    hit = [r for r in rows if r.get("domain") == "tiktok.com"]
    expect(area, "vi phạm được chuẩn hoá và gắn mã máy",
           bool(hit) and hit[0].get("machine_code") == "WB-A1",
           f"{hit[0].get('domain')} trên {hit[0].get('machine_code')}" if hit else "",
           f"{rows[:1]}")
    expect(area, "vi phạm gắn được với luật đã chặn",
           bool(hit) and hit[0].get("rule_id"),
           detail_bad="không truy được lần chặn thuộc luật nào")

    # Tên miền vô nghĩa bị chặn ngay lúc tạo luật.
    st, _ = call("POST", "/api/website-rules", {"pattern": "khong-phai-ten-mien"}, t)
    expect(area, "từ chối tên miền không hợp lệ", st >= 400, detail_bad=f"HTTP {st}")


# --------------------------------------------------------------------------- #
# Luồng: cập nhật máy khách
# --------------------------------------------------------------------------- #

def flow_app_update(t: str, ids: dict) -> None:
    """Máy trạm phải thấy đúng bản mới nhất, và băm là bắt buộc."""
    area = "4i. Cập nhật máy khách"

    _, res = call("POST", "/api/machines", {"machine_code": "AU-01"}, t)
    m = data_of(res) or {}
    if not m.get("id"):
        report.add(area, "chuẩn bị máy để kiểm", BROKEN, "không tạo được máy")
        return
    # Máy mới không có khoá: mã máy là đủ.
    token = ""

    good = "a" * 64

    def publish(version, checksum=good, required=False, platform="windows-amd64"):
        return call("POST", "/api/app-updates", {
            "version": version, "platform": platform,
            "file_url": "https://example.invalid/vnet-client.exe",
            "checksum": checksum, "file_size": 1024,
            "changelog": "ban " + version, "is_required": required,
        }, t)

    def latest(current, platform="windows-amd64"):
        st, r = call("GET",
                     f"/api/machines/by-code/AU-01/app-update?platform={platform}&current={current}")
        return st, (data_of(r) or {})

    # Băm là bắt buộc và phải là SHA-256 hex.
    for bad in ("", "khong-phai-bam", "z" * 64):
        st, _ = publish("9.9.9", checksum=bad)
        if not expect(area, f"từ chối công bố bản không có băm hợp lệ ({bad[:12] or 'rỗng'})",
                      st >= 400, detail_bad=f"chấp nhận băm {bad!r} (HTTP {st})"):
            break

    st, _ = publish("1.2.0")
    expect(area, "công bố được bản cập nhật", st in (200, 201), detail_bad=f"HTTP {st}")

    st, d = latest("1.2.0")
    expect(area, "đang ở bản mới nhất thì không báo cập nhật",
           st == 200 and d.get("has_update") is False,
           detail_bad=f"HTTP {st} — {d}")

    st, d = latest("1.1.0")
    expect(area, "bản cũ hơn thì báo có cập nhật và trả kèm băm",
           st == 200 and d.get("has_update") is True
           and d.get("version") == "1.2.0" and d.get("checksum") == good,
           f"1.1.0 → {d.get('version')}", f"HTTP {st} — {d}")

    # So phiên bản bằng chuỗi là sai: "1.10.0" < "1.9.0" theo thứ tự chữ cái.
    publish("1.10.0")
    st, d = latest("1.9.0")
    expect(area, "so phiên bản theo số, không theo chuỗi",
           d.get("has_update") is True and d.get("version") == "1.10.0",
           f"1.9.0 → {d.get('version')}",
           f"1.9.0 → {d.get('version')}; nếu ra 1.2.0 thì đang so chuỗi")

    # Bản cho nền tảng khác không được trả về.
    publish("2.0.0", platform="linux-amd64")
    st, d = latest("1.10.0")
    expect(area, "không trả bản của nền tảng khác",
           d.get("has_update") is False,
           detail_bad=f"máy windows nhận được bản {d.get('version')}")

    # Tắt bản cập nhật thì máy trạm không thấy nữa.
    st, res = call("GET", "/api/app-updates?platform=windows-amd64&page_size=50", None, t)
    rows = items_of(res)
    latest_row = [r for r in rows if r.get("version") == "1.10.0"]
    if latest_row:
        call("PUT", f"/api/app-updates/{latest_row[0]['id']}/active", {"is_active": False}, t)
        st, d = latest("1.9.0")
        expect(area, "tắt bản cập nhật thì máy trạm không nhận nữa",
               d.get("version") != "1.10.0",
               f"1.9.0 → {d.get('version')}",
               "vẫn trả bản đã tắt")

    # Trùng phiên bản cho cùng nền tảng.
    st, _ = publish("1.2.0")
    expect(area, "từ chối công bố trùng phiên bản cùng nền tảng", st >= 400,
           detail_bad=f"HTTP {st}")

    # Route by-code mở: hỏi bản cập nhật chỉ bằng mã máy.
    st, _ = call("GET", "/api/machines/by-code/AU-01/app-update?platform=windows-amd64&current=1.0.0")
    expect(area, "hỏi được bản cập nhật bằng mã máy", st == 200, f"{st}", f"HTTP {st}")

    expect(area, "công bố bản cập nhật có vào nhật ký",
           sql("select count(*) from audit_logs where action = 'publish_app_update'") != "0",
           detail_bad="không ghi lại ai đã công bố bản nào")


# --------------------------------------------------------------------------- #
# Luồng: đánh giá dịch vụ
# --------------------------------------------------------------------------- #

def flow_feedback(t: str, ids: dict) -> None:
    """Đánh giá phải gắn đúng máy hội viên đang ngồi, và không tin mã máy do
    máy khách gửi."""
    area = "4j. Đánh giá dịch vụ"

    # Đăng nhập trên máy trạm MỞ LUÔN phiên — đó là cả điểm của màn hình khoá.
    st, res = call("POST", "/api/auth/member-login",
                   {"username": "kt_hoivien", "password": "test1234", "machine_code": "KT-01"})
    d = data_of(res) or {}
    mt, session_id = d.get("access_token", ""), d.get("session_id", "")
    if not mt:
        report.add(area, "đăng nhập hội viên", BROKEN, f"member-login → {st} {res.get('message','')[:70]}")
        return
    expect(area, "đăng nhập trên máy trạm mở luôn phiên", bool(session_id),
           session_id[:8], "không có session_id — khách ngồi máy mà không gì tính tiền")

    # Trả máy rồi thì không còn phiên nào để gắn đánh giá.
    call("POST", f"/api/sessions/{session_id}/end", token=t)
    st, res = call("POST", "/api/feedback", {"rating": 5, "content": "tot"}, mt)
    expect(area, "chưa vào phiên thì không đánh giá được", st >= 400,
           f"{st} — {res.get('message', '')[:60]}", f"HTTP {st}")

    # Phiên này phải được đóng trước khi rời giai đoạn: để máy ở trạng thái
    # đang dùng sẽ làm giai đoạn gói cước phía sau không mở được máy nào, và
    # mục HỎNG đó là lỗi của kịch bản chứ không phải của sản phẩm.
    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    if st >= 300:
        report.add(area, "mở phiên để đánh giá", BROKEN, f"sessions/start → {st} {res.get('message','')[:90]}")
        return
    session_id = (data_of(res) or {}).get("id", "")

    st, res = call("POST", "/api/feedback", {"rating": 5, "content": "Máy chạy mượt"}, mt)
    d = data_of(res) or {}
    expect(area, "hội viên trong phiên gửi được đánh giá",
           st in (200, 201) and d.get("rating") == 5,
           detail_bad=f"HTTP {st} — {res.get('message', '')[:80]}")

    # Máy phải suy ra từ phiên, KHÔNG lấy từ request: gửi kèm mã máy khác cũng
    # không đổi được kết quả.
    st, res = call("POST", "/api/feedback",
                   {"rating": 1, "content": "gan may khac", "machine_id": ids["machine2"]}, mt)
    d = data_of(res) or {}
    expect(area, "không tin mã máy do máy khách gửi",
           st in (200, 201) and d.get("machine_id") == ids["machine1"],
           f"gắn vào máy đang chơi, bỏ qua machine_id trong request",
           f"đánh giá bị gắn vào {d.get('machine_id')} thay vì máy đang chơi")

    expect(area, "điểm ngoài khoảng 1–5 bị từ chối",
           call("POST", "/api/feedback", {"rating": 0}, mt)[0] >= 400
           and call("POST", "/api/feedback", {"rating": 6}, mt)[0] >= 400,
           detail_bad="chấp nhận điểm ngoài 1–5")

    # Quầy gỡ máy khỏi danh sách trong lúc khách đang ngồi: xoá là xoá MỀM nên
    # phiên vẫn chạy, nhưng nếu truy vấn máy lọc theo deleted_at thì khách nhận
    # câu "không tìm thấy máy" và không gửi được gì.
    sql(f"update machines set deleted_at = now() where id = '{ids['machine1']}'")
    st, res = call("POST", "/api/feedback", {"rating": 3, "content": "may bi go giua chung"}, mt)
    sql(f"update machines set deleted_at = null where id = '{ids['machine1']}'")
    expect(area, "máy bị xoá mềm giữa phiên vẫn đánh giá được", st in (200, 201),
           detail_bad=f"HTTP {st} — {res.get('message', '')[:60]}")

    # Danh sách đánh giá cũng phải giữ mã máy sau khi máy bị gỡ, nếu không cột
    # "Máy" trống trơn đúng chỗ quán cần để biết máy nào hay bị chê.
    sql(f"update machines set deleted_at = now() where id = '{ids['machine1']}'")
    _, res = call("GET", "/api/feedback?page_size=5", token=t)
    rows = items_of(res)
    sql(f"update machines set deleted_at = null where id = '{ids['machine1']}'")
    on_m1 = [r for r in rows if r.get("machine_id") == ids["machine1"]]
    expect(area, "danh sách đánh giá giữ mã máy kể cả khi máy đã xoá mềm",
           bool(on_m1) and all(r.get("machine_code") for r in on_m1),
           f"{len(on_m1)} dòng đều có mã máy",
           "machine_code rỗng — cột Máy trên giao diện trống")

    # Đánh giá gắn đơn: mỗi đơn một lần.
    st, res = call("POST", "/api/orders",
                   {"member_id": ids["member"],
                    "items": [{"product_id": ids["product"], "quantity": 1}]}, t)
    oid = (data_of(res) or {}).get("id", "")
    if oid:
        st1, _ = call("POST", "/api/feedback",
                      {"rating": 4, "content": "ngon", "order_id": oid}, mt)
        st2, res2 = call("POST", "/api/feedback",
                         {"rating": 2, "content": "danh gia lai", "order_id": oid}, mt)
        expect(area, "mỗi đơn chỉ đánh giá được một lần",
               st1 in (200, 201) and st2 == 409,
               f"lần đầu {st1}, lần hai {st2}",
               f"lần đầu {st1}, lần hai {st2} — mong 409")

    # Không đánh giá đơn của người khác.
    st, res = call("POST", "/api/orders",
                   {"items": [{"product_id": ids["product"], "quantity": 1}]}, t)
    other = (data_of(res) or {}).get("id", "")
    if other:
        st, _ = call("POST", "/api/feedback", {"rating": 1, "order_id": other}, mt)
        expect(area, "không đánh giá được đơn của người khác", st >= 400,
               detail_bad=f"HTTP {st}")

    # Tổng hợp.
    st, res = call("GET", "/api/feedback/summary", None, t)
    d = data_of(res) or {}
    dist = d.get("distribution") or {}
    expect(area, "tổng hợp có điểm trung bình và đủ 5 mức sao",
           st == 200 and d.get("total", 0) > 0 and len(dist) == 5,
           f"trung bình {d.get('average')}, {d.get('total')} lượt",
           f"HTTP {st} — {d}")

    # Phân bố phải có đủ 5 mức kể cả mức chưa ai chấm: thiếu cột sẽ khiến
    # "không ai chấm 1 sao" trông giống "chưa có dữ liệu".
    expect(area, "mức sao chưa ai chấm vẫn hiện là 0",
           all(str(i) in dist for i in range(1, 6)),
           detail_bad=f"phân bố thiếu mức: {sorted(dist.keys())}")

    st, res = call("GET", "/api/feedback?with_content=true&page_size=20", None, t)
    rows = items_of(res)
    expect(area, "danh sách kèm mã máy và tên hội viên",
           st == 200 and bool(rows) and rows[0].get("machine_code")
           and rows[0].get("member_username"),
           detail_bad=f"{rows[:1]}")

    # Hội viên không được xem danh sách đánh giá của cả quán.
    st, _ = call("GET", "/api/feedback", None, mt)
    expect(area, "hội viên không xem được đánh giá của người khác", st == 403,
           f"{st}", f"HTTP {st}")

    if session_id:
        call("POST", f"/api/sessions/{session_id}/end", {}, t)


# --------------------------------------------------------------------------- #
# Luồng: gói cước, đặt chỗ, ca trực
# --------------------------------------------------------------------------- #

def flow_combo_booking_shift(t: str, ids: dict) -> None:
    area = "6. Gói cước"
    st, res = probe(area, "tạo gói cước", "POST", "/api/combos",
                    {"name": "KT-Goi-3h", "type": "prepaid", "price": 50000,
                     "total_minutes": 180, "validity_days": 30}, t)
    cid = (data_of(res) or {}).get("id", "")
    if cid:
        bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        st, res = probe(area, "mua gói bằng số dư", "POST", f"/api/combos/{cid}/purchase",
                        {"member_id": ids["member"], "payment_method": "balance"}, t)
        pid = (data_of(res) or {}).get("id", "")
        bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        expect(area, "mua gói có trừ tiền", bal_before - bal_after == 50000,
               f"-{bal_before - bal_after}₫", f"số dư {bal_before} → {bal_after}")
        if pid:
            probe(area, "kích hoạt gói", "POST", f"/api/combos/{pid}/activate",
                  {"machine_id": ids["machine1"]}, t)

        # Số dư 0 vẫn mua được gói là lỗ hổng đã sửa.
        sql(f"update members set balance = 0 where id = '{ids['member']}'")
        st, _ = call("POST", f"/api/combos/{cid}/purchase",
                     {"member_id": ids["member"], "payment_method": "balance"}, t)
        expect(area, "hết tiền thì không mua được gói", st >= 400,
               detail_bad="vẫn mua được gói dù số dư bằng 0")
        sql(f"update members set balance = 500000 where id = '{ids['member']}'")

    area = "7. Đặt chỗ"
    frm = (datetime.now(VN_TZ) + timedelta(days=1)).replace(microsecond=0)
    st, res = probe(area, "đặt chỗ kèm tiền cọc", "POST", "/api/bookings",
                    {"machine_id": ids["machine1"], "member_id": ids["member"],
                     "customer_name": "Khach KT", "customer_phone": "0900000001",
                     "booked_from": frm.isoformat(),
                     "booked_to": (frm + timedelta(hours=2)).isoformat(),
                     "deposit_amount": 50000}, t)
    booking = data_of(res) or {}
    bid = booking.get("id", "")
    # Cột "Mã máy" trên trang Đặt chỗ đọc machine_code; DTO không trả trường này
    # nên ô đó luôn trống và câu xác nhận check-in thành "tại máy .".
    expect(area, "đặt chỗ trả kèm mã máy", booking.get("machine_code") == "KT-01",
           booking.get("machine_code"),
           f"machine_code={booking.get('machine_code')!r} — giao diện không biết đang giữ máy nào")
    if bid:
        expect(area, "tiền cọc được thu thật",
               sql("select count(*) from member_transactions "
                   f"where transaction_type = 'booking_deposit' and member_id = '{ids['member']}'") != "0",
               detail_bad="không có bút toán thu cọc — đặt rồi huỷ sẽ sinh tiền")
        bal_before = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        probe(area, "huỷ đặt chỗ", "POST", f"/api/bookings/{bid}/cancel", token=t)
        bal_after = int(sql(f"select balance from members where id = '{ids['member']}'") or 0)
        expect(area, "hoàn đúng phần đã thu", bal_after - bal_before == 50000,
               f"+{bal_after - bal_before}₫", f"{bal_before} → {bal_after}")

    # Sửa / đánh vắng / xoá đặt chỗ: ba endpoint đã có từ lâu nhưng trang Đặt chỗ
    # không có nút nào gọi tới, nên cũng chưa từng được kiểm.
    st, res = call("POST", "/api/bookings",
                   {"machine_id": ids["machine2"], "customer_name": "Khach vang lai",
                    "customer_phone": "0900000002",
                    "booked_from": (frm + timedelta(days=1)).isoformat(),
                    "booked_to": (frm + timedelta(days=1, hours=2)).isoformat()}, t)
    b2 = (data_of(res) or {}).get("id", "")
    if b2:
        st, _ = call("PUT", f"/api/bookings/{b2}",
                     {"customer_name": "Khach da doi ten", "customer_phone": "0900000003",
                      "notes": "doi ten tai quay"}, t)
        name = sql(f"select customer_name from machine_bookings where id = '{b2}'")
        expect(area, "sửa đặt chỗ ghi được vào database", name == "Khach da doi ten",
               name, f"database vẫn là {name!r}")

        st, _ = call("POST", f"/api/bookings/{b2}/no-show", token=t)
        status = sql(f"select status from machine_bookings where id = '{b2}'")
        expect(area, "đánh vắng đổi trạng thái sang no_show",
               st == 200 and status == "no_show",
               status, f"HTTP {st}, trạng thái {status!r}")

        st, _ = call("DELETE", f"/api/bookings/{b2}", token=t)
        left = sql(f"select count(*) from machine_bookings where id = '{b2}' and deleted_at is null")
        expect(area, "xoá đặt chỗ", st == 200 and left == "0",
               detail_bad=f"HTTP {st}, còn {left} dòng")

    area = "8. Ca trực"
    st, res = probe(area, "mở ca", "POST", "/api/shifts/open",
                    {"opening_balance": 500000, "notes": "kiem thu"}, t)
    shift = (data_of(res) or {}).get("id", "")
    if shift:
        probe(area, "ghi nhận tiền mặt vào két", "POST", f"/api/shifts/{shift}/handover",
              {"amount": 100000, "handover_type": "cash_in", "reason": "kiem thu"}, t)
        st, res = probe(area, "đóng ca", "POST", f"/api/shifts/{shift}/close",
                        {"closing_balance": 700000, "notes": "ket thuc"}, t)
        sh = data_of(res) or {}
        # Cột "Người dùng" trên trang Ca làm việc đọc user_name; thiếu trường này
        # bảng hiện UUID thô.
        _, res = call("GET", "/api/shifts?page_size=5", token=t)
        rows = items_of(res)
        expect(area, "danh sách ca trực kèm tên người trực",
               bool(rows) and all(r.get("user_name") for r in rows),
               rows[0].get("user_name") if rows else "",
               "user_name rỗng — giao diện hiện UUID thô")

        expect(area, "tiền dự kiến khác 0", (sh.get("expected_total") or 0) != 0,
               f"{sh.get('expected_total')}₫",
               "expected_total = 0 (truy vấn trạng thái đơn không tồn tại)")
        expect(area, "chênh lệch được tính và lưu", sh.get("discrepancy") is not None,
               f"{sh.get('discrepancy')}₫", "không có số chênh lệch")

        st, _ = call("POST", "/api/shifts/open", {"opening_balance": 0}, t)
        if st in (200, 201):
            s2 = sql("select id from shifts where status = 'open' order by started_at desc limit 1")
            st2, _ = call("POST", f"/api/shifts/{s2}/close", {"closing_balance": 0}, t)
            report.add(area, "đóng ca với két rỗng (closing_balance=0)",
                       OK if st2 in (200, 201) else WRONG,
                       f"{st2} — không đóng được ca khi két rỗng" if st2 not in (200, 201) else "")


# --------------------------------------------------------------------------- #
# Giới nghiêm
# --------------------------------------------------------------------------- #

def flow_curfew(t: str, ids: dict) -> None:
    area = "9. Giới nghiêm"
    now = datetime.now(VN_TZ)

    st, res = call("POST", "/api/curfew",
                   {"day_of_week": now.weekday() + 1 if now.weekday() < 6 else 0,
                    "curfew_start": "00:00:01", "curfew_end": "23:59:59",
                    "max_minor_hours": 2, "is_active": True}, t)
    if st not in (200, 201):
        report.add(area, "tạo lịch giới nghiêm", BROKEN, f"{st} {res.get('message','')}")
        return
    report.add(area, "tạo lịch giới nghiêm", OK)

    # Weekday của Go là 0=CN; Python là 0=T2. Đặt đúng thứ hôm nay theo chuẩn Go.
    go_dow = (now.weekday() + 1) % 7
    sql(f"update curfew_policies set day_of_week = {go_dow} where is_active = true")

    st, res = call("POST", "/api/members",
                   {"username": "kt_vithanhnien", "password": "test1234",
                    "full_name": "Vi thanh nien",
                    "date_of_birth": (now - timedelta(days=365 * 15)).date().isoformat() + "T00:00:00+07:00"}, t)
    minor = (data_of(res) or {}).get("id", "")
    if not minor:
        report.add(area, "tạo hội viên vị thành niên", BROKEN, f"{st} {res.get('message','')}")
        return
    sql(f"update members set balance = 200000 where id = '{minor}'")

    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine2"], "member_id": minor}, t)
    expect(area, "vị thành niên bị chặn trong khung giờ cấm", st >= 400,
           f"bị từ chối: {res.get('message','')}"[:80],
           "vẫn mở được máy — giới nghiêm không có hiệu lực")

    sql("update curfew_policies set is_active = false")
    ids["minor"] = minor


# --------------------------------------------------------------------------- #
# Luồng: vai trò, tài khoản nhân viên, phân quyền, đổi mật khẩu
# --------------------------------------------------------------------------- #

def flow_roles_users(t: str, ids: dict) -> None:
    """CreateRole/UpdateRole/DeleteRole đã viết đủ nhưng không có handler; bảng
    role_permissions chỉ được ghi một lần bởi cmd/seed. Cài xong là không đổi
    được vai trò hay quyền của ai nữa."""
    area = "10b. Vai trò & phân quyền"

    st, res = probe(area, "tạo vai trò", "POST", "/api/systemManage/addRole",
                    {"roleName": "KT-Vai-tro", "roleCode": "KT-Vai-tro",
                     "roleDesc": "Vai trò kiểm chứng"}, t)
    role_id = (data_of(res) or {}).get("id", "")
    if not role_id:
        return

    st, _ = call("POST", "/api/systemManage/updateRole",
                 {"id": role_id, "roleName": "KT-Vai-tro-2", "roleDesc": "Đã sửa"}, t)
    name = sql(f"select name from roles where id = '{role_id}'")
    expect(area, "sửa vai trò ghi được vào database", name == "KT-Vai-tro-2",
           name, f"database vẫn là {name!r} — trang Vai trò báo thành công mà không gửi gì")

    # --- phân quyền --------------------------------------------------------
    st, res = call("GET", "/api/systemManage/getAllPermissions", token=t)
    perms = data_of(res) or []
    expect(area, "liệt kê được toàn bộ mã quyền", isinstance(perms, list) and len(perms) > 0,
           f"{len(perms) if isinstance(perms, list) else 0} mã quyền",
           "không có mã quyền nào để gán")
    if not perms:
        return

    pick = [p["id"] for p in perms if p.get("code") in ("members.view", "orders.view")]
    st, _ = call("POST", "/api/systemManage/updateRolePermissions",
                 {"roleId": role_id, "permissionIds": pick}, t)
    report.add(area, "gán quyền cho vai trò", OK if st == 200 else BROKEN,
               "" if st == 200 else str(st))
    count = sql(f"select count(*) from role_permissions where role_id = '{role_id}'")
    expect(area, "quyền được ghi vào role_permissions", count == str(len(pick)),
           f"{count} dòng", f"{count} dòng, mong {len(pick)}")

    st, res = call("GET", f"/api/systemManage/getRolePermissions?roleId={role_id}", token=t)
    got = data_of(res) or []
    expect(area, "đọc lại đúng danh sách quyền vừa gán", sorted(got) == sorted(pick),
           f"{len(got)} mã", f"nhận {got}")

    # Bỏ tick hết phải gỡ sạch quyền, không phải bị coi là "không có gì để làm".
    call("POST", "/api/systemManage/updateRolePermissions",
         {"roleId": role_id, "permissionIds": []}, t)
    count = sql(f"select count(*) from role_permissions where role_id = '{role_id}'")
    expect(area, "gỡ hết quyền là gỡ thật", count == "0",
           detail_bad=f"còn {count} dòng — không bỏ tick hết được")

    st, res = call("POST", "/api/systemManage/updateRolePermissions",
                   {"roleId": role_id, "permissionIds": ["00000000-0000-0000-0000-000000000000"]}, t)
    expect(area, "mã quyền không tồn tại bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:60], f"nhận {st}")

    # --- tài khoản nhân viên ------------------------------------------------
    st, res = probe(area, "tạo tài khoản nhân viên", "POST", "/api/systemManage/addUser",
                    {"userName": "kt_nhanvien", "password": "test1234",
                     "nickName": "Nhan vien kiem thu", "userRoles": ["KT-Vai-tro-2"]}, t)
    user_id = (data_of(res) or {}).get("id", "")
    expect(area, "tài khoản mới thực sự đăng nhập được",
           call("POST", "/api/auth/login", {"username": "kt_nhanvien", "password": "test1234"})[0] == 200,
           detail_bad="đăng nhập thất bại — trang Người dùng báo thành công mà không tạo gì")

    st, res = call("POST", "/api/systemManage/addUser",
                   {"userName": "kt_khong_mat_khau", "nickName": "Thieu mat khau"}, t)
    expect(area, "tạo tài khoản không mật khẩu bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:60], f"nhận {st} — tài khoản không đăng nhập được")

    # --- đổi mật khẩu -------------------------------------------------------
    _, res = call("POST", "/api/auth/login", {"username": "kt_nhanvien", "password": "test1234"}, None)
    staff_token = (data_of(res) or {}).get("access_token", "")

    st, res = call("PUT", "/api/auth/change-password",
                   {"old_password": "sai-mat-khau", "new_password": "moi12345"}, staff_token)
    expect(area, "đổi mật khẩu sai mật khẩu cũ bị từ chối", st == 400,
           f"400: {res.get('message','')}"[:60], f"nhận {st} — đổi được mà không cần biết mật khẩu cũ")

    st, _ = call("PUT", "/api/auth/change-password",
                 {"old_password": "test1234", "new_password": "moi12345"}, staff_token)
    report.add(area, "đổi mật khẩu", OK if st == 200 else BROKEN, "" if st == 200 else str(st))
    expect(area, "mật khẩu mới đăng nhập được",
           call("POST", "/api/auth/login", {"username": "kt_nhanvien", "password": "moi12345"})[0] == 200,
           detail_bad="mật khẩu mới không dùng được")
    expect(area, "mật khẩu cũ hết tác dụng",
           call("POST", "/api/auth/login", {"username": "kt_nhanvien", "password": "test1234"})[0] >= 400,
           detail_bad="mật khẩu cũ vẫn đăng nhập được")

    # --- xoá ----------------------------------------------------------------
    st, res = call("DELETE", "/api/systemManage/deleteRole", {"id": role_id}, t)
    expect(area, "vai trò còn người dùng thì không xoá được", st == 400,
           f"400: {res.get('message','')}"[:60],
           f"nhận {st} — xoá vai trò làm nhân viên mất sạch quyền")

    if user_id:
        st, _ = call("DELETE", "/api/systemManage/deleteUser", {"id": user_id}, t)
        report.add(area, "xoá tài khoản nhân viên", OK if st == 200 else BROKEN, "" if st == 200 else str(st))
    # Tài khoản là xoá mềm nên dòng user_roles vẫn còn: nếu phép đếm không loại
    # tài khoản đã xoá, vai trò này sẽ vĩnh viễn không xoá được.
    st, res = call("DELETE", "/api/systemManage/deleteRole", {"id": role_id}, t)
    expect(area, "xoá vai trò được sau khi tài khoản đã xoá mềm", st == 200,
           detail_bad=f"{st} {res.get('message','')} — phép đếm không loại tài khoản đã xoá")


# --------------------------------------------------------------------------- #
# Ma trận phân quyền
# --------------------------------------------------------------------------- #

def jwt_like_token(admin_token: str) -> str:
    """Tạo tài khoản, lấy token, rồi xoá HẲN tài khoản — token còn hợp lệ về chữ
    ký nhưng trỏ tới một người không còn tồn tại."""
    call("POST", "/api/systemManage/addUser",
         {"userName": "kt_ma", "password": "test1234", "nickName": "Tai khoan ma"}, admin_token)
    _, res = call("POST", "/api/auth/login", {"username": "kt_ma", "password": "test1234"})
    tok = (data_of(res) or {}).get("access_token", "")
    sql("delete from user_roles where user_id in (select id from users where username = 'kt_ma')")
    sql("delete from users where username = 'kt_ma'")
    return tok


def phase_permissions(tokens: dict, ids: dict) -> None:
    area = "10. Phân quyền"

    st, res = call("POST", "/api/auth/member-login",
                   {"username": "kt_hoivien", "password": "test1234", "machine_code": "KT-01"})
    d = data_of(res) or {}
    mt, mrt = d.get("access_token"), d.get("refresh_token")
    if not mt:
        report.add(area, "đăng nhập hội viên", BROKEN, f"{st} {res.get('message','')}")
        return
    report.add(area, "đăng nhập hội viên", OK)
    # Đăng nhập máy trạm mở phiên; đóng lại ngay để không kẹt máy cho giai đoạn sau.
    if d.get("session_id"):
        call("POST", f"/api/sessions/{d['session_id']}/end", token=tokens["admin"])

    staff_only = [
        ("GET", "/api/members"), ("GET", "/api/machines"), ("GET", "/api/orders"),
        ("GET", "/api/sessions/active"), ("GET", "/api/shifts"), ("GET", "/api/suppliers"),
        ("GET", "/api/promotions"), ("GET", "/api/bookings"), ("GET", "/api/combos"),
        ("GET", "/api/curfew"), ("GET", "/api/printers"), ("GET", "/api/units"),
        ("GET", "/api/backups"), ("GET", "/api/audit-logs"), ("GET", "/api/transactions"),
        ("GET", "/api/reports/daily-revenue"), ("GET", "/api/systemManage/getUserList"),
        ("GET", "/api/admin/notifications"),
    ]
    blocked = sum(1 for m, p in staff_only if call(m, p, token=mt)[0] == 403)
    expect(area, f"hội viên bị chặn khỏi {len(staff_only)} nhóm quản trị",
           blocked == len(staff_only), f"{blocked}/{len(staff_only)}",
           f"chỉ chặn {blocked}/{len(staff_only)} — còn lọt")

    st, _ = call("GET", f"/api/members/{ids['member']}", token=mt)
    expect(area, "hội viên đọc được hồ sơ của chính mình", st == 200, detail_bad=f"{st}")

    # /auth/me chỉ tra bảng users; hội viên nằm ở bảng members nên gọi bằng token
    # hội viên trước nay luôn hỏng — đúng cái bẫy đã làm hỏng đổi PIN từ máy trạm.
    st, res = call("GET", "/api/auth/me", token=mt)
    me = data_of(res) or {}
    expect(area, "/auth/me chạy được với token hội viên",
           st == 200 and me.get("username") == "kt_hoivien",
           me.get("username"), f"HTTP {st} — {res.get('message', '')[:60]}")

    # Token ký đúng nhưng trỏ tới tài khoản không còn tồn tại (xoá tài khoản, hoặc
    # phục hồi database từ bản sao lưu khác) phải là 401 mã 8888 để admin đá về
    # trang đăng nhập. Trả 404 thì axios không nhận ra và giao diện treo ở màn
    # hình chờ khởi động.
    ghost = jwt_like_token(tokens["admin"])
    if ghost:
        st, res = call("GET", "/api/auth/me", token=ghost)
        expect(area, "token trỏ tới tài khoản đã xoá bị buộc đăng nhập lại",
               st == 401 and res.get("code") == 8888,
               f"{st} / code {res.get('code')}",
               f"{st} / code {res.get('code')} — admin sẽ treo ở màn hình chờ")

    other = ids.get("minor") or ids["member"]
    if other != ids["member"]:
        st, _ = call("GET", f"/api/members/{other}", token=mt)
        expect(area, "hội viên không đọc được hồ sơ người khác", st == 403, detail_bad=f"{st}")

    st, res = call("GET", "/api/members", token=mrt)
    expect(area, "refresh token không dùng thay access token được",
           st == 401, detail_bad=f"{st} — refresh token đi qua được")

    st, _ = call("GET", "/api/chat/rooms/00000000-0000-0000-0000-000000000000/messages", token=mt)
    expect(area, "hội viên không đọc được phòng chat của người khác",
           st in (403, 404), detail_bad=f"{st}")

    if "manager" in tokens:
        for path in ("/api/backups", "/api/audit-logs", "/api/systemManage/getUserList"):
            st, _ = call("GET", path, token=tokens["manager"])
            expect(area, f"manager bị chặn khỏi {path}", st == 403, detail_bad=f"{st}")

    st, _ = call("GET", "/api/members?sort=id;DROP+TABLE+users--", token=tokens["admin"])
    still = sql("select count(*) from users")
    expect(area, "chèn SQL qua tham số sắp xếp bị vô hiệu",
           st == 200 and still not in ("", "0"), f"bảng users còn {still} dòng")

    # Khoá máy trạm đã bỏ: route by-code mở, nhận diện chỉ bằng mã máy.
    body = {"cpu_temp": 50, "gpu_temp": 55}

    st, _ = call("POST", "/api/machines/by-code/KT-02/heartbeat", body)
    expect(area, "báo cáo bằng mã máy có thật được nhận", st == 200,
           f"{st}", f"HTTP {st}")

    # Mã máy vẫn phải có thật: nó là thứ duy nhất còn lại để chặn rác.
    st, _ = call("POST", "/api/machines/by-code/KHONG-CO-MAY-NAY/heartbeat", body)
    expect(area, "mã máy không tồn tại bị từ chối", st in (401, 404),
           f"{st}", f"HTTP {st} — ghi được cho máy không tồn tại")

    # Route cấp khoá đã xoá.
    st, _ = call("POST", f"/api/machines/{ids['machine1']}/agent-token", {}, tokens["admin"])
    expect(area, "route cấp khoá đã bị xoá", st == 404,
           f"{st}", f"HTTP {st} — route agent-token lẽ ra không còn")

# --------------------------------------------------------------------------- #
# Luồng: cài đặt có thực sự điều khiển hệ thống không
# --------------------------------------------------------------------------- #

def flow_settings_enforced(t: str, ids: dict) -> None:
    """Ba nhóm cài đặt (invoice, limits, printer) lưu được nhưng không service
    nào đọc: người vận hành điền form, thấy "lưu thành công", và không có gì
    thay đổi. Ở đây đặt cấu hình rồi kiểm chính hành vi mà nó phải điều khiển."""
    area = "13b. Cài đặt có hiệu lực"

    # --- nhóm invoice: chữ trên tờ hoá đơn ---------------------------------
    # Bản xem trước chạy được cả khi chưa khai máy in nào (rơi về khổ 58mm), nên
    # đây là cách đọc đúng nội dung sẽ ra giấy mà không cần máy in thật.
    _, res = call("POST", "/api/orders",
                  {"items": [{"product_id": ids["product"], "quantity": 1}]}, t)
    oid = (data_of(res) or {}).get("id", "")
    if not oid:
        report.add(area, "tạo đơn để xem trước hoá đơn", BROKEN, "không tạo được đơn")
        return

    def preview():
        _, r = call("GET", f"/api/orders/{oid}/receipt-preview", token=t)
        return (data_of(r) or {}).get("text", "")

    before = preview()
    expect(area, "hoá đơn khởi điểm dùng chữ mặc định",
           "HOA DON THANH TOAN" in before and "Cam on quy khach" in before,
           detail_bad="không thấy chữ mặc định — dữ liệu nền không sạch")

    st, _ = call("PUT", "/api/settings/invoice",
                 {"invoice_title": "PHIEU THANH TOAN KT",
                  "invoice_footer": "Hen gap lai quy khach - wifi: vnet2026",
                  "tax_code": "0123456789"}, t)
    report.add(area, "lưu nhóm hoá đơn", OK if st == 200 else BROKEN, "" if st == 200 else str(st))

    after = preview()
    expect(area, "tiêu đề hoá đơn đổi theo cài đặt", "PHIEU THANH TOAN KT" in after,
           detail_bad="giấy vẫn in chuỗi cứng HOA DON THANH TOAN")
    expect(area, "chân hoá đơn đổi theo cài đặt", "Hen gap lai quy khach" in after,
           detail_bad="giấy vẫn in chuỗi cứng Cam on quy khach!")
    expect(area, "mã số thuế được in", "0123456789" in after,
           detail_bad="mã số thuế lưu rồi nhưng không lên giấy")

    # --- nhóm limits: trần nợ ------------------------------------------------
    # Giai đoạn trước có thể để lại phiên đang chạy; nếu không dọn thì cả ba phép
    # kiểm dưới đây đo nhầm "máy đang bận" thay vì luật trần nợ, và hai trong ba
    # phép sẽ ĐẠT vì lý do sai.
    sql("update machine_sessions set is_active = false, ended_at = now() where is_active = true")
    sql("update machines set status = 'available' where deleted_at is null")
    sql(f"update members set balance = -20000, bonus_balance = 0 where id = '{ids['member']}'")
    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    expect(area, "nợ 20.000 bị chặn khi chưa đặt trần", st >= 400,
           f"{res.get('message', '')[:60]}", "mở được máy dù đang nợ")

    call("PUT", "/api/settings/limits", {"max_debt": 50000}, t)
    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    sid = (data_of(res) or {}).get("id", "")
    expect(area, "trần nợ 50.000 cho phép khách nợ 20.000 mở máy", st in (200, 201),
           detail_bad=f"HTTP {st} — {res.get('message', '')[:70]}")
    if sid:
        call("POST", f"/api/sessions/{sid}/end", token=t)
    sql("update machines set status = 'available' where deleted_at is null")

    sql(f"update members set balance = -80000 where id = '{ids['member']}'")
    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    expect(area, "nợ vượt trần vẫn bị chặn", st >= 400,
           f"{res.get('message', '')[:60]}", "trần nợ không có tác dụng chặn")
    call("PUT", "/api/settings/limits", {"max_debt": 0}, t)
    sql(f"update members set balance = 500000 where id = '{ids['member']}'")

    # --- nhóm limits: trần đặt chỗ ------------------------------------------
    call("PUT", "/api/settings/limits", {"max_bookings_per_day": 1, "cancel_before_minutes": 0}, t)
    day = (datetime.now(VN_TZ) + timedelta(days=5)).replace(hour=9, minute=0, second=0, microsecond=0)

    def book(machine, hour_offset, name="Khach tran"):
        frm = day + timedelta(hours=hour_offset)
        return call("POST", "/api/bookings",
                    {"machine_id": machine, "customer_name": name,
                     "customer_phone": "0900000009",
                     "booked_from": frm.isoformat(),
                     "booked_to": (frm + timedelta(hours=1)).isoformat()}, t)

    st1, r1 = book(ids["machine1"], 0)
    b1 = (data_of(r1) or {}).get("id", "")
    expect(area, "đặt chỗ đầu tiên trong ngày được nhận", st1 in (200, 201),
           detail_bad=f"HTTP {st1} — {r1.get('message', '')[:60]}")
    st2, r2 = book(ids["machine2"], 3)
    expect(area, "vượt trần đặt chỗ mỗi ngày bị từ chối", st2 >= 400,
           f"{r2.get('message', '')[:60]}", f"HTTP {st2} — trần mỗi ngày không có tác dụng")

    # --- nhóm limits: cửa sổ huỷ --------------------------------------------
    call("PUT", "/api/settings/limits", {"max_bookings_per_day": 0, "cancel_before_minutes": 60}, t)
    if b1:
        # Kéo giờ giữ máy về sát hiện tại để rơi vào trong cửa sổ cấm huỷ.
        sql(f"update machine_bookings set booked_from = now() + interval '10 minutes', "
            f"booked_to = now() + interval '70 minutes' where id = '{b1}'")
        st, res = call("POST", f"/api/bookings/{b1}/cancel", token=t)
        expect(area, "huỷ sát giờ bị từ chối", st >= 400,
               f"{res.get('message', '')[:60]}", f"HTTP {st} — cửa sổ huỷ không có tác dụng")

        sql(f"update machine_bookings set booked_from = now() + interval '5 hours', "
            f"booked_to = now() + interval '6 hours' where id = '{b1}'")
        st, _ = call("POST", f"/api/bookings/{b1}/cancel", token=t)
        expect(area, "huỷ sớm vẫn được", st == 200, detail_bad=f"HTTP {st}")
        call("DELETE", f"/api/bookings/{b1}", token=t)
    call("PUT", "/api/settings/limits", {"cancel_before_minutes": 0}, t)

    # --- nhóm topup: mệnh giá máy khách đọc ---------------------------------
    st, res = call("GET", "/api/settings/topup", token=t)
    rows = data_of(res) if isinstance(data_of(res), list) else items_of(res)
    presets = next((r for r in rows if r.get("key") == "presets"), None)
    expect(area, "mệnh giá nạp nằm ở nhóm topup, khoá presets", presets is not None,
           detail_bad="máy khách tìm nhóm topup/khoá presets nhưng không có")
    if presets:
        expect(area, "mệnh giá nạp lưu dạng {values: [...]}",
               "values" in str(presets.get("value", "")),
               str(presets.get("value"))[:50],
               f"dạng lạ: {str(presets.get('value'))[:50]}")

    st, _ = call("PUT", "/api/settings/topup", {"presets": {"values": [7000, 21000]}}, t)
    _, res = call("GET", "/api/settings/topup", token=t)
    rows = data_of(res) if isinstance(data_of(res), list) else items_of(res)
    saved = next((r for r in rows if r.get("key") == "presets"), {})
    expect(area, "sửa mệnh giá nạp ghi được", "7000" in str(saved.get("value", "")),
           detail_bad=f"đọc lại nhận {saved.get('value')!r}")
    call("PUT", "/api/settings/topup",
         {"presets": {"values": [5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000]}}, t)

    # Trả nhóm invoice về mặc định để giai đoạn in hoá đơn phía sau không đọc
    # phải chữ của giai đoạn này.
    call("PUT", "/api/settings/invoice",
         {"invoice_title": "", "invoice_footer": "", "tax_code": ""}, t)
    call("DELETE", f"/api/orders/{oid}", token=t)


# --------------------------------------------------------------------------- #
# Các nhóm còn lại + endpoint nguy hiểm
# --------------------------------------------------------------------------- #

def phase_remaining(t: str, ids: dict) -> None:
    area = "11. Nhóm còn lại"
    for name, path in [
        ("khuyến mãi", "/api/promotions"), ("phần thưởng quay số", "/api/lucky-spin/rewards"),
        ("nhật ký kiểm toán", "/api/audit-logs"), ("sổ giao dịch", "/api/transactions"),
        ("thông báo quản trị", "/api/admin/notifications"), ("bản sao lưu", "/api/backups"),
        ("phòng chat", "/api/chat/rooms"), ("cài đặt (tất cả)", "/api/settings"),
        ("người dùng hệ thống", "/api/systemManage/getUserList"),
        ("vai trò", "/api/systemManage/getRoleList"),
        ("cây menu", "/api/systemManage/getMenuTree"),
        ("tài sản máy", "/api/machine-assets"),
        ("lịch sử phần cứng", f"/api/machines/{ids['machine1']}/hardware"),
    ]:
        probe(area, name, "GET", path, token=t)

    area = "12. Báo cáo"
    for name, path in [
        ("doanh thu ngày", "/api/reports/daily-revenue"),
        ("doanh thu tháng", "/api/reports/monthly-revenue"),
        ("theo hội viên", "/api/reports/by-member"),
        ("theo máy", "/api/reports/by-machine"),
        ("theo nhân viên", "/api/reports/by-employee"),
        ("sản phẩm bán chạy", "/api/reports/top-products"),
        ("hiệu quả khuyến mãi", "/api/reports/promotion-usage"),
    ]:
        probe(area, name, "GET", path, token=t)

    # Endpoint trả 200 không có nghĩa là bảng hiện được gì. Báo cáo "theo máy"
    # từng trả machine_name/total_sales/usage_hours trong khi giao diện đọc
    # machine_code/revenue/total_hours/session_count — bốn trên năm cột trống,
    # cột còn lại luôn là "0 ₫", mà endpoint vẫn 200.
    st, res = call("GET", "/api/reports/by-machine", token=t)
    rows = data_of(res) if isinstance(data_of(res), list) else items_of(res)
    row = rows[0] if rows else {}
    for key in ("machine_code", "total_sales", "usage_hours", "session_count"):
        expect(area, f"báo cáo theo máy trả trường {key}", key in row,
               detail_bad=f"thiếu {key} — cột tương ứng trên giao diện sẽ trống")
    expect(area, "báo cáo theo máy có mã máy, kể cả máy đã xoá mềm",
           bool(row.get("machine_code")),
           row.get("machine_code"),
           "mã máy rỗng — doanh thu thành dòng không tên")

    # Nhật ký hoạt động tồn tại để trả lời "AI đã làm việc này". Ghi hành động mà
    # bỏ trống người thực hiện là ghi một nửa.
    st, res = call("GET", "/api/audit-logs?page_size=50", token=t)
    logs = items_of(res)
    named = [r for r in logs if r.get("user_id")]
    expect(area, "nhật ký có ghi người thực hiện", len(named) > 0,
           f"{len(named)}/{len(logs)} dòng có user_id",
           f"0/{len(logs)} dòng ghi lại ai làm — nhật ký không truy được trách nhiệm")

    area = "13. Cài đặt"
    st, _ = call("PUT", "/api/settings/general",
                 {"store_name": "Tiem net kiem thu", "store_address": "123 Le Loi"}, t)
    expect(area, "lưu cài đặt dạng chữ", st in (200, 201), detail_bad=f"{st} — cột jsonb từ chối")
    st, res = call("GET", "/api/settings/general", token=t)
    rows = {r.get("key"): r.get("value") for r in (data_of(res) or [])}
    expect(area, "đọc lại đúng nội dung đã lưu",
           rows.get("store_name") == "Tiem net kiem thu",
           detail_bad=f"nhận {rows.get('store_name')!r}")

    area = "14. Thông báo"
    st, res = probe(area, "tạo thông báo", "POST", "/api/admin/notifications",
                    {"type": "promotion", "title": "KT Thong bao", "content": "Noi dung"}, t)
    nid = (data_of(res) or {}).get("id", "")
    if nid:
        probe(area, "gửi tới hội viên", "POST", f"/api/admin/notifications/{nid}/dispatch", token=t)
        expect(area, "thông báo vào được hộp thư hội viên",
               sql("select count(*) from member_notifications") != "0",
               detail_bad="hộp thư hội viên rỗng — ghi vào bảng không ai đọc")

    area = "15. Endpoint nguy hiểm"
    st, res = call("POST", f"/api/printers/{ids['printer']}/test", token=t, timeout=15)
    report.add(area, "in thử (IP dải TEST-NET)", OK if st >= 400 else WRONG,
               f"{st} — timeout như dự kiến" if st >= 400 else "trả 200 dù không có máy in")

    st, res = call("POST", f"/api/machines/{ids['machine1']}/remote/shutdown", {}, t)
    report.add(area, "lệnh từ xa khi không có máy kết nối",
               OK if st >= 400 else WRONG,
               f"{st} — từ chối đúng" if st >= 400 else "báo thành công dù không máy nào nhận")

    st, res = call("POST", "/api/backups", {"notes": "kiem thu"}, t)
    if st in (200, 201):
        time.sleep(3)
        row = sql("select status || ' / ' || file_size from backup_logs order by started_at desc limit 1")
        report.add(area, "tạo bản sao lưu", WRONG if "failed" in row else OK,
                   f"trạng thái: {row}")
    else:
        report.add(area, "tạo bản sao lưu", BROKEN, f"{st}")

    # Phục hồi được kiểm ở giai đoạn RIÊNG (phase_restore), chạy sau cùng:
    # pg_restore --clean xoá và dựng lại toàn bộ bảng, nên không thể chạy giữa
    # chừng — mọi giai đoạn sau nó sẽ đọc phải database vừa bị ghi đè.


# --------------------------------------------------------------------------- #
# Tác vụ định kỳ
# --------------------------------------------------------------------------- #

def phase_scheduler(t: str, ids: dict) -> None:
    area = "16. Tác vụ định kỳ"

    sql(f"update machines set status = 'available', last_heartbeat = now() - interval '10 minutes' "
        f"where id = '{ids['machine1']}'")
    frm = datetime.now(VN_TZ) - timedelta(hours=2)
    sql("insert into machine_bookings (machine_id, customer_name, customer_phone, booked_from, "
        f"booked_to, status) values ('{ids['machine1']}', 'KT qua gio', '0900000002', "
        f"'{frm.isoformat()}', '{(frm + timedelta(hours=1)).isoformat()}', 'pending')")

    # --- sessions:enforce-limits ---------------------------------------------
    # Tác vụ này trước đây KHÔNG có phép kiểm nào. Hai lý do tự đóng phiên:
    # hết phút gói trả trước, và hết khung giờ cố định. Dựng cả hai.
    sql("update machine_sessions set is_active = false, ended_at = now() where is_active = true")
    sql("update machines set status = 'available' where deleted_at is null")

    limit_ids = {}
    for key, machine, setup in (
        ("minutes", ids["machine1"],
         "remaining_minutes = 30, started_at = now() - interval '45 minutes'"),
        ("slot", ids["machine2"],
         "slot_end = now() - interval '5 minutes'"),
    ):
        sql(f"update members set balance = 500000, bonus_balance = 0 where id = '{ids['member']}'")
        sql(f"update machine_sessions set is_active = false, ended_at = now() "
            f"where member_id = '{ids['member']}' and is_active = true")
        st, res = call("POST", "/api/sessions/start",
                       {"machine_id": machine, "member_id": ids["member"]}, t)
        sid = (data_of(res) or {}).get("id", "")
        if not sid:
            report.add(area, f"mở phiên để kiểm tự đóng ({key})", BROKEN,
                       f"{st} {res.get('message', '')[:60]}")
            continue
        limit_ids[key] = sid
        sql(f"update machine_sessions set {setup} where id = '{sid}'")

    # --- curfew:enforce — cưỡng chế TRẢ máy, không chỉ chặn lúc mở ------------
    # Phép kiểm cũ chỉ đo lúc mở máy; nhánh đuổi khách đang ngồi chưa từng chạy.
    now = datetime.now(VN_TZ)
    go_dow = (now.weekday() + 1) % 7
    sql("delete from curfew_policies")
    minor = ids.get("minor", "")
    curfew_sid = ""
    if minor:
        sql(f"update members set balance = 200000, bonus_balance = 0 where id = '{minor}'")
        sql(f"update machine_sessions set is_active = false, ended_at = now() "
            f"where member_id = '{minor}' and is_active = true")
        sql(f"update machines set status = 'available' where id = '{ids['machine3']}'")
        st, res = call("POST", "/api/sessions/start",
                       {"machine_id": ids["machine3"], "member_id": minor}, t)
        curfew_sid = (data_of(res) or {}).get("id", "")
        if not curfew_sid:
            report.add(area, "mở phiên vị thành niên để kiểm giới nghiêm", BROKEN,
                       f"{st} {res.get('message', '')[:60]}")
        else:
            # Bật khung cấm SAU khi đã mở phiên: mở trước thì bị chặn ngay ở cửa.
            sql("insert into curfew_policies (day_of_week, curfew_start, curfew_end, "
                f"max_minor_hours, is_active) values ({go_dow}, '00:00:01', '23:59:59', 0, true)")

    # --- members:refresh-tier — chạy tay, không chờ chu kỳ 15 phút ------------
    sql(f"update members set total_spent = 999999 where id = '{ids['member']}'")
    st, res = call("POST", "/api/members/refresh-tiers", {}, t)
    moved = (data_of(res) or {}).get("moved")
    expect(area, "xếp lại hạng chạy được ngay, không phải chờ 15 phút",
           st == 200 and isinstance(moved, int),
           f"{moved} hội viên đổi hạng", f"HTTP {st} — {res.get('message', '')[:60]}")
    group_min = sql(f"select g.min_spent from members m join member_groups g on g.id = m.group_id "
                    f"where m.id = '{ids['member']}'")
    expect(area, "hội viên chi 999.999₫ được xếp vào hạng cao nhất đủ điều kiện",
           group_min.isdigit() and int(group_min) > 0,
           f"ngưỡng hạng {group_min}₫",
           f"vẫn ở hạng ngưỡng {group_min!r} — so tổng chi tiêu không có tác dụng")

    print("      … chờ 70 giây để các tác vụ định kỳ chạy", flush=True)
    time.sleep(70)

    expect(area, "máy mất tín hiệu bị chuyển sang ngoại tuyến",
           sql(f"select status from machines where id = '{ids['machine1']}'") == "offline",
           detail_bad="máy vẫn hiển thị sẵn sàng")
    expect(area, "đặt chỗ quá giờ bị đánh dấu không đến",
           sql("select count(*) from machine_bookings where status = 'no_show'") != "0",
           detail_bad="đặt chỗ quá hạn vẫn giữ chỗ")

    if "minutes" in limit_ids:
        expect(area, "hết phút gói trả trước thì tự trả máy",
               sql(f"select is_active from machine_sessions where id = '{limit_ids['minutes']}'") == "f",
               detail_bad="phiên vẫn chạy dù đã hết phút — khách chơi miễn phí")
    if "slot" in limit_ids:
        expect(area, "hết khung giờ cố định thì tự trả máy",
               sql(f"select is_active from machine_sessions where id = '{limit_ids['slot']}'") == "f",
               detail_bad="phiên vẫn chạy dù khung giờ đã qua")

    if curfew_sid:
        expect(area, "tới giờ giới nghiêm thì cưỡng chế trả máy vị thành niên",
               sql(f"select is_active from machine_sessions where id = '{curfew_sid}'") == "f",
               detail_bad="vị thành niên vẫn ngồi máy trong giờ cấm")
        expect(area, "việc cưỡng chế được ghi vào nhật ký",
               sql("select count(*) from audit_logs where action = 'curfew_enforced'") != "0",
               detail_bad="không có dấu vết nào của lần cưỡng chế")
    sql("delete from curfew_policies")


# --------------------------------------------------------------------------- #
# Luồng: cột chết — cột nào nhận giá trị thì phải có nơi đọc
# --------------------------------------------------------------------------- #

def flow_dead_columns(t: str, ids: dict) -> None:
    """Mỗi cột dưới đây từng khai trong model rồi hoặc không ai ghi, hoặc ghi
    rồi không ai đọc. Ở đây kiểm chính hành vi mà cột đó phải điều khiển."""
    area = "18. Cột chết"

    # --- max_minor_hours: trần giờ chơi của vị thành niên ---------------------
    now = datetime.now(VN_TZ)
    go_dow = (now.weekday() + 1) % 7
    sql("delete from curfew_policies")
    # Khung giờ cấm đặt vào quá khứ xa để KHÔNG phủ lúc này: phép kiểm phải đo
    # trần giờ, không phải đo khung giờ cấm.
    st, res = call("POST", "/api/curfew",
                   {"day_of_week": go_dow, "curfew_start": "03:00:00", "curfew_end": "03:01:00",
                    "max_minor_hours": 2, "is_active": True}, t)
    report.add(area, "đặt trần 2 giờ/ngày cho vị thành niên",
               OK if st in (200, 201) else BROKEN, "" if st in (200, 201) else str(st))
    sql(f"update curfew_policies set day_of_week = {go_dow} where is_active = true")

    minor = ids.get("minor")
    if not minor:
        st, res = call("POST", "/api/members",
                       {"username": "kt_vtn_gio", "password": "test1234", "full_name": "VTN gio",
                        "date_of_birth": (now - timedelta(days=365 * 15)).date().isoformat() + "T00:00:00+07:00"}, t)
        minor = (data_of(res) or {}).get("id", "")
    if not minor:
        report.add(area, "tạo hội viên vị thành niên", BROKEN, "không tạo được")
        return
    sql(f"update members set balance = 300000, bonus_balance = 0 where id = '{minor}'")
    sql(f"delete from machine_sessions where member_id = '{minor}'")
    sql("update machines set status = 'available' where deleted_at is null")

    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": minor}, t)
    sid = (data_of(res) or {}).get("id", "")
    expect(area, "chưa chơi giờ nào thì vẫn mở máy được", st in (200, 201),
           detail_bad=f"{st} — {res.get('message', '')[:70]}")
    if sid:
        call("POST", f"/api/sessions/{sid}/end", token=t)
        # Trần giờ tính theo NGÀY và reset lúc nửa đêm (MinutesPlayedToday lọc
        # started_at >= 00:00 hôm nay). Bản cũ lùi started_at 3 tiếng, nên chạy
        # bộ kiểm trong khoảng 00:00–03:00 thì phiên rơi sang hôm qua và phép
        # kiểm tự báo SAI — một mục lúc đạt lúc không tuỳ giờ bấm nút thì vô
        # dụng. Ghi thẳng phiên ĐÃ KẾT THÚC vào đầu ngày hôm nay: giờ nào chạy
        # cũng cho cùng kết quả.
        sql("update machine_sessions set is_active = false, "
            "started_at = date_trunc('day', now()) + interval '10 minutes', "
            "ended_at = date_trunc('day', now()) + interval '3 hours 10 minutes', "
            f"duration_minutes = 180 where id = '{sid}'")
    sql("update machines set status = 'available' where deleted_at is null")

    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": minor}, t)
    expect(area, "chơi quá trần giờ thì bị chặn mở máy mới", st >= 400,
           f"{res.get('message', '')[:70]}",
           f"{st} — max_minor_hours vẫn chỉ là con số trong database")

    # Người lớn không bị trần giờ này chạm tới.
    sql("update machines set status = 'available' where deleted_at is null")
    sql(f"update machine_sessions set is_active = false, ended_at = now() where member_id = '{ids['member']}' and is_active = true")
    st, res = call("POST", "/api/sessions/start",
                   {"machine_id": ids["machine1"], "member_id": ids["member"]}, t)
    adult_sid = (data_of(res) or {}).get("id", "")
    expect(area, "người lớn không bị trần giờ vị thành niên", st in (200, 201),
           detail_bad=f"{st} — {res.get('message', '')[:70]}")

    # --- total_played_minutes & last_visit_at --------------------------------
    if adult_sid:
        sql(f"update machine_sessions set started_at = started_at - interval '90 minutes' where id = '{adult_sid}'")
        before = sql(f"select total_played_minutes from members where id = '{ids['member']}'")
        call("POST", f"/api/sessions/{adult_sid}/end", token=t)
        after = sql(f"select total_played_minutes from members where id = '{ids['member']}'")
        expect(area, "trả máy có cộng số phút đã chơi",
               before.isdigit() and after.isdigit() and int(after) - int(before) >= 89,
               f"{before} → {after} phút",
               f"{before} → {after} — cột total_played_minutes không ai cộng")

    visit = sql(f"select last_visit_at is not null from members where id = '{ids['member']}'")
    expect(area, "mở máy có ghi lần ghé gần nhất", visit == "t",
           detail_bad="last_visit_at rỗng — không biết khách nào lâu rồi không tới")
    sql("update machines set status = 'available' where deleted_at is null")
    sql("delete from curfew_policies")

    # --- giấy tờ hội viên -----------------------------------------------------
    st, res = call("PUT", f"/api/members/{ids['member']}",
                   {"id_card_image_url": "/uploads/kt/cccd.jpg",
                    "parent_consent_file_url": "/uploads/kt/dongy.jpg"}, t)
    d = data_of(res) or {}
    expect(area, "DTO hội viên nhận được hai đường dẫn giấy tờ",
           d.get("id_card_image_url") == "/uploads/kt/cccd.jpg"
           and d.get("parent_consent_file_url") == "/uploads/kt/dongy.jpg",
           detail_bad=f"nhận {d.get('id_card_image_url')!r} / {d.get('parent_consent_file_url')!r}")

    # --- last_login_at lộ ra API ---------------------------------------------
    st, res = call("GET", "/api/systemManage/getUserList?current=1&size=20", token=t)
    rows = items_of(res)
    admin_row = next((r for r in rows if r.get("userName") == "admin"), {})
    expect(area, "danh sách người dùng trả về lần đăng nhập gần nhất",
           bool(admin_row.get("lastLoginTime")),
           admin_row.get("lastLoginTime"),
           "lastLoginTime rỗng — cột ghi rồi mà không API nào đọc")

    # --- OrderItem.Status: luồng bếp ------------------------------------------
    # Mã đơn sinh theo mã lớn nhất đang thấy; bỏ qua dòng đã xoá mềm sẽ cấp lại
    # một mã đã tồn tại và va chỉ mục duy nhất — quầy xoá một đơn là không tạo
    # được đơn nào nữa.
    sql("update orders set deleted_at = now() where deleted_at is null "
        "and order_code = (select max(order_code) from orders where deleted_at is null)")
    st, res = call("POST", "/api/orders",
                   {"items": [{"product_id": ids["product"], "quantity": 2}]}, t)
    order = data_of(res) or {}
    oid = order.get("id", "")
    expect(area, "xoá đơn xong vẫn tạo được đơn mới", st in (200, 201) and bool(oid),
           order.get("order_code"),
           f"HTTP {st} — {res.get('message', '')[:80]}")
    item_id = (order.get("items") or [{}])[0].get("id", "")
    if not item_id and oid:
        _, res = call("GET", f"/api/orders/{oid}", token=t)
        item_id = ((data_of(res) or {}).get("items") or [{}])[0].get("id", "")
    if not item_id:
        report.add(area, "lấy được món trong đơn để đổi trạng thái", BROKEN, "đơn không có món")

    if item_id:
        st, res = call("POST", f"/api/orders/{oid}/items/{item_id}/status", {"status": "khong-co-that"}, t)
        expect(area, "trạng thái món không hợp lệ bị từ chối", st == 400,
               f"{res.get('message', '')[:60]}", f"nhận {st}")

        for status in ("preparing", "ready", "served"):
            st, _ = call("POST", f"/api/orders/{oid}/items/{item_id}/status", {"status": status}, t)
            got = sql(f"select status from order_items where id = '{item_id}'")
            expect(area, f"đổi trạng thái món sang {status}", st == 200 and got == status,
                   got, f"HTTP {st}, database là {got!r}")

        call("POST", f"/api/orders/{oid}/status", {"status": "cancelled"}, t)
        st, res = call("POST", f"/api/orders/{oid}/items/{item_id}/status", {"status": "preparing"}, t)
        expect(area, "đơn đã huỷ thì không đổi trạng thái món", st == 400,
               f"{res.get('message', '')[:60]}", f"nhận {st} — sinh phiếu bếp cho đơn khách đã bỏ")

    # --- TopupCard.SoldTo / SoldAt --------------------------------------------
    st, res = call("POST", "/api/topup-cards/generate", {"count": 1, "face_value": 50000}, t)
    card = ((data_of(res) or {}).get("cards") or [{}])[0]
    cid = card.get("id", "")
    if cid:
        st, res = call("POST", f"/api/topup-cards/{cid}/sell", {"member_id": ids["member"]}, t)
        expect(area, "bán thẻ ở quầy ghi được người mua", st == 200,
               detail_bad=f"{st} — {res.get('message', '')[:70]}")
        sold = sql(f"select sold_to is not null and sold_at is not null from topup_cards where id = '{cid}'")
        expect(area, "sold_to/sold_at được ghi vào database", sold == "t",
               detail_bad="hai cột vẫn rỗng sau khi bán")

        st, res = call("POST", f"/api/topup-cards/{cid}/sell", {"member_id": ids["member"]}, t)
        expect(area, "thẻ đã bán không bán lại được", st == 400,
               f"{res.get('message', '')[:60]}", f"nhận {st}")

    # --- BlockApp / UnblockApp -------------------------------------------------
    for action in ("block-app", "unblock-app"):
        st, res = call("POST", f"/api/machines/{ids['machine1']}/remote/{action}",
                       {"process": "chrome"}, t)
        # 409 = máy chưa kết nối, nghĩa là LỆNH ĐƯỢC CHẤP NHẬN; 400 mới là lệnh lạ.
        expect(area, f"backend chấp nhận lệnh {action}", st == 409,
               f"409 — {res.get('message', '')[:50]}",
               f"nhận {st} — {res.get('message', '')[:70]}")


# --------------------------------------------------------------------------- #
# Dọn dẹp
# --------------------------------------------------------------------------- #

def phase_cleanup(t: str, ids: dict) -> None:
    area = "17. Dọn dẹp"
    for label, path in [
        ("tài sản máy", f"/api/machine-assets/{ids.get('asset','')}"),
        ("máy 1", f"/api/machines/{ids.get('machine1','')}"),
        ("máy 2", f"/api/machines/{ids.get('machine2','')}"),
        ("máy 3", f"/api/machines/{ids.get('machine3','')}"),
        ("sản phẩm", f"/api/products/{ids.get('product','')}"),
        ("nhà cung cấp", f"/api/suppliers/{ids.get('supplier','')}"),
        ("máy in", f"/api/printers/{ids.get('printer','')}"),
    ]:
        if path.endswith("/"):
            continue
        probe(area, f"xoá {label}", "DELETE", path, token=t)

    # Danh mục còn sản phẩm phải bị TỪ CHỐI: cho xoá thì sản phẩm mồ côi, và
    # trang Sản phẩm tra tên danh mục không thấy nên hiện UUID thô ra màn hình.
    cat = ids.get("category", "")
    if cat:
        left = sql(f"select count(*) from products where category_id = '{cat}' and deleted_at is null")
        if left != "0":
            st, res = call("DELETE", f"/api/categories/{cat}", token=t)
            expect(area, "danh mục còn sản phẩm thì không xoá được", st >= 400,
                   f"{res.get('message', '')[:60]}",
                   f"nhận {st} dù còn {left} sản phẩm")
            sql(f"update products set deleted_at = now() where category_id = '{cat}' and deleted_at is null")
        probe(area, "xoá danh mục", "DELETE", f"/api/categories/{cat}", token=t)

    expect(area, "xoá là xoá mềm, dữ liệu còn phục hồi được",
           sql(f"select count(*) from machines where id = '{ids.get('machine1','')}' "
               "and deleted_at is not null") == "1",
           detail_bad="bản ghi bị xoá vật lý — không phục hồi được")


# --------------------------------------------------------------------------- #
# Phục hồi sao lưu — CHẠY SAU CÙNG
# --------------------------------------------------------------------------- #

def flow_ui_contract(t: str, ids: dict) -> None:
    """Giao diện đọc phản hồi theo TÊN KHOÁ. Một khoá đổi tên hoặc không tồn tại
    thì cột im lặng trống — không lỗi, không cảnh báo, chỉ là một ô rỗng mà
    người vận hành tưởng là "chưa có dữ liệu". Ở đây chốt lại đúng những khoá
    mà bảng trên màn hình đang trỏ vào.
    """
    area = "20. Hợp đồng khoá với giao diện"

    # --- báo cáo doanh thu tháng --------------------------------------------
    st, res = call("GET", "/api/reports/monthly-revenue", token=t)
    rows = data_of(res) or []
    row = rows[0] if isinstance(rows, list) and rows else {}
    expect(area, "doanh thu tháng có dòng để kiểm", bool(row),
           detail_bad=f"HTTP {st}, {len(rows) if isinstance(rows, list) else '?'} dòng")
    if row:
        for key in ("month", "revenue", "total_orders"):
            expect(area, f"doanh thu tháng trả khoá {key}", key in row,
                   detail_bad=f"chỉ có {sorted(row)}")

    st, res = call("GET", "/api/reports/daily-revenue", token=t)
    rows = data_of(res) or []
    row = rows[0] if isinstance(rows, list) and rows else {}
    if row:
        for key in ("date", "revenue", "total_orders"):
            expect(area, f"doanh thu ngày trả khoá {key}", key in row,
                   detail_bad=f"chỉ có {sorted(row)}")

    # --- nhật ký sao lưu: cột "Ngày tạo" đọc started_at, không phải created_at
    st, res = call("GET", "/api/backups", token=t)
    rows = (data_of(res) or {}).get("items", [])
    row = rows[0] if rows else {}
    expect(area, "nhật ký sao lưu có dòng để kiểm", bool(row), detail_bad=f"HTTP {st}")
    if row:
        expect(area, "nhật ký sao lưu trả started_at", bool(row.get("started_at")),
               detail_bad=f"chỉ có {sorted(row)}")
        expect(area, "nhật ký sao lưu KHÔNG có created_at (cột cũ luôn trống)",
               "created_at" not in row,
               detail_bad="phản hồi có created_at — nếu backend đổi lại thì giao diện phải đổi theo")


def flow_agent_ws_and_features(t: str, ids: dict) -> None:
    """Hai việc nền tảng cho kiến trúc máy trạm mới.

    (1) WebSocket nhận khoá máy. Trước đây kết nối chỉ mở SAU KHI khách đăng
    nhập, bằng token của khách — máy không có ai ngồi thì không có kết nối nào,
    nên nhân viên KHÔNG tắt hay khoá được máy trống, đúng lúc cần nhất.

    (2) Một mã máy giữ được NHIỀU kết nối. Tiến trình nền và giao diện là hai
    tiến trình riêng, cùng khai một mã. Bản cũ giữ đúng một client mỗi mã nên bên
    nối sau đá bên nối trước ra.
    """
    area = "27. Kết nối bằng khoá máy"

    code = "KT-AGENT"
    sql(f"delete from machines where machine_code = '{code}'")
    st, res = call("POST", "/api/machines", {"machine_code": code}, t)
    mach_id = (data_of(res) or {}).get("id", "")
    if not mach_id:
        report.add(area, "tạo máy để kiểm", BROKEN, f"{st} {res.get('message','')[:60]}")
        return

    # --- máy mới: nối được ngay, không cần khoá ------------------------------
    try:
        moi = FakeMachine(code)
        moi.close()
        report.add(area, "máy chưa cấp khoá nối được chỉ bằng mã máy", OK)
    except Exception as e:  # noqa: BLE001
        report.add(area, "máy chưa cấp khoá nối được chỉ bằng mã máy", BROKEN, str(e)[:70])

    # --- mã máy bịa vẫn phải bị chặn ----------------------------------------
    try:
        FakeMachine("KT-KHONG-CO-THAT")
        report.add(area, "mã máy không tồn tại bị từ chối", WRONG, "", "nối được bằng mã bịa")
    except Exception as e:  # noqa: BLE001
        report.add(area, "mã máy không tồn tại bị từ chối", OK, str(e)[:50])

    # --- hai kết nối cùng một mã máy ----------------------------------------
    # Khoá máy trạm đã bỏ: nối chỉ bằng mã máy.
    try:
        service = FakeMachine(code)   # tiến trình nền
        ui = FakeMachine(code)        # giao diện
    except Exception as e:  # noqa: BLE001
        report.add(area, "nối được bằng mã máy", BROKEN, str(e)[:70])
        return
    report.add(area, "nối được bằng mã máy, không cần ai đăng nhập", OK)
    time.sleep(0.5)

    st, res = call("POST", f"/api/machines/{mach_id}/remote/shutdown", token=t)
    expect(area, "máy trống vẫn nhận được lệnh tắt", st == 200,
           detail_bad=f"HTTP {st} — {res.get('message', '')[:70]}")

    def wait(fake, want, timeout=3.0):
        deadline = time.time() + timeout
        while time.time() < deadline:
            evt = fake.recv(timeout=max(0.2, deadline - time.time()))
            if evt is None:
                return None
            if evt.get("type") == want:
                return evt
        return None

    expect(area, "tiến trình nền nhận được lệnh", wait(service, "remote:shutdown") is not None,
           detail_bad="không nhận được — kết nối bị đá ra")
    expect(area, "giao diện cũng nhận được lệnh", wait(ui, "remote:shutdown") is not None,
           detail_bad="chỉ một trong hai nhận được — hub vẫn giữ một client mỗi mã máy")

    # Giao diện tắt: tiến trình nền phải vẫn nhận. Đây là cả điểm của việc tách.
    ui.close()
    time.sleep(0.5)
    st, _ = call("POST", f"/api/machines/{mach_id}/remote/restart", token=t)
    expect(area, "giao diện tắt thì tiến trình nền vẫn nhận lệnh",
           st == 200 and wait(service, "remote:restart") is not None,
           detail_bad=f"HTTP {st} — mất kết nối khi một bên rời đi")
    service.close()

    sql(f"delete from machines where machine_code = '{code}'")

    # --- công tắc bật/tắt tính năng -----------------------------------------
    area = "28. Công tắc tính năng"
    sql("delete from system_settings where group_name = 'features'")

    mid = ids.get("member", "")
    st, res = call("POST", "/api/auth/member-login",
                   {"username": "kt_hoivien", "password": "test1234", "machine_code": "KT-01"})
    d = data_of(res) or {}
    mtok, sid = d.get("access_token", ""), d.get("session_id", "")
    if not mtok:
        report.add(area, "đăng nhập hội viên để thử", BROKEN, f"{st} {res.get('message','')[:60]}")
        return

    # Nhóm chưa tồn tại (404) → phải coi như BẬT, không phải tắt.
    st, _ = call("GET", "/api/attendance/status", token=mtok)
    expect(area, "nhóm cài đặt chưa có thì tính năng vẫn bật", st == 200,
           detail_bad=f"HTTP {st} — quán chưa mở tab Cài đặt đã mất tính năng")

    call("PUT", "/api/settings/features",
         {"attendance_enabled": "false", "feedback_enabled": "false"}, t)

    st, res = call("GET", "/api/attendance/status", token=mtok)
    expect(area, "tắt điểm danh thì bị từ chối", st >= 400,
           res.get("message", "")[:60], f"HTTP {st} — tắt rồi vẫn gọi được")
    st, res = call("POST", "/api/feedback", {"rating": 5, "content": "tot"}, mtok)
    expect(area, "tắt đánh giá thì bị từ chối", st >= 400,
           res.get("message", "")[:60], f"HTTP {st} — tắt rồi vẫn gửi được")

    call("PUT", "/api/settings/features",
         {"attendance_enabled": "true", "feedback_enabled": "true"}, t)
    st, _ = call("GET", "/api/attendance/status", token=mtok)
    expect(area, "bật lại thì dùng được", st == 200, detail_bad=f"HTTP {st}")

    # Máy trạm đọc được nhóm này bằng token hội viên — đó là cách nó ẩn nút.
    st, res = call("GET", "/api/settings/features", token=mtok)
    rows = data_of(res) or []
    keys = {r.get("key") for r in rows} if isinstance(rows, list) else set()
    expect(area, "máy trạm đọc được công tắc bằng token hội viên",
           st == 200 and "attendance_enabled" in keys,
           detail_bad=f"HTTP {st}, khoá nhận được: {sorted(keys)}")

    if sid:
        call("POST", f"/api/sessions/{sid}/end", token=t)
    sql("delete from system_settings where group_name = 'features'")
    sql("update machines set status = 'available' where deleted_at is null")


def flow_per_minute_billing(t: str, ids: dict) -> None:
    """Tiền phải được trừ mỗi phút, không dồn tới lúc trả máy.

    Trước đây tiền chỉ chuyển động đúng một lần trong EndSession, nên khách hết
    sạch tiền vẫn ngồi vô hạn — enforceSessionLimits chỉ dừng phiên khi hết khung
    giờ hoặc hết phút gói, mà phiên tính theo giờ thường không có cả hai.

    Phép tính là TÍCH LUỸ TỚI ĐÍCH: mỗi lượt hỏi "tới giờ đáng lẽ đã thu bao
    nhiêu" rồi thu phần chênh. Hai tính chất phải giữ, và đây là chỗ đo chúng.
    """
    area = "26. Trừ tiền theo phút"

    code = "KT-01"
    mach_id = sql(f"select id from machines where machine_code = '{code}'")

    sql("update machines set status = 'available' where deleted_at is null")
    sql("delete from machine_sessions where member_id in "
        "(select id from members where username = 'kt_phut')")
    sql("delete from members where username = 'kt_phut'")
    st, res = call("POST", "/api/members",
                   {"username": "kt_phut", "password": "test1234", "full_name": "KT theo phut",
                    "group_id": ids.get("member_group", "")}, t)
    mid = (data_of(res) or {}).get("id", "")
    if not mid:
        report.add(area, "tạo hội viên để thử", BROKEN, str(st))
        return
    sql(f"update members set balance = 500000, bonus_balance = 0 where id = '{mid}'")

    def start() -> str:
        sql("update machines set status = 'available' where deleted_at is null")
        st, res = call("POST", "/api/sessions/start", {"machine_id": mach_id, "member_id": mid}, t)
        return (data_of(res) or {}).get("id", "")

    def bal() -> int:
        return int(sql(f"select balance from members where id = '{mid}'") or 0)

    def charged(sid: str) -> int:
        return int(sql(f"select charged_amount from machine_sessions where id = '{sid}'") or 0)

    sid = start()
    if not sid:
        report.add(area, "mở phiên để thử", BROKEN, "không mở được máy")
        return

    # Đơn giá phải có NGAY từ lúc mở máy — trước đây chỉ ghi lúc trả máy nên
    # phiên đang chạy luôn hiện 0 và màn hình không tính được gì.
    price = int(sql(f"select price_per_hour from machine_sessions where id = '{sid}'") or 0)
    expect(area, "đơn giá được ghi ngay khi mở máy", price > 0, f"{price}₫/giờ",
           "price_per_hour = 0 — màn hình không có gì để tính thời gian còn lại")
    expect(area, "mốc hết tiền được tính ngay khi mở máy",
           sql(f"select affordable_until is not null from machine_sessions where id = '{sid}'") == "t",
           detail_bad="affordable_until rỗng — không màn hình nào đếm ngược được")

    # Nghe bằng WebSocket của chính máy đó trong lúc lượt tính chạy. Đây là thứ
    # quyết định con số khách NHÌN THẤY: máy trạm đọc số dư đúng một lần lúc đăng
    # nhập, nên không có sự kiện thì màn hình đứng yên trong khi tiền rút đi mỗi
    # phút — chơi ba tiếng ở giá 20.000₫/giờ là lệch 60.000₫.
    nghe = None
    try:
        nghe = FakeMachine(code)
    except Exception as e:
        report.add(area, "mở WebSocket để nghe lượt trừ tiền", BROKEN, str(e)[:120])

    # Lùi giờ 30 phút rồi ép chạy một lượt tính.
    before = bal()
    sql(f"update machine_sessions set started_at = started_at - interval '30 minutes' where id = '{sid}'")
    sql(f"update machines set last_heartbeat = now() where id = '{mach_id}'")  # giả lập máy trạm còn sống
    time.sleep(62)  # chờ đúng một nhịp của tác vụ định kỳ

    after = bal()

    if nghe:
        su_kien = None
        for _ in range(40):
            ev = nghe.recv(timeout=2)
            if ev is None:
                break
            if ev.get("type") == "balance:updated":
                su_kien = ev
                break
        nghe.close()

        expect(area, "lượt trừ tiền báo về máy trạm", su_kien is not None,
               detail_bad="không có sự kiện balance:updated nào — màn hình khách "
                          "giữ nguyên số dư lúc đăng nhập trong suốt phiên")

        if su_kien:
            d = su_kien.get("data") or {}
            expect(area, "số dư báo về khớp với số dư thật trong database",
                   d.get("balance") == after,
                   detail_ok=f"{d.get('balance')}₫",
                   detail_bad=f"báo {d.get('balance')}₫ nhưng database là {after}₫")
            expect(area, "sự kiện kèm mốc hết tiền do máy chủ tính",
                   bool(d.get("affordable_until")),
                   detail_bad="thiếu affordable_until — máy trạm phải tự ước lượng, "
                              "mà ước lượng đó không biết thời lượng tối thiểu và gói khung giờ")
            expect(area, "sự kiện ghi rõ hội viên nào", d.get("member_id") == mid,
                   detail_bad=f"member_id = {d.get('member_id')!r} — máy trạm lọc theo khoá này, "
                              "sai là sự kiện bị bỏ qua lặng lẽ")
    taken = before - after
    expect(area, "tiền bị trừ giữa phiên, không đợi tới lúc trả máy", taken > 0,
           f"đã trừ {taken}₫", "số dư không đổi — vẫn dồn tới lúc trả máy")

    # Số tiền phải khớp với chính công thức mà trang quản trị xem trước.
    st, res = call("GET", f"/api/sessions/calculate-cost?machine_id={mach_id}&member_id={mid}&duration_minutes=30", token=t)
    want = (data_of(res) or {}).get("final_cost", -1)
    got = charged(sid)
    expect(area, "tiền đã trừ khớp với bảng giá", got == want,
           f"{got}₫ = {want}₫", f"đã trừ {got}₫ nhưng bảng giá nói {want}₫")

    # Lượt tính lặp lại chỉ thu phần chênh.
    #
    # Đo thẳng BẤT BIẾN chứ không đo số tiền: tổng đã trừ phải luôn bằng đúng giá
    # của quãng đã chơi theo bảng giá. Đo tiền với một dung sai cứng là phép kiểm
    # dở — phiên vắt qua ranh giới khung giờ cao điểm thì giá đổi và tổng luỹ kế
    # nhảy theo, đúng như thiết kế, mà phép kiểm lại báo sai.
    sql(f"update machines set last_heartbeat = now() where id = '{mach_id}'")  # giả lập máy trạm còn sống
    time.sleep(62)
    elapsed = int(sql("select floor(extract(epoch from (now() - started_at)) / 60) "
                      f"from machine_sessions where id = '{sid}'") or 0)

    def price_of(minutes: int) -> int:
        _, r = call("GET", f"/api/sessions/calculate-cost?machine_id={mach_id}&member_id={mid}"
                           f"&duration_minutes={max(0, minutes)}", token=t)
        return (data_of(r) or {}).get("final_cost", -1)

    got = charged(sid)
    ceiling = price_of(elapsed)
    # Tổng đã trừ là "tính tới NHỊP GẦN NHẤT", không phải tới giây này: lượt tính
    # chạy mỗi phút nên nó luôn chậm hơn thực tế tối đa một nhịp. Đòi bằng đúng
    # giá tại thời điểm đọc là phép kiểm sai, không phải sản phẩm sai.
    floor_ = price_of(elapsed - 2)

    expect(area, "tổng đã trừ không bao giờ vượt giá của quãng đã chơi", got <= ceiling,
           f"{got}₫ ≤ {ceiling}₫",
           f"đã trừ {got}₫ nhưng giá {elapsed} phút chỉ là {ceiling}₫ — thu quá tay")
    expect(area, "tổng đã trừ bám sát, chậm nhiều nhất một nhịp", got >= floor_,
           f"{got}₫ ≥ {floor_}₫",
           f"đã trừ {got}₫, giá {elapsed - 2} phút đã là {floor_}₫ — lượt tính không chạy")

    # Trả máy: tổng thu không được vượt giá của trọn phiên.
    st, res = call("POST", f"/api/sessions/{sid}/end", token=t)
    total = (data_of(res) or {}).get("total_cost", 0)
    final_charged = charged(sid)
    expect(area, "trả máy không thu lại phần đã trừ dọc đường", final_charged == total,
           f"đã trừ {final_charged}₫ = tiền phiên {total}₫",
           f"đã trừ {final_charged}₫ nhưng tiền phiên là {total}₫ — thu hai lần")

    # Sổ giao dịch: ĐÚNG MỘT dòng cho cả phiên, và bất biến sổ cái phải đúng.
    rows = sql(f"select count(*) from member_transactions where reference_id = '{sid}' "
               "and transaction_type = 'session_fee'")
    expect(area, "sổ giao dịch chỉ có một dòng cho cả phiên", rows == "1",
           detail_bad=f"{rows} dòng — mỗi phút một dòng sẽ làm ngập sổ của khách")
    ok_ledger = sql(f"select (balance_after - balance_before) + (bonus_after - bonus_before) = amount "
                    f"from member_transactions where reference_id = '{sid}' "
                    "and transaction_type = 'session_fee'")
    expect(area, "sổ cái cân: chênh lệch số dư đúng bằng số tiền ghi", ok_ledger == "t",
           detail_bad="bonus_before/bonus_after bỏ trống nên sổ không đối soát được")

    # --- phiên đang chạy dở lúc triển khai ----------------------------------
    # charged_amount khác 0 mà chưa đồng nào rời khỏi số dư. Dòng sổ đầu tiên
    # phải ghi số tiền THẬT SỰ trừ, không phải tổng luỹ kế — lấy tổng thì sổ ghi
    # -15.000₫ trong khi số dư chỉ giảm 833₫.
    sql(f"update members set balance = 500000, bonus_balance = 0 where id = '{mid}'")
    sid = start()
    if sid:
        # charged_amount phải THẤP HƠN giá của 30 phút, nếu không chênh lệch bằng
        # 0 và phép kiểm chẳng đo được gì.
        sql(f"update machine_sessions set started_at = started_at - interval '30 minutes', "
            f"charged_amount = 3000 where id = '{sid}'")
        before = bal()
        sql(f"update machines set last_heartbeat = now() where id = '{mach_id}'")  # giả lập máy trạm còn sống
        time.sleep(62)
        moved = before - bal()
        row = sql(f"select -amount from member_transactions where reference_id = '{sid}' "
                  "and transaction_type = 'session_fee'")
        expect(area, "sau backfill, dòng sổ ghi đúng số tiền thật sự trừ",
               row != "" and int(row) == moved,
               f"sổ ghi {row}₫ = số dư giảm {moved}₫",
               f"sổ ghi {row}₫ nhưng số dư chỉ giảm {moved}₫ — bất biến sổ cái vỡ ngay dòng đầu")
        call("POST", f"/api/sessions/{sid}/end", token=t)
        sql(f"delete from member_transactions where reference_id = '{sid}'")

    # --- hết tiền thì tự trả máy -------------------------------------------
    sql(f"update members set balance = 500000, bonus_balance = 0 where id = '{mid}'")
    sid = start()
    if sid:
        sql(f"update members set balance = 1, bonus_balance = 0 where id = '{mid}'")
        sql(f"update machine_sessions set started_at = started_at - interval '5 minutes' where id = '{sid}'")
        sql(f"update machines set last_heartbeat = now() where id = '{mach_id}'")  # giả lập máy trạm còn sống
        time.sleep(62)
        expect(area, "hết tiền thì phiên tự đóng",
               sql(f"select is_active from machine_sessions where id = '{sid}'") == "f",
               detail_bad="khách hết sạch tiền vẫn ngồi máy")
        # KHÔNG so với "available": chờ 62 giây thì máy cũng quá hạn heartbeat và
        # bị markStaleMachinesOffline chuyển sang "offline". Thứ cần đo là máy
        # không còn kẹt ở "in_use".
        status = sql(f"select status from machines where id = '{mach_id}'")
        expect(area, "hết tiền thì máy được giải phóng", status != "in_use",
               status, f"máy kẹt ở {status!r} — không ai ngồi được nữa")

    sql("update machines set status = 'available' where deleted_at is null")
    sql(f"delete from member_transactions where member_id = '{mid}'")
    sql(f"delete from machine_sessions where member_id = '{mid}'")
    sql(f"delete from members where id = '{mid}'")


def flow_remote_shutdown_ends_session(t: str, ids: dict) -> None:
    """Tắt máy từ xa phải CHỐT PHIÊN.

    Trước đây RemoteAction chỉ đẩy một sự kiện WebSocket xuống máy rồi ghi nhật
    ký, không chạm gì tới phiên: tắt máy xong phiên vẫn is_active = true nên
    khách không bị tính tiền, máy kẹt ở "in_use", và chính khách đó lần sau
    không mở được máy nào vì "member already has an active session".
    """
    area = "24. Tắt máy từ xa chốt phiên"

    code = "KT-01"
    mach_id = sql(f"select id from machines where machine_code = '{code}'")
    other_id = ids.get("machine2", "")

    def reset() -> str:
        sql("update machines set status = 'available' where deleted_at is null")
        sql(f"delete from machine_sessions where member_id = '{mid}'")
        sql(f"update members set balance = 200000, bonus_balance = 0 where id = '{mid}'")
        st, res = call("POST", "/api/sessions/start", {"machine_id": mach_id, "member_id": mid}, t)
        return (data_of(res) or {}).get("id", "")

    sql("delete from members where username = 'kt_tatmay'")
    st, res = call("POST", "/api/members",
                   {"username": "kt_tatmay", "password": "test1234", "full_name": "KT tat may"}, t)
    mid = (data_of(res) or {}).get("id", "")
    if not mid:
        report.add(area, "tạo hội viên để thử", BROKEN, str(st))
        return

    # --- máy NGOẠI TUYẾN: không được đụng vào phiên -------------------------
    sid = reset()
    if not sid:
        report.add(area, "mở phiên để thử", BROKEN, "không mở được máy")
        return
    st, res = call("POST", f"/api/machines/{mach_id}/remote/shutdown", token=t)
    expect(area, "máy ngoại tuyến thì báo 409", st == 409,
           res.get("message", "")[:60], f"HTTP {st}")
    expect(area, "máy ngoại tuyến thì KHÔNG chốt phiên",
           sql(f"select is_active from machine_sessions where id = '{sid}'") == "t",
           detail_bad="phiên bị đóng dù lệnh không tới được máy — tính tiền khi chưa chắc khách đã rời")
    expect(area, "máy ngoại tuyến thì số dư không đổi",
           sql(f"select balance from members where id = '{mid}'") == "200000",
           detail_bad="đã trừ tiền dù lệnh không gửi được")

    # --- máy ĐANG kết nối: phải chốt phiên -----------------------------------
    for action in ("shutdown", "restart"):
        sid = reset()
        if not sid:
            report.add(area, f"mở phiên trước khi {action}", BROKEN, "không mở được máy")
            continue
        # Lùi giờ để phiên có tiền, nếu không total_cost = 0 và phép kiểm vô nghĩa.
        sql(f"update machine_sessions set started_at = started_at - interval '90 minutes' where id = '{sid}'")

        try:
            fake = FakeMachine(code, t)
        except Exception as e:  # noqa: BLE001
            report.add(area, f"máy trạm kết nối WS trước khi {action}", BROKEN, str(e)[:60])
            continue
        time.sleep(0.5)

        st, res = call("POST", f"/api/machines/{mach_id}/remote/{action}", token=t)
        expect(area, f"lệnh {action} gửi được tới máy đang kết nối", st == 200,
               detail_bad=f"HTTP {st} — {res.get('message', '')[:70]}")

        ended = (data_of(res) or {}).get("session") or {}
        expect(area, f"{action} trả về kết quả chốt tiền", bool(ended),
               f"{ended.get('total_cost')}₫",
               "phản hồi không kèm phiên — nhân viên không biết đã thu bao nhiêu")

        expect(area, f"{action}: phiên đã đóng",
               sql(f"select is_active from machine_sessions where id = '{sid}'") == "f",
               detail_bad="phiên vẫn is_active — khách không bị tính tiền")
        expect(area, f"{action}: máy trả về sẵn sàng",
               sql(f"select status from machines where id = '{mach_id}'") == "available",
               detail_bad="máy kẹt ở in_use, không ai ngồi được nữa")
        expect(area, f"{action}: có ghi giao dịch tiền phiên",
               sql(f"select count(*) from member_transactions where member_id = '{mid}' "
                   "and transaction_type = 'session_fee'") != "0",
               detail_bad="không có dòng tiền nào — doanh thu bốc hơi")

        # Đúng lỗi mà người dùng gặp: sau khi tắt máy phải mở được máy khác.
        if other_id:
            sql("update machines set status = 'available' where deleted_at is null")
            st, _ = call("POST", "/api/sessions/start",
                         {"machine_id": other_id, "member_id": mid}, t)
            expect(area, f"{action}: khách mở lại được máy khác", st in (200, 201),
                   detail_bad=f"HTTP {st} — vẫn kẹt 'member already has an active session'")
            sql(f"update machine_sessions set is_active = false, ended_at = now() where member_id = '{mid}'")

        fake.close()

    # --- lệnh KHÔNG kết thúc phiên -------------------------------------------
    sid = reset()
    try:
        fake = FakeMachine(code, t)
        time.sleep(0.5)
        for action in ("lock", "unlock", "message"):
            st, _ = call("POST", f"/api/machines/{mach_id}/remote/{action}", {"text": "kt"}, t)
            expect(area, f"lệnh {action} KHÔNG đụng tới phiên",
                   st == 200 and sql(f"select is_active from machine_sessions where id = '{sid}'") == "t",
                   detail_bad=f"HTTP {st} — khoá màn hình mà cũng chốt tiền thì không ai dám bấm")
        fake.close()
    except Exception as e:  # noqa: BLE001
        report.add(area, "lệnh không kết thúc phiên", BROKEN, str(e)[:60])

    sql("update machines set status = 'available' where deleted_at is null")
    sql(f"delete from machine_sessions where member_id = '{mid}'")
    sql(f"delete from members where id = '{mid}'")



def flow_hardware_retention(t: str, ids: dict) -> None:
    """Lịch sử phần cứng: đọc đúng thứ tự, và dữ liệu cũ tự biến mất.

    Bảng này lớn nhanh nhất hệ thống — mỗi máy một dòng cho mỗi lần báo cáo, tức
    4 dòng/phút. Kiểm trên PostgreSQL thật chứ không bằng SQL giả: câu DELETE
    dùng truy vấn con có LIMIT, và đó là thứ chỉ database thật mới nói được đúng
    hay sai.
    """
    area = "31. Lịch sử phần cứng"

    mid = ids.get("machine3", "")
    if not mid:
        report.add(area, "chuẩn bị máy để kiểm", BROKEN, "không lấy được máy KT-03")
        return

    # --- thứ tự đọc ----------------------------------------------------------
    # Bốn lần báo cáo liên tiếp, uptime tăng dần: dòng mới nhất phải nằm trên cùng.
    for i in range(4):
        call("POST", "/api/machines/by-code/KT-03/heartbeat",
             {"cpu_temp": 40 + i, "cpu_usage": 10 + i, "uptime": 1000 + i})

    st, res = call("GET", f"/api/machines/{mid}/hardware?page=1&page_size=10", None, t)
    ups = [r.get("uptime") for r in items_of(res)]
    expect(area, "nhật ký sắp theo thời gian giảm dần, không theo id",
           ups[:4] == [1003, 1002, 1001, 1000],
           detail_ok="mới nhất nằm trên cùng",
           detail_bad=f"HTTP {st}, uptime theo thứ tự trả về = {ups[:6]} — "
                      f"mặc định 'id desc' là thứ tự UUID ngẫu nhiên")

    # --- dọn dẹp -------------------------------------------------------------
    # Chèn thẳng bằng SQL: chờ dữ liệu tự già đi thì kịch bản này chạy mất bảy ngày.
    sql(f"insert into machine_hardware_snapshots (machine_id, cpu_temp, uptime, created_at) "
        f"select '{mid}', 41, 1, now() - interval '30 days' from generate_series(1, 25)")
    cu = sql(f"select count(*) from machine_hardware_snapshots "
             f"where machine_id = '{mid}' and created_at < now() - interval '7 days'")
    expect(area, "dựng được dữ liệu cũ để dọn", cu == "25", detail_bad=f"chèn được {cu} dòng")

    # Máy chủ dọn lúc khởi động, nên khởi động lại là quan sát được ngay thay vì
    # phải đợi hết một giờ.
    subprocess.run(["docker", "restart", APP_CONTAINER], capture_output=True, text=True)
    for _ in range(45):
        st, _ = call("GET", "/api/health", timeout=3)
        if st == 200:
            break
        time.sleep(1)

    con_lai_cu = sql(f"select count(*) from machine_hardware_snapshots "
                     f"where machine_id = '{mid}' and created_at < now() - interval '7 days'")
    expect(area, "số đo cũ hơn hạn giữ bị xoá lúc máy chủ khởi động",
           con_lai_cu == "0",
           detail_ok="25 dòng cũ đã sạch",
           detail_bad=f"còn {con_lai_cu} dòng cũ — tác vụ dọn dẹp không chạy")

    con_lai_moi = sql(f"select count(*) from machine_hardware_snapshots "
                      f"where machine_id = '{mid}' and created_at >= now() - interval '7 days'")
    expect(area, "số đo trong hạn giữ KHÔNG bị xoá lây",
           con_lai_moi not in ("", "0"),
           detail_ok=f"{con_lai_moi} dòng còn nguyên",
           detail_bad="dọn dẹp xoá cả dữ liệu còn hạn")


def flow_telemetry(t: str, ids: dict) -> None:
    """Máy trạm báo cáo định kỳ: giữ máy online, cập nhật mạng và cấu hình máy.

    Không kiểm được bằng máy Windows thật từ đây, nên kịch bản này đóng vai máy
    trạm: gửi đúng gói tin mà agent.go gửi, rồi soi lại database.
    """
    area = "30. Báo cáo từ máy trạm"

    # KT-02 cố ý KHÔNG được cấp khoá: đây cũng là đường đi mặc định của mọi máy
    # sau khi khoá trở thành tuỳ chọn.
    def report_in(body: dict):
        return call("POST", "/api/machines/by-code/KT-02/heartbeat", body)[0]

    day = "select ip_address, mac_address, cpu_name, gpu_name, ram_gb, storage_gb " \
          "from machines where machine_code = 'KT-02'"

    # --- gói tin đầy đủ ------------------------------------------------------
    st = report_in({
        "cpu_temp": 52.5, "gpu_temp": 61.0, "ip": "192.168.9.21",
        "mac": "AA:BB:CC:00:00:21", "cpu_usage": 33.5, "ram_usage": 61.25,
        "disk_usage": 72.5, "uptime": 90061,
        "cpu_name": "Intel Core i5-12400F", "gpu_name": "NVIDIA GeForce RTX 3060",
        "ram_gb": 16, "storage_gb": 512,
    })
    row = sql(day).split("|")
    expect(area, "gói tin đầy đủ ghi được mạng và cấu hình máy",
           st == 200 and row[:6] == ["192.168.9.21", "AA:BB:CC:00:00:21",
                                     "Intel Core i5-12400F", "NVIDIA GeForce RTX 3060", "16", "512"],
           detail_ok=" · ".join(row[:6]), detail_bad=f"HTTP {st}, database = {row}")

    # --- gói tin thiếu ip/mac: KHÔNG được xoá trắng --------------------------
    # Đây là lỗi chính. Vòng giám sát 60 giây gửi gói không có ip/mac, mà máy chủ
    # ghi đè vô điều kiện — mỗi phút địa chỉ máy lại bị xoá rồi ghi lại, nên trên
    # trang Máy hai cột đó nhấp nháy lúc có lúc không.
    st = report_in({"cpu_temp": 53.0, "gpu_temp": 62.0, "cpu_usage": 30.0, "uptime": 90121})
    row = sql(day).split("|")
    expect(area, "gói tin thiếu ip/mac KHÔNG xoá trắng địa chỉ đang có",
           st == 200 and row[0] == "192.168.9.21" and row[1] == "AA:BB:CC:00:00:21",
           detail_ok=f"{row[0]} · {row[1]}",
           detail_bad=f"HTTP {st}, ip={row[0]!r} mac={row[1]!r}")

    expect(area, "gói tin thiếu cấu hình KHÔNG xoá trắng cấu hình đang có",
           row[2:6] == ["Intel Core i5-12400F", "NVIDIA GeForce RTX 3060", "16", "512"],
           detail_bad=f"database = {row[2:6]}")

    # --- uptime có chỗ để rơi vào -------------------------------------------
    # Máy trạm vẫn gửi trường này từ trước, nhưng DTO máy chủ không có nó và bảng
    # lịch sử cũng không có cột — con số rơi vào hư không ở mỗi lần báo.
    got = sql("select uptime from machine_hardware_snapshots s "
              "join machines m on m.id = s.machine_id "
              "where m.machine_code = 'KT-02' order by s.created_at desc limit 1")
    expect(area, "uptime được lưu vào lịch sử phần cứng", got == "90121",
           detail_ok=f"{got} giây", detail_bad=f"nhận được {got!r}, mong 90121")

    # --- tên CPU dài hơn bề rộng cột ----------------------------------------
    # varchar(100). PostgreSQL từ chối CẢ câu lệnh khi chuỗi quá dài chứ không
    # cắt hộ, nên không cắt ở tầng ứng dụng là mất luôn cả nhịp tim lẫn dòng
    # lịch sử của lần báo đó.
    dai = "Intel(R) Core(TM) i9-14900KS Processor 6.20 GHz " + "x" * 120
    st = report_in({"cpu_temp": 54.0, "cpu_name": dai, "uptime": 90181})
    stored = sql("select cpu_name from machines where machine_code = 'KT-02'")
    expect(area, "tên CPU quá dài bị cắt chứ không làm hỏng cả lần báo",
           st == 200 and len(stored) == 100 and stored == dai[:100],
           detail_ok=f"cắt còn {len(stored)} ký tự",
           detail_bad=f"HTTP {st}, dài {len(stored)} ký tự")

    # --- máy offline tự bật lại thành sẵn sàng ------------------------------
    trang_thai = "select status from machines where machine_code = 'KT-02'"
    sql("update machines set status = 'offline' where machine_code = 'KT-02'")
    st = report_in({"cpu_temp": 50.0, "uptime": 90241})
    sau = sql(trang_thai)
    expect(area, "máy đang offline báo cáo thì tự chuyển thành sẵn sàng",
           st == 200 and sau == "available",
           detail_bad=f"HTTP {st}, trạng thái = {sau!r}")



def flow_one_login_box(t: str, ids: dict) -> None:
    """Máy trạm có MỘT ô đăng nhập; tên tài khoản quyết định đường đi.

    Trước đây màn hình khoá có hai ô: ô chính cho hội viên và một nút "Đăng nhập
    quản trị" nằm khuất bên dưới. Nhân viên phải nhớ mình thuộc loại nào trước
    khi gõ, mà gõ nhầm ô thì máy chủ báo "sai mật khẩu" — sai chỗ để đi tìm.
    """
    area = "32. Một ô đăng nhập"

    code = "KT-01"
    sql("update machines set status = 'available' where machine_code = '" + code + "'")

    # --- tên nhân viên -------------------------------------------------------
    st, res = call("POST", "/api/auth/client-login",
                   {"username": "staff", "password": "admin123", "machine_code": code})
    d = data_of(res) or {}
    expect(area, "tài khoản nhân viên vào được bằng ô chung",
           st == 200 and d.get("kind") == "staff",
           detail_ok=f"kind={d.get('kind')}",
           detail_bad=f"HTTP {st}, kind={d.get('kind')!r}")
    expect(area, "nhân viên vào máy thì KHÔNG mở phiên tính tiền",
           not d.get("session_id"),
           detail_bad=f"đã mở phiên {d.get('session_id')} — nhân viên trông máy bị tính tiền")

    # --- tên hội viên --------------------------------------------------------
    sql("update machines set status = 'available' where machine_code = '" + code + "'")
    sql("delete from machine_sessions where member_id in "
        "(select id from members where username = 'kt_mot_o')")
    sql("delete from members where username = 'kt_mot_o'")
    st, res = call("POST", "/api/members",
                   {"username": "kt_mot_o", "password": "test1234", "full_name": "KT mot o",
                    "group_id": ids.get("member_group", "")}, t)
    mid = (data_of(res) or {}).get("id", "")
    if not mid:
        report.add(area, "tạo hội viên để thử", BROKEN, str(st))
        return
    sql(f"update members set balance = 200000 where id = '{mid}'")

    st, res = call("POST", "/api/auth/client-login",
                   {"username": "kt_mot_o", "password": "test1234", "machine_code": code})
    d = data_of(res) or {}
    expect(area, "tài khoản hội viên vào được bằng ĐÚNG ô đó",
           st == 200 and d.get("kind") == "member",
           detail_ok=f"kind={d.get('kind')}",
           detail_bad=f"HTTP {st}, kind={d.get('kind')!r}")
    expect(area, "hội viên vào thì phiên tính tiền mở ngay",
           bool(d.get("session_id")),
           detail_bad="không có session_id — khách ngồi máy mà không ai tính tiền")

    # --- một tài khoản, một máy: đang chơi KT-01 thì máy khác bị chặn ---------
    # kt_mot_o vừa mở phiên trên KT-01 ngay trên. Thử đăng nhập máy thứ hai.
    code2 = "MOT-O-2"
    sql(f"delete from machine_sessions where machine_id in "
        f"(select id from machines where machine_code = '{code2}')")
    sql(f"delete from machines where machine_code = '{code2}'")
    call("POST", "/api/machines",
         {"machine_code": code2, "group_id": ids.get("machine_group", "")}, t)
    st_hai, res_hai = call("POST", "/api/auth/client-login",
                           {"username": "kt_mot_o", "password": "test1234", "machine_code": code2})
    expect(area, "đang chơi máy này, đăng nhập máy khác bị chặn", st_hai == 401,
           detail_bad=f"HTTP {st_hai} — một tài khoản mở được hai máy cùng lúc")
    msg = res_hai.get("message", "")
    expect(area, "thông báo tiếng Việt, nêu tên máy đang chơi",
           ("KT-01" in msg) and ("trả máy" in msg),
           detail_ok=msg, detail_bad=f"thông báo: {msg!r}")

    # --- sai mật khẩu --------------------------------------------------------
    st, _ = call("POST", "/api/auth/client-login",
                 {"username": "staff", "password": "sai-hoan-toan", "machine_code": code})
    expect(area, "sai mật khẩu nhân viên bị từ chối", st == 401, f"{st}", f"HTTP {st}")

    st, _ = call("POST", "/api/auth/client-login",
                 {"username": "khong-co-ai-ten-nay", "password": "gi-cung-duoc",
                  "machine_code": code})
    expect(area, "tên không có ở cả hai bảng bị từ chối", st == 401, f"{st}", f"HTTP {st}")

    # Dọn phiên vừa mở để giai đoạn sau không vướng máy.
    sid = sql(f"select id from machine_sessions where member_id = '{mid}' and is_active = true")
    if sid:
        call("POST", f"/api/sessions/{sid}/end", token=t)


def flow_reboot_closes_session(t: str, ids: dict) -> None:
    """Máy đóng băng reboot giữa phiên: phiên cũ phải đóng, login sau là phiên mới.

    Tiền do máy chủ tính nên phiên vẫn sống và vẫn trừ tiền sau khi máy reboot —
    máy nằm ở màn hình khoá mà đồng hồ vẫn chạy, và khách kế tiếp bị chặn "máy
    đang bận". Máy chủ suy ra reboot từ uptime: phiên bắt đầu TRƯỚC mốc khởi
    động hiện tại là phiên còn sót.
    """
    area = "33. Reboot đóng phiên"

    # Máy RIÊNG, không cấp khoá: KT-01 đã bị cấp khoá ở khu 27 nên heartbeat
    # keyless sẽ bị 401. Máy mới thì cắm vào là chạy.
    code = "RBT-01"
    sql(f"delete from machine_sessions where machine_id in "
        f"(select id from machines where machine_code = '{code}')")
    sql(f"delete from machines where machine_code = '{code}'")
    st, res = call("POST", "/api/machines",
                   {"machine_code": code, "group_id": ids.get("machine_group", "")}, t)
    mach_id = (data_of(res) or {}).get("id", "")
    if not mach_id:
        report.add(area, "tạo máy riêng để kiểm", BROKEN, f"{st} {res.get('message','')[:60]}")
        return

    for u in ("kt_reboot_a", "kt_reboot_b"):
        sql("delete from machine_sessions where member_id in "
            f"(select id from members where username = '{u}')")
        sql(f"delete from members where username = '{u}'")
    ma = (data_of(call("POST", "/api/members",
          {"username": "kt_reboot_a", "password": "test1234", "full_name": "Reboot A",
           "group_id": ids.get("member_group", "")}, t)[1]) or {}).get("id", "")
    mb = (data_of(call("POST", "/api/members",
          {"username": "kt_reboot_b", "password": "test1234", "full_name": "Reboot B",
           "group_id": ids.get("member_group", "")}, t)[1]) or {}).get("id", "")
    if not (ma and mb):
        report.add(area, "tạo hội viên để thử", BROKEN, "không tạo được")
        return
    sql(f"update members set balance = 500000 where id in ('{ma}', '{mb}')")

    sid_a = (data_of(call("POST", "/api/sessions/start",
             {"machine_id": mach_id, "member_id": ma}, t)[1]) or {}).get("id", "")
    if not sid_a:
        report.add(area, "mở phiên cho A", BROKEN, "không mở được")
        return

    # Lùi phiên A 30 phút: giả lập A đã chơi nửa tiếng rồi máy mới reboot.
    sql(f"update machine_sessions set started_at = started_at - interval '30 minutes' where id = '{sid_a}'")

    # Trước reboot: uptime cao → mốc khởi động nằm TRƯỚC lúc phiên bắt đầu.
    st_hb = agent_hb(code, uptime=7200)
    expect(area, "heartbeat máy keyless được nhận", st_hb == 200,
           detail_bad=f"HTTP {st_hb} — máy mới lẽ ra cắm vào là chạy")
    con_song = sql(f"select is_active from machine_sessions where id = '{sid_a}'")
    expect(area, "trước reboot: phiên A vẫn sống", con_song == "t",
           detail_bad=f"is_active={con_song!r} — heartbeat bình thường không được đóng phiên")

    # Reboot: uptime rơi về gần 0 → mốc khởi động nhảy tới hiện tại, SAU lúc
    # phiên A bắt đầu.
    agent_hb(code, uptime=20)
    la_reboot = sql(f"select m.booted_at > s.started_at + interval '2 minutes' "
                    f"from machines m join machine_sessions s on s.machine_id = m.id "
                    f"where s.id = '{sid_a}'")
    expect(area, "máy chủ nhận ra máy đã reboot giữa phiên", la_reboot == "t",
           detail_bad=f"booted_at không vượt started_at — {la_reboot!r}")

    # --- Đường ĐĂNG NHẬP (tất định): B ngồi vào, phải là phiên mới ------------
    st_code, res = call("POST", "/api/auth/client-login",
                        {"username": "kt_reboot_b", "password": "test1234", "machine_code": code})
    sid_b = (data_of(res) or {}).get("session_id", "")
    expect(area, "khách kế tiếp đăng nhập được, mở PHIÊN MỚI",
           st_code == 200 and bool(sid_b) and sid_b != sid_a,
           detail_ok=f"phiên mới {sid_b[:8]}",
           detail_bad=f"HTTP {st_code}, session_id={sid_b[:8] or 'rỗng'} (phiên A={sid_a[:8]})")

    da = sql(f"select is_active from machine_sessions where id = '{sid_a}'")
    expect(area, "phiên A cũ đã đóng khi B đăng nhập", da == "f",
           detail_bad=f"is_active={da!r} — phiên trước reboot vẫn sống")
    end_at = sql(f"select ended_at is not null from machine_sessions where id = '{sid_a}'")
    expect(area, "phiên A được chốt sổ (có ended_at)", end_at == "t",
           detail_bad="đóng mà không chốt sổ")

    # --- Đường TÁC VỤ ĐỊNH KỲ: máy reboot rồi bỏ đó, không ai đăng nhập -------
    if sid_b:
        sql(f"update machine_sessions set started_at = started_at - interval '30 minutes' where id = '{sid_b}'")
        agent_hb(code, uptime=15)
        dong = False
        for _ in range(26):  # tối đa ~130s, đủ cho hai nhịp scheduler
            time.sleep(5)
            if sql(f"select is_active from machine_sessions where id = '{sid_b}'") == "f":
                dong = True
                break
        expect(area, "máy reboot rồi bỏ đó: tác vụ định kỳ tự đóng phiên", dong,
               detail_ok="scheduler đã đóng phiên sót",
               detail_bad="sau ~130s phiên vẫn sống — máy nằm không mà đồng hồ vẫn chạy")

    for sid in (sid_a, sid_b):
        if sid:
            call("POST", f"/api/sessions/{sid}/end", token=t)



def flow_offline_closes_session(t: str, ids: dict) -> None:
    """Máy tắt/rút mạng mà không đăng xuất: phiên phải tự đóng, chốt sổ tại nhịp
    tim cuối (không tính phần máy đã tắt).

    Tiền do máy chủ tính nên máy tắt vẫn bị đếm; không có lớp này thì phiên chạy
    tới khi hết sạch tiền — máy đã tắt vẫn tính hàng chục tiếng.
    """
    area = "34. Máy mất tín hiệu đóng phiên"

    code = "OFF-01"
    sql(f"delete from machine_sessions where machine_id in "
        f"(select id from machines where machine_code = '{code}')")
    sql(f"delete from machines where machine_code = '{code}'")
    mach_id = (data_of(call("POST", "/api/machines",
               {"machine_code": code, "group_id": ids.get("machine_group", "")}, t)[1]) or {}).get("id", "")
    sql("delete from machine_sessions where member_id in "
        "(select id from members where username = 'kt_offline')")
    sql("delete from members where username = 'kt_offline'")
    mid = (data_of(call("POST", "/api/members",
           {"username": "kt_offline", "password": "test1234", "full_name": "KT offline",
            "group_id": ids.get("member_group", "")}, t)[1]) or {}).get("id", "")
    if not (mach_id and mid):
        report.add(area, "tạo máy/hội viên để thử", BROKEN, "không tạo được")
        return
    sql(f"update members set balance = 500000 where id = '{mid}'")

    sid = (data_of(call("POST", "/api/sessions/start",
           {"machine_id": mach_id, "member_id": mid}, t)[1]) or {}).get("id", "")
    if not sid:
        report.add(area, "mở phiên", BROKEN, "không mở được")
        return

    # Máy đã chơi 40 phút, nhịp tim cuối là 5 phút trước (giả lập tắt máy 5 phút
    # trước và không đăng xuất). Ngưỡng đóng là 2 phút → phải đóng.
    sql(f"update machine_sessions set started_at = started_at - interval '40 minutes' where id = '{sid}'")
    sql(f"update machines set last_heartbeat = now() - interval '5 minutes' where id = '{mach_id}'")

    dong = False
    for _ in range(20):  # tối đa ~100s cho một nhịp scheduler
        time.sleep(5)
        if sql(f"select is_active from machine_sessions where id = '{sid}'") == "f":
            dong = True
            break
    expect(area, "máy tắt quá 2 phút thì phiên tự đóng", dong,
           detail_bad="sau ~100s phiên vẫn sống — máy tắt vẫn bị tính tiền")

    if dong:
        # Chốt sổ tại nhịp tim cuối: thời lượng tính tiền phải quanh 35 phút
        # (40 phút chơi trừ 5 phút cuối máy đã tắt), KHÔNG phải 40+ phút.
        dur = sql(f"select duration_minutes from machine_sessions where id = '{sid}'")
        try:
            dm = int(dur)
        except ValueError:
            dm = -1
        expect(area, "chốt sổ tại nhịp tim cuối, không tính phần máy đã tắt",
               33 <= dm <= 37,
               detail_ok=f"{dm} phút (≈35, đã trừ 5 phút offline)",
               detail_bad=f"duration_minutes={dur!r} — tính cả phần máy đã tắt")

        end_near = sql(f"select abs(extract(epoch from (ended_at - "
                       f"(now() - interval '5 minutes')))) < 90 "
                       f"from machine_sessions where id = '{sid}'")
        expect(area, "ended_at đặt tại nhịp tim cuối", end_near == "t",
               detail_bad="ended_at không khớp nhịp tim cuối")

    st = sql(f"select status from machines where machine_code = '{code}'")
    expect(area, "máy được giải phóng", st != "in_use",
           detail_ok=st, detail_bad=f"status={st}")

    # Máy CHƯA từng báo cáo (last_heartbeat NULL): phiên nhân viên mở tay không
    # được đụng — đó không phải máy tắt, mà là máy không có máy trạm.
    code_null = "OFF-NULL"
    sql(f"delete from machine_sessions where machine_id in "
        f"(select id from machines where machine_code = '{code_null}')")
    sql(f"delete from machines where machine_code = '{code_null}'")
    mn = (data_of(call("POST", "/api/machines",
          {"machine_code": code_null, "group_id": ids.get("machine_group", "")}, t)[1]) or {}).get("id", "")
    sid_n = (data_of(call("POST", "/api/sessions/start",
             {"machine_id": mn, "member_id": mid}, t)[1]) or {}).get("id", "")
    if sid_n:
        # last_heartbeat vẫn NULL (chưa heartbeat lần nào). Chờ một nhịp scheduler.
        time.sleep(63)
        alive = sql(f"select is_active from machine_sessions where id = '{sid_n}'")
        expect(area, "máy chưa từng báo cáo (NULL) thì KHÔNG bị đụng", alive == "t",
               detail_bad="phiên máy chưa cài máy trạm bị đóng oan")
        call("POST", f"/api/sessions/{sid_n}/end", token=t)


def flow_menu_contract(t: str, ids: dict) -> None:
    """Ba điều kiện mà thực đơn máy trạm dựa vào, không cái nào lộ ra khi đọc code.

    App.GetMenu (client/src/app.go) xin page_size=100 và sắp theo sort_order, rồi
    đổi image_url tương đối thành tuyệt đối. Nếu máy chủ hạ trần phân trang, bỏ
    sắp xếp theo cột, hay ngừng phục vụ /uploads thì thực đơn hỏng lặng lẽ: khách
    chỉ thấy 20 món đầu, hoặc thấy toàn ô ảnh trắng. Không có màn hình nào báo lỗi.
    """
    area = "29. Hợp đồng thực đơn máy trạm"

    # --- lấy trọn thực đơn trong một lần gọi ---------------------------------
    # Máy khách không có nút sang trang, nên page_size phải được tôn trọng
    # nguyên vẹn. Bỏ nó đi là mặc định 20 và khách không thấy phần còn lại.
    st, res = call("GET", "/api/products?page_size=500", None, t)
    d = data_of(res) or {}
    expect(area, "page_size=500 được tôn trọng nguyên vẹn", d.get("page_size") == 500,
           detail_bad=f"HTTP {st}, page_size={d.get('page_size')} — thực đơn dài hơn thế là khách không thấy hết")

    # --- lọc theo danh mục ---------------------------------------------------
    # category_id có trong Swagger và thực đơn máy trạm vẫn gửi lên, nhưng handler
    # từng không đọc: mọi thẻ danh mục đều trả về nguyên cả thực đơn.
    cat_id = ids.get("category")
    if cat_id:
        st, res = call("GET", f"/api/products?page_size=500&category_id={cat_id}", None, t)
        items = items_of(res)
        expect(area, "category_id thật sự lọc thực đơn",
               bool(items) and all(p.get("category_id") == cat_id for p in items),
               detail_ok=f"{len(items)} món, đúng một danh mục",
               detail_bad=f"HTTP {st}, {len(items)} món, "
                          f"{len({p.get('category_id') for p in items})} danh mục khác nhau")

        st, res = call("GET", "/api/products?page_size=500&category_id="
                              "00000000-0000-0000-0000-000000000000", None, t)
        expect(area, "danh mục không tồn tại trả danh sách rỗng", items_of(res) == [],
               detail_bad=f"HTTP {st}, {len(items_of(res))} món")

    # --- thứ tự thực đơn -----------------------------------------------------
    # Máy khách KHÔNG gửi tham số sort (handler bỏ qua nó), nên thứ tự mặc định
    # chính là thứ tự khách nhìn thấy trên màn hình đặt món. Phải tự tạo món có
    # sort_order khác nhau: mọi món sẵn có đều là 0, và một danh sách toàn số 0
    # thì "đã sắp xếp" đúng kể cả khi máy chủ trả về theo thứ tự ngẫu nhiên.
    made = []
    for name, order in (("KT-Thu-tu-C", 30), ("KT-Thu-tu-A", 10), ("KT-Thu-tu-B", 20)):
        st, res = call("POST", "/api/products",
                       {"name": name, "price": 1000, "sort_order": order,
                        "category_id": cat_id or None, "is_retail": True}, t)
        pid = (data_of(res) or {}).get("id")
        if pid:
            made.append(pid)

    if len(made) == 3:
        st, res = call("GET", "/api/products?page_size=500&search=KT-Thu-tu", None, t)
        got = [p.get("name") for p in items_of(res)]
        expect(area, "thứ tự mặc định là sort_order tăng dần",
               got == ["KT-Thu-tu-A", "KT-Thu-tu-B", "KT-Thu-tu-C"],
               detail_ok="A(10) → B(20) → C(30)",
               detail_bad=f"HTTP {st}, nhận được {got}")
    else:
        report.add(area, "thứ tự mặc định là sort_order tăng dần", BROKEN,
                   f"chỉ tạo được {len(made)}/3 món để so sánh")

    for pid in made:
        call("DELETE", f"/api/products/{pid}", None, t)

    # --- ảnh có thật sự tải được không --------------------------------------
    # Đây là chỗ duy nhất phân biệt "đã lưu đường dẫn" với "ảnh hiện lên màn
    # hình". Lưu một chuỗi vào cột image_url thì luôn thành công.
    png = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
    boundary = "----vnetverify"
    body = (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="file"; filename="kt.png"\r\n'
        "Content-Type: image/png\r\n\r\n"
    ).encode() + png + f"\r\n--{boundary}--\r\n".encode()

    req = urllib.request.Request(
        BASE + "/api/upload", method="POST", data=body,
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}",
                 "Authorization": "Bearer " + t})
    url = ""
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            url = ((json.loads(resp.read().decode()) or {}).get("data") or {}).get("url", "")
    except Exception as e:
        report.add(area, "tải ảnh lên", BROKEN, f"{type(e).__name__}: {e}"[:160])

    expect(area, "ảnh tải lên trả đường dẫn tương đối", url.startswith("/"),
           detail_ok=url, detail_bad=f"url={url!r}")

    if url:
        try:
            with urllib.request.urlopen(BASE + url, timeout=30) as resp:
                st, ctype, size = resp.status, resp.headers.get("Content-Type", ""), len(resp.read())
        except Exception as e:
            st, ctype, size = 0, f"{type(e).__name__}: {e}", 0
        expect(area, "ảnh vừa tải lên đọc lại được qua /uploads",
               st == 200 and ctype.startswith("image/") and size == len(png),
               detail_ok=f"{ctype}, {size} byte",
               detail_bad=f"HTTP {st}, type={ctype}, {size} byte")

    # Đường dẫn không tồn tại phải là 404 gọn ghẽ. Nếu nó rơi vào trang quản trị
    # nhúng và trả về 200 kèm HTML thì thẻ <img> im lặng hiện ô trắng, và người
    # sửa lỗi sẽ đi tìm nhầm phía database.
    try:
        with urllib.request.urlopen(BASE + "/uploads/khong-ton-tai.png", timeout=15) as resp:
            st, ctype = resp.status, resp.headers.get("Content-Type", "")
    except urllib.error.HTTPError as e:
        st, ctype = e.code, e.headers.get("Content-Type", "")
    except Exception as e:
        st, ctype = 0, str(e)
    expect(area, "ảnh không tồn tại trả 404 chứ không trả HTML", st == 404,
           detail_bad=f"HTTP {st}, type={ctype}")


def flow_ws_audience(t: str, ids: dict) -> None:
    """Sự kiện mang dữ liệu của MỘT hội viên không được phát sang máy của người
    khác. hub.Broadcast gửi tới mọi client kể cả máy trạm, nên session:started
    từng đẩy tên khách này sang màn hình máy khách kia — mở DevTools trên máy
    trạm là đọc được.

    Nhưng máy trạm VẪN phải nhận sự kiện của CHÍNH NÓ, nếu không đồng hồ đếm giờ
    và số dư trên máy đứng im. Đây là phần dễ làm hỏng nhất khi bịt lỗ rò.

    Quan trọng: phải nối WebSocket bằng token HỘI VIÊN. Hub xếp loại client theo
    role_id trong token (hub.go), nên nối bằng token nhân viên sẽ thành client
    "admin" và phép kiểm đo nhầm chính nó.
    """
    area = "25. Ai được nhận sự kiện"

    code_a, code_b = "KT-01", "KT-02"
    mach_a = sql(f"select id from machines where machine_code = '{code_a}'")

    sql("update machines set status = 'available' where deleted_at is null")
    sql("delete from machine_sessions where member_id in "
        "(select id from members where username in ('kt_ws_a', 'kt_ws_b'))")
    sql("delete from members where username in ('kt_ws_a', 'kt_ws_b')")

    def make_member(username: str, full_name: str) -> str:
        st, res = call("POST", "/api/members",
                       {"username": username, "password": "test1234", "full_name": full_name}, t)
        member_id = (data_of(res) or {}).get("id", "")
        if member_id:
            sql(f"update members set balance = 100000 where id = '{member_id}'")
        return member_id

    mid_a = make_member("kt_ws_a", "Nguoi Choi A")
    mid_b = make_member("kt_ws_b", "Nguoi Choi B")
    if not (mid_a and mid_b):
        report.add(area, "tạo hai hội viên để thử", BROKEN, "không tạo được")
        return

    def member_token(username: str, code: str) -> tuple[str, str]:
        st, res = call("POST", "/api/auth/member-login",
                       {"username": username, "password": "test1234", "machine_code": code})
        d = data_of(res) or {}
        return d.get("access_token", ""), d.get("session_id", "")

    tok_b, _ = member_token("kt_ws_b", code_b)
    tok_a, sid_a = member_token("kt_ws_a", code_a)
    if not (tok_a and tok_b and sid_a):
        report.add(area, "hai hội viên đăng nhập trên hai máy", BROKEN,
                   f"a={bool(tok_a)} b={bool(tok_b)} phiên_a={bool(sid_a)}")
        return

    try:
        watcher_a = FakeMachine(code_a, tok_a)   # máy đang có phiên
        watcher_b = FakeMachine(code_b, tok_b)   # máy của người KHÁC
    except Exception as e:  # noqa: BLE001
        report.add(area, "hai máy trạm kết nối WS", BROKEN, str(e)[:70])
        return
    time.sleep(0.5)

    def collect(fake, want_type: str, timeout: float = 3.0):
        deadline = time.time() + timeout
        while time.time() < deadline:
            evt = fake.recv(timeout=max(0.2, deadline - time.time()))
            if evt is None:
                return None
            if evt.get("type") == want_type:
                return evt
        return None

    # --- kết thúc phiên: ai nghe được? ---------------------------------------
    call("POST", f"/api/sessions/{sid_a}/end", token=t)
    got_a = collect(watcher_a, "session:ended")
    got_b = collect(watcher_b, "session:ended")
    expect(area, "máy đang chơi VẪN nhận session:ended của chính nó", got_a is not None,
           detail_bad="máy trạm không biết phiên đã kết thúc")
    expect(area, "máy của người khác KHÔNG nhận session:ended", got_b is None,
           detail_bad="total_cost của khách này lộ sang máy khách kia")

    # --- mở phiên: ai nghe được? ---------------------------------------------
    sql("update machines set status = 'available' where machine_code = '" + code_a + "'")
    st, res = call("POST", "/api/sessions/start", {"machine_id": mach_a, "member_id": mid_a}, t)
    sid_a = (data_of(res) or {}).get("id", "")
    if not sid_a:
        report.add(area, "mở lại phiên để thử", BROKEN, f"{st} {res.get('message','')[:50]}")
        watcher_a.close(); watcher_b.close()
        return

    got_a = collect(watcher_a, "session:started")
    got_b = collect(watcher_b, "session:started")
    expect(area, "máy đang chơi VẪN nhận session:started của chính nó", got_a is not None,
           detail_bad="máy trạm không nhận được — đồng hồ đếm giờ trên máy sẽ đứng im")
    expect(area, "máy của người khác KHÔNG nhận session:started", got_b is None,
           detail_bad=f"nhận được {json.dumps((got_b or {}).get('data', {}), ensure_ascii=False)[:80]}"
                      " — tên và giờ chơi của khách này lộ sang máy khách kia")
    if got_a:
        expect(area, "sự kiện gửi đúng máy vẫn kèm tên hội viên",
               (got_a.get("data") or {}).get("member_name") == "Nguoi Choi A",
               detail_bad=f"member_name={(got_a.get('data') or {}).get('member_name')!r}")

    # --- đơn hàng là việc của quầy, máy trạm không cần biết ------------------
    st, res = call("POST", "/api/orders/topup-request",
                   {"amount": 50000, "member_id": mid_a, "machine_code": code_a}, tok_a)
    expect(area, "gửi được yêu cầu nạp tiền từ máy trạm", st in (200, 201),
           detail_bad=f"HTTP {st} — {res.get('message', '')[:70]}")
    leak = collect(watcher_b, "order:new", timeout=2.0)
    expect(area, "máy trạm KHÔNG nhận order:new", leak is None,
           detail_bad="đơn hàng của quán lộ xuống máy khách")

    watcher_a.close()
    watcher_b.close()
    call("POST", f"/api/sessions/{sid_a}/end", token=t)
    sql("update machines set status = 'available' where deleted_at is null")
    sql(f"delete from machine_sessions where member_id in ('{mid_a}', '{mid_b}')")
    sql(f"delete from orders where member_id = '{mid_a}'")
    sql(f"delete from members where id in ('{mid_a}', '{mid_b}')")


def flow_client_login_gate(t: str, ids: dict) -> None:
    """/auth/member-login là màn hình khoá mà KHÁCH tự gõ mật khẩu vào — cửa vào
    máy thật sự. Trước đây nó chỉ kiểm mật khẩu: machine_code được máy khách gửi
    lên rồi bị bỏ qua hoàn toàn, số dư không ai đọc, giới nghiêm không ai hỏi.
    Cài máy khách với mã máy bịa ra cũng vào được, và khách số dư 0 vẫn ngồi máy
    mà không có phiên nào tính tiền.
    """
    area = "23. Cửa đăng nhập máy khách"

    machine = "KT-01"
    sql(f"update machines set status = 'available' where machine_code = '{machine}'")
    sql("delete from machine_sessions where member_id in "
        "(select id from members where username = 'kt_cua_may')")
    sql("delete from members where username = 'kt_cua_may'")

    st, res = call("POST", "/api/members",
                   {"username": "kt_cua_may", "password": "test1234", "full_name": "KT cua may"}, t)
    mid = (data_of(res) or {}).get("id", "")
    if not mid:
        report.add(area, "tạo hội viên để thử", BROKEN, str(st))
        return
    sql(f"update members set balance = 0, bonus_balance = 0 where id = '{mid}'")

    login = lambda code: call("POST", "/api/auth/member-login",
                              {"username": "kt_cua_may", "password": "test1234", "machine_code": code})

    # --- máy chưa có trong danh sách -----------------------------------------
    st, res = login("KHONG-CO-MAY-NAY")
    expect(area, "mã máy không có trong danh sách bị từ chối", st >= 400,
           res.get("message", "")[:70],
           f"HTTP {st} — máy lạ vẫn vào được, mọi phiên trên đó là doanh thu không thu được")

    st, res = login("")
    expect(area, "không khai mã máy thì bị từ chối", st >= 400,
           res.get("message", "")[:70], f"HTTP {st}")

    # --- số dư 0 -------------------------------------------------------------
    st, res = login(machine)
    expect(area, "số dư 0 không đăng nhập được vào máy", st >= 400,
           res.get("message", "")[:70],
           f"HTTP {st} — khách hết tiền vẫn ngồi máy mà không phiên nào tính tiền")

    # --- máy bị khoá ---------------------------------------------------------
    sql(f"update machines set is_active = false where machine_code = '{machine}'")
    st, res = login(machine)
    expect(area, "máy bị khoá thì không đăng nhập được", st >= 400,
           res.get("message", "")[:70], f"HTTP {st}")
    sql(f"update machines set is_active = true where machine_code = '{machine}'")

    # --- nạp tiền rồi thì vào được, VÀ phải mở luôn phiên --------------------
    sql(f"update members set balance = 50000 where id = '{mid}'")
    st, res = login(machine)
    sid = (data_of(res) or {}).get("session_id", "")
    expect(area, "nạp tiền xong thì đăng nhập được", st == 200,
           detail_bad=f"HTTP {st} — {res.get('message', '')[:70]}")
    expect(area, "đăng nhập mở luôn phiên tính tiền", bool(sid), sid[:8],
           "không có session_id — khách ngồi máy mà không gì tính tiền")
    expect(area, "máy chuyển sang trạng thái đang dùng",
           sql(f"select status from machines where machine_code = '{machine}'") == "in_use",
           detail_bad="máy vẫn 'available' — trang quản trị không biết ai đang ngồi")

    # --- đang giữa phiên thì vào lại được dù hết tiền ------------------------
    # Máy khách khởi động lại giữa chừng phải vào lại được: tiền đã trả rồi,
    # chặn ở đây là nhốt khách ngoài chính máy họ đang thuê. Và phải trả về ĐÚNG
    # phiên cũ, không mở phiên thứ hai tính tiền chồng lên.
    sql(f"update members set balance = 0, bonus_balance = 0 where id = '{mid}'")
    st, res = login(machine)
    sid2 = (data_of(res) or {}).get("session_id", "")
    expect(area, "đang giữa phiên thì vào lại được dù hết tiền", st == 200,
           detail_bad=f"HTTP {st} — {res.get('message', '')[:70]}")
    expect(area, "vào lại trả đúng phiên cũ, không mở phiên thứ hai", sid2 == sid,
           detail_bad=f"{sid2[:8]} khác {sid[:8]} — tính tiền hai lần trên một máy")
    expect(area, "chỉ có đúng một phiên đang chạy",
           sql(f"select count(*) from machine_sessions where member_id = '{mid}' and is_active = true") == "1",
           detail_bad="có nhiều hơn một phiên đang chạy cho cùng một khách")

    if sid:
        call("POST", f"/api/sessions/{sid}/end", token=t)
    sql(f"update machines set status = 'available' where machine_code = '{machine}'")
    sql(f"delete from machine_sessions where member_id = '{mid}'")
    sql(f"delete from members where id = '{mid}'")


def flow_date_formats(t: str, ids: dict) -> None:
    """Ô ngày của Element Plus sinh ra "2000-01-01", còn ô bỏ trống gửi "".
    encoding/json không đọc được dạng nào trong hai dạng đó vào *time.Time, nên
    hộp thoại Thêm hội viên từng hỏng theo CẢ HAI hướng — bỏ trống cũng lỗi, mà
    chọn ngày cũng lỗi, và cùng trả về một câu "Dữ liệu không hợp lệ".
    """
    area = "22. Định dạng ngày từ giao diện"

    cases = [
        ("bỏ trống ngày sinh", "", True, None),
        ("ngày sinh dạng ElDatePicker (YYYY-MM-DD)", "2000-01-01", True, "2000-01-01"),
        ("ngày sinh dạng RFC3339", "2000-01-01T00:00:00+07:00", True, "2000-01-01"),
        ("chuỗi rác bị từ chối", "hôm qua", False, None),
    ]

    for i, (ten, dob, nen_thanh_cong, ngay) in enumerate(cases):
        body = {
            "username": f"kt_ngay_{i}",
            "password": "test1234",
            "full_name": "KT ngay",
            # Giao diện gửi chuỗi rỗng cho MỌI ô không điền, không bỏ khoá đi.
            "phone": "", "email": "", "group_id": "",
            "id_card_number": "", "id_card_image_url": "",
            "parent_consent_file_url": "", "notes": "",
            "date_of_birth": dob,
        }
        st, res = call("POST", "/api/members", body, t)
        ok = (st in (200, 201)) if nen_thanh_cong else (st == 400)
        expect(area, ten, ok, f"HTTP {st}",
               f"HTTP {st} — {res.get('message', '')[:90]}")

        if nen_thanh_cong and st in (200, 201) and ngay:
            got = (data_of(res) or {}).get("date_of_birth") or ""
            expect(area, f"{ten}: lưu đúng ngày", got[:10] == ngay,
                   detail_bad=f"lưu {got!r}, mong {ngay} — lệch múi giờ làm phép kiểm vị thành niên sai một ngày")

    # Thông báo lỗi phải nói được trường nào sai, nếu không lần sửa sau lại mò.
    st, res = call("POST", "/api/members",
                   {"username": "kt_ngay_x", "password": "test1234", "date_of_birth": "hôm qua"}, t)
    msg = res.get("message", "")
    expect(area, "thông báo lỗi nêu được giá trị sai", "hôm qua" in msg,
           msg[:70], f"chỉ nhận {msg!r}")

    for i in range(len(cases)):
        sql(f"delete from members where username = 'kt_ngay_{i}'")
    sql("delete from members where username = 'kt_ngay_x'")


def flow_backup_delete(t: str, ids: dict) -> None:
    """Trước đây nút Xoá của trang Sao lưu bị vô hiệu hoá kèm chú thích "sắp có",
    và backend cũng không có endpoint nào. Mỗi lần bấm Tạo sao lưu là thêm một
    tệp nằm lại vĩnh viễn trên đĩa máy chủ.
    """
    area = "21. Xoá bản sao lưu"

    st, res = call("POST", "/api/backups", {"notes": "kiem thu xoa"}, t)
    bid = (data_of(res) or {}).get("id", "")
    if not bid:
        report.add(area, "tạo bản sao lưu để xoá", BROKEN, str(st))
        return

    # Đang chạy thì phải TỪ CHỐI: xoá lúc này để lại một tệp cụt không ai trỏ tới.
    st, res = call("DELETE", f"/api/backups/{bid}", token=t)
    if st == 400:
        report.add(area, "từ chối xoá bản sao lưu đang chạy", OK, res.get("message", "")[:60])
    # pg_dump chạy nền — chờ chốt trạng thái rồi mới xoá thật.
    path = ""
    for _ in range(30):
        time.sleep(1)
        row = sql(f"select status || '|' || coalesce(file_path, '') from backup_logs where id = '{bid}'")
        if row and not row.startswith("running"):
            _, _, path = row.partition("|")
            break
    if not path:
        report.add(area, "bản sao lưu hoàn tất", BROKEN, "hết giờ chờ")
        return

    on_disk = file_exists_in_app(path)
    expect(area, "tệp .sql có thật trên đĩa trước khi xoá", on_disk,
           detail_bad=f"không thấy {path}")

    st, res = call("DELETE", f"/api/backups/{bid}", token=t)
    expect(area, "xoá được bản sao lưu", st == 200,
           detail_bad=f"HTTP {st} — {res.get('message', '')[:80]}")

    left = sql(f"select count(*) from backup_logs where id = '{bid}'")
    expect(area, "dòng nhật ký biến mất", left == "0", detail_bad=f"còn {left} dòng")

    still = file_exists_in_app(path)
    expect(area, "tệp .sql trên đĩa cũng bị xoá", not still,
           detail_bad=f"{path} vẫn còn — đĩa máy chủ đầy dần mà không ai dọn được")


def phase_restore(t: str, ids: dict) -> None:
    """pg_restore --clean xoá và dựng lại TOÀN BỘ bảng, nên đây phải là giai
    đoạn cuối: mọi thứ chạy sau nó đều đọc phải database vừa bị ghi đè.

    Bản cũ truyền đường dẫn tệp vào --dbname, tức là lấy tên tệp làm tên
    database — lệnh đó không thể nào chạy đúng. Sửa rồi nhưng chưa ai chạy thật,
    nên tới giờ vẫn chỉ là niềm tin.
    """
    area = "19. Phục hồi sao lưu"

    marker = "KT-RESTORE-MARK"
    st, res = call("POST", "/api/machines", {"machine_code": marker}, t)
    mid = (data_of(res) or {}).get("id", "")
    if not mid:
        report.add(area, "tạo dấu mốc trước khi sao lưu", BROKEN,
                   f"{st} {res.get('message', '')[:60]}")
        return
    report.add(area, "tạo dấu mốc trước khi sao lưu", OK, marker)

    st, res = call("POST", "/api/backups", {"notes": "kiem thu phuc hoi"}, t)
    bid = (data_of(res) or {}).get("id", "")
    if not bid:
        report.add(area, "tạo bản sao lưu để phục hồi", BROKEN, f"{st}")
        return

    # pg_dump chạy nền; chờ tới khi trạng thái chốt lại.
    status, path = "", ""
    for _ in range(30):
        time.sleep(1)
        row = sql(f"select status || '|' || coalesce(file_path, '') from backup_logs where id = '{bid}'")
        if row and not row.startswith("running"):
            status, _, path = row.partition("|")
            break
    if status != "completed":
        report.add(area, "bản sao lưu hoàn tất", BROKEN, f"trạng thái {status!r}")
        return
    report.add(area, "bản sao lưu hoàn tất", OK, path)

    # Xoá HẲN dấu mốc (không phải xoá mềm): phục hồi phải mang nó về.
    sql(f"delete from machines where id = '{mid}'")
    gone = sql(f"select count(*) from machines where machine_code = '{marker}'")
    if gone != "0":
        report.add(area, "xoá dấu mốc trước khi phục hồi", BROKEN, f"còn {gone} dòng")
        return
    report.add(area, "xoá dấu mốc trước khi phục hồi", OK)

    st, res = call("POST", f"/api/backups/{bid}/restore", token=t, timeout=180)
    expect(area, "lệnh phục hồi chạy được", st == 200,
           detail_bad=f"HTTP {st} — {res.get('message', '')[:200]}")

    back = sql(f"select count(*) from machines where machine_code = '{marker}'")
    expect(area, "dấu mốc quay lại sau khi phục hồi", back == "1",
           detail_bad=f"{back} dòng — dữ liệu KHÔNG được phục hồi")

    # Phục hồi mà làm hỏng cấu trúc thì API sau đó phải chết; gọi thử một
    # endpoint đọc để chắc database còn dùng được.
    st, _ = call("GET", "/api/machines?page_size=1", token=t)
    expect(area, "hệ thống còn chạy được sau khi phục hồi", st == 200,
           detail_bad=f"GET /api/machines trả {st}")

    sql(f"delete from machines where machine_code = '{marker}'")


# --------------------------------------------------------------------------- #

def main() -> int:
    print(f"\nKiểm chứng VNET Core — {BASE}\n" + "=" * 78)

    # Kịch bản này giả định database SẠCH. Chạy lại trên dữ liệu của lần trước
    # sẽ cho kết quả sai lệch — mã trùng, kho đã cạn, máy đã ở trạng thái khác —
    # và những mục HỎNG đó không phản ánh lỗi thật của sản phẩm.
    leftover = sql("select count(*) from machines where machine_code like 'KT-%'")
    if leftover not in ("", "0"):
        print("\n  ⚠  Database đã có dữ liệu của lần chạy trước "
              f"({leftover} máy KT-*).")
        print("     Kết quả sẽ không đáng tin. Hãy dựng lại database sạch:")
        print("     docker compose -p vnetverify down -v && ... up -d && migrate && seed\n")

    tokens = phase_auth()
    if "admin" not in tokens:
        print("\nKhông đăng nhập được bằng tài khoản admin — dừng.")
        return 1
    t = tokens["admin"]

    ids = phase_setup(t)
    phase_zero_values(t, ids)
    flow_session(t, ids)
    flow_pricing(t, ids)
    flow_orders(t, ids)
    flow_promotions(t, ids)
    flow_lucky_spin(t, ids)
    flow_remote_control(t, ids)
    flow_printing(t, ids)
    flow_cards(t, ids)
    flow_inventory_count(t, ids)
    flow_attendance(t, ids)
    flow_website_block(t, ids)
    flow_app_update(t, ids)
    flow_feedback(t, ids)
    flow_combo_booking_shift(t, ids)
    flow_curfew(t, ids)
    flow_roles_users(t, ids)
    phase_permissions(tokens, ids)
    flow_settings_enforced(t, ids)
    phase_remaining(t, ids)
    phase_scheduler(t, ids)
    flow_dead_columns(t, ids)
    flow_ui_contract(t, ids)
    flow_backup_delete(t, ids)
    flow_date_formats(t, ids)
    flow_client_login_gate(t, ids)
    flow_remote_shutdown_ends_session(t, ids)
    flow_per_minute_billing(t, ids)
    flow_agent_ws_and_features(t, ids)
    flow_ws_audience(t, ids)
    flow_menu_contract(t, ids)
    flow_one_login_box(t, ids)
    flow_reboot_closes_session(t, ids)
    flow_offline_closes_session(t, ids)
    flow_telemetry(t, ids)
    flow_hardware_retention(t, ids)
    phase_cleanup(t, ids)
    phase_restore(t, ids)

    c = report.counts()
    print("\n" + "=" * 78)
    print(f"Tổng: {len(report.results)} mục — "
          f"{c[OK]} đạt · {c[WRONG]} sai · {c[BROKEN]} hỏng · {c[SKIP]} bỏ qua")

    print("\n  Phần Windows (khoá phím, in nhiệt, tệp hosts, cài cập nhật, chụp màn hình)")
    print("  không kiểm được từ đây — xem scripts/KIEM-CHUNG-WINDOWS.md")

    for status, title in ((BROKEN, "HỎNG"), (WRONG, "SAI")):
        rows = [r for r in report.results if r.status == status]
        if rows:
            print(f"\n{title}:")
            for r in rows:
                print(f"  · [{r.area}] {r.name}" + (f" — {r.detail}" if r.detail else ""))

    with open("verify-results.json", "w") as f:
        json.dump([r.__dict__ for r in report.results], f, ensure_ascii=False, indent=2)
    print("\nChi tiết đã ghi vào verify-results.json")

    return 1 if c[BROKEN] else 0


if __name__ == "__main__":
    sys.exit(main())
