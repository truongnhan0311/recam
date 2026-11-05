# Recam

Biến điện thoại cũ thành camera giám sát, xem từ xa trên điện thoại chính. Một app, hai chế độ:
**Camera** (điện thoại cũ) và **Viewer** (xem, chia sẻ cho người khác).

Hai gói: **Free** (có quảng cáo, chia sẻ cho 1 người) và **Plan** (không quảng cáo, chia sẻ cho 10 người).
Số người xem cùng lúc phụ thuộc sức chịu của điện thoại camera.

Trạng thái: **bản nháp thiết kế (S1)**. Chưa có mã server hay app.

## Thiết kế

- Media: **WebRTC** chuẩn (ICE, DTLS-SRTP, H.264 + Opus).
- Kênh điều khiển, vòng đời thiết bị, định dạng thông điệp: **bám sát thiết kế của Tuya** (MQTT, kích hoạt bằng token, `header`/`msg`).
- Server **chỉ viết bằng Go**, gồm ba loại tách rời, mỗi loại có thể là một cụm:
  `cam-api` (REST), `cam-mq` (MQTT), `cam-stream` (TURN).

| Tài liệu | Nội dung |
|---|---|
| [`plan.md`](plan.md) | Mục đích, giải pháp, yêu cầu phần cứng, các bước, rủi ro, tiêu chí kiểm tra |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Sơ đồ quan hệ App – Camera – Server, chạy cụm, định tuyến |
| [`docs/PROTOCOL.md`](docs/PROTOCOL.md) | Giao thức: xác thực, vòng đời, MQTT, phiên xem, DataChannel, log |
| [`docs/openapi.yaml`](docs/openapi.yaml) | Đặc tả REST (OpenAPI 3.1) và các thông điệp MQTT/DataChannel |
| [`checks.md`](checks.md), [`progress.md`](progress.md) | Kết quả kiểm tra và tiến độ |

## Kiểm tra tài liệu

Cần Go (toolchain 1.26; `go` sẽ tự tải nếu máy có bản cũ hơn).

```bash
make proto-check
```
