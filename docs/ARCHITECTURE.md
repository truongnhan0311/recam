# ARCHITECTURE.md: quan hệ App – Camera – Server

Bản nháp S1 v0.3. Các sơ đồ dưới đây vẽ bằng mermaid; ảnh PNG tương ứng nằm trong `diagrams/`.
Đi kèm `PROTOCOL.md` (chi tiết thông điệp) và `openapi.yaml`.

## 0. Quyết định kiến trúc đang áp dụng

| # | Quyết định | Nguồn |
|---|---|---|
| A1 | Toàn bộ server và hạ tầng **chỉ viết bằng Go** | Chỉ đạo |
| A2 | **Ba loại server tách rời**: **Server API**, **Server MQTT**, **Server Stream** (TURN). Mỗi loại chạy được trên máy riêng và mỗi loại **có thể là một cụm (cluster)** | Chỉ đạo |
| A3 | Ở bản PoC cả ba chạy chung một máy dưới dạng ba tiến trình, vẫn tách ranh giới như khi chạy riêng | Thiết kế |
| A4 | Bám sát thiết kế Tuya cho vòng đời, MQTT và định dạng thông điệp; media dùng WebRTC chuẩn | Chỉ đạo |

Tên ba chương trình Go (đề xuất): `cam-api`, `cam-mq`, `cam-stream`.

> Chưa xác minh: thư viện Go dự kiến cho `cam-mq` là `mochi-mqtt/server` (bản mới nhất trên proxy ghi ngày 2025-03-01, lâu hơn các thư viện khác) và cho `cam-stream` là `pion/turn` (bản 2026-01). Theo hiểu biết của mình `mochi-mqtt` **không có cụm sẵn**, nên thiết kế dưới đây chia broker theo `devid` (mục 3) để không cần cụm broker. Cần kiểm lại khi cài ở S3.

---

## 1. Bức tranh tổng thể

![Tổng thể](diagrams/01-tong-the.png)

```mermaid
flowchart TB
    subgraph PH["HAI ĐIỆN THOẠI"]
        direction LR
        CAMAPP["APP CAMERA<br/>điện thoại cũ, Camera mode<br/>giữ khóa, mã hóa, gửi video"]
        VIEWAPP["APP XEM<br/>Viewer mode<br/>xem video, hiện quảng cáo"]
    end
    subgraph SRV["SERVER - tất cả viết bằng Go"]
        direction LR
        MQ["SERVER MQTT<br/>hiện diện, đẩy lệnh,<br/>chuyển signaling"]
        API["SERVER API<br/>REST: tài khoản, kích hoạt,<br/>chia sẻ, tạo phiên, log"]
        ST["SERVER STREAM<br/>TURN relay media"]
    end
    subgraph EXT["KHO DỮ LIỆU VÀ DỊCH VỤ NGOÀI"]
        direction LR
        DB[("PostgreSQL")]
        KV[("Valkey")]
        PUSH["FCM / APNs"]
        STORE["Apple / Google<br/>thanh toán"]
        ADS["AdMob"]
    end

    CAMAPP <==>|"WebRTC trực tiếp P2P<br/>video, audio, lệnh ctrl"| VIEWAPP
    CAMAPP -->|"HTTPS có ký"| API
    VIEWAPP -->|"HTTPS và JWT"| API
    CAMAPP <-->|"MQTT TLS"| MQ
    VIEWAPP <-->|"MQTT TLS, chỉ nhận"| MQ
    CAMAPP -.->|"TURN nếu P2P hỏng"| ST
    VIEWAPP -.->|"TURN nếu P2P hỏng"| ST
    API <-->|"publish và nhận"| MQ
    API -->|"cấp thông tin TURN"| ST
    API --> DB
    API --> KV
    API --> PUSH
    STORE --> API
    VIEWAPP --> ADS
    PUSH -.->|"đánh thức camera"| CAMAPP
```

