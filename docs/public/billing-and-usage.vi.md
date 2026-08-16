# Thanh toán và mức sử dụng

Hai endpoint cho phép client trả lời, mà không cần mở dashboard, hai câu hỏi
*"lần gọi này sẽ tốn bao nhiêu"* và *"tôi đã tiêu bao nhiêu"*.

## Hệ số nhân giá

```bash
curl {{SITE_ORIGIN}}/v1/sub2api/billing \
  -H "Authorization: Bearer $API_KEY"
```

```json
{
  "object": "sub2api.key_billing",
  "schema_version": 1,
  "billing_scope": "token",
  "group_rate_multiplier": 1.0,
  "resolved_rate_multiplier": 0.8,
  "peak_rate_enabled": true,
  "peak_start": "09:00",
  "peak_end": "18:00",
  "peak_rate_multiplier": 1.5,
  "applied_peak_multiplier": 1.5,
  "effective_rate_multiplier": 1.2,
  "timezone": "Asia/Shanghai",
  "observed_at": "2026-08-14T10:30:00Z"
}
```

Hãy đọc từ dưới lên — `effective_rate_multiplier` mới là con số thực sự định
giá cho request tiếp theo của bạn. Các trường còn lại giải thích nó được tính
ra như thế nào:

| Trường | Ý nghĩa |
| --- | --- |
| `billing_scope` | Đối tượng được đo lường. `token` nghĩa là tính phí theo token. |
| `group_rate_multiplier` | Hệ số nhân cơ bản của nhóm mà key bạn thuộc về. |
| `user_rate_multiplier` | Chỉ xuất hiện khi bạn có hệ số ghi đè (override) cá nhân. |
| `resolved_rate_multiplier` | Hệ số nhân sau khi áp dụng hệ số ghi đè cá nhân (nếu có). |
| `peak_rate_enabled` | Nhóm có tính giá khác cho giờ cao điểm hay không. |
| `peak_start`, `peak_end`, `timezone` | Khung giờ cao điểm, theo múi giờ của nhóm. |
| `peak_rate_multiplier` | Hệ số phụ phí giờ cao điểm. |
| `applied_peak_multiplier` | Chỉ xuất hiện khi bạn đang ở trong khung giờ đó. |
| `effective_rate_multiplier` | resolved × hệ số cao điểm đang áp dụng. Đây là mức bạn phải trả ngay bây giờ. |

Các trường tùy chọn sẽ bị lược bỏ, không đặt là null. Không có
`user_rate_multiplier` nghĩa là bạn không có hệ số ghi đè; không có
`applied_peak_multiplier` nghĩa là thời điểm hiện tại đang ngoài giờ cao điểm.

Phản hồi mang header `Cache-Control: no-store`, vì hệ số nhân hiệu lực sẽ thay
đổi khi một khung giờ cao điểm mở ra hoặc kết thúc. Hãy đọc khi cần, thay vì
cache nó cho cả ngày.

Có hai trường hợp sẽ trả về lỗi thay vì dữ liệu:

- `403` `permission_error` — key chưa có nhóm, nên không thể xác định hệ số
  nhân nào.
- `404` `not_found_error` — bản triển khai đang chạy ở chế độ simple, hoàn
  toàn không có mô hình tính phí.

## Mức tiêu thụ

```bash
curl "{{SITE_ORIGIN}}/v1/usage?days=7" \
  -H "Authorization: Bearer $API_KEY"
```

Phạm vi giới hạn ở key đang gọi request. `days` là tùy chọn và phải nằm trong
khoảng từ 1 đến 90; giá trị ngoài khoảng này sẽ trả về `400`:

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid days, allowed range is 1-90"
  }
}
```

Phản hồi chứa tổng số của key cùng với số liệu chi tiết theo từng ngày. Thống
kê mức sử dụng được thu thập theo cơ chế best-effort: nếu kho lưu trữ thống kê
tạm thời không khả dụng, endpoint vẫn trả về các trường cơ bản thay vì làm
request thất bại, vì vậy hãy hiểu phần chi tiết bị thiếu là *"tạm thời chưa có
sẵn"*, không phải là bằng không.

## Những gì không bị tính phí

- `POST /v1/messages/count_tokens` — kiểm tra hạn mức và gói của bạn, không
  ghi nhận mức sử dụng, và không chiếm slot đồng thời nào.
- Kiểm tra một task hình ảnh bất đồng bộ hoặc trạng thái video. Công việc đã
  được tính phí khi bạn gửi yêu cầu; việc đọc kết quả là miễn phí.

## Khi hạn mức hết

Hạn mức đã cạn sẽ trả về `429`. Trên các đường dẫn tương thích OpenAI, nó dùng
đúng cấu trúc của OpenAI, để logic retry của SDK có thể nhận diện được:

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

Hãy nạp thêm, hoặc nhờ bên vận hành nâng giới hạn. Việc retry sẽ không giúp
ích gì cho tới khi một trong hai việc đó xảy ra — xem [Lỗi](/docs/errors).
