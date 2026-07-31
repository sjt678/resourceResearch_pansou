#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
检查 docker-compose.yml 中 CHANNELS 列表里的 TG 频道是否还存活，
并清理掉明确死掉 / 私有的频道（保守策略：超时 / 限流等不确定的一律保留）。

判定逻辑（基于 https://t.me/s/{channel} 公开网页预览接口的响应）：
  - 最终 URL 跳到 telegram.org 首页        -> DEAD   （频道不存在）
  - HTTP 404 / 410                          -> DEAD
  - 最终停在 t.me/{channel} 但 body 含
    "private" / "not available" 等文案      -> PRIVATE（无法抓取，等同死）
  - 200 且 body 含频道名 / post 标记         -> ALIVE
  - 超时 / 连接错误 / 429 / 5xx             -> UNKNOWN（保留，避免误删好频道）

抓取走 curl 子进程（比 urllib 更兼容 Clash/Mihomo 这类 HTTP 代理的 HTTPS 隧道）。

增量复用：若上次已生成报告，已确认 ALIVE/DEAD/PRIVATE 的直接复用，
          只重测上次的 UNKNOWN，重跑很快。

用法：
  python3 tools/check_tg_channels.py                 # 检查并直接清理写入 compose
  python3 tools/check_tg_channels.py --dry-run       # 只打印，不改文件
  TG_PROXY=http://127.0.0.1:7897 \\
  python3 tools/check_tg_channels.py                 # 自定义代理