**Đọc sơ đồ:** có ba "đường" tách biệt.
1. **Đường điều khiển (REST)** đi tới Server API: tài khoản, kích hoạt, tạo phiên, chia sẻ, log.
2. **Đường đẩy tin (MQTT)** đi qua Server MQTT: lệnh xuống camera, hiện diện, `answer`/`candidate` giữa hai điện thoại. Người dùng chỉ **nhận** trên MQTT, không ghi.
3. **Đường media (WebRTC)** đi **thẳng giữa hai điện thoại**. Server Stream chỉ chuyển gói đã mã hóa khi đi thẳng thất bại. Không server nào thấy nội dung video hay khóa.

### Ai nói chuyện với ai

| Từ | Đến | Giao thức | Mục đích |
|---|---|---|---|
| App camera | Server API | HTTPS, yêu cầu có ký (`seckey`) | Kích hoạt, báo thông tin khi khởi động, báo cáo phiên, trả log |
| App camera | Server MQTT | MQTT qua TLS (8883, dự phòng 443) | Nhận lệnh và signaling, báo DP, báo hiện diện |
| App xem | Server API | HTTPS, JWT | Đăng nhập, danh sách camera, tạo phiên, gửi `offer`/`candidate`, chia sẻ |
| App xem | Server MQTT | MQTT qua TLS | **Chỉ nhận**: `answer`, `candidate`, `disconnect`, DP |
| Server API | Server MQTT | MQTT (client) | Ghi lệnh/signaling vào chủ đề của camera và của người xem; nhận thông điệp camera gửi lên và hiện diện |
| Server API | Server Stream | Bí mật chung (TURN REST) | Cấp thông tin TURN có hạn cho từng phiên |
| Camera ↔ App xem | Nhau | WebRTC (DTLS-SRTP, DataChannel) | Video, audio, lệnh `ctrl` |
| Camera, App xem | Server Stream | TURN (UDP 3478, TCP/TLS 443) | Chỉ khi đi thẳng thất bại |
| Server API | FCM / APNs | HTTPS | Đánh thức camera ở nền |
| Apple / Google | Server API | Thông báo máy chủ | Cập nhật gói thuê bao |

---

## 2. Từng thành phần giữ gì

| Thành phần | Giữ dữ liệu gì | Không bao giờ giữ |
|---|---|---|
| **App camera** | `uuid`, `authkey`, `devid`, `seckey`, `localkey` trong Keystore/Keychain | Khóa người dùng khác |
| **App xem** | Token đăng nhập, cấu hình MQTT có hạn | `seckey` của camera |
| **Server API** | Tài khoản, camera, chia sẻ, gói, lịch sử yêu cầu log, **băm** của `authkey` và lời mời; `seckey` mã hóa bằng KMS | Nội dung media, khóa DTLS |
| **Server MQTT** | Kết nối đang mở, hàng đợi QoS 1 ngắn hạn, thông điệp retained của hiện diện | Tài khoản, quyền (hỏi Server API khi kiểm đăng nhập) |
| **Server Stream** | Allocation TURN đang chạy (trong bộ nhớ) | Mọi thứ khác; không biết ai là ai ngoài `sessionid` trong tên đăng nhập TURN |
| **PostgreSQL** | Dữ liệu bền: tài khoản, camera, chia sẻ, gói | |
| **Valkey** | Phiên đang chạy, hiện diện nhanh, giới hạn tốc độ | |

---

## 3. Chạy thành cụm (cluster)

Ba loại server **độc lập**: có thể mở rộng, thay thế hay đặt ở vùng khác nhau mà không đụng loại kia.

![Cụm](diagrams/02-cum.png)

