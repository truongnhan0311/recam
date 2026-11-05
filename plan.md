# PLAN: Recam, app 2 chế độ (Camera + Viewer) dựa trên giao thức chuẩn

Trạng thái: **DRAFT, chờ phê duyệt.** Chưa viết dòng code nào. Đây là dự án độc lập, không liên quan hệ thống khác.
Ngày: 2026-11-05. Ngôn ngữ: tiếng Việt. Các con số sizing là **ước lượng**, phải đo lại ở bước S7.

---

## 0. Inputs (theo harness)

| Trường | Giá trị | Ghi chú |
|---|---|---|
| TASK | Thiết kế và dựng PoC → MVP của một app di động chạy được 2 chế độ: **Camera** (điện thoại cũ làm camera) và **Viewer** (xem từ xa), dùng hoàn toàn giao thức chuẩn, có **chia sẻ cho người khác xem** và **hai gói: Free (có quảng cáo) và Plan trả phí** | |
| LIMIT (giới hạn thử lại) | Mỗi bước được thử sửa tối đa 3 lần. Sau 3 lần vẫn chưa đạt thì dừng bước đó, ghi lý do vào `progress.md` và hỏi người phụ trách, không thử mãi | Để không lặp vô hạn |
| CHECKS (cách kiểm tra) | Danh sách "làm thế nào biết bước này đã xong" nằm ở mục 9 (C0–C10). Mỗi check viết **trước** khi làm bước, rồi **chạy sau** khi làm xong để lấy bằng chứng | Ví dụ: "go test pass", "2 điện thoại xem được nhau" |

---

## 1. Mục đích

**Vấn đề cần giải quyết**
- Người dùng có điện thoại cũ nằm không và muốn biến nó thành camera giám sát (ví dụ trông em bé), rồi xem từ xa trên điện thoại chính, qua mạng bất kỳ (NAT, 4G, Wi-Fi khác nhau).
- Cần một app duy nhất làm được cả hai vai, xây trên giao thức chuẩn công khai để không phải tự duy trì giao thức riêng, tận dụng được thư viện sẵn có và bảo mật theo chuẩn.

**Mục tiêu của dự án này**
1. Một app duy nhất, hai chế độ: Camera và Viewer, cùng một tài khoản.
2. Dùng chuẩn cho từng lớp: WebRTC (media), MQTT (kênh điều khiển, theo Tuya), TURN (relay), JWT (người dùng), khóa riêng từng thiết bị (theo Tuya).
3. Ưu tiên P2P trực tiếp để giảm băng thông server; chỉ fallback qua TURN khi cần.
4. Dự án mới hoàn toàn, không có gánh nặng tương thích ngược.
5. Chia sẻ camera cho người khác xem, và có mô hình kinh doanh hai gói (xem mục 2.1):

| | **Free** | **Plan (trả phí)** |
|---|---|---|
| Quảng cáo | Có (chỉ ở phía Viewer) | **Không** |
| Chia sẻ camera cho | **1 người** | **10 người** |


**Không phải mục tiêu (phase này)**
- Tích hợp với bất kỳ hệ camera hay hạ tầng có sẵn nào khác.
- Ghi hình lưu cloud, AI phát hiện. Xem mục 10. (Nhiều người xem cùng lúc được xử lý ở gói trả phí, mục 2.1.)

**Tiêu chí thành công của PoC**
- Hai điện thoại ở hai mạng khác nhau xem được nhau, độ trễ glass-to-glass trung vị ≤ 800 ms qua P2P, ≤ 1500 ms qua TURN.
- Tỉ lệ P2P thành công được đo và báo cáo theo loại mạng.
- Camera mode chạy liên tục ≥ 8 giờ trên Android mà không bị hệ điều hành giết.
- (MVP) Gói Free chặn đúng ở người chia sẻ thứ 2; gói Plan cho đủ 10 người và tắt quảng cáo; mua gói thử trên cả hai store làm quyền thay đổi trong ≤ 60 giây.

---

## 2. Kiến trúc đề xuất

```
┌──────────────────┐  HTTPS + MQTT (TLS)   ┌────────────────────────────┐
│ App - Camera mode│ ───────────────────► │  Server API (Go)           │
│ (WebRTC peer)    │ ◄─── push (FCM/APNs)─│  - đăng nhập, ghép đôi QR  │
└────────┬─────────┘                      │  - trao đổi SDP/ICE        │
         │                                │  - cấp TURN credential     │
         │  media: P2P qua ICE            │  - presence (MQTT→Valkey)  │
         │  (thất bại → qua TURN)         └──────┬──────────┬──────────┘
         ▼                                       │          │
┌──────────────────┐                      ┌──────▼───┐  ┌───▼──────────┐
│ App - Viewer mode│ ──HTTPS + MQTT (TLS)►│PostgreSQL│  │ Valkey       │
│ (WebRTC peer)    │                      └──────────┘  └──────────────┘
└──────────────────┘
         ▲ TURN (UDP 3478 / TCP+TLS 443) ◄── Server Stream (Go, các vùng)
```

**Sơ đồ chi tiết quan hệ App – Camera – Server, chạy cụm và định tuyến:** `docs/ARCHITECTURE.md` (kèm ảnh trong `docs/diagrams/`). Ba loại server tách rời (Server API, Server MQTT, Server Stream), mỗi loại có thể là một cụm.

**Luồng xem một lần**
1. Viewer gọi `POST /sessions {camera_id}`; server kiểm quyền và gửi push đánh thức Camera (nếu app ở nền).
2. Viewer tạo offer SDP và gửi qua REST; server chuyển cho camera qua MQTT; camera trả answer; hai bên trao ICE (xem `docs/PROTOCOL.md`, mục 7). Đây là cách app của Tuya làm: **viewer tạo offer, camera trả lời**.
3. ICE thử trực tiếp trước; không được thì dùng TURN với credential ngắn hạn (HMAC theo thời gian, RFC 8489 §9.2 / TURN REST).
4. Media đi DTLS-SRTP (H.264, dự phòng VP8 cho máy không có H.264 phần cứng; Opus); lệnh điều khiển (đổi độ phân giải, bật đèn, talkback) đi **DataChannel** dạng JSON có phiên bản.

**So sánh hai cách truyền cho nhiều viewer: P2P hay SFU**
| | P2P 1 camera → 1 viewer | SFU (LiveKit/mediasoup) |
|---|---|---|
| Băng thông server | Gần 0 khi P2P được | Mọi stream đi qua server |
| Tải điện thoại camera | Mã hóa 1 lần/viewer | Mã hóa 1 lần dù bao nhiêu viewer |
| Phù hợp | MVP, 1–2 viewer | ≥3 viewer, ghi hình, nhiều camera/viewer |

**Quyết định đề xuất (cập nhật theo yêu cầu: số người xem tùy sức chịu của điện thoại):**
- **Mọi gói đều dùng P2P + TURN fallback.** Không dựng SFU ở MVP, nên không có server media đắt tiền.
- Điện thoại camera tự gửi luồng cho từng viewer. Số viewer xem cùng lúc tối đa là `min(giới hạn của gói, khả năng của điện thoại camera)`. Chi tiết ở mục 2.1.
- SFU chỉ là phương án dự phòng sau này, nếu đo thực tế cho thấy điện thoại phổ thông chịu được quá ít viewer (mục 10).

---

## 2.1 Chia sẻ và gói dịch vụ

**Quy tắc (giả định ở mục 11):**

| | Free | Plan |
|---|---|---|
| Quảng cáo | Có, chỉ phía Viewer. **Không** có ở Camera mode, không chèn lên khung video | Không |
| Người được chia sẻ mỗi camera | 1 | 10 |
| Số viewer xem cùng lúc, tối đa theo gói | Chủ + 1 = 2 | Chủ + 10 = 11 |
| **Số viewer thực tế** | `min(gói, khả năng điện thoại camera)` | `min(gói, khả năng điện thoại camera)` |
| Đường media | P2P, fallback TURN | P2P, fallback TURN |
| Số camera mỗi tài khoản | 1 camera, tối đa 480p | 5 camera, tối đa 720p |

