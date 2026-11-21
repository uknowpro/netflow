# Reproduce the README demo / 데모 재현

The GIF is a presentation of actual CLI output, not fabricated output or a terminal screen recording. The source header is omitted to keep host identity out of the recording. All displayed result lines, endpoints, timings and exit codes come from the executed binary. It uses local HTTP fixtures, not Azure resources, a real WAF, or DB authentication.

GIF는 실제 CLI 실행 결과를 렌더링한 것입니다. 터미널 화면 녹화는 아니며, 호스트 정보가 노출되지 않도록 출발지 헤더만 생략했습니다. 표시한 결과·주소·시간·종료 코드는 실제 출력입니다. 로컬 HTTP 서버를 사용하며 Azure·WAF·DB 인증 검사가 아닙니다.

From the repository root, with Python 3, Pillow, and the DejaVu Sans Mono font installed:

```bash
python3 scripts/record-demo.py /absolute/path/to/netflow
```

This replaces `docs/assets/demo.gif` and `docs/assets/demo.txt`. The script opens a temporary loopback listener and shuts it down when done. `/health` returns 200; `/denied` returns 403; `/slow` waits 500ms while the probe timeout is 200ms. It checks the expected process exit codes. Ports and timing vary; the README's expanded text is a captured sample and should be updated if the assets are re-recorded.

[Plain-text output](assets/demo.txt) · [English README](../README.md) · [한국어 README](README.ko.md)