```mermaid
flowchart TB
    subgraph CLIENT["ỨNG DỤNG"]
        CAM["App camera xN"]
        VIEW["App xem xN"]
    end
    LB["Cân bằng tải HTTPS"]
    subgraph APIC["CỤM SERVER API - không giữ trạng thái"]
        A1["cam-api 1"]
        A2["cam-api 2"]
        A3["cam-api N"]
    end
    subgraph MQC["CỤM SERVER MQTT - chia theo devid"]
        M1["cam-mq shard 1"]
        M2["cam-mq shard 2"]
        M3["cam-mq shard N"]
    end
    subgraph STC["CỤM SERVER STREAM - theo vùng"]
        T1["cam-stream sg"]
        T2["cam-stream eu"]
        T3["cam-stream us"]
    end
    subgraph DATA["DỮ LIỆU"]
        PG[("PostgreSQL<br/>chính và bản sao")]
        VK[("Valkey")]
    end

    CAM -->|HTTPS| LB
    VIEW -->|HTTPS| LB
    LB --> APIC
    CAM -->|"MQTT: nối vào shard<br/>do API chỉ định"| MQC
    VIEW -->|"MQTT: nối vào shard<br/>do API chỉ định"| MQC
    APIC -->|"publish tới đúng shard<br/>shard = băm của devid"| MQC
    MQC -->|"hiện diện, thông điệp từ camera"| APIC
    APIC --> PG
    APIC --> VK
    APIC -->|"chọn node gần nhất<br/>cấp thông tin TURN"| STC
    CAM <==>|"media P2P"| VIEW
    CAM -.->|"nếu cần"| STC
    VIEW -.->|"nếu cần"| STC
```

### 3.1 Server API: cụm không giữ trạng thái
- Nhiều bản `cam-api` sau cân bằng tải. Mọi trạng thái bền nằm trong PostgreSQL, trạng thái nóng trong Valkey. Bản nào cũng xử lý được mọi yêu cầu.
- Mỗi bản `cam-api` là **client MQTT** của mọi shard (để ghi vào chủ đề của camera bất kỳ).

### 3.2 Server MQTT: chia shard theo `devid`
Vấn đề của cụm: camera chỉ nối vào **một** node broker. Muốn gửi `offer` tới camera, Server API phải ghi **đúng node đang giữ kết nối đó**.

Cách giải (không cần broker có cụm sẵn):
- Mỗi thiết bị có một **shard cố định** `shard = consistent_hash(devid)` trên danh sách node broker đang sống; lưu trong CSDL (`device.mq_node`).
- Địa chỉ broker **không** cố định trong app. Nó đến từ API: camera nhận `mqtt_url` trong `PUT /devices/{devid}/info` (giống `mqttsSelfUrl` của Tuya); app xem nhận `url` trong `POST /mqtt/access-config` (shard của `uid`).
- Mất một node: thiết bị nối lại thất bại → hỏi API lấy `mqtt_url` mới (shard được gán lại) → nối vào node khác. Broker mới có hàng đợi trống, nên thông điệp đang bay có thể mất; phiên đang bay hết hạn theo thời hạn pha (PROTOCOL.md 7.2).
- Nếu sau này chọn broker có cụm sẵn thì **bỏ phần shard**, mọi node nhận mọi chủ đề.

![Định tuyến shard](diagrams/03-dinh-tuyen-shard.png)

```mermaid
sequenceDiagram
    autonumber
    participant V as App xem
    participant LB as Cân bằng tải
    participant A as cam-api (node bất kỳ)
    participant DB as PostgreSQL
    participant MC as Broker shard của camera
    participant C as App camera
    participant MV as Broker shard của người xem

    V->>LB: POST /sessions/id/signal (offer)
    LB->>A: chuyển tới một node API
    A->>DB: kiểm quyền, lấy shard của camera
    A->>MC: publish cam/device/in/devid (offer)
    MC->>C: offer (camera đang nối vào shard này)
    C->>MC: publish cam/device/out/devid (answer)
    MC->>A: answer (API đăng ký out/+ trên mọi shard)
    A->>DB: lấy shard của người xem
    A->>MV: publish cam/user/uid (answer)
    MV->>V: answer (app xem đang nối vào shard này)
```

### 3.3 Server Stream: node độc lập, theo vùng
- Mỗi node `cam-stream` chạy TURN **không cần biết các node khác**. Tất cả dùng chung một bí mật HMAC với `cam-api` (TURN REST), nên thông tin đăng nhập do `cam-api` tạo hợp lệ trên mọi node.
- `cam-api` chọn **vài node gần cả hai đầu** (theo vùng và tải) rồi đưa vào `token`, `udp_token`, `tcp_token` của phiên. Mở rộng = thêm node và đăng ký vào danh sách; không phải đồng bộ trạng thái.
- Tên "Server Stream" để dành chỗ cho loại server media khác về sau (ví dụ SFU) mà không đổi sơ đồ.

