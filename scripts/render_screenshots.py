#!/usr/bin/env python3
import sys
import re
import html
import subprocess
import os

def parse_ansi_line(line):
    # Regex to capture ANSI escape codes: \x1b[...m
    # Returns list of (text, fg, bg, bold, dim)
    pos = 0
    fg = "#d0d7de"
    bg = None
    bold = False
    dim = False

    segments = []
    current_text = ""

    i = 0
    n = len(line)
    while i < n:
        if line[i:i+2] == "\x1b[":
            m = re.match(r'\x1b\[([0-9;]*)m', line[i:])
            if m:
                if current_text:
                    segments.append((current_text, fg, bg, bold, dim))
                    current_text = ""
                codes = m.group(1).split(';') if m.group(1) else ['0']
                idx = 0
                while idx < len(codes):
                    c = codes[idx]
                    if c == '' or c == '0':
                        fg = "#d0d7de"
                        bg = None
                        bold = False
                        dim = False
                    elif c == '1':
                        bold = True
                    elif c == '2':
                        dim = True
                    elif c == '38' and idx + 4 < len(codes) and codes[idx+1] == '2':
                        r, g, b = codes[idx+2], codes[idx+3], codes[idx+4]
                        fg = f"#{int(r):02x}{int(g):02x}{int(b):02x}"
                        idx += 4
                    elif c == '48' and idx + 4 < len(codes) and codes[idx+1] == '2':
                        r, g, b = codes[idx+2], codes[idx+3], codes[idx+4]
                        bg = f"#{int(r):02x}{int(g):02x}{int(b):02x}"
                        idx += 4
                    elif c == '39':
                        fg = "#d0d7de"
                    elif c == '49':
                        bg = None
                    idx += 1
                i += len(m.group(0))
                continue
        current_text += line[i]
        i += 1

    if current_text:
        segments.append((current_text, fg, bg, bold, dim))
    return segments

def ansi_to_svg(ansi_text, title="otel-tui"):
    lines = ansi_text.rstrip('\r\n').split('\n')
    
    char_w = 8.5
    line_h = 20
    pad_x = 24
    pad_top = 48
    pad_bottom = 20

    # Max line length in visible characters
    max_len = 0
    for l in lines:
        stripped = re.sub(r'\x1b\[[0-9;]*m', '', l)
        max_len = max(max_len, len(stripped))
    
    max_len = max(max_len, 102)
    
    width = int(max_len * char_w + pad_x * 2)
    height = int(len(lines) * line_h + pad_top + pad_bottom)

    svg_parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">',
        f'<rect x="1" y="1" width="{width-2}" height="{height-2}" rx="10" fill="#13141c" stroke="#2b2d3a" stroke-width="1.5"/>',
        # macOS title bar dots
        '<circle cx="22" cy="22" r="5.5" fill="#ff5f56"/>',
        '<circle cx="40" cy="22" r="5.5" fill="#ffbd2e"/>',
        '<circle cx="58" cy="22" r="5.5" fill="#27c93f"/>',
        f'<text x="{width/2}" y="26" text-anchor="middle" fill="#7d8590" font-family="-apple-system, BlinkMacSystemFont, \'Segoe UI\', sans-serif" font-size="12" font-weight="500">{html.escape(title)}</text>',
        f'<line x1="1" y1="40" x2="{width-1}" y2="40" stroke="#1f212d" stroke-width="1"/>',
    ]

    # Render background rectangles first, then text
    y = pad_top + 14
    for line_idx, line in enumerate(lines):
        segments = parse_ansi_line(line)
        x = pad_x
        # Background rects
        for text, fg, bg, bold, dim in segments:
            w = len(text) * char_w
            if bg:
                svg_parts.append(f'<rect x="{x:.1f}" y="{y - 14:.1f}" width="{w:.1f}" height="{line_h:.1f}" fill="{bg}"/>')
            x += w

        # Foreground text
        x = pad_x
        for text, fg, bg, bold, dim in segments:
            w = len(text) * char_w
            weight = "700" if bold else "400"
            opacity = "0.7" if dim else "1.0"
            escaped = html.escape(text).replace(' ', '&#160;')
            svg_parts.append(f'<text x="{x:.1f}" y="{y:.1f}" fill="{fg}" opacity="{opacity}" font-family="ui-monospace, \'SF Mono\', \'Menlo\', \'Monaco\', \'Cascadia Code\', monospace" font-size="13" font-weight="{weight}" xml:space="preserve">{escaped}</text>')
            x += w

        y += line_h

    svg_parts.append('</svg>')
    return '\n'.join(svg_parts)

def main():
    screens = [
        ("trace-list", "otel-tui — Live Distributed Trace Stream"),
        ("waterfall", "otel-tui — Distributed Trace Waterfall & Timeline"),
        ("flamegraph", "otel-tui — Stacked Flamegraph View"),
        ("topology", "otel-tui — Service Dependency Architecture Topology"),
        ("trace-diff", "otel-tui — Trace Comparison & Latency Regression Diff"),
        ("inspector", "otel-tui — Span Inspector Drawer & Correlated Logs"),
    ]

    out_dir = "docs/assets"
    os.makedirs(out_dir, exist_ok=True)

    for name, title in screens:
        ansi_path = f"/tmp/otel-screens/{name}.ansi"
        if not os.path.exists(ansi_path):
            print(f"Skipping {name}, not found")
            continue
        ansi_text = open(ansi_path, "r", encoding="utf-8", errors="ignore").read()
        svg = ansi_to_svg(ansi_text, title)
        svg_path = f"{out_dir}/{name}.svg"
        with open(svg_path, "w", encoding="utf-8") as f:
            f.write(svg)
        
        png_path = f"{out_dir}/{name}.png"
        subprocess.run(["magick", "-density", "144", svg_path, png_path], check=True)
        print(f"Rendered {svg_path} -> {png_path}")

if __name__ == "__main__":
    main()