**Giới hạn theo khả năng điện thoại camera**

Số người được *cấp quyền* xem (1 hoặc 10) khác với số người xem *cùng một lúc*. Chủ camera Plan có thể chia sẻ cho 10 người, nhưng điện thoại cũ chỉ gánh được vài luồng. Mỗi viewer là một kết nối WebRTC riêng, camera phải mã hóa (thường tối đa 2–4 bộ mã hóa phần cứng đồng thời trên điện thoại) và upload thêm một luồng.

Cách làm:
1. **Đo khi bật Camera mode:** xác định mẫu máy và bộ mã hóa phần cứng, đo tốc độ upload thật (thử đẩy dữ liệu ngắn), rồi tính `max_viewers` ban đầu. Gợi ý ban đầu: `max_viewers = floor(upload_khả_dụng × 0.7 / bitrate_mỗi_luồng)`, kẹp bởi giới hạn bộ mã hóa của máy.
2. **Báo cho server:** camera báo `max_viewers` (DP 103) qua MQTT; server lưu cùng trạng thái hiện diện (presence).
3. **Điều chỉnh động:** khi CPU, nhiệt độ hoặc upload xấu đi, camera hạ bitrate/độ phân giải cho mọi viewer (ví dụ 720p → 480p → 360p), rồi mới giảm `max_viewers`. Khi ổn định thì tăng lại từ từ.
4. **Server cưỡng chế:** khi số phiên đang chạy đã đạt `min(gói, max_viewers)`, viewer mới nhận lỗi `camera_busy` kèm số người đang xem. App hiển thị "Camera đang đạt giới hạn người xem". Viewer vào trước được giữ; chủ camera luôn được ưu tiên (có thể đẩy người khác ra).
5. **Chủ thấy được:** màn hình Camera hiển thị số viewer hiện tại và giới hạn, kèm gợi ý "cắm sạc / dùng Wi-Fi để tăng giới hạn".

Giá trị `max_viewers` ban đầu chỉ là công thức giả định; bảng số liệu thật theo mẫu máy sẽ có ở S7.

**Cách chia sẻ:** chủ camera tạo lời mời (liên kết hoặc QR, dùng một lần, hết hạn sau 24 giờ). Người nhận đăng nhập và chấp nhận. Người được chia sẻ chỉ xem được, không chia sẻ tiếp, không đổi cài đặt. Chủ thu hồi quyền bất cứ lúc nào và phiên đang xem bị cắt trong ≤ 5 giây.

**Mô hình dữ liệu (thêm vào PostgreSQL):** `users`, `cameras(owner_id)`, `shares(camera_id, user_id, status, invited_at)`, `invites(token_hash, camera_id, expires_at, used_at)`, `subscriptions(user_id, plan, status, expires_at, store, store_txn_id)`.

**API thêm vào:** `POST /cameras/{id}/invites`, `POST /invites/{token}/accept`, `DELETE /cameras/{id}/shares/{user_id}`, `GET /me/entitlements`. Server **tự kiểm giới hạn ở mọi lần tạo lời mời và tạo phiên**, không tin app. Vượt giới hạn trả `402 plan_limit_reached`.

**Hết hạn hoặc hạ gói:** không xóa dữ liệu. Giữ lại người chia sẻ cũ nhất trong giới hạn Free, tạm khóa phần còn lại, và khôi phục khi gia hạn.

**Quảng cáo:** Google AdMob (`google_mobile_ads`), cộng hộp thoại đồng ý (UMP) cho EU/UK và ATT trên iOS. Vị trí: banner ở màn hình danh sách camera; quảng cáo xen kẽ (interstitial) tối đa 1 lần mỗi vài phút, không bao giờ khi đang xem video. Ở môi trường dev chỉ dùng ad unit thử.

**Thanh toán:** dùng Apple In-App Purchase và Google Play Billing (bắt buộc với gói thuê bao số, store giữ khoảng 15–30%). Server nhận thông báo từ store (App Store Server Notifications v2, Google Real-time Developer Notifications), kiểm biên lai và cập nhật `subscriptions`. Có thể dùng RevenueCat để đỡ phải tự viết.

**Chi phí cần lưu ý:** người dùng Free tốn băng thông TURN nhưng chỉ đem lại tiền quảng cáo. Phải đo chi phí TURN trên mỗi người dùng Free ở S7 rồi so với doanh thu quảng cáo dự kiến, trước khi mở rộng. Vì gói Plan cũng dùng P2P, mỗi viewer phải đi TURN tốn thêm băng thông server (đã tính trong mục 4.2 theo tỉ lệ phiên qua TURN).

---

## 2.2 Vòng đời thiết bị (Camera mode)

Mô tả điều một camera làm từ lúc có nguồn đến lúc phục vụ người xem, áp dụng cho điện thoại ở Camera mode. Phần "tham khảo" bên dưới lấy từ tài liệu phát triển camera công khai của Tuya (SDK IPC 4.x/5.x). Đó là **quy trình của một nền tảng riêng**, không phải chuẩn của một tổ chức tiêu chuẩn. Theo chỉ đạo, dự án **bám sát thiết kế của Tuya** về vòng đời, kênh MQTT và định dạng thông điệp, và dùng WebRTC chuẩn cho phần media.

**Quyết định thiết kế (chỉ đạo ngày 2026-11-05): bám sát Tuya.** Máy trạng thái, các bước và kiểu thông điệp lấy theo mã nguồn mở của Tuya; chỉ khác ở bảo mật media (DTLS-SRTP chuẩn) và các điểm do điện thoại làm camera, nhiều viewer. Bảng so sánh đầy đủ ở `docs/PROTOCOL.md` mục 1.

**Các trạng thái (theo Tuya):** `IDLE → START → DATA_LOAD → TOKEN_PENDING → ACTIVATING → STARTUP_UPDATE → MQTT_CONNECTING → MQTT_YIELD`, nhánh `MQTT_RECONNECT`, `RESTART`, `RESET`, `STOP`.

| # | Giai đoạn | Việc xảy ra | Giao thức |
|---|---|---|---|
| 1 | **Danh tính** | Sinh `uuid` và `authkey`, cất trong kho khóa của hệ điều hành | |
| 2 | **Ghép đôi** | Chủ tạo token đúng 8 ký tự, sống 10 phút, một lần; camera quét QR hoặc nhập; kích hoạt giới hạn sai theo token, IP và `uuid` | HTTPS |
| 3 | **Tìm địa chỉ** | Hỏi `url_config` theo vùng để biết địa chỉ API và MQTT | HTTPS |
| 4 | **Kích hoạt** | Gửi token + `uuid` + `authkey`, nhận `devid`, `seckey`, `localkey` | HTTPS |
| 5 | **Cập nhật khi khởi động** | Báo phiên bản, khả năng, DP ban đầu; nhận cấu hình động và đồng bộ giờ | HTTPS có ký |
| 6 | **Kết nối MQTT** | `client_id = devid`, mật khẩu dẫn xuất từ `seckey`, Last Will cho hiện diện | MQTT qua TLS |
| 7 | **Giữ liên lạc và hiện diện** | Keepalive 60 giây, báo DP định kỳ, chạy nền, push đánh thức | MQTT, FCM/APNs |
| 8 | **Phục vụ phiên xem** | Nhận offer, trả answer, trao ICE, phát media, nhận lệnh `ctrl` | MQTT, WebRTC, DataChannel |
| 9 | **Làm mới và nối lại** | Thông tin MQTT/TURN có hạn, bỏ client cũ khi xin mới, nối lại có backoff | HTTPS, MQTT |
| 10 | **Đặt lại và gỡ** | 5 loại reset như Tuya; chủ gỡ camera thì cắt phiên trong ≤ 5 giây | HTTPS, MQTT |

