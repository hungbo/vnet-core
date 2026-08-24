#!/usr/bin/env python3
"""Dò lỗi giao diện admin mà bản build vẫn xanh — chạy từ thư mục gốc của core/.

Bốn phép dò, mỗi phép bắt một lớp lỗi khác nhau:

  1. Khoá `vnetPages.*` được dùng trong `views/vnet/**` nhưng thiếu ở một bảng
     ngôn ngữ  → trên màn hình hiện nguyên chuỗi khoá.
  2. Khoá trong bảng tiếng Việt còn nguyên chữ tiếng Anh  → dịch sót.
  3. Thư mục trong `views/vnet/` không nằm trong nhóm nào ở
     `store/modules/route/shared.ts`  → trang rơi xuống nhánh `ungrouped` và
     nằm phẳng ở cấp cao nhất của menu.
  4. Nút Thêm / Xoá hàng loạt của `<TableHeaderOperation>` không nghe `@add` /
     `@delete` và cũng không bị tắt bằng `:show-add` / `:show-delete`  → nút
     hiện trên màn hình nhưng bấm vào không có gì xảy ra.

Mã thoát: 0 nếu sạch, 1 nếu có phát hiện.
"""

from __future__ import annotations

import os
import re
import sys
import unicodedata

ADMIN = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "admin")
LANGS = ("vi-vn", "en-us", "zh-cn")

# Thuật ngữ giữ nguyên trong tiếng Việt — không tính là dịch sót.
KEEP_AS_IS = {
    "email", "combo", "wifi", "id", "ip", "logo", "menu", "vip", "game",
    "website", "admin", "server", "app", "cpu", "gpu", "ram", "ram (gb)",
    "serial", "model", "laser", "inkjet", "vnet", "tab", "plugin",
}


def flatten(path: str) -> dict[str, str]:
    """Bộ đọc thô cho tệp locale thuần literal — đủ dùng, không cần parser JS."""
    out: dict[str, str] = {}
    stack: list[str] = []
    # Prettier xuống dòng những chuỗi dài thành `khoá:` rồi `'giá trị'` ở dòng
    # sau. Bản đọc cũ chỉ khớp một dòng nên báo THIẾU cho hàng chục khoá vẫn có
    # thật — dương tính giả kiểu này làm cả bộ dò mất tin cậy.
    pending_key: str | None = None
    for line in open(path, encoding="utf-8").read().splitlines():
        s = line.strip()

        if pending_key is not None:
            m = re.match(r"^'(.*)',?$|^\"(.*)\",?$", s)
            if m:
                out[pending_key] = m.group(1) if m.group(1) is not None else m.group(2)
            pending_key = None
            if m:
                continue

        m = re.match(r"^([A-Za-z_$][\w$]*|'[^']+')\s*:\s*\{\s*$", s)
        if m:
            stack.append(m.group(1).strip("'"))
            continue
        if s in ("},", "}", "};"):
            if stack:
                stack.pop()
            continue
        m = re.match(r"^([A-Za-z_$][\w$]*|'[^']+')\s*:\s*(?:'(.*)'|\"(.*)\"),?$", s)
        if m:
            val = m.group(2) if m.group(2) is not None else m.group(3)
            out[".".join(stack + [m.group(1).strip("'")])] = val
            continue
        m = re.match(r"^([A-Za-z_$][\w$]*|'[^']+')\s*:$", s)
        if m:
            pending_key = ".".join(stack + [m.group(1).strip("'")])
    return out


def has_vietnamese(s: str) -> bool:
    return any(ord(c) > 127 for c in s)


def dem_node_goc(tpl: str) -> int:
    """Đếm số node gốc của một <template>. Bỏ qua chú thích và khoảng trắng."""
    void = {"br", "hr", "img", "input", "meta", "link", "source"}
    depth = nodes = i = 0
    n = len(tpl)
    while i < n:
        if tpl[i] == "<":
            if tpl.startswith("<!--", i):
                j = tpl.find("-->", i)
                i = (j + 3) if j > 0 else n
                continue
            m = re.match(r"</\s*([A-Za-z][-\w.]*)\s*>", tpl[i:])
            if m:
                depth -= 1
                i += m.end()
                continue
            m = re.match(r"<\s*([A-Za-z][-\w.]*)", tpl[i:])
            if m:
                j, quote = i, None
                while j < n:
                    ch = tpl[j]
                    if quote:
                        if ch == quote:
                            quote = None
                    elif ch in "\"'":
                        quote = ch
                    elif ch == ">":
                        break
                    j += 1
                if depth == 0:
                    nodes += 1
                if not (tpl[j - 1] == "/" or m.group(1).lower() in void):
                    depth += 1
                i = j + 1
                continue
            i += 1
            continue
        if depth == 0 and tpl[i].strip():
            nodes += 1
        j = tpl.find("<", i)
        i = j if j > 0 else n
    return nodes


