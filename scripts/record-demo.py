#!/usr/bin/env python3
"""Record real loopback probes and render excerpts. Requires Python 3 + Pillow.
Usage: python3 scripts/record-demo.py /absolute/path/to/netflow
"""
import http.server
import pathlib
import subprocess
import sys
import threading
import time
from PIL import Image, ImageDraw, ImageFont

root = pathlib.Path(__file__).resolve().parents[1]
assets = root / 'docs' / 'assets'
assets.mkdir(parents=True, exist_ok=True)
binary = str(pathlib.Path(sys.argv[1]).resolve())
version = subprocess.check_output([binary, 'version'], text=True).strip()

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/slow':
            time.sleep(0.5)
        try:
            self.send_response(403 if self.path == '/denied' else 200)
            self.end_headers()
        except (BrokenPipeError, ConnectionResetError):
            pass
    def log_message(self, *args):
        pass

server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
records = []
try:
    for title, path, expected in [('HTTP 200: checks passed', '/health', 0),
                                  ('HTTP 403: TCP passed, HTTP failed', '/denied', 1),
                                  ('Slow response: TCP passed, HTTP timed out', '/slow', 1)]:
        args = ['probe', '--protocol', 'http', '--url',
                f'http://127.0.0.1:{server.server_port}{path}', '--timeout', '200ms']
        result = subprocess.run([binary, *args], text=True, capture_output=True, check=False)
        if result.returncode != expected:
            raise RuntimeError(result.stderr or result.stdout)
        # Omit only the source inventory/header (host identity); retain result lines verbatim.
        lines = result.stdout.splitlines()
        start = next(i for i, line in enumerate(lines) if line.startswith(('[PASS]', '[FAIL]')))
        excerpt = '\n'.join(lines[start:])
        records.append((title, '$ netflow ' + ' '.join(args), excerpt, result.returncode))
finally:
    server.shutdown()
    server.server_close()
    thread.join()

transcript = [f'{version} | Linux loopback demo',
              'Real CLI output excerpts; only the source header is omitted.',
              'Local HTTP fixtures: /health=200, /denied=403, /slow=500ms delay.',
              'Not an Azure/AKS/firewall test. Ports and timings vary.\n']
frames = []
font = ImageFont.truetype('DejaVuSansMono.ttf', 17)
small = ImageFont.truetype('DejaVuSansMono.ttf', 15)
for title, command, excerpt, code in records:
    transcript.extend([title, command, excerpt, f'Exit code: {code}\n'])
    im = Image.new('RGB', (1140, 510), '#0d1117')
    draw = ImageDraw.Draw(im)
    draw.rounded_rectangle((12, 12, 1128, 498), radius=16, fill='#161b22', outline='#30363d')
    draw.text((32, 28), f'{version}  /  ACTUAL LOCAL RUN', font=small, fill='#8b949e')
    draw.text((32, 59), title, font=font, fill='#58a6ff')
    # Split long command at flag boundaries without changing the recorded invocation.
    cmd = command.replace(' --url', ' \\\n  --url')
    y = 99
    for line in cmd.splitlines():
        draw.text((32, y), line, font=small, fill='#d2a8ff'); y += 23
    y += 12
    for line in excerpt.splitlines():
        color = ('#7ee787' if code == 0 else '#f97583') if line.startswith('PASS=') else '#f97583' if 'FAIL' in line or 'TIMEOUT' in line else '#7ee787' if 'PASS' in line else '#c9d1d9'
        draw.text((32, y), line, font=small, fill=color); y += 23
    draw.text((32, 465), f'Exit {code} | Loopback HTTP fixture. No Azure or firewall policy verdict.', font=small, fill='#8b949e')
    frames.append(im)
frames[0].save(assets / 'demo.gif', save_all=True, append_images=frames[1:], duration=[4500]*3, loop=0, optimize=True)
(assets / 'demo.txt').write_text('\n'.join(transcript) + '\n')
print('\n'.join(transcript))
