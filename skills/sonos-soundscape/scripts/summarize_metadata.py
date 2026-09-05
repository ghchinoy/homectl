#!/usr/bin/env python3
"""
Summarizes verbose Sonos UPnP DIDL-Lite XML into a compact JSON payload.
Designed as a deterministic helper for the sonos-soundscape Skill with Code.
Reduces LLM prompt token consumption by 90-96%.
"""

import sys
import json
import html
import re

def parse_tag(xml_str, tag):
    pattern = rf'<(?:\w+:)?{tag}[^>]*>(.*?)</(?:\w+:)?{tag}>'
    match = re.search(pattern, xml_str, re.DOTALL | re.IGNORECASE)
    if match:
        return html.unescape(match.group(1).strip())
    return ""

def summarize(raw_xml):
    if not raw_xml or raw_xml.strip() == "NOT_IMPLEMENTED":
        return {
            "title": "",
            "artist": "",
            "album": "",
            "stream_content": "",
            "audio_format": ""
        }

    title = parse_tag(raw_xml, "title")
    artist = parse_tag(raw_xml, "creator")
    album = parse_tag(raw_xml, "album")
    stream = parse_tag(raw_xml, "streamContent")
    
    audio_format = ""
    proto_match = re.search(r'protocolInfo="([^"]+)"', raw_xml)
    if proto_match:
        audio_format = proto_match.group(1)

    result = {
        "title": title,
        "artist": artist,
        "album": album,
    }
    if stream:
        result["stream_content"] = stream
    if audio_format:
        result["audio_format"] = audio_format

    return result

def main():
    if len(sys.argv) > 1 and sys.argv[1] not in ("-", "--help"):
        raw_input = sys.argv[1]
    else:
        raw_input = sys.stdin.read()

    if not raw_input.strip():
        print("Usage: summarize_metadata.py '<DIDL-Lite XML>' or pipe via stdin", file=sys.stderr)
        sys.exit(1)

    summary = summarize(raw_input)
    output_json = json.dumps(summary, indent=2)

    raw_tokens = max(1, len(raw_input) // 4)
    summary_tokens = max(1, len(output_json) // 4)
    savings = max(0, 100 - (summary_tokens * 100 // raw_tokens))

    print(output_json)
    if sys.stderr.isatty() or "--stats" in sys.argv:
        print(f"\n[Token Economics] Raw: ~{raw_tokens} tokens | Summary: ~{summary_tokens} tokens | Savings: ~{savings}%", file=sys.stderr)

if __name__ == "__main__":
    main()