**Thêm theo yêu cầu: gửi log theo yêu cầu.** Server có thể xin log của camera hoặc app khi cần chẩn đoán (`log.request` qua MQTT, tải lên bằng URL có chữ ký, che dữ liệu nhạy cảm, có kiểm toán). Chi tiết ở `PROTOCOL.md` mục 10.

**Danh sách khả năng (`capabilities`)**, theo ý tưởng "data point" của Tuya, nhưng dùng tên chữ có phiên bản thay vì số hiệu: `video.h264`, `audio.opus`, `ptz` (nếu có), `torch`, `talkback`, `max_viewers`, `max_resolution`. Camera gửi danh sách này khi online; viewer chỉ hiển thị điều khiển nào camera hỗ trợ.

**Tham khảo từ Tuya (đã đọc trong tài liệu, có ý nghĩa thiết kế):**

| Ý tưởng | Dùng trong plan |
|---|---|
| UUID + AUTHKEY nạp sẵn cho mỗi máy | Mỗi cài đặt có `device_id` + khóa riêng (giai đoạn 1) |
| Token ghép đôi do cloud tạo, hết hạn 10 phút | Mã QR ghép đôi hết hạn ngắn (giai đoạn 2) |
| Kênh MQTT, đồng bộ giờ sau khi online | Giữ nguyên: MQTT qua TLS và đồng bộ giờ (giai đoạn 5–6) |
| Heartbeat 30–120 giây khi ngủ | Giai đoạn 5 |
| Mô hình data point cho chức năng | `capabilities` (ở trên) |
| Đánh thức qua kênh phụ khi ngủ | Push (giai đoạn 6) |

### 2.2.1 Chi tiết từ mã nguồn mở của Tuya (đã đọc mã)

Nguồn: `tuya-connect-kit-for-mqtt-embedded-c` (thiết bị ↔ cloud, README ghi **không còn được bảo trì**) và `tuya-device-sharing-sdk` (phía app/người dùng được chia sẻ). Đây là mã tham khảo cho thiết kế, **không sao chép**.

**A. Máy trạng thái của thiết bị** (`src/tuya_iot.c`)

`IDLE → START → DATA_LOAD → TOKEN_PENDING → ACTIVATING → STARTUP_UPDATE → MQTT_CONNECT_START → MQTT_CONNECTING → MQTT_YIELD` (chạy bình thường); mất kết nối thì `MQTT_RECONNECT`; có thêm `RESTART`, `RESET`, `STOP`, `EXIT`.
Trạng thái hiển thị cho người dùng: chưa kích hoạt → chờ cấu hình mạng → chưa nối router → đã nối Wi-Fi → đã nối MQTT.
Sự kiện: reset, bắt đầu ghép đôi, nhận token, kích hoạt thành công, MQTT nối/ngắt, nhận lệnh, có bản cập nhật, **đồng bộ giờ**, báo đã ghép.
Kiểu reset: reset nhà máy, hủy kích hoạt từ xa, hủy kích hoạt cục bộ, reset nhà máy từ xa, xóa dữ liệu.

**B. Luồng khởi động thật**
1. Hỏi dịch vụ **tìm địa chỉ** (HTTPS `POST /v2/url_config` tới một tên miền cố định, gửi `region`) để biết địa chỉ HTTPS và MQTT của vùng đó.
2. **Kích hoạt**: HTTPS `POST /d.json` (kiểu gọi theo tên API, ví dụ `tuya.device.*`), gửi token ghép đôi, phiên bản phần mềm, mã sản phẩm, UUID/AUTHKEY. Cloud trả về `devid`, `seckey`, `localkey`. Thiết bị lưu cục bộ; cờ `resetFactory` từ cloud buộc thiết bị reset nếu cần.
3. **Báo phiên bản** và lấy cấu hình động (múi giờ, biểu giá).
4. **Kết nối MQTT**: `clientId` và `username` là `devid`, mật khẩu dẫn xuất từ khóa. Chủ đề riêng cho mỗi thiết bị: nhận lệnh ở `smart/device/in/<devid>`, gửi ở `smart/device/out/<devid>`. Mọi thông điệp có khung riêng (tiêu đề cố định, số thứ tự, nguồn, CRC32). Mất kết nối thì thử lại với thời gian chờ **tăng dần** (backoff).
5. Cloud có thể ra lệnh reset hoặc hủy ghép; thiết bị xóa dữ liệu kích hoạt rồi về trạng thái ban đầu.
6. Cập nhật firmware (OTA) hỏi và báo trạng thái qua cùng kiểu API HTTPS.

**C. Phía app và người được chia sẻ** (`tuya_sharing/`)
- Đăng nhập bằng **mã QR**: app lấy token QR từ cloud, người dùng quét bằng ứng dụng chính, app hỏi kết quả để nhận `access_token` và `refresh_token`.
- Mọi lời gọi API đều ký (header gồm khóa app, mã yêu cầu, giờ, token, chữ ký) và nội dung được mã hóa bằng AES-GCM. Token hết hạn thì tự làm mới.
- Thông tin MQTT cho app được cấp qua API (`/v1.0/m/life/ha/access/config`) và có hạn (code nói khoảng 2 giờ); app đăng ký theo chủ đề của chủ và của từng thiết bị. Bản 0.2.14 sửa lỗi "bão kết nối lại" khi `client_id` hết hạn: phải **bỏ hẳn client cũ và xin thông tin mới**, không để thư viện tự nối lại bằng thông tin cũ.
- **Xin địa chỉ luồng video** qua một API: gọi cloud với loại luồng (`rtsp`, `hls`, `flv` hoặc `rtmp`) và nhận về một URL. Đây là cách app lấy luồng của camera.

**D. Điều rút ra cho plan**

| Phát hiện | Áp dụng |
|---|---|
| Trạng thái thiết bị tách rõ (chưa kích hoạt → đang kích hoạt → online → mất kết nối → reset) | Dùng làm máy trạng thái của Camera mode (mục 2.2, các giai đoạn 1–10) |
| Dịch vụ tìm địa chỉ theo vùng | Cấu hình tải về theo vùng để chọn signaling và TURN gần nhất |
| Kích hoạt đổi token ghép đôi lấy khóa riêng của thiết bị | Mã QR ghép đôi dùng một lần → `device_id` + khóa riêng (giai đoạn 1–2) |
| Chủ đề riêng cho từng thiết bị, thông tin kết nối có hạn và xin lại | Giữ nguyên: MQTT với thông tin có hạn; xin cấu hình mới trước khi hết hạn, bỏ client cũ rồi nối bằng client mới (giai đoạn 9) |
| Backoff khi mất kết nối; lỗi "bão kết nối lại" | Nối lại với backoff tăng dần có ngẫu nhiên (jitter) để tránh cả đàn nối lại cùng lúc |
| QR đăng nhập cho người được chia sẻ | Có thể dùng cho viewer: quét mã trên máy chủ để vào, không phải gõ mật khẩu (xem S11) |
| API xin URL luồng theo loại | Viewer gọi `POST /sessions`, không giữ URL cố định |

**Không bắt chước:**
- Mã hóa **AES-128-ECB** và **CRC32** cho thông điệp MQTT, vì ECB là chế độ yếu. Dự án dùng TLS cho kênh MQTT/HTTPS và DTLS-SRTP cho media (khác Tuya duy nhất ở bảo mật media, xem `PROTOCOL.md` mục 1.2, D1).
- Chứng chỉ CA nhúng cứng trong firmware (khó cập nhật).
- Mô hình khóa dùng chung UUID/AUTHKEY nạp ở nhà máy: app có thể sinh khóa riêng khi cài.

**E. Luồng video P2P (mã nguồn `tuyaopen/src/tuya_p2p`)**

Repo `arduino-tuyaopen` chỉ là lớp bọc cho board Arduino: kết nối cloud và một camera cục bộ (chụp ảnh JPEG hoặc ghi H.264 ra thẻ SD, hiển thị lên màn hình), **không có truyền video qua mạng**. Phần truyền video thật nằm ở `TuyaOpen/src/tuya_p2p`.