def main() -> int:
    tables = {lg: flatten(os.path.join(ADMIN, "src/locales/langs", f"{lg}.ts")) for lg in LANGS}
    problems: list[str] = []

    # --- 1. khoá được dùng nhưng thiếu bản dịch ------------------------------
    used: set[str] = set()
    views = os.path.join(ADMIN, "src/views/vnet")
    for root, _dirs, files in os.walk(views):
        for f in files:
            if not f.endswith(".vue"):
                continue
            src = open(os.path.join(root, f), encoding="utf-8").read()
            used |= set(re.findall(r"\$t\(\s*'(vnetPages\.[\w.]+)'", src))
            used |= set(re.findall(r"\$t\(\s*`(vnetPages\.[\w.]+)\.\$\{", src))  # khoá ghép: chỉ lấy tiền tố

    for key in sorted(used):
        for lg in LANGS:
            table = tables[lg]
            # Khoá ghép động (`...days.${d}`) chỉ kiểm tiền tố có tồn tại nhánh con.
            if key in table or any(k.startswith(key + ".") for k in table):
                continue
            problems.append(f"[thiếu bản dịch] {lg}: {key}")

    # --- 2. tiếng Việt còn nguyên chữ tiếng Anh ------------------------------
    vi, en = tables["vi-vn"], tables["en-us"]
    for key, value in sorted(vi.items()):
        if not key.startswith("vnetPages."):
            continue
        if en.get(key) != value or not value or len(value) <= 2 or value.isdigit():
            continue
        if has_vietnamese(value) or value.lower() in KEEP_AS_IS:
            continue
        problems.append(f"[chưa dịch] vnetPages: {key} = {value!r}")

    # --- 3. trang không thuộc nhóm menu nào ----------------------------------
    shared = open(os.path.join(ADMIN, "src/store/modules/route/shared.ts"), encoding="utf-8").read()
    # Chỉ đọc mảng `groups`: iconMap phía trên cũng chứa khoá vnet_* nhưng có
    # biểu tượng không có nghĩa là đã được xếp vào nhóm nào.
    block = shared[shared.index("const groups = ["):]
    grouped = set(re.findall(r"'vnet_([\w-]+)'", block))
    for entry in sorted(os.listdir(views)):
        # Chỉ tính thư mục thật sự là một trang: elegant-router chỉ sinh route
        # cho thư mục có index.vue, thư mục rỗng không xuất hiện trong menu.
        if not os.path.isfile(os.path.join(views, entry, "index.vue")):
            continue
        if entry not in grouped:
            problems.append(f"[ngoài nhóm menu] views/vnet/{entry} — sẽ rơi xuống nhánh ungrouped")

    # --- 4. nút Thêm / Xoá hàng loạt câm --------------------------------------
    for entry in sorted(os.listdir(views)):
        f = os.path.join(views, entry, "index.vue")
        if not os.path.isfile(f):
            continue
        src = open(f, encoding="utf-8").read()
        m = re.search(r"<TableHeaderOperation\b", src)
        if not m:
            continue
        gt = src.index(">", m.start())
        if src[gt - 1] == "/":
            block = src[m.start():gt + 1]
        else:
            block = src[m.start():src.index("</TableHeaderOperation>", gt)]
        # Trang ghi đè slot #default thì tự dựng nút của mình, không xét.
        if re.search(r"<template\s+#default", block):
            continue
        if "@add" not in block and ':show-add="false"' not in block:
            problems.append(f"[nút câm] views/vnet/{entry}: nút Thêm không nghe @add")
        if "@delete" not in block and ':show-delete="false"' not in block:
            problems.append(f"[nút câm] views/vnet/{entry}: nút Xoá hàng loạt không nghe @delete")

    # --- 5. trang có nhiều node gốc ------------------------------------------
    # GlobalContent bọc mỗi trang trong <Transition mode="out-in">. Vue chỉ gắn
    # được hiệu ứng vào MỘT node gốc là phần tử; trang có fragment root khiến
    # Transition không nhận hook enter, cờ isLeaving kẹt ở true, và từ lần
    # chuyển trang THỨ HAI trở đi mọi trang đều trắng — không lỗi, không cảnh
    # báo ở bản dựng production. Đây là cách hai hộp thoại đặt nhầm ra ngoài thẻ
    # <div> gốc đã làm hỏng toàn bộ điều hướng.
    for root, _dirs, files in os.walk(views):
        for f in sorted(files):
            if not f.endswith(".vue"):
                continue
            path = os.path.join(root, f)
            m = re.search(r"<template>\n(.*?)\n</template>", open(path, encoding="utf-8").read(), re.S)
            if not m:
                continue
            count = dem_node_goc(m.group(1))
            if count != 1:
                rel = os.path.relpath(path, ADMIN)
                problems.append(f"[nhiều node gốc] {rel}: {count} node gốc — phải gói trong đúng một phần tử")

    for p in problems:
        print(p)
    print(f"\n{len(problems)} phát hiện")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
