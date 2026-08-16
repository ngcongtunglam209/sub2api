# Tài liệu API

Base URL: `{{SITE_ORIGIN}}`. Mọi đường dẫn dưới đây đều cần API key — xem
[Xác thực](/docs/authentication).

Request body và response body giữ nguyên định dạng của chính nhà cung cấp
upstream. Cổng này không tự đặt ra một schema mới: request body của Anthropic
vẫn là request body của Anthropic. Vì vậy hãy đọc mỗi dòng endpoint dưới đây
như *"gửi đến đâu"*, còn tài liệu chính thức của nhà cung cấp là *"bên trong
chứa gì"*. Endpoint duy nhất thuộc riêng về cổng này là
`GET /v1/sub2api/billing`, được mô tả trong
[Thanh toán và mức sử dụng](/docs/billing-and-usage).

Tính khả dụng phụ thuộc vào nền tảng của nhóm mà key bạn thuộc về. Một đường
dẫn mà nền tảng của nhóm bạn không phục vụ sẽ trả về `403`, không phải `404`.

## Văn bản

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `POST` | `/v1/messages` | Anthropic Messages. Streaming với `"stream": true`. |
| `POST` | `/v1/messages/count_tokens` | Đếm token. Kiểm tra hạn mức, không ghi nhận mức sử dụng. Cũng có tại `/messages/count_tokens`. |
| `POST` | `/v1/chat/completions` | OpenAI Chat Completions. |
| `POST` | `/v1/responses` | OpenAI Responses. Cũng có tại `/responses`. |
| `POST` | `/v1/responses/{subpath}` | Tài nguyên con của Responses, ví dụ như hủy. |
| `GET` | `/v1/responses` | Lấy một response. |
| `POST` | `/v1/embeddings` | OpenAI embeddings. |
| `GET` | `/v1/models` | Các model mà key bạn có thể gọi. Cũng có tại `/models`. |

## Gemini

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `GET` | `/v1beta/models` | Liệt kê model. |
| `GET` | `/v1beta/models/{model}` | Lấy thông tin một model. |
| `POST` | `/v1beta/models/{model}:generateContent` | Tạo nội dung. |
| `POST` | `/v1beta/models/{model}:streamGenerateContent` | Tạo nội dung dạng streaming. |

Chỉ dành cho các nhóm nền tảng Google. Lỗi ở đây dùng cấu trúc lỗi của Google.

## Hình ảnh

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `POST` | `/v1/images/generations` | Tạo đồng bộ. |
| `POST` | `/v1/images/edits` | Chỉnh sửa đồng bộ. |
| `POST` | `/v1/images/generations/async` | Gửi và nhận task id. |
| `POST` | `/v1/images/edits/async` | Gửi và nhận task id. |
| `GET` | `/v1/images/tasks/{task_id}` | Kiểm tra một task bất đồng bộ. |

Các tác vụ tạo mất nhiều thời gian chính là lý do tồn tại của bất đồng bộ:
gửi, giữ `task_id`, rồi kiểm tra. Việc kiểm tra là thao tác đọc và không bị
tính phí thêm lần nữa.

### Xử lý hình ảnh theo lô (batch)

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `POST` | `/v1/images/batches` | Gửi một lô. |
| `GET` | `/v1/images/batches` | Liệt kê các lô của bạn. |
| `GET` | `/v1/images/batches/models` | Các model khả dụng cho xử lý theo lô. |
| `GET` | `/v1/images/batches/{id}` | Trạng thái của lô. |
| `GET` | `/v1/images/batches/{id}/items` | Trạng thái từng mục. |
| `GET` | `/v1/images/batches/{id}/items/{custom_id}/content` | Kết quả của một mục. |
| `GET` | `/v1/images/batches/{id}/download` | Tải toàn bộ lô về trong một lần. |
| `POST` | `/v1/images/batches/{id}/cancel` | Hủy các mục đang chờ xử lý. |

Một hướng dẫn thực hành đầy đủ với request body nằm trong dashboard, tại mục
**Batch Image Guide** — bạn cần đăng nhập để đọc.

## Video

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `POST` | `/v1/videos`, `/v1/videos/generations` | Bắt đầu tạo. |
| `POST` | `/v1/videos/edits` | Bắt đầu chỉnh sửa. |
| `POST` | `/v1/videos/extensions` | Mở rộng một video đã có. |
| `GET` | `/v1/videos/{request_id}` | Trạng thái. Cũng có `/v1/videos/generations/{request_id}` và các biến thể `edits` / `extensions`. |
| `GET` | `/v1/videos/{request_id}/content` | Tải kết quả về. |

Video luôn hoạt động bất đồng bộ: bắt đầu, kiểm tra trạng thái, rồi lấy nội
dung.

## Âm thanh

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `POST` | `/v1/tts` | Chuyển văn bản thành giọng nói. |
| `POST` | `/v1/stt` | Chuyển giọng nói thành văn bản. |
| `POST` | `/v1/custom-voices` | Tạo một giọng nói tùy chỉnh. |
| `GET` | `/v1/custom-voices` | Liệt kê các giọng nói tùy chỉnh. |
| `GET` | `/v1/custom-voices/{voice_id}` | Lấy thông tin một giọng nói. |
| `GET` | `/v1/custom-voices/{voice_id}/audio` | Lấy mẫu âm thanh của giọng nói đó. |

## Realtime và live

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `GET` | `/v1/realtime` | Phiên realtime. |
| `POST` | `/v1/live` | Bắt đầu một cuộc gọi live. |
| `GET` | `/v1/live/{call_id}` | Kênh phụ (sideband) của live. |
| `POST` | `/backend-api/codex/realtime/calls` | Cuộc gọi live trực tiếp qua Codex. |

## Tìm kiếm

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `POST` | `/v1/web_search` | Tìm kiếm web. Nhóm Grok. |
| `POST` | `/v1/x_search` | Tìm kiếm X. Nhóm Grok. |
| `POST` | `/v1/alpha/search` | Cũng có tại `/alpha/search`. |

## Tài khoản

| Method | Đường dẫn | Ghi chú |
| --- | --- | --- |
| `GET` | `/v1/sub2api/billing` | Hệ số nhân giá đang áp dụng cho key này. |
| `GET` | `/v1/usage` | Mức tiêu thụ của bạn. Tùy chọn `days`, 1–90. |

Xem [Thanh toán và mức sử dụng](/docs/billing-and-usage).
