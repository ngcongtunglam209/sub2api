# Lỗi

## Ba cấu trúc, mỗi giao thức một kiểu

Lỗi được viết theo đúng cấu trúc của giao thức mà bạn đã gọi, để client hiện
tại của bạn phân tích được mà không cần viết trường hợp đặc biệt riêng cho
cổng này.

### Đường dẫn Anthropic — `/v1/messages`, và các endpoint của cổng

```json
{
  "type": "error",
  "error": {
    "type": "permission_error",
    "message": "..."
  }
}
```

`error.type` là một trong các giá trị `authentication_error`,
`permission_error`, `invalid_request_error`, `not_found_error`, hoặc
`api_error`.

### Đường dẫn tương thích OpenAI

```json
{
  "error": {
    "message": "...",
    "type": "insufficient_quota",
    "param": null,
    "code": "insufficient_quota"
  }
}
```

### Đường dẫn Gemini — `/v1beta`

```json
{
  "error": {
    "code": 403,
    "message": "...",
    "status": "PERMISSION_DENIED"
  }
}
```

### Từ chối ở cấp cổng

Một request bị chặn trước khi đến bộ xử lý giao thức — ví dụ như header key
sai định dạng — sẽ dùng một cặp phẳng (flat pair) thay vì cấu trúc trên:

```json
{
  "code": "api_key_in_query_deprecated",
  "message": "..."
}
```

## Mã trạng thái

| Trạng thái | Ý nghĩa | Cần làm gì |
| --- | --- | --- |
| `400` | Request sai định dạng, tham số nằm ngoài phạm vi cho phép, hoặc body vượt quá giới hạn kích thước. | Sửa lại request. Đọc `message`; nó nêu rõ trường gây lỗi. Các endpoint văn bản có giới hạn kích thước body chặt hơn so với endpoint hình ảnh và video. |
| `401` | Không có key, key không xác định, hoặc key đã bị vô hiệu hóa. | Kiểm tra header. Xem [Xác thực](/docs/authentication). |
| `403` | Key chưa có nhóm, hoặc nhóm của nó không phục vụ endpoint này. | Nhờ bên vận hành gán nhóm cho key, hoặc gọi endpoint mà nền tảng của bạn hỗ trợ. |
| `404` | Đường dẫn không tồn tại, hoặc tính năng này đang tắt trong bản triển khai này. | Xem [Tài liệu API](/docs/api-reference). |
| `429` | Giới hạn tốc độ, giới hạn số lượng đồng thời, hoặc hạn mức đã cạn. | Giảm tốc độ và thử lại — trừ trường hợp `insufficient_quota`, khi đó thử lại không có tác dụng. |
| `5xx` | Lỗi ở cổng hoặc ở upstream. | Thử lại với backoff. Nếu vẫn tiếp diễn, vấn đề không nằm ở request của bạn. |

## Thử lại (retry)

- Thử lại với `429` và `5xx`. Không thử lại với `400`, `401`, hoặc `403` —
  cùng một request sẽ thất bại theo cùng một cách.
- Dùng exponential backoff kèm jitter. Việc nhiều client cùng retry đồng loạt
  sẽ biến một khoảnh khắc chậm thoáng qua thành một sự cố toàn diện.
- Coi `insufficient_quota` là trạng thái cuối cùng (terminal). Về mã trạng
  thái nó là `429`, nhưng nguyên nhân nằm ở số dư của bạn chứ không phải lưu
  lượng, và chờ đợi bao lâu cũng không tự hết.
- Phản hồi streaming có thể thất bại sau byte đầu tiên. Một luồng (stream) kết
  thúc sớm vẫn là một thất bại dù dòng trạng thái ghi `200`; hãy xử lý một
  luồng bị cắt ngang giống như cách bạn xử lý `5xx`.

## Chẩn đoán nhanh

1. `curl {{SITE_ORIGIN}}/v1/models -H "Authorization: Bearer $API_KEY"` — nếu
   lệnh này trả về kết quả, key và nhóm của nó đều ổn, vấn đề nằm ở request
   body hoặc ở endpoint cụ thể.
2. Nếu trả về `401`, key bị sai. Nếu `403`, key cần được gán nhóm.
3. So sánh model bạn đã gửi với danh sách mà endpoint đó trả về. Một model mà
   nhóm của bạn không có là nguyên nhân phổ biến nhất khiến một request trông
   có vẻ đúng nhưng vẫn bị từ chối.