"""
import os
import re
import sys
import time
import argparse
import subprocess
import tempfile

COMPOSE = "pansou-web/docker-compose.yml"
REPORT = "tools/tg_channel_report.txt"
PROXY = os.environ.get("TG_PROXY", "http://127.0.0.1:7897")
UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

# 私有 / 不可用的文案特征（小写匹配）
PRIVATE_MARKERS = [
    "this channel is private",
    "channel is not available",
    "this channel is not available",
    "you don't have permission",
    "you do not have permission",
    "this channel is not",
]
# 存活的 post 标记
ALIVE_MARKERS = ["tgme_widget", "channel_post", "data-post", "tgme_channel"]


def load_channels():
    s = open(COMPOSE, encoding="utf-8").read()
    m = re.search(r"CHANNELS=([^\n]+)", s)
    if not m:
        sys.exit("ERROR: 在 %s 中找不到 CHANNELS=" % COMPOSE)
    return [c.strip() for c in m.group(1).split(",") if c.strip()]


def load_known():
    """读取上次报告，复用已确认的结果（ALIVE/DEAD/PRIVATE）。"""
    known = {}
    if os.path.exists(REPORT):
        for line in open(REPORT, encoding="utf-8"):
            m = re.match(r"^(ALIVE|DEAD|PRIVATE|UNKNOWN)\s+(\S+)\s*(.*)$", line)
            if m:
                known[m.group(2)] = (m.group(1), m.group(3).strip())
    return known


def fetch(url):
    """用 curl 走代理抓取，返回 (final_url, body, status_code, error)。"""
    fd, tmp = tempfile.mkstemp(suffix=".html")
    os.close(fd)
    try:
        proc = subprocess.run(
            ["curl", "-sL", "--retry", "2", "--retry-delay", "1", "--max-time", "25",
             "-A", UA, "-x", PROXY,
             "-o", tmp, "-w", "%{http_code} %{url_effective}", url],
            capture_output=True, text=True, timeout=45)
        out = proc.stdout.strip()
        parts = out.split(" ", 1)
        status = int(parts[0]) if parts and parts[0].isdigit() else 0
        final = parts[1] if len(parts) > 1 else url
        with open(tmp, encoding="utf-8", errors="ignore") as f:
            body = f.read()
        err = None if proc.returncode == 0 else (proc.stderr.strip() or "curl rc=%d" % proc.returncode)
        return final, body, status, err
    except Exception as e:
        return url, "", 0, str(e)
    finally:
        try:
            os.remove(tmp)
        except Exception:
            pass


def classify(channel):
    url = "https://t.me/s/%s" % channel
    last_err = None
    for attempt in range(2):
        final, body, status, err = fetch(url)
        low = body.lower()
        if err:
            last_err = err
            time.sleep(1.0)
            continue
        if "telegram.org" in final and "t.me" not in final:
            return "DEAD", "redirect -> telegram.org (不存在)"
        if status == 404 or status == 410:
            return "DEAD", "http %d" % status
        if status == 429 or status >= 500:
            last_err = "http %d" % status
            time.sleep(1.5)
            continue
        if status == 200:
            if any(mk in low for mk in PRIVATE_MARKERS):
                return "PRIVATE", "私有/不可用频道"
            if channel.lower() in low or any(mk in low for mk in ALIVE_MARKERS):
                return "ALIVE", "http 200"
            return "UNKNOWN", "http 200 但无法确认存活"
        return "UNKNOWN", "http %d" % status
    return "UNKNOWN", "err: %s" % last_err


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true", help="只检查不写入 compose")
    ap.add_argument("--all", action="store_true", help="忽略上次报告，全部重测")
    args = ap.parse_args()

    raw = load_channels()
    seen, ordered = set(), []
    for c in raw:
        if c not in seen:
            seen.add(c)
            ordered.append(c)

    known = {} if args.all else load_known()
    reused = sum(1 for c in ordered if known.get(c, (None,))[0] in ("ALIVE", "DEAD", "PRIVATE"))

    print("待检查频道数: %d（代理 %s）" % (len(ordered), PROXY))
    if reused:
        print("增量模式：复用上次 %d 个已确认结果，仅重测 %d 个未知\n" %
              (reused, len(ordered) - reused))

    results = {}
    alive = dead = priv = unk = 0
    for c in ordered:
        if known.get(c, (None,))[0] in ("ALIVE", "DEAD", "PRIVATE"):
            st, why = known[c]
            why = why or "复用上次结果"
        else:
            st, why = classify(c)
            time.sleep(0.3)
        results[c] = (st, why)
        if st == "ALIVE":
            alive += 1
        elif st == "DEAD":
            dead += 1
        elif st == "PRIVATE":
            priv += 1
        else:
            unk += 1
        print("  %-7s %-28s %s" % (st, c, why))

    cleaned = [c for c in ordered if results[c][0] in ("ALIVE", "UNKNOWN")]
    removed = [c for c in ordered if results[c][0] in ("DEAD", "PRIVATE")]

    print("\n========== 汇总 ==========")
    print("存活 ALIVE   : %d" % alive)
    print("死掉 DEAD    : %d" % dead)
    print("私有 PRIVATE : %d" % priv)
    print("未知 UNKNOWN : %d (已保留)" % unk)
    print("清理后剩余   : %d (原 %d)" % (len(cleaned), len(ordered)))
    if removed:
        print("\n将被移除的频道:")
        for c in removed:
            print("  - %s  (%s)" % (c, results[c][1]))

    with open(REPORT, "w", encoding="utf-8") as f:
        f.write("TG 频道存活检查报告\n")
        f.write("生成时间: %s\n" % time.strftime("%Y-%m-%d %H:%M:%S"))
        f.write("代理: %s\n" % PROXY)
        f.write("总数: %d  存活: %d  死: %d  私有: %d  未知: %d\n\n" %
                (len(ordered), alive, dead, priv, unk))
        for c in ordered:
            f.write("%-7s %-28s %s\n" % (results[c][0], c, results[c][1]))
    print("\n报告已写入 %s" % REPORT)

    if args.dry_run:
        print("\n[DRY-RUN] 未修改 %s" % COMPOSE)
        return

    if not removed:
        print("\n没有需要清理的频道，compose 保持不变。")
        return

    s = open(COMPOSE, encoding="utf-8").read()
    new_line = "CHANNELS=" + ",".join(cleaned)
    s2 = re.sub(r"CHANNELS=[^\n]+", new_line, s, count=1)
    open(COMPOSE, "w", encoding="utf-8").write(s2)
    print("\n已更新 %s：%d -> %d 个频道" % (COMPOSE, len(ordered), len(cleaned)))


if __name__ == "__main__":
    main()