Trình tự khi app muốn xem (đọc từ tên hàm, kiểu dữ liệu và chú thích trong mã, chưa chạy thử):
1. Cloud gửi thông điệp signaling xuống camera qua kênh MQTT của thiết bị (`smart/device/in/<devid>`). Camera đưa vào hàng đợi, một luồng xử lý chạy ICE. Nếu app và camera **cùng mạng LAN** thì signaling đi thẳng qua LAN (loại khung `0x20`), không qua cloud.
2. Hai bên trao **SDP offer/answer** theo dạng WebRTC: phần media audio và video mô tả bằng `UDP/TLS/RTP/SAVPF`, vai DTLS `actpass/active/passive`, video có `rtx`, audio mặc định PCMU 8000 Hz. Có thêm một phần media riêng dạng `application ... tuya` cho kênh lệnh.
3. **ICE** thực hiện bằng thư viện `pjnath` (STUN/TURN). Candidate được trao qua cùng đường signaling; trả lời của camera đi lên bằng `smart/device/out/<devid>` (hoặc qua LAN).
4. Dữ liệu chạy trên **KCP** (giao thức tin cậy xây trên UDP, có kiểm soát nghẽn và pacing; gói có tiêu đề 24 byte, lệnh `81` push, `82` ack, `83` hỏi cửa sổ, `84` báo cửa sổ). Có ba kênh logic: **0 = lệnh, 1 = video, 2 = audio**. Có mã hóa **AES-128-CBC** trên kênh này.
5. Kênh lệnh có tiêu đề với giá trị nhận dạng cố định và bước kiểm mật khẩu. Các lệnh: xem trực tiếp, âm thanh, phát lại, tạm dừng, nói chuyện (loa).
6. Video: các khối H.264 (NAL) được đóng gói thành **RTP** rồi gửi qua kênh 1. Camera nhận các sự kiện: bắt đầu/dừng xem, **yêu cầu khung I (I-frame)**, **đổi bitrate** (kbps), nhận âm thanh từ app.
7. Trong mã, nếu đã có một phiên xem đang hoạt động thì phiên mới bị từ chối (`already active, reject`).

| Rút ra cho plan |
|---|
| Thiết kế của Tuya **rất giống WebRTC**: SDP offer/answer, ICE với STUN/TURN, vai DTLS, RTP cho video, kênh lệnh riêng. Điều này xác nhận chọn WebRTC cho stream là hợp lý, và một camera thật cũng dùng cùng khung kiến trúc |
| Tuya thêm lớp KCP và AES riêng, còn plan dùng DataChannel (SCTP) và DTLS-SRTP chuẩn nên không cần các lớp đó |
| **Đường đi trong LAN** (signaling trực tiếp khi cùng mạng) là ý hay để giảm độ trễ và chi phí; thêm vào danh sách "để dành" (mục 10) |
| Sự kiện **yêu cầu khung I** và **đổi bitrate** là việc camera phải xử lý mỗi khi có viewer mới hoặc mạng xấu đi. Đưa vào S12 (giới hạn người xem theo máy) và `capabilities` |
| Camera Tuya chỉ cho **một phiên xem tại một thời điểm** trong đoạn mã này; với plan, `max_viewers` sẽ phụ thuộc phần cứng, nên cần đo thật |

**F. Định dạng signaling và bảo mật (đọc trong `base_ice/src/tuya_media_service_rtc.c`)**

*JSON signaling.* Mọi thông điệp có hai phần `header` và `msg`:

| Trường | Ý nghĩa |
|---|---|
| `header.from`, `header.to` | Mã định danh hai đầu (thiết bị và app) |
| `header.sessionid` | Mã phiên; offer có mã mới thì phiên cũ bị hủy |
| `header.type` | Loại thông điệp: `offer`, `answer`, `candidate`, ngoài ra có loại báo lỗi và loại đóng phiên |
| `header.trace_id`, `moto_id`, `is_pre`, `p2p_skill`, `path`, `sub_dev_id` | Theo dõi, định tuyến và khả năng |
| `header.security_level` | Mức bảo mật của phiên (số 0–5); thiếu thì mặc định là **3** |
| `msg.sdp` | Chuỗi SDP đầy đủ (với offer/answer) |
| `msg.token`, `udp_token`, `tcp_token` | **Thông tin đăng nhập cho máy chủ ICE (STUN/TURN)** do cloud cấp, gửi kèm trong thông điệp |
| `msg.candidate` | Một candidate ICE (trao dần, kiểu trickle: SDP có `a=ice-options:trickle`) |
| `msg.mode`, `auth`, `stream_type`, `preconnect`, `replay` | Chế độ, xác thực, loại luồng, kết nối trước, phát lại |
| `msg.reason` / `close_reason` | Lý do đóng phiên |
| `msg.handle`, `error`, `seq` | Thông điệp báo lỗi |

*Bảo mật: DTLS và AES phối hợp thế nào.* Theo mã đọc được thì **chúng không phối hợp**:
1. SDP có khai báo kiểu WebRTC (`a=fingerprint`, `a=setup`, vai DTLS), và mã có cấu trúc chứa khóa SRTP từ DTLS. Nhưng phần cài SRTP đã bị **comment** và mình **không tìm thấy lời gọi bắt tay DTLS** trong ba module `base_ice`, `svc_streaming_p2p`, `svc_ipc_core`. Nên khai báo DTLS trong SDP chủ yếu để giữ dạng WebRTC.
2. Bảo vệ thật nằm ở lớp riêng: khóa **AES 16 byte** nằm ngay **trong SDP** (dòng `a=aes-key:<hex>` ở phần media `tuya`), do bên gọi sinh và gửi trong offer. Bên nhận lấy khóa từ SDP.
3. Mỗi mảnh dữ liệu được mã hóa **AES-128-CBC**, IV ngẫu nhiên gắn ở đầu mỗi gói, đệm theo từng khối 16 byte.
4. Mức bảo mật **3** (mặc định) thêm **HMAC-SHA1** vào cuối mỗi gói KCP, ký bằng cùng khóa AES; gói sai chữ ký bị bỏ. Mức **4** có thêm 16 byte chữ ký, nhưng đoạn mã ghi chú "GCM" mà vẫn gọi CBC nên mình **không chắc** mức này hoạt động thế nào.

*Hệ quả.* Khóa phiên đi qua đường signaling (qua cloud), nên **cloud có thể đọc khóa và giải mã media**, không phải mã hóa đầu-cuối thực sự. An toàn của luồng video phụ thuộc vào kênh signaling được bảo vệ. Đây cũng là chỗ khác biệt lớn với WebRTC chuẩn, nơi khóa SRTP sinh từ bắt tay DTLS giữa hai đầu và signaling không thấy khóa.

| Rút ra cho plan |
|---|
| Dùng **DTLS-SRTP chuẩn của WebRTC**: khóa sinh trực tiếp giữa hai đầu, không đi qua signaling. Đây là điểm cải thiện rõ so với cách của Tuya, và hợp với cam kết riêng tư cho camera trong nhà |
| Cấp **thông tin đăng nhập TURN kèm trong thông điệp signaling** (hoặc trong phản hồi tạo phiên), có hạn ngắn, giống `msg.token` của Tuya |
| Mỗi phiên có `sessionid`; offer mới cho cùng cặp thì hủy phiên cũ |
| Có trường **loại thông điệp** gồm offer, answer, candidate, lỗi, đóng phiên (kèm lý do); dùng **trickle ICE** để giảm thời gian chờ |
| Có `security_level` cho tương thích về sau: ở plan có thể gọi là `proto_version` và bắt buộc DTLS |
| Thêm `trace_id` vào mọi thông điệp để dò lỗi từ app đến server |

