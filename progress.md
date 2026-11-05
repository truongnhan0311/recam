# progress.md

## Plan
- Đã xong: S1 (spec giao thức) v0.3 về mặt tạo file và C1: bám sát Tuya, ba loại server tách rời có thể thành cụm, chỉ Go, có tài liệu kiến trúc với 7 sơ đồ.
- Đang chờ: người phụ trách xem xét S1 và xác nhận các giả định mục 11 của `plan.md` (S0). Điểm D1 đã được xác nhận.
- Bị chặn: không có.

## Outputs
- `docs/openapi.yaml` (OpenAPI 3.1: 30 thao tác, schema lỗi, `x-mqtt` mô tả chủ đề, 9 thông điệp `Mq*`, 9 thông điệp `Ctrl*`, giao thức log theo yêu cầu)
- `docs/PROTOCOL.md` v0.2 (so sánh với Tuya, vai trò, xác thực, máy trạng thái, 10 giai đoạn, MQTT, trình tự phiên, DataChannel, DP, sức chứa, log theo yêu cầu, bảo mật, mã lỗi, việc còn mở)
- `docs/ARCHITECTURE.md` + `docs/diagrams/*.png` (7 sơ đồ: tổng thể, cụm, định tuyến shard, chủ đề MQTT, vòng đời, đường media, triển khai)
- `tools/protocheck/main.go` (checker bằng Go), `go.mod`, `Makefile` (`make proto-check`)
- `checks.md`, `progress.md`, `plan.md` (đã cập nhật cho khớp MQTT)

## Decisions
- **Bám sát Tuya** (chỉ đạo 2026-11-05): kênh điều khiển MQTT; máy trạng thái, kích hoạt bằng token+`uuid`+`authkey` nhận `devid/seckey/localkey`; khung `header`+`msg`; viewer tạo offer, camera trả answer; `token/udp_token/tcp_token`; lệnh `video.start`, `iframe.request`, `bitrate.set`; reset 5 loại; DP; yêu cầu REST có ký.
- **Khác Tuya duy nhất về bảo mật**: media dùng DTLS-SRTP chuẩn (D1), không dùng khóa AES trong SDP. **Đã được xác nhận: dùng WebRTC chuẩn.**
- Khác vì điện thoại/nhiều viewer: D4 (nhiều viewer, `max_viewers`), D5 (khóa do app sinh), D6 (signaling viewer qua REST).
- Công thức mật khẩu MQTT là của dự án (thuật toán Tuya chưa xác minh).
- Camera chỉ mã hóa sau `video.start`.
- Log theo yêu cầu: URL tải lên có chữ ký 15 phút, che dữ liệu nhạy cảm, nhân viên hỗ trợ cần cho phép của người dùng, có kiểm toán.
- **Chỉ Go** cho server và công cụ (chỉ đạo): `cam-api`, `cam-mq` (dự kiến `mochi-mqtt`), `cam-stream` (dự kiến `pion/turn`), checker bằng Go.
- **Ba loại server tách rời, mỗi loại có thể là cụm** (chỉ đạo): API không giữ trạng thái sau cân bằng tải; MQTT chia shard theo `devid` (địa chỉ shard trả qua `mqtt_url`); Stream là các node TURN độc lập theo vùng dùng chung bí mật HMAC.
- Trường `on` bị cấm (bẫy YAML 1.1); dùng `enabled`.
- Mã lỗi `plan_limit_reached` trả HTTP 402.

- **v0.3.1 (đọc lại thiết kế):** v1 không đàm phán lại SDP, talkback dùng audio `sendrecv` + `replaceTrack`; thêm `MqSessionState` báo viewer camera đã thức; giới hạn kích hoạt theo IP (10 sai/giờ) và `uuid` (5 sai/10 phút); CSDL `mq_node` là nguồn quyết định shard, băm chỉ dùng khi gán; retained MQTT là nguồn hiện diện, Valkey là bản sao; thêm `timeout_answer`.

## Open issues
- `make proto-check` chưa chạy lại trên bản v0.3.1 (phiên cloud chặn tải Go 1.26); đã kiểm bằng script Python tương đương.
- Cân bằng lại tải shard chủ động chưa thiết kế (S3).
- Chưa ai xem xét thiết kế; mọi con số (nhịp tim, thời hạn, bậc chất lượng, công thức `max_viewers`) là đề xuất.
- Chưa có endpoint bật/tắt "cho phép hỗ trợ xem log".
- libwebrtc có thể mã hóa riêng cho mỗi PeerConnection → `max_viewers` có thể thấp hơn kỳ vọng.
- Đánh thức camera iOS ở nền không đáng tin; Android 12+ hạn chế khởi động Foreground Service.
- `mochi-mqtt` chưa xác minh về cụm, tải, bảo trì (bản mới nhất 2025-03-01); `pion/turn` chưa đo so với coturn. Nếu không đạt thì phải quyết định lại vì phương án khác lệch Go-only.
- `cam-api` phải đăng ký `out/+` trên mọi shard MQTT; chưa đo chi phí.
- Ảnh 03, 05, 06 trong `docs/diagrams/` chưa xem bằng mắt.
- Kiểm tra mermaid chưa nằm trong `make proto-check`.
- Mục 11 của `plan.md` (giả định) chưa được xác nhận.

## Next
- Chờ duyệt S1. Sau đó: S2 (tài liệu dựng 1 VM chạy ba tiến trình Go, chưa tạo VM) song song với S3 (backend Go: kích hoạt, Server MQTT, ICE/TURN, log). **Đã đặt tên chương trình nhưng chưa viết dòng code server nào.**
