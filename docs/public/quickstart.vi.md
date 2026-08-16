# Bắt đầu nhanh

## 1. Lấy key

Đăng ký, đăng nhập, mở mục **API Keys** và tạo một key. Key chỉ hiển thị một
lần và bắt đầu bằng `sk-`. Hãy lưu trữ nó như bạn lưu trữ mọi bí mật khác —
dashboard không thể hiển thị lại nó.

Kiểm tra key được gán vào nhóm nào ngay trên cùng màn hình đó. Nhóm sẽ quyết
định endpoint nào bên dưới sẽ phản hồi.

## 2. Gửi request

Base URL là `{{SITE_ORIGIN}}`. Chọn giao thức khớp với nhóm của key bạn.

### Anthropic Messages

```bash
curl {{SITE_ORIGIN}}/v1/messages \
  -H "x-api-key: $API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 256,
    "messages": [{ "role": "user", "content": "Say hello in five words." }]
  }'
```

### OpenAI Chat Completions

```bash
curl {{SITE_ORIGIN}}/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "content-type: application/json" \
  -d '{
    "model": "gpt-5",
    "messages": [{ "role": "user", "content": "Say hello in five words." }]
  }'
```

### Google Gemini

```bash
curl "{{SITE_ORIGIN}}/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "x-goog-api-key: $API_KEY" \
  -H "content-type: application/json" \
  -d '{
    "contents": [{ "parts": [{ "text": "Say hello in five words." }] }]
  }'
```

## 3. Trỏ SDK về đây

Mọi SDK chính thức đều nhận một Base URL. Đó là thay đổi duy nhất.

### Python — `openai`

```python
from openai import OpenAI

client = OpenAI(api_key="sk-...", base_url="{{SITE_ORIGIN}}/v1")

response = client.chat.completions.create(
    model="gpt-5",
    messages=[{"role": "user", "content": "Say hello in five words."}],
)
print(response.choices[0].message.content)
```

### Python — `anthropic`

```python
from anthropic import Anthropic

client = Anthropic(api_key="sk-...", base_url="{{SITE_ORIGIN}}")

message = client.messages.create(
    model="claude-sonnet-4-5",
    max_tokens=256,
    messages=[{"role": "user", "content": "Say hello in five words."}],
)
print(message.content[0].text)
```

### Node — `openai`

```javascript
import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: process.env.API_KEY,
  baseURL: '{{SITE_ORIGIN}}/v1',
})

const response = await client.chat.completions.create({
  model: 'gpt-5',
  messages: [{ role: 'user', content: 'Say hello in five words.' }],
})
console.log(response.choices[0].message.content)
```

### Claude Code

```bash
export ANTHROPIC_BASE_URL="{{SITE_ORIGIN}}"
export ANTHROPIC_AUTH_TOKEN="sk-..."
claude
```

### Codex CLI

```bash
export OPENAI_BASE_URL="{{SITE_ORIGIN}}/v1"
export OPENAI_API_KEY="sk-..."
codex
```

## 4. Streaming

Streaming được truyền nguyên vẹn, nên SDK bạn đang dùng vẫn xử lý được mà
không cần thay đổi. Với HTTP thuần, đặt `"stream": true` và đọc các
server-sent events:

```bash
curl -N {{SITE_ORIGIN}}/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "content-type: application/json" \
  -d '{
    "model": "gpt-5",
    "stream": true,
    "messages": [{ "role": "user", "content": "Count to five." }]
  }'
```

## 5. Kiểm tra model bạn có thể gọi

```bash
curl {{SITE_ORIGIN}}/v1/models -H "Authorization: Bearer $API_KEY"
```

Danh sách này phản ánh nhóm của key bạn, không phải toàn bộ nền tảng, nên hãy
coi đây là câu trả lời chính xác cho câu hỏi *"key này có thể gọi được gì?"*.

## Khi có lỗi xảy ra

- `401` — key bị thiếu, sai, hoặc đã bị vô hiệu hóa. Xem
  [Xác thực](/docs/authentication).
- `403` — key chưa có nhóm, hoặc nhóm đó không phục vụ endpoint này.
- `429` — vượt giới hạn tốc độ hoặc số lượng đồng thời, hoặc hạn mức đã hết.

Cấu trúc đầy đủ và danh sách mã trạng thái: [Lỗi](/docs/errors).
