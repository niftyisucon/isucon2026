#!/usr/bin/env python3
"""Print/check representative API metrics from every saved LTSV row (seconds)."""
import argparse
from decimal import Decimal
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('job', type=Path)
parser.add_argument('--check', action='store_true')
args = parser.parse_args()
lines = ['| Host | API (method / path / status) | n | avg apptime (seconds) |',
         '|---|---|---:|---:|']
aggregate = (args.job / 'aggregate.txt').read_text().split('===== isu')[1:]
for host in (1, 2, 3):
    rows = [dict(field.split(':', 1) for field in line.split('\t'))
            for line in (args.job / f'isu{host}-ltsv.txt').read_text().splitlines()]
    for label, method, path, status, prefix in (
        ('posts', 'POST', '/api/posts', '201', False),
        ('feeds', 'GET', '/api/feeds/', '200', True),
        ('images', 'GET', '/api/images/', '200', True),
    ):
        values = []
        for row in rows:
            uri = row['uri'].split('?', 1)[0]
            matches = uri.startswith(path) if prefix else uri == path
            if row['method'] == method and row['status'] == status and matches:
                values.append(Decimal(row['apptime']))
        mean = sum(values) / len(values)
        lines.append(f'| isu{host} | {method} {path}{"*" if prefix else ""} / {status} | {len(values)} | {mean:.3f} |')
        if args.check and label == 'posts':
            expected = f'POST /api/posts [201] n={len(values)} avg={mean:.3f} '
            if len(aggregate) != 3 or expected not in aggregate[host - 1]:
                raise SystemExit(f'aggregate.txt mismatch: isu{host} {expected}')
table = '\n'.join(lines)
print(table)
if args.check:
    summary = (args.job / 'summary.md').read_text()
    start = '<!-- representative-api-metrics -->\n'
    end = '\n<!-- /representative-api-metrics -->'
    if start not in summary or end not in summary:
        raise SystemExit('summary.md: metric table markers missing')
    actual = summary.split(start, 1)[1].split(end, 1)[0]
    if actual != table:
        raise SystemExit('summary.md mismatch: replace marked table with output above')
    print('OK: summary representative metrics and aggregate POST /api/posts agree')
