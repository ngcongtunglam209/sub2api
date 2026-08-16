# Xác thực

Mỗi request đến cổng đều cần một API key. Cổng chấp nhận ba loại header, theo
thứ tự ưu tiên sau, để client hiện tại của bạn vẫn hoạt động mà không cần sửa
đổi.

## Các header được chấp nhận

### `Authorization: Bearer` — kiểu OpenAI

```bash
curl {{SITE_ORIGIN}}/v1/chat/completions \
  -H "Authorization: Bearer sk-..."
```

Scheme được so khớp không phân biệt hoa thường, nên `bearer` cũng hoạt động.

### `x-api-key` — kiểu Anthropic

```bash
curl {{SITE_ORIGIN}}/v1/messages \
  -H "x-api-key: sk-..."
```

### `x-goog-api-key` — tương thích Gemini CLI

```bash
curl "{{SITE_ORIGIN}}/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "x-goog-api-key: sk-..."
```

Chỉ cần gửi một header. Nếu có nhiều header cùng lúc, thứ tự ưu tiên là
`Authorization` trước, rồi đến `x-api-key`, cuối cùng là `x-goog-api-key`.

## Tham số query `api_key` bị từ chối

Việc truyền key trong URL sẽ bị từ chối thẳng, với mã `400`:

```json
{
  "code": "api_key_in_query_deprecated",
  "message": "API key in query parameter is deprecated. Please use Authorization header instead."
}
```

Đây là chủ ý. Một key nằm trong query string sẽ lọt vào lịch sử trình duyệt,
log của proxy, và các header referrer. Hãy chuyển nó sang header.

## Thiếu key

Khi không có header nào được nhận diện, cổng trả về `401`:

```json
{
  "code": "API_KEY_REQUIRED",
  "message": "API key is required in Authorization header (Bearer scheme), x-api-key header, or x-goog-api-key header"
}
```

## Gán nhóm

Xác thực chỉ chứng minh bạn là ai. Bản thân nó không cấp quyền truy cập: key
còn phải được gán vào một nhóm. Một key chưa được gán sẽ bị từ chối với mã
`403`, kèm thông báo theo đúng cấu trúc lỗi của giao thức đó:

```json
{
  "type": "error",
  "error": {
    "type": "permission_error",
    "message": "API Key is not assigned to any group and cannot be used. Please contact the administrator to assign it to a group."
  }
}
```

Bên vận hành có thể cho phép các key không thuộc nhóm nào thông qua cài đặt hệ
thống, khi đó kiểm tra này sẽ không kích hoạt. Nếu bạn gặp lỗi này, cách khắc
phục nằm ở phía vận hành, không phải ở code của bạn.

## Sử dụng key một cách an toàn

- Lưu key trong biến môi trường hoặc trình quản lý bí mật (secret manager),
  không bao giờ đặt trong repository hay trong bundle của frontend. Một key
  xuất hiện trong code client đã phát hành coi như đã bị công khai.
- Dùng một key cho mỗi ứng dụng, để khi thu hồi một key không ảnh hưởng đến
  các ứng dụng khác.
- Khi xoay vòng key, tạo key mới trước, triển khai nó, rồi mới xóa key cũ.
- Xóa ngay một key khi bạn nghi ngờ nó đã bị lộ. Việc xóa có hiệu lực ngay lập
  tức với các request mới.
- Dashboard chỉ hiển thị giá trị đầy đủ của key tại thời điểm tạo.