---

## 4. Chủ đề MQTT và quyền

![Chủ đề MQTT](diagrams/04-chu-de-mqtt.png)

```mermaid
flowchart LR
    CAM["App camera<br/>(devid)"]
    VIEW["App xem<br/>(uid)"]
    API["cam-api"]
    subgraph TOPICS["Server MQTT - chủ đề"]
        IN["cam/device/in/devid"]
        OUT["cam/device/out/devid"]
        PRES["cam/presence/devid<br/>retained + Last Will"]
        USR["cam/user/uid"]
    end

    API ==>|"ghi lệnh xuống camera"| IN
    IN -->|"camera NGHE"| CAM
    CAM ==>|"ghi tin camera gửi lên"| OUT
    OUT -->|"API NGHE"| API
    CAM ==>|"ghi: online, offline qua LWT"| PRES
    PRES -->|"API NGHE"| API
    API ==>|"ghi tin cho người xem"| USR
    USR -->|"app xem NGHE"| VIEW
    RULE["QUY TẮC<br/>App xem chỉ NGHE, không ghi<br/>Chỉ cam-api ghi vào in và user"]
    VIEW ~~~ RULE
```

| Chủ đề | Thông điệp trong đó |
|---|---|
| `cam/device/in/{devid}` | `offer`, `candidate`, `disconnect`, `dp.set`, `reset`, `log.request`, `error` |
| `cam/device/out/{devid}` | `answer`, `candidate`, `disconnect`, `dp.report`, `error` |
| `cam/presence/{devid}` | `{"v":1,"online":true/false}` (retained, Last Will) |
| `cam/user/{uid}` | `answer`, `candidate`, `disconnect`, `dp.report`, `log.request`, `error` |

Quy tắc ACL: thiết bị chỉ nghe `in/<devid của mình>`, chỉ ghi `out` và `presence` của mình. Người dùng chỉ nghe `user/<uid của mình>` và **không ghi**. Chỉ `cam-api` ghi vào `in/*` và `user/*`.

---

## 5. Từ camera mới đến lúc xem được

![Vòng đời](diagrams/05-vong-doi.png)

```mermaid
sequenceDiagram
    autonumber
    participant O as Chủ (app xem)
    participant C as App camera (điện thoại cũ)
    participant A as Server API
    participant M as Server MQTT
    participant S as Server Stream

    Note over O,C: GHÉP ĐÔI VÀ KÍCH HOẠT (một lần)
    O->>A: POST /pairing-tokens
    A-->>O: token 8 ký tự, 10 phút
    O-->>C: quét QR hoặc nhập token
    C->>A: POST /url_config (vùng)
    A-->>C: địa chỉ API
    C->>A: POST /devices/activate (token, uuid, authkey)
    A-->>C: devid, seckey, localkey
    C->>A: PUT /devices/devid/info (có ký)
    A-->>C: cấu hình, giờ server, mqtt_url
    Note over C,M: GIỮ LIÊN LẠC (liên tục)
    C->>M: nối MQTT, presence online, Last Will offline
    C->>M: dp.report (pin, max_viewers) định kỳ
    Note over O,S: XEM (mỗi lần)
    O->>A: POST /sessions (camera_id)
    A-->>O: thông tin ICE kèm TURN
    O->>A: POST signal (offer)
    A->>M: offer tới camera
    M->>C: offer
    C->>M: answer
    M->>O: answer (qua API)
    O->>C: video.start, audio.start (DataChannel)
    C-->>O: video và audio (DTLS-SRTP)
    Note over O,S: nếu đi thẳng thất bại thì qua Server Stream
```

---

## 6. Hai đường đi của media

![Media](diagrams/06-media.png)