**Vẫn chưa xác minh:** mức bảo mật 0, 1, 2, 5 và hành vi thật của mức 4; thuật toán dẫn xuất mật khẩu MQTT (dòng `digest[i+4]` trong `mqtt_service.c`); trong các mô-đun khác ngoài ba mô-đun đã tìm, có thể có bắt tay DTLS mà mình chưa thấy. Mình đọc có chọn lọc theo từ khóa nên có thể bỏ sót.

---

## 3. Stack công nghệ (đề xuất, chờ duyệt)

| Lớp | Lựa chọn | Lý do | Phương án khác |
|---|---|---|---|
| App | **Kotlin Multiplatform** trên SDK nội bộ `kmp_bootstrap` (chỉ đạo 2026-11-05) | Camera mode chủ yếu là việc native (Foreground Service, MediaCodec, Keystore); KMP gọi thẳng API Android, phần chung (giao thức, máy trạng thái, MQTT, SDP) viết một lần | Flutter + `flutter_webrtc` (phương án cũ) |
| Camera nền Android | Foreground Service + WakeLock | Cách chuẩn để chạy lâu | |
| Camera nền iOS | Giữ app foreground, màn hình mờ, cắm sạc | iOS không cho chạy camera nền lâu dài | Giới hạn sản phẩm, xem rủi ro R1 |
| **Ngôn ngữ server** | **Chỉ Go** (chỉ đạo) | Một ngôn ngữ cho server, hạ tầng và công cụ kiểm tra | |
| Server API (`cam-api`) | Go, framework **Goravel** (chỉ đạo 2026-11-05) | REST; cụm không giữ trạng thái; có sẵn route, middleware, validation, ORM, queue, schedule, hash, rate limiter | Go `net/http` thuần |
| Server MQTT (`cam-mq`) | Go, thư viện `mochi-mqtt/server` | Kênh điều khiển như Tuya: hiện diện bằng Last Will, QoS 1, ACL theo chủ đề; xác thực bằng hook Go. Dự phòng MQTT qua WSS 443. **Chưa xác minh** về cụm và bảo trì (bản mới nhất trên proxy ghi 2025-03-01) | EMQX (Erlang), Mosquitto (C), vì lệch Go-only |
| DB | PostgreSQL | Tài khoản, camera, quyền | MySQL |
| Presence/pubsub | Valkey (hoặc Redis) | Bản sao presence để đọc nhanh (nguồn gốc là retained MQTT `cam/presence`), phiên đang chạy, giới hạn tốc độ | |
| Server Stream (`cam-stream`) | Go, thư viện `pion/turn` | TURN theo vùng, các node độc lập, dùng chung bí mật HMAC với `cam-api`. **Chưa đo tải** so với coturn | coturn (C), vì lệch Go-only |
| Auth | JWT **HS256** qua guard của Goravel (access ngắn) + refresh token ngẫu nhiên tự viết, đăng nhập email/Apple/Google | Chuẩn, App Store yêu cầu Sign in with Apple nếu có đăng nhập bên thứ ba | |
| Push | FCM, gửi cả Android và iOS (FCM chuyển tiếp qua APNs) | Miễn phí, một thư viện | APNs trực tiếp |
| Quảng cáo | Google AdMob + UMP (đồng ý) + ATT (iOS) | Phổ biến; gọi SDK native mỗi nền tảng qua `expect/actual` | |
| Thanh toán | Apple IAP + Google Play Billing, qua RevenueCat hoặc tự kiểm biên lai | Store bắt buộc với thuê bao số | |
| Giám sát | Prometheus + Grafana + Loki | Bộ giám sát phổ biến, mã nguồn mở | |

### 3.1 Thư viện Go phía server

Bản mới nhất theo Go module proxy ngày 2026-11-05. **Chỉ `cam-api` dùng Goravel**; `cam-mq` và `cam-stream` là server mạng chạy lâu (MQTT, TURN), viết Go thuần cho gọn. Server không cần `pion/webrtc` vì media đi thẳng giữa hai điện thoại.

| Server | Việc | Module | Bản |
|---|---|---|---|
| `cam-api` | Framework | `github.com/goravel/framework` | v1.18.0 |
| `cam-api` | HTTP driver | `github.com/goravel/gin` (hoặc `goravel/fiber`) | v1.18.0 |
| `cam-api` | PostgreSQL (ORM, migration) | `github.com/goravel/postgres` | v1.18.0 |
| `cam-api` | Valkey: cache, giới hạn tốc độ, bản sao hiện diện (cùng giao thức Redis) | `github.com/goravel/redis` | v1.18.0 |
| `cam-api` | Kho log tải lên, URL có chữ ký | `github.com/goravel/s3` (hoặc `goravel/minio` nếu tự host) | v1.18.0 |
| `cam-api` | Client MQTT 5 (ghi `in/*`, `user/*`; nghe `out/+`, `presence/+`) | `github.com/eclipse/paho.golang` | v0.23.0 |
| `cam-api` | Gán shard, băm nhất quán có nút ảo | `github.com/buraksezer/consistent` | v1.1.0 |
| `cam-api` | Push đánh thức (FCM) | `firebase.google.com/go/v4` | v4.22.0 |
| `cam-api` | Kiểm giao dịch App Store, Google Play (nếu không dùng RevenueCat) | `github.com/awa/go-iap` | v1.59.0 |
| `cam-api` | UUID | `github.com/google/uuid` | v1.6.0 |
| `cam-mq` | Broker MQTT, xác thực và ACL qua hàm móc | `github.com/mochi-mqtt/server/v2` | v2.7.9 (bản cuối 2025-03-01, rủi ro bảo trì, thử ở S3) |
| `cam-stream` | TURN (UDP 3478, TCP/TLS 443, HMAC theo thời gian) | `github.com/pion/turn/v4` | v4.1.4 |
| Chung | Số đo | `github.com/prometheus/client_golang` | v1.25.0 |
| Chung | Sinh code từ `openapi.yaml` (tùy chọn) | `github.com/oapi-codegen/oapi-codegen/v2` | v2.8.2 |
| Chung | Mã hóa bổ sung | `golang.org/x/crypto` | v0.58.0 |

Băm mật khẩu dùng Hash của Goravel (argon2id); HMAC, SHA-256 dùng thư viện chuẩn của Go. Không dùng `sideshow/apns2` (bản cuối 2024-10) vì FCM đã gửi được tới iOS.

Test tự động (S7, CI): bot camera và viewer giả bằng `github.com/pion/webrtc/v4` v4.2.26, chỉ dùng trong test, không chạy trong sản phẩm.

### 3.2 App: Kotlin Multiplatform trên `kmp_bootstrap`

