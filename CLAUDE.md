# CLAUDE.md

Dự án Recam: hiện chỉ có tài liệu thiết kế (S1). Đọc `README.md`, `plan.md`, `progress.md`, `checks.md` trước khi sửa.

## Kiểm tra
- `make proto-check` (Go, `tools/protocheck`): kiểm `docs/openapi.yaml` và `docs/PROTOCOL.md`. Chạy sau mỗi lần sửa tài liệu giao thức.
- Phải tải toolchain Go 1.26; nếu môi trường chặn tải, ghi `unresolved` trong `checks.md`, không ghi `pass`.

## Rule (rút ra từ lỗi đã gặp)
- Không dùng tên trường `on` trong YAML (YAML 1.1 hiểu thành `true`); dùng `enabled`.
- Không dùng `;` trong ghi chú mermaid.
- Mỗi checker mới phải được thử phá cố ý một lần để chứng minh bắt được lỗi.
- Mỗi trạng thái chờ (ví dụ `waking`) phải có thông điệp báo bên đang chờ khi trạng thái đổi.
- Mỗi dữ liệu chỉ có **một** nguồn quyết định; bản sao (Valkey) phải ghi rõ là bản sao.
- Giới hạn chống dò phải theo cả đối tượng bị dò (token) lẫn bên dò (IP, `uuid`).
- Mã lỗi/lý do đóng phải đúng nghĩa; timeout dùng mã `timeout_*` riêng.
- Thêm thông điệp `Mq*` mới thì thêm vào `x-mqtt` và cho kế thừa `MqEnvelope`.
- Ghi tên thư viện hay thuật toán vào tài liệu thì kiểm trước: phiên bản trên `proxy.golang.org`, và đọc mã của framework xem có hỗ trợ không (ví dụ Goravel chỉ ký JWT HS256).
- App là KMP trên `kmp_bootstrap` (đọc `AGENTS.md` của repo đó trước khi sửa kit; commit kit bằng tiếng Việt, đứng tên chủ dự án, không thêm trailer). Phần riêng của Recam để trong app, không đưa vào kit.
- Test client có keepAlive bằng `runTest`: thời gian ảo tự tua nên PINGREQ sẽ chen vào; đặt `keepAliveSeconds = 0` nếu test không nói về ping. Nghe `messages` (SharedFlow) TRƯỚC khi nối, nếu không tin đến sớm bị mất.
