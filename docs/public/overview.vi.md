# Tổng quan

Đây là một cổng API cho AI. Bạn chỉ cần một API key và một Base URL để sử dụng
các định dạng request mà bạn đã quen thuộc — Anthropic Messages, OpenAI Chat
Completions, OpenAI Responses và Google Gemini — tất cả đều được phục vụ từ
cùng một host.

Nếu code của bạn đã gọi Anthropic, OpenAI hoặc Gemini, bạn chỉ cần đổi hai thứ:
Base URL và key. Mọi thứ khác giữ nguyên.

## Base URL

```
{{SITE_ORIGIN}}
```

Có ba tiền tố nằm dưới host đó:

| Tiền tố | Giao thức | Ví dụ |
| --- | --- | --- |
| `/v1` | Anthropic và tương thích OpenAI | `POST /v1/messages`, `POST /v1/chat/completions` |
| `/v1beta` | Google Gemini | `POST /v1beta/models/{model}:generateContent` |
| `/backend-api/codex` | Truy cập trực tiếp Codex | `POST /backend-api/codex/responses` |

Một số đường dẫn kiểu OpenAI cũng được phục vụ ở root, không có tiền tố `/v1`
— `POST /responses`, `GET /models`, `POST /messages/count_tokens` — nên các
client hardcode đường dẫn không có version vẫn hoạt động bình thường.

## Key và nhóm (group)

Hai yếu tố quyết định một request có thể làm gì:

- **API key của bạn** xác định danh tính và mang theo hạn mức của bạn. Nó bắt
  đầu bằng `sk-` và được tạo trong dashboard tại mục **API Keys**.
- **Nhóm mà key của bạn được gán vào** quyết định nền tảng upstream nào sẽ phục
  vụ request — Anthropic, OpenAI, Grok, Google, hoặc một nhóm hỗn hợp định
  tuyến theo model — và mức phí (billing rate) áp dụng.

Một key không được gán vào nhóm nào sẽ bị từ chối với mã `403`, trừ khi quản
trị viên đã cho phép rõ ràng các key không thuộc nhóm nào. Nếu bạn thấy thông
báo *"API Key is not assigned to any group"*, hãy nhờ bên vận hành gán key đó
vào một nhóm.

Nhóm rất quan trọng khi bạn chọn endpoint: một nhóm nền tảng Anthropic phục vụ
`/v1/messages`; một nhóm nền tảng OpenAI phục vụ `/v1/chat/completions`,
`/v1/responses`, `/v1/embeddings` cùng các endpoint hình ảnh và video; một nhóm
Google phục vụ `/v1beta`. Các nhóm hỗn hợp (composite) phân phối request theo
trường `model` trong body, nên một key có thể tiếp cận nhiều nền tảng.

## Những gì được hỗ trợ

- **Văn bản** — Messages, Chat Completions, Responses, cả ba đều hỗ trợ
  streaming.
- **Đếm token** — `POST /v1/messages/count_tokens`, không tính phí.
- **Embeddings** — `POST /v1/embeddings`.
- **Hình ảnh** — đồng bộ, bất đồng bộ và theo lô (batch). Xem
  [Tài liệu API](/docs/api-reference).
- **Video** — tạo mới, chỉnh sửa, mở rộng, cùng với việc kiểm tra trạng thái và
  nội dung.
- **Âm thanh** — TTS, STT và giọng nói tùy chỉnh.
- **Tìm kiếm** — tìm kiếm web và tìm kiếm X trên các nhóm Grok.

## Bước tiếp theo

- [Bắt đầu nhanh](/docs/quickstart) — thực hiện request đầu tiên trong khoảng
  một phút.
- [Xác thực](/docs/authentication) — nên gửi header nào, và vì sao tham số
  query sẽ không hoạt động.
- [Tài liệu API](/docs/api-reference) — danh sách endpoint.
- [Thanh toán và mức sử dụng](/docs/billing-and-usage) — đọc hệ số nhân giá và
  mức tiêu thụ của bạn qua API.
- [Lỗi](/docs/errors) — cấu trúc phản hồi theo từng giao thức, và ý nghĩa của
  từng mã trạng thái.
