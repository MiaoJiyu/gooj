#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
将 Gooj 的题目批量转换为 HydroOJ (Hydro) 支持的导入 zip 格式。

Gooj 题目存储在:
  - data/problem/<id>/config.json   评测配置 (memory_limit MB, time_limit ms, test_cases 分组)
  - data/problem/<id>/<n>.in, <n>.ans  测试数据
  - data/problem/<id>/statement.md     题面 (通常只有标题)
  - data/app.db 的 problems 表         标题 / 完整题面 / 时空间限制

Hydro 导入格式 (hydro-compress) 每个题目一个顶层目录 <id>/:
  <id>/problem.yaml         pid / title / tag / nSubmit / nAccept
  <id>/problem_zh.md        题面 (markdown)
  <id>/testdata/config.yaml type / time / memory / subtasks
  <id>/testdata/<n>.in
  <id>/testdata/<n>.out     (Gooj 的 .ans 重命名为 .out)
  <id>/solution/<name>.md   (可选, 本脚本不生成)

用法:
  python3 convert_to_hydro.py
  python3 convert_to_hydro.py --src data/problem --db data/app.db --out data/HydroExport.zip
  python3 convert_to_hydro.py --per-problem --out-dir data/HydroProblems
"""

import os
import sys
import json
import shutil
import sqlite3
import argparse
import tempfile
import zipfile


# --------------------------------------------------------------------------- #
# 极简 YAML 写出 (不依赖 PyYAML)
# --------------------------------------------------------------------------- #
def _y(v):
    """把标量写成 YAML 字面量 (字符串一律双引号转义, 最稳妥)。"""
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return str(v)
    s = str(v).replace("\\", "\\\\").replace('"', '\\"')
    return '"' + s + '"'


def build_problem_yaml(pid, title, tags, n_submit, n_accept):
    out = []
    out.append("pid: " + _y(str(pid)))
    out.append("owner: 1")
    out.append("title: " + _y(title if title else str(pid)))
    if tags:
        out.append("tag:")
        for t in tags:
            out.append("  - " + _y(t))
    else:
        out.append("tag: []")
    if n_submit is not None:
        out.append("nSubmit: " + str(n_submit))
    if n_accept is not None:
        out.append("nAccept: " + str(n_accept))
    return "\n".join(out) + "\n"


def build_testdata_config(memory_mb, time_ms, subtasks):
    """
    subtasks: [ {id, score, cases:[(in_file, out_file), ...]}, ... ]
    """
    out = []
    out.append("type: default")
    if time_ms:
        out.append("time: %dms" % time_ms)
    out.append("memory: %dMB" % memory_mb)
    out.append("subtasks:")
    for st in subtasks:
        out.append("  - score: " + str(st["score"]))
        out.append("    if: []")          # Gooj 无子任务依赖信息, 置空
        out.append("    id: " + str(st["id"]))
        out.append("    type: min")       # 子任务内全部通过才得分, 对应 Gooj 分组语义
        out.append("    cases:")
        for inp, outp in st["cases"]:
            out.append("      - input: " + _y(inp))
            out.append("        output: " + _y(outp))
    return "\n".join(out) + "\n"


# --------------------------------------------------------------------------- #
# 读取数据源
# --------------------------------------------------------------------------- #
def load_db(db_path):
    """返回 {id: dict(title, description, time_limit_ms, mem_limit_mb, nSubmit, nAccept)}。"""
    problems = {}
    if not db_path or not os.path.exists(db_path):
        return problems
    try:
        con = sqlite3.connect(db_path)
        cur = con.cursor()
        cur.execute(
            "SELECT id, title, description, time_limit_ms, mem_limit_mb "
            "FROM problems"
        )
        for pid, title, desc, tl, ml in cur.fetchall():
            problems[pid] = {
                "title": title or "",
                "description": desc or "",
                "time_limit_ms": tl or 0,
                "mem_limit_mb": ml or 0,
                "nSubmit": None,
                "nAccept": None,
            }
        # 提交统计
        try:
            cur.execute(
                "SELECT problem_id, COUNT(*), "
                "SUM(CASE WHEN status IN ('accepted','ok') THEN 1 ELSE 0 END) "
                "FROM submissions GROUP BY problem_id"
            )
            for pid, total, acc in cur.fetchall():
                if pid in problems:
                    problems[pid]["nSubmit"] = total or 0
                    problems[pid]["nAccept"] = acc or 0
        except sqlite3.Error:
            pass
        con.close()
    except sqlite3.Error as e:
        print("警告: 读取数据库失败: %s" % e, file=sys.stderr)
    return problems


def natural_key(name):
    return [int(t) if t.isdigit() else t for t in
            "".join(("1" if c.isdigit() else "0") + c for c in name).split("0")
            if t]


def load_config(src_dir):
    """读取 config.json, 失败返回 None。"""
    cfg_path = os.path.join(src_dir, "config.json")
    if os.path.exists(cfg_path):
        try:
            with open(cfg_path, "r", encoding="utf-8") as f:
                return json.load(f)
        except (ValueError, OSError) as e:
            print("警告: 解析 %s 失败: %s" % (cfg_path, e), file=sys.stderr)
    return None


# --------------------------------------------------------------------------- #
# 转换单题 -> 暂存目录
# --------------------------------------------------------------------------- #
def convert_problem(pid, src_dir, db_info, stage_dir):
    cfg = load_config(src_dir)

    # 时空间限制: 优先 DB, 回退 config.json
    mem_mb = 0
    time_ms = 0
    if db_info:
        mem_mb = db_info.get("mem_limit_mb") or 0
        time_ms = db_info.get("time_limit_ms") or 0
    if not mem_mb and cfg:
        mem_mb = cfg.get("memory_limit") or 0
    if not time_ms and cfg:
        time_ms = cfg.get("time_limit") or 0
    if not mem_mb:
        mem_mb = 256
    if not time_ms:
        time_ms = 1000

    # 题面/标题: 优先 DB
    title = (db_info or {}).get("title") or ""
    desc = (db_info or {}).get("description") or ""
    stmt_path = os.path.join(src_dir, "statement.md")
    if not desc and os.path.exists(stmt_path):
        with open(stmt_path, "r", encoding="utf-8") as f:
            desc = f.read()
    if not title:
        # 用 statement 第一行或目录名
        first = (desc.splitlines() or [str(pid)])[0].strip()
        title = first or str(pid)

    prob_dir = os.path.join(stage_dir, str(pid))
    td_dir = os.path.join(prob_dir, "testdata")
    os.makedirs(td_dir, exist_ok=True)

    # 子任务
    subtasks = []
    if cfg and cfg.get("test_cases"):
        for i, grp in enumerate(cfg["test_cases"], start=1):
            cases = []
            for n in grp.get("cases", []):
                inp = "%s.in" % n
                outp = "%s.out" % n
                s_in = os.path.join(src_dir, "%s.in" % n)
                s_out = os.path.join(src_dir, "%s.ans" % n)
                if os.path.exists(s_in):
                    shutil.copy(s_in, os.path.join(td_dir, inp))
                if os.path.exists(s_out):
                    shutil.copy(s_out, os.path.join(td_dir, outp))
                cases.append((inp, outp))
            if cases:
                subtasks.append({
                    "id": i,
                    "score": grp.get("score", 0),
                    "cases": cases,
                })
    else:
        # 回退: 自动发现 *.in, 全部归入一个子任务
        ins = sorted(
            (f for f in os.listdir(src_dir) if f.endswith(".in")),
            key=natural_key,
        )
        cases = []
        for f in ins:
            n = f[:-3]
            outp = n + ".out"
            shutil.copy(os.path.join(src_dir, f), os.path.join(td_dir, f))
            s_out = os.path.join(src_dir, n + ".ans")
            if os.path.exists(s_out):
                shutil.copy(s_out, os.path.join(td_dir, outp))
            cases.append((f, outp))
        if cases:
            subtasks.append({"id": 1, "score": 100, "cases": cases})

    if not subtasks:
        print("警告: 题目 %s 没有找到任何测试数据, 跳过" % pid, file=sys.stderr)
        return False

    # 写出文件
    tags = []  # Gooj 没有 tag 概念, 留空; 如需可在此扩展
    with open(os.path.join(prob_dir, "problem.yaml"), "w", encoding="utf-8") as f:
        f.write(build_problem_yaml(
            pid, title, tags,
            (db_info or {}).get("nSubmit"),
            (db_info or {}).get("nAccept"),
        ))
    with open(os.path.join(prob_dir, "problem_zh.md"), "w", encoding="utf-8") as f:
        f.write(desc if desc else title)
    with open(os.path.join(td_dir, "config.yaml"), "w", encoding="utf-8") as f:
        f.write(build_testdata_config(mem_mb, time_ms, subtasks))
    return True


# --------------------------------------------------------------------------- #
# 打包
# --------------------------------------------------------------------------- #
def zip_tree(zf, tree_root):
    for root, _dirs, files in os.walk(tree_root):
        for fn in files:
            full = os.path.join(root, fn)
            arc = os.path.relpath(full, tree_root)
            zf.write(full, arc)


def main():
    ap = argparse.ArgumentParser(description="Gooj -> Hydro 题目导出")
    ap.add_argument("--src", default="data/problem",
                    help="Gooj 题目目录 (默认 data/problem)")
    ap.add_argument("--db", default="data/app.db",
                    help="Gooj SQLite 数据库 (默认 data/app.db)")
    ap.add_argument("--out", default="data/HydroExport.zip",
                    help="合并导出的 zip 路径 (默认 data/HydroExport.zip)")
    ap.add_argument("--per-problem", action="store_true",
                    help="每题单独一个 zip, 输出到 --out-dir")
    ap.add_argument("--out-dir", default="data/HydroProblems",
                    help="--per-problem 时的输出目录")
    args = ap.parse_args()

    if not os.path.isdir(args.src):
        print("错误: 题目目录不存在: %s" % args.src, file=sys.stderr)
        sys.exit(1)

    db_info = load_db(args.db)
    print("已从数据库读取 %d 道题的元信息" % len(db_info))

    pids = sorted(
        (d for d in os.listdir(args.src)
         if d.isdigit() and os.path.isdir(os.path.join(args.src, d))),
        key=lambda x: int(x),
    )
    if not pids:
        print("错误: %s 下没有找到数字命名的题目目录" % args.src, file=sys.stderr)
        sys.exit(1)

    ok = 0
    if args.per_problem:
        os.makedirs(args.out_dir, exist_ok=True)
        for pid in pids:
            with tempfile.TemporaryDirectory() as stage:
                if convert_problem(pid, os.path.join(args.src, pid),
                                   db_info.get(int(pid)), stage):
                    zpath = os.path.join(args.out_dir, "%s.zip" % pid)
                    with zipfile.ZipFile(zpath, "w", zipfile.ZIP_DEFLATED) as zf:
                        zip_tree(zf, stage)
                    ok += 1
                    print("已导出: %s" % zpath)
    else:
        os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
        with tempfile.TemporaryDirectory() as stage:
            for pid in pids:
                if convert_problem(pid, os.path.join(args.src, pid),
                                   db_info.get(int(pid)), stage):
                    ok += 1
            with zipfile.ZipFile(args.out, "w", zipfile.ZIP_DEFLATED) as zf:
                zip_tree(zf, stage)
        print("已导出 %d 道题到: %s" % (ok, args.out))

    print("完成, 成功 %d / 共 %d 题" % (ok, len(pids)))


if __name__ == "__main__":
    main()