App dùng SDK nội bộ [`truongnhan0311/kmp_bootstrap`](https://github.com/truongnhan0311/kmp_bootstrap) (nhánh `claude/keen-pascal-ae6xfi`, chưa merge vào `main`). Tạo dự án: `node tools/new-app.mjs --name recam --id <id> --out ../recam-app --modules network,push,platform,ui,mqtt,webrtc`.

| Việc | Module của kit | Ghi chú |
|---|---|---|
| REST, JWT, làm mới phiên | `appkit-network` | Refresh token dùng một lần, khớp `PROTOCOL.md` 3.1 |
| Push đánh thức (FCM) | `appkit-push` | |
| Khóa thiết bị (`authkey`, `seckey`, `localkey`) | `appkit-core` (`AppStorage`, két an toàn Keystore/Keychain) | |
| MQTT | `appkit-mqtt` (mới) | MQTT 3.1.1 qua WebSocket: Last Will, QoS 1, nối lại `[0, min(60, 2^n)]` giây, hỏi lại mật khẩu và `mqtt_url` mỗi lần nối. Đã test trên JVM, cả WebSocket thật ở localhost |
| WebRTC | `appkit-webrtc` (mới) | `SdpTools` (H.264 rồi VP8, kiểm `a=fingerprint`), `CandidateQueue`, `replaceTrack` cho talkback. Android: `AndroidRtcEngine` trên libwebrtc `io.github.webrtc-sdk:android` 150.7871.04, **chưa biên dịch, chưa chạy trên máy**. **iOS chưa có bản cài** (rủi ro R14) |

Phần riêng của Recam (chủ đề `cam/...`, khung `header`/`msg`, `migrate`, `session.state`, lệnh `ctrl`, `max_viewers`, bậc chất lượng) viết trong app, không đưa vào kit.

---

## 4. Yêu cầu phần cứng

### 4.1 Điện thoại (client)

| | Camera mode | Viewer mode |
|---|---|---|
| Android | ≥ 8.0, nên có bộ mã hóa H.264 phần cứng (không có thì dùng VP8 phần mềm, chỉ 1 viewer), RAM ≥ 3 GB | ≥ 7.0, RAM ≥ 2 GB |
| iOS | iOS ≥ 15, iPhone 8 trở lên | iOS ≥ 15 |
| Mạng | Upload ổn định ≥ 1.5 Mbps **cho mỗi viewer** ở 720p (ví dụ 3 viewer cần ≥ 4.5 Mbps) | Download ≥ 2 Mbps |
| Khác | **Luôn cắm sạc** (tránh nhiệt và chai pin), tắt tối ưu pin cho app | |

Băng thông video mỗi phiên (ước lượng): 480p ≈ 0.5 Mbps, 720p ≈ 1.0–1.5 Mbps, audio Opus ≈ 32 kbps.

### 4.2 Server

Giả định chung: 30% phiên cần TURN (phải đo ở S7), 1 phiên 720p ≈ 1.2 Mbps, mỗi phiên qua TURN tốn 1.2 Mbps vào + 1.2 Mbps ra.

| Hạng mục | Tier A: PoC (≤ 50 phiên đồng thời) | Tier B: MVP (≤ 1.000 camera online, ≤ 200 phiên đồng thời) | Tier C: tăng trưởng (≤ 20.000 camera online, ≤ 3.000 phiên đồng thời) |
|---|---|---|---|
| API + cầu nối MQTT | 1 VM chung: 2 vCPU / 4 GB | 2 VM × (2 vCPU / 4 GB), sau load balancer | 3+ VM × (4 vCPU / 8 GB) |
| Server Stream (TURN, Go) | Cùng VM | 2 VM × (4 vCPU / 8 GB, NIC ≥ 1 Gbps), 2 vùng | N vùng × 2 VM × (8 vCPU / 16 GB, NIC 10 Gbps), theo địa lý người dùng |
| PostgreSQL | Cùng VM | 1 VM 2 vCPU / 4 GB + backup hằng ngày | Managed hoặc primary + replica, 4 vCPU / 16 GB |
| Valkey | Cùng VM | 1 GB, 1 node | 3 node (cluster) |
| Băng thông TURN (đỉnh) | < 20 Mbps | ≈ 60 phiên TURN × 2.4 = **≈ 150 Mbps** | ≈ 900 phiên TURN × 2.4 = **≈ 2.2 Gbps** |
| Ổ đĩa | 20 GB | 50 GB | 200 GB + log lên Loki |
| Ghi chú | Dùng 1 VM rẻ, có IP public | Đủ cho beta kín | Cần hợp đồng băng thông, xem chi phí bên dưới |

**Lưu ý:** gói Plan có nhiều viewer mỗi camera nên tỉ lệ phiên đi qua TURN có thể làm băng thông TURN tăng nhiều hơn mô hình 1 camera - 1 viewer. Con số TURN ở bảng trên coi mỗi phiên là một cặp camera-viewer, nên đã gồm trường hợp nhiều viewer nhưng chưa biết số viewer trung bình thật. S7 phải đo.

**Cơ sở ước lượng chung**
- Kết nối MQTT giữ liên lạc: mỗi camera online giữ một kết nối nhỏ trên Server MQTT. Số kết nối mỗi node của `mochi-mqtt` **chưa đo**; broker khác thường được công bố hàng trăm nghìn kết nối mỗi node. Chia shard theo `devid` để mở rộng ngang. **TURN mới là điểm nghẽn băng thông**; cần đo ở S3 và S7.
- Server Stream (`pion/turn`): chưa có số đo; coturn thường được công bố khoảng 0.5–1 Gbps mỗi vCPU với UDP, nên dùng làm mốc so sánh. Phải đo ở S7.

**Mạng và cổng cần mở**

| Cổng | Giao thức | Dùng cho |
|---|---|---|
| 443 | TCP | HTTPS, MQTT qua WSS (dự phòng) và TURN-over-TLS (qua SNI/chia IP) |
| 8883 | TCP | MQTT qua TLS |
| 3478 | UDP + TCP | STUN/TURN |
| 5349 | TCP | TURN TLS |
| 40000–49999 | UDP | Dải relay của Server Stream (≈ 10.000 allocation đồng thời) |

**Chi phí băng thông (rất thô):** 1 Mbps ≈ 0.45 GB/giờ. 60 phiên TURN × 2 chiều × 1.2 Mbps ≈ 65 GB/giờ đỉnh. Với giá egress cloud công cộng ≈ 0.05–0.09 USD/GB, tương đương vài USD/giờ đỉnh. Nên chọn nhà cung cấp tính theo băng thông gộp (flat-rate) cho coturn. **Cần đo tỉ lệ TURN thật để chốt, vì tỉ lệ này quyết định chi phí.**

**Trung Quốc:** UDP hay bị chặn/giới hạn; phải có TURN TCP/TLS 443 và có thể cần nút TURN đặt riêng trong khu vực. Không chắc app được duyệt trong store Trung Quốc; cần xác nhận riêng.

---

## 5. Các bước thực hiện

Quy ước: **[P]** = có thể chạy song song với các bước cùng nhóm. Mỗi bước có check viết trước (mục 9). Không bước nào vượt mục 7 (Quyền và an toàn).

| ID | Bước | Đọc | Ghi | Check (xem mục 9) | Song song |
|---|---|---|---|---|---|
| S0 | Xác nhận các giả định (mục 11) | `plan.md` | `plan.md` (cập nhật) | C0 | |
| S1 | Spec giao thức: định dạng signaling (REST + MQTT, bám sát Tuya), **vòng đời thiết bị và danh sách `capabilities`** (mục 2.2), schema DataChannel JSON có phiên bản, mô hình quyền | `plan.md` mục 2, 2.1, 2.2 | `docs/PROTOCOL.md`, `docs/openapi.yaml` | C1 | |
| S2 | Hạ tầng PoC **chỉ Go**: tài liệu dựng 1 VM chạy ba tiến trình `cam-api`, `cam-mq`, `cam-stream` (đơn vị systemd, chứng chỉ bằng `autocert`), **chưa tự tạo VM** | `plan.md` mục 4, `ARCHITECTURE.md` | `infra/poc/README.md`, các tệp đơn vị systemd mẫu, `cmd/cam-stream` tối giản | C2 | [P] với S3 |
| S3 | Backend signaling tối thiểu (Go): đăng ký/đăng nhập, kích hoạt thiết bị, Server MQTT nhúng (`cam-mq`), chia shard theo `devid`, cấp thông tin ICE/TURN, nhận log theo yêu cầu | `PROTOCOL.md`, `openapi.yaml` | `server/**` + test | C3 | [P] với S2 |
| S4 | App KMP PoC (`kmp_bootstrap`): Camera mode + Viewer mode, 1-1 | `PROTOCOL.md` | `app/**` | C4 | |
| S5 | Chế độ nền Android (foreground service, wakelock) + thử iOS foreground | `app/**` | `app/android/**`, `docs/BACKGROUND.md` | C5 | |
| S6 | Push đánh thức (FCM/APNs) và presence | `server/**` | `server/push/**`, cập nhật app | C6 | |
| S7 | **Đo đạc**: tỉ lệ P2P/TURN theo mạng, độ trễ, CPU/pin camera, tải Server Stream | `app/**`, `infra/**` | `docs/MEASUREMENTS.md` (số thật, không phải ước lượng) | C7 | |
| S8 | Bảo mật: token ngắn hạn, khóa riêng từng camera, rate limit, kiểm tra quyền truy cập | `server/**` | `docs/SECURITY.md`, test | C8 | [P] với S9 |
| S9 | Giám sát: metrics Prometheus, dashboard, log có cấu trúc | `server/**` | `server/metrics.go`, JSON dashboard | C9 | [P] với S8 |
| S10 | **Cổng quyết định cuối PoC.** Cập nhật sizing bằng số đo S7; báo cáo tổng kết; người phụ trách quyết định đi tiếp sang MVP, đổi hướng hay dừng | tất cả | `plan.md`, `checks.md`, `progress.md` | C10 | |
| S11 | *(MVP)* Chia sẻ và quyền: lời mời, chấp nhận, thu hồi, giới hạn theo gói, `entitlements` | `plan.md` mục 2.1, `openapi.yaml` | `server/sharing/**`, `server/entitlements/**`, cập nhật `openapi.yaml`, test | C11 | |
| S12 | *(MVP)* Giới hạn người xem theo khả năng điện thoại: đo năng lực khi bật Camera mode, báo `max_viewers`, hạ bitrate thích ứng, server cưỡng chế `camera_busy` | `server/**`, `app/**` | `app/lib/capacity/**`, `server/capacity/**`, `docs/CAPACITY.md` | C12 | [P] với S13 |
| S13 | *(MVP)* Quảng cáo: AdMob, đồng ý (UMP/ATT), tần suất, tắt cho Plan | `app/**` | `app/lib/ads/**` | C13 | [P] với S12 |
| S14 | *(MVP)* Thanh toán thuê bao: IAP/Play Billing, thông báo từ store, kiểm biên lai, mở/khóa gói | `server/**`, `app/**` | `server/billing/**`, `app/lib/billing/**` | C14 | |

Thứ tự PoC: S0 → S1 → (S2 ∥ S3) → S4 → S5 → S6 → S7 → (S8 ∥ S9) → S10.
Thứ tự MVP (chỉ bắt đầu sau khi được duyệt ở S10): S11 → (S12 ∥ S13) → S14.
Hai phiên làm việc song song không được cùng sửa một file (S2 và S3 ghi vào thư mục khác nhau, S8 và S9 cũng vậy; S12 sửa phần media của app còn S13 chỉ sửa `app/lib/ads/**`).

---

## 6. Phân công và công cụ

- Việc nào cần áp dụng **mọi lần** thì làm thành hook/script, không để nhớ bằng prompt:
  - `Makefile`: `make test`, `make lint`, `make proto-check` (kiểm tra OpenAPI hợp lệ).
  - Pre-commit: `gofmt`, `go vet`; app: `./gradlew -PdesktopOnly=true desktopTest` (test phần chung trên JVM).
- Công cụ cần có: **Go (toolchain 1.26)**; app: JDK 17+, Android Studio, Xcode (Mac) cho bản Android/iOS. Mọi công cụ phía server và kiểm tra viết bằng Go, không dùng Python hay Node. (Kiểm tra render sơ đồ mermaid vẫn chạy thủ công bằng Node, chỉ để xem tài liệu, không thuộc server.) Nếu môi trường hiện tại thiếu công cụ nào thì ghi blocker vào `progress.md`, không bỏ qua check.
- Hai thiết bị thật (1 Android, 1 iPhone) là điều kiện cho S5 và S7. Emulator không đo được độ trễ, nhiệt, pin. Nếu không có thiết bị thật, S5/S7 để trạng thái `unresolved`, không giả lập kết quả.

---

## 7. Quyền và an toàn

- Chỉ làm việc trong thư mục gốc của dự án này.
- **Hỏi trước khi:** tạo VM/domain/chi tiêu, đẩy lên App Store/Play Store, gửi push thật, tạo tài khoản AdMob/Apple Developer/Google Play, tạo sản phẩm thuê bao, hoặc dùng ad unit/khóa thanh toán thật.
- Môi trường dev chỉ dùng **ad unit thử** và **sandbox** của store. Không bao giờ chạm tiền thật khi chưa hỏi.
- Không lưu secret trong repo; dùng biến môi trường và file `.env.example`.
- Dữ liệu hình ảnh trẻ em: không ghi log nội dung media, không ghi hình ở phase này.

---

## 8. Rủi ro

| ID | Rủi ro | Mức | Hướng giảm |
|---|---|---|---|
| R1 | iOS không cho camera chạy nền lâu → không phù hợp theo dõi em bé qua đêm | **Cao** | Camera mode iOS yêu cầu foreground + cắm sạc; khuyến nghị Android làm camera; nêu rõ trong sản phẩm |
| R2 | Tỉ lệ TURN cao hơn dự đoán → chi phí băng thông vượt | Cao | Đo ở S7 trước khi mở rộng; nhà cung cấp flat-rate |
| R3 | UDP bị chặn (Trung Quốc, mạng doanh nghiệp) | Trung bình | TURN TCP/TLS 443 |
| R4 | Nhiệt/pin điện thoại làm camera | Trung bình | Giới hạn 480p/15fps chế độ tiết kiệm; cảnh báo nhiệt |
| R5 | Quyền riêng tư, pháp lý (COPPA/GDPR, chính sách store) | Cao | Rà soát pháp lý trước beta công khai |
| R6 | Điện thoại camera gánh nhiều viewer thì nóng, tụt pin, hết upload; gói Plan 10 người có thể không bao giờ đạt được trên máy yếu | **Cao** | `max_viewers` theo khả năng máy (mục 2.1), hạ bitrate thích ứng, thông báo rõ cho người dùng; nếu S7 cho thấy máy phổ thông chỉ chịu 2–3 viewer thì cân nhắc SFU (mục 10) hoặc đổi cách quảng bá gói Plan |
| R13 | Số bộ mã hóa phần cứng đồng thời trên điện thoại bị giới hạn (thường vài luồng). Trên Android, libwebrtc (`io.github.webrtc-sdk:android`) **không có H.264 phần mềm**, nên hết H.264 phần cứng thì chỉ còn VP8 phần mềm, rất tốn CPU | Trung bình | Offer khai H.264 và VP8 (`PROTOCOL.md` 7.3); camera dùng VP8 thì `hw_encoder_limit` = 1; S7 đo codec thực tế theo mẫu máy |
| R14 | `appkit-webrtc` chưa có bản cài iOS; `AndroidRtcEngine` chưa biên dịch và chưa chạy trên máy (môi trường dựng kit không có Android SDK). Thư viện KMP dùng chung cho WebRTC (`webrtc-kmp` 0.125.11) đã cũ | **Cao** | S4 làm Android trước; iOS cài `RtcEngine` bằng Swift với pod `WebRTC-SDK` hoặc cinterop. Biên dịch `appkit-webrtc` bản Android trên máy có Android SDK trước khi viết app |
| R7 | Số liệu sizing sai vì chỉ là ước lượng | Trung bình | S7 thay toàn bộ bằng số đo thật |
| R8 | Phí store 15–30% và quy định thanh toán trong app | Trung bình | Tính vào giá; không dẫn người dùng ra ngoài store để trả tiền |
| R9 | Quảng cáo gặp quy định đồng ý (GDPR/UMP, ATT) và chính sách nội dung; app liên quan tới trẻ em dễ bị soi kỹ | Cao | Quảng cáo chỉ phía Viewer, không trong khung video; dùng UMP/ATT; rà chính sách AdMob và store trước khi phát hành; cần tư vấn pháp lý |
| R10 | Người dùng Free tốn băng thông TURN nhiều hơn tiền quảng cáo mang lại | Cao | Đo ở S7; nếu cần, giới hạn chất lượng hoặc thời lượng cho Free (mục 11) |
| R11 | Lộ liên kết mời hoặc người nhận chia sẻ lại cho người khác | Trung bình | Lời mời dùng một lần, hết hạn 24 giờ, chủ thu hồi được, người nhận không chia sẻ tiếp |
| R12 | Lỗi khi hạ gói, hoàn tiền, hết hạn: người dùng mất quyền hoặc giữ quyền sai | Trung bình | Dùng thông báo từ store làm nguồn sự thật; test các trường hợp hết hạn, hoàn tiền, khôi phục mua |

---

## 9. Checks viết trước (acceptance)

Định dạng kết quả sẽ lưu ở `checks.md`: `bước | yêu cầu | verdict | evidence` (verdict: pass / fail / unresolved).

| ID | Yêu cầu | Cách kiểm chứng (tín hiệu thật) |
|---|---|---|
| C0 | Các giả định ở mục 11 đã được người phụ trách xác nhận hoặc điều chỉnh | Đọc `plan.md` |
| C1 | `openapi.yaml` hợp lệ; mọi endpoint có schema lỗi; DataChannel schema có trường `v`; `PROTOCOL.md` có sơ đồ trạng thái và trình tự đủ 10 giai đoạn của mục 2.2 | `make proto-check` |
| C2 | `cam-stream` lên được; một chương trình thử viết bằng Go (thư viện `pion/turn`, client) xin được allocation qua UDP **và** TCP/TLS 443 bằng thông tin đăng nhập do `cam-api` cấp | Output của chương trình thử |
| C3 | `go vet`, `go test ./...` pass; test: đăng nhập, ghép đôi, từ chối truy cập camera không thuộc mình, TURN credential hết hạn đúng giờ | `make test` |
| C4 | Hai thiết bị cùng Wi-Fi xem được nhau; video hiển thị, audio hai chiều | Ảnh chụp màn hình + log ICE |
| C5 | Camera Android chạy ≥ 8 giờ không bị giết | Log thời gian + trạng thái kết nối |
| C6 | App ở nền bị đánh thức bằng push và vào phiên ≤ 5 giây | Log thời gian push → ICE connected |
| C7 | Có bảng đo thật: tỉ lệ P2P/TURN theo ≥ 4 loại mạng (Wi-Fi↔Wi-Fi, 4G↔Wi-Fi, 4G↔4G, mạng chặn UDP), độ trễ trung vị, CPU/pin camera, băng thông coturn, và **số viewer đồng thời tối đa mỗi mẫu điện thoại chịu được** (1, 2, 3, 5, 10) | `MEASUREMENTS.md` + dữ liệu thô |
| C8 | Không truy cập chéo tài khoản được; token hết hạn bị từ chối; rate limit hoạt động | Test tự động |
| C9 | Metrics `/metrics` có: số phiên, tỉ lệ P2P/TURN, lỗi ICE; dashboard hiển thị | Ảnh chụp dashboard |
| C10 | `plan.md` mục 4.2 được cập nhật bằng số đo; có khuyến nghị đi tiếp có số liệu | Diff `plan.md` |

| C11 | Free: lời mời thứ 2 bị từ chối `402 plan_limit_reached`. Plan: nhận đủ 10 người, người thứ 11 bị từ chối. Lời mời hết hạn và dùng lại đều bị từ chối. Thu hồi cắt phiên đang xem ≤ 5 giây. Người được chia sẻ không tự mời thêm được | Test tự động + log thời gian cắt phiên |
| C12 | Với ít nhất 2 mẫu điện thoại khác nhau (một máy mạnh, một máy yếu): `max_viewers` báo về server khác nhau và hợp lý; viewer vượt giới hạn nhận `camera_busy`; khi nhiệt/CPU xấu đi, bitrate hạ trước rồi `max_viewers` mới giảm; camera không bị treo hay bị hệ điều hành giết khi chạy ở giới hạn trong 30 phút | Log `max_viewers` theo thời gian + đo CPU/nhiệt/upload trong `MEASUREMENTS.md` |
| C13 | Quảng cáo chỉ hiện với tài khoản Free ở phía Viewer; Plan không thấy; Camera mode không có; hộp thoại đồng ý hiện với thiết bị EU; không có quảng cáo khi đang phát video | Ảnh chụp màn hình các trường hợp |
| C14 | Mua thuê bao bằng sandbox trên cả Apple và Google; quyền đổi trong ≤ 60 giây; hết hạn và hoàn tiền hạ gói đúng; "khôi phục mua" hoạt động; biên lai giả bị server từ chối | Log thông báo từ store + test |

Điều kiện dừng toàn bộ: C0–C10 (PoC) đều có verdict; C11–C14 áp dụng cho MVP sau khi được duyệt ở S10. Hoặc có blocker/giới hạn lặp được ghi rõ ở trạng thái `unresolved`. 

---

## 10. Ngoài phạm vi PoC (để dành)

- SFU (ví dụ LiveKit): chỉ cân nhắc nếu S7 cho thấy điện thoại chịu được quá ít viewer, hoặc khi cần ghi hình cloud. Khi đó camera gửi một luồng và SFU nhân bản.
- Ghi hình lưu cloud.
- Cầu nối RTSP/ONVIF (go2rtc) để xem thêm camera IP thông thường trong cùng app qua WHEP.
- Matter 1.5 camera (cần kiểm tra lại đặc tả trước khi cam kết).
- AI: phát hiện tiếng khóc, chuyển động.
- Thông báo sự kiện qua MQTT.
- Đường signaling trực tiếp trong mạng LAN (khi viewer và camera cùng mạng) để giảm độ trễ.

---

## 11. Giả định và quyết định đã chốt

Các điểm dưới đây là giả định làm nền cho plan. Nếu một giả định thay đổi thì cập nhật mục liên quan.

| # | Chủ đề | Giả định / quyết định |
|---|---|---|
| 1 | Mô hình sản phẩm | Hai gói: **Free** (có quảng cáo, chia sẻ cho 1 người) và **Plan trả phí** (không quảng cáo, chia sẻ cho 10 người). Giá chưa chốt |
| 2 | Nền tảng | iOS và Android; Camera mode ưu tiên Android vì iOS hạn chế chạy nền |
| 3 | Thị trường | Chưa nhắm Trung Quốc trong PoC |
| 4 | Hạ tầng PoC | Một VM rẻ trên nhà cung cấp cloud bất kỳ (DigitalOcean, AWS, Hetzner…); chưa tạo gì khi chưa được duyệt chi |
| 5 | Mã nguồn | Dự án độc lập, repo riêng |
| 6 | Thiết bị đo | Cần ít nhất 1 Android và 1 iPhone thật; thiếu thì S5/S7 để `unresolved` |
| 7 | Công nghệ app | Kotlin Multiplatform trên `kmp_bootstrap` (đổi từ Flutter, 2026-11-05) |
| 8 | Số người xem cùng lúc | `min(giới hạn của gói, khả năng điện thoại camera)`. Không dùng SFU ở MVP |
| 9 | Ưu tiên khi quá tải | Viewer vào trước được giữ; chủ camera luôn được ưu tiên; viewer bị từ chối thấy thông báo `camera_busy` |
| 10 | Giới hạn gói Free | 1 camera, tối đa 480p. Plan: 5 camera, tối đa 720p. Chưa giới hạn thời lượng xem |
| 11 | Người được chia sẻ | Xem miễn phí. Quảng cáo theo gói của **chủ camera** (chủ Free thì người xem thấy quảng cáo) |
| 12 | Quảng cáo và thanh toán | Google AdMob; Apple IAP và Google Play Billing. Chưa có tài khoản nào, cần tạo ở giai đoạn MVP |

---

## 12. Giao nhận

Khi hoàn tất: mã nguồn dự án, `plan.md`, `checks.md`, `progress.md`, diff cho `CLAUDE.md` nếu có quy tắc mới (sẽ đề xuất tạo, không tự tạo khi chưa duyệt), và harness này lưu thành template dùng lại sau khi được xem lại.

**⏸ Dừng ở đây. Cần phê duyệt `plan.md` (và xác nhận các giả định ở mục 11) trước khi chuyển sang S1.**
