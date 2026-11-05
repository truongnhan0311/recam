# checks.md

Định dạng: `bước | yêu cầu | verdict | evidence` (verdict: pass / fail / unresolved)

Phiên bản đang kiểm: S1 v0.3 (bám sát Tuya, MQTT, giao thức log, kiến trúc ba loại server tách rời có thể thành cụm, chỉ Go).

| bước | yêu cầu | verdict | evidence |
|---|---|---|---|
| S1 | C1: `openapi.yaml` hợp lệ (OpenAPI 3.1), mọi `$ref` trỏ đúng, từ khóa schema (`type`, `required`, `enum`) đúng dạng | pass | `make proto-check` (viết bằng Go, `libopenapi-validator` v0.16.2): "openapi.yaml is valid (3.1.0)", "every $ref resolves", "schema keywords … well-formed" |
| S1 | C1: mọi endpoint có schema lỗi `Problem` | pass | `make proto-check`: "all 30 operations declare Problem error responses" |
| S1 | C1: DataChannel (`Ctrl*`) và MQTT (`Mq*`) có trường `v` bắt buộc; mọi `Mq*` kế thừa `MqEnvelope`; mọi thông điệp nêu trong `x-mqtt` đều có schema | pass | `make proto-check`: "Ctrl* (9) and Mq* (9) schemas carry 'v'; x-mqtt messages all defined" |
| S1 | C1: `PROTOCOL.md` có sơ đồ trạng thái, sơ đồ trình tự, đủ 10 giai đoạn | pass | `make proto-check`: "has state diagram, sequence diagram, and 10 stages" |
| S1 | Checker thật sự bắt lỗi (không pass giả) | pass | Bản Go thử phá cố ý trên bản sao, mỗi lần thoát mã 1 và nêu đúng lỗi: `type` viết sai, `required` chứa boolean, `$ref` trỏ nơi không có, `MqHeader` bỏ `v`, `x-mqtt` nêu thông điệp không có, bỏ tiêu đề giai đoạn 7, đổi `sequenceDiagram`, `GET /me` thiếu lỗi. Lần thử đầu cho thấy bản Go bỏ sót lỗi `type`/`required` (yếu hơn bản Python cũ) nên đã bổ sung kiểm tra từ khóa schema |
| S1 | Mọi sơ đồ mermaid render được (3 trong `PROTOCOL.md`, 7 trong `ARCHITECTURE.md`) | pass | Chạy thủ công `@mermaid-js/mermaid-cli@11.4.2` với chromium có sẵn: tạo được đủ ảnh. **Chưa nằm trong `make proto-check`** (cần Node, trình duyệt; ngoài ràng buộc Go-only vì chỉ để xem tài liệu) |
| S1 | Sơ đồ trong `ARCHITECTURE.md` đọc được bằng mắt | unresolved | Đã xem ảnh 01, 02, 04, 07 và sửa 01, 04 vì rối. Ảnh 03, 05, 06 chỉ biết là render không lỗi, **chưa xem bằng mắt** |
| S1 | Bám sát Tuya: mỗi mục "giữ giống" trong `PROTOCOL.md` 1.1 có nguồn trong mã Tuya đã đọc | unresolved | Mình đối chiếu thủ công khi viết; chưa có người kiểm chéo. Các chỗ chưa xác minh được liệt kê ở 1.2 (D7) và mục 15 (#5) |
| S1 | Toàn bộ server và công cụ viết bằng Go (chỉ đạo) | pass | Checker đã chuyển từ Python sang Go (`tools/protocheck`). Còn lại ngoài Go: PostgreSQL, Valkey (kho dữ liệu dùng sẵn), kiểm tra mermaid bằng Node chạy tay. `cam-mq`, `cam-stream` dự kiến bằng Go (`mochi-mqtt`, `pion/turn`) nhưng **chưa xác minh bằng code** |
| S1 | Điểm D1 (DTLS-SRTP thay vì AES-trong-SDP) được người phụ trách xác nhận | pass | Người phụ trách chọn "dùng bản tiêu chuẩn WebRTC" (2026-11-05); đã ghi vào `PROTOCOL.md` mục 1.2 |
| S1 | Nội dung giao thức đúng và đủ về mặt thiết kế | unresolved | Chưa có người xem xét; chưa có cài đặt nào kiểm chứng. Xem `PROTOCOL.md` mục 15 |
| S1 | Mô hình quyền và giới hạn gói khớp với quyết định của người phụ trách (mục 11 của plan) | unresolved | Các giả định mục 11 chưa được xác nhận |

Lỗi đã gặp và sửa trong lúc làm S1 (để khỏi lặp lại):
1. Trường tên `on` bị YAML 1.1 hiểu thành `true` → đổi thành `enabled`.
2. Dấu `;` trong ghi chú sơ đồ mermaid là dấu ngắt lệnh → đổi thành dấu phẩy.
3. Checker in dòng "ok" khi còn lỗi (chuỗi tìm kiếm viết sai) → sửa; phép thử phá cũng từng viết sai (thiếu một dấu `#`) nên chưa chứng minh được gì, đã làm lại.
4. Bản 0.1 lệch Tuya ở nhiều chỗ mà không nêu rõ (WebSocket thay MQTT, camera tạo offer thay vì viewer, ES256 thay khóa cấp lúc kích hoạt, thiếu lệnh xin khung I/đổi bitrate) → viết lại 0.2 và thêm bảng so sánh ở `PROTOCOL.md` mục 1.
5. Checker Go ban đầu bỏ sót lỗi bên trong từ khóa JSON Schema (`type: intgr`, `required: [true]`) mà bản Python bắt được → bổ sung kiểm tra, và thử phá lại.
6. Sơ đồ 01 và 04 ban đầu rối (đường chéo, nhãn đè) → vẽ lại, xem lại ảnh.
7. `go get` kéo toolchain Go 1.26 (do `libopenapi` yêu cầu) và ghi `go 1.26.0` vào `go.mod`; máy hiện có Go 1.24.7 và tự tải toolchain.