```mermaid
flowchart LR
    subgraph NA["Mạng nhà người đặt camera - sau NAT"]
        CAM["App camera"]
    end
    subgraph NB["Mạng của người xem - sau NAT"]
        VIEW["App xem"]
    end
    STUN["STUN<br/>(trong Server Stream)"]
    TURN["TURN relay<br/>(Server Stream)"]
    CAM -.->|"hỏi địa chỉ công cộng"| STUN
    VIEW -.->|"hỏi địa chỉ công cộng"| STUN
    CAM ==>|"ĐƯỜNG 1 - ưu tiên:<br/>đi thẳng P2P, mã hóa DTLS-SRTP"| VIEW
    CAM -->|"ĐƯỜNG 2 - dự phòng:<br/>gói đã mã hóa"| TURN
    TURN -->|"chuyển tiếp nguyên gói"| VIEW
```

- Hai bên thử đường 1 trước (ICE). Không được trong 20 giây thì dùng đường 2.
- Ở cả hai đường, **khóa media sinh giữa hai điện thoại** (DTLS), nên Server Stream chỉ thấy gói mã hóa.
- Mạng chặn UDP: TURN qua TCP/TLS cổng 443 (`tcp_token`).

---

## 7. Cách triển khai

![Triển khai](diagrams/07-trien-khai.png)

```mermaid
flowchart TB
    subgraph POC["GIAI ĐOẠN PoC - một máy"]
        direction TB
        VM["1 VM, 2 vCPU / 4 GB"]
        P1["cam-api<br/>HTTPS 443"]
        P2["cam-mq<br/>MQTT 8883"]
        P3["cam-stream<br/>TURN 3478 và 443"]
        P4[("PostgreSQL")]
        VM --- P1
        VM --- P2
        VM --- P3
        VM --- P4
    end
    subgraph PROD["MVP VÀ SAU ĐÓ - tách máy, thành cụm"]
        direction TB
        L["Cân bằng tải"]
        G1["Nhóm máy cam-api<br/>2 trở lên"]
        G2["Nhóm máy cam-mq<br/>chia shard"]
        G3["Nhóm máy cam-stream<br/>theo vùng"]
        G4[("PostgreSQL chính + bản sao<br/>Valkey")]
        L --> G1
        G1 --> G2
        G1 --> G3
        G1 --> G4
    end
    POC -->|"tách tiến trình ra máy riêng,<br/>không đổi giao thức"| PROD
```

- Ba chương trình Go **tách ranh giới ngay từ PoC**: chúng nói với nhau bằng đúng các giao thức ở mục 1 dù cùng một máy, nên chuyển sang cụm không đổi giao thức.
- Chứng chỉ TLS: dùng thư viện ACME của Go (`autocert`), không cần chương trình ngoài.
- PostgreSQL và Valkey là kho dữ liệu dùng sẵn, không phải mã do dự án viết.

---

## 8. Việc còn mở về kiến trúc

| # | Vấn đề | Cách giải |
|---|---|---|
| 1 | `mochi-mqtt` chưa xác minh về cụm, bảo trì (bản mới nhất 2025-03-01), tải thực tế | Thử ở S3; phương án dự phòng: viết broker tối giản bằng Go hoặc chấp nhận một broker ngoài Go (cần bạn quyết định vì lệch chỉ đạo Go-only) |
| 2 | `pion/turn` chưa đo tải so với coturn | Đo ở S7; coturn là phương án dự phòng (viết bằng C, cũng lệch Go-only) |
| 3 | `cam-api` đăng ký `out/+` trên **mọi** shard: số kết nối API × shard tăng nhanh | Đo ở S3; có thể dùng đăng ký chia sẻ của MQTT 5 nếu broker hỗ trợ (chưa xác minh) |
| 4 | Shard bị lệch tải hoặc mất node: cách gán lại | Băm nhất quán + lưu `mq_node` trong CSDL; chốt ở S3 |
| 5 | Hiện diện: lấy từ `presence` retained, hay đồng bộ vào Valkey | Chốt ở S3 |
| 6 | Cần vẽ thêm sơ đồ cho luồng chia sẻ, thanh toán, log theo yêu cầu | Log đã có ở `PROTOCOL.md` mục 10; chia sẻ và thanh toán vẽ ở S11, S14 |
