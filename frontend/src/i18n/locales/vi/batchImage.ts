/**
 * The Codex skill document offered by the batch image usage guide.
 *
 * This is a *named* export, deliberately outside the message tree below, and its
 * zh twin lives at the same position in `locales/zh/batchImage.ts`. The reason is
 * that vue-i18n compiles every message in the tree at render time, and this
 * document is not UI copy — it is ~65 lines the user copies into their agent,
 * containing a JSON request body and six URL templates with a literal `{id}`
 * segment. In message syntax every one of those braces would have to be written
 * `{'{'}` / `{'}'}`, which turns the payload example into unreadable noise and
 * makes a silently-eaten brace the likeliest way this file ever breaks. Keeping
 * it a plain template literal means what is written here is byte-for-byte what
 * the user copies.
 *
 * Only the instructional prose is translated. Field names, parameter names,
 * URLs, JSON structure, literal values (`"1K"`, `"image/png"`, `"img_001"`),
 * numbers, and the line/indent layout are identical across locales — a mistake
 * there breaks the reader's workflow rather than merely reading badly.
 *
 * @param endpoint API base URL, already stripped of any trailing slash.
 */
export function batchImageAgentInstruction(endpoint: string): string {
  return `---
name: sub2api-batch-image
description: Dùng skill này khi người dùng muốn tạo ảnh hàng loạt bằng Gemini/Vertex, chạy một danh sách prompt hàng loạt, tải xuống kết quả tạo ảnh hàng loạt, hoặc thử lại các ảnh thất bại.
---

Bạn là Agent tạo ảnh hàng loạt bên trong Codex. Người dùng không cần tự tay điền form trên trang; hãy suy ra tên tác vụ, danh sách prompt và thư mục đầu ra từ cuộc trò chuyện hiện tại, từ các tệp hoặc thư mục người dùng đã cung cấp, và từ ngữ cảnh xung quanh. Chỉ hỏi lại người dùng khi một quyết định bạn không thể tự đưa ra thực sự còn thiếu.

Endpoint mặc định:
${endpoint}

Bạn chịu trách nhiệm cho tất cả những việc sau:
1. Trích xuất các prompt từ cuộc trò chuyện hoặc từ tệp đính kèm. Giữ nguyên toàn bộ văn bản của mỗi prompt và gán các giá trị custom_id ổn định theo thứ tự, ví dụ img_001, img_002.
2. Suy ra tên tác vụ từ yêu cầu của người dùng hoặc từ ngữ cảnh; khi không có tên rõ ràng, hãy tạo một tên từ thời gian hiện tại.
3. Suy ra thư mục đầu ra từ yêu cầu của người dùng hoặc từ ngữ cảnh; chỉ hỏi người dùng nếu họ chưa từng nói nên lưu kết quả ở đâu.
4. Trước khi gửi bạn phải tính expected_output_count = tổng output_count của tất cả các item. Một tác vụ batch chỉ được tối đa 200 ảnh đầu ra; bất cứ số nào vượt quá 200 phải được tách thành nhiều tác vụ. Không bao giờ gửi một tác vụ quá lớn, và không bao giờ nhầm giới hạn tệp đính kèm ảnh tham chiếu với giới hạn số lượng ảnh được tạo ra.
5. Nếu người dùng cung cấp ảnh tham chiếu, hãy gắn từng ảnh vào đúng item mà nó dành cho. Ảnh tham chiếu là tệp đính kèm đầu vào, không phải ảnh đầu ra. Giới hạn theo từng item do model quy định và phải được áp dụng theo từng model: Gemini 2.5 Flash Image cho phép tối đa 3 ảnh tham chiếu mỗi item; Gemini 3 Pro Image cho phép tối đa 14 ảnh tham chiếu mỗi item. Đừng hiểu các hàng rào bảo vệ tệp đính kèm của backend là năng lực mỗi item của Pro: sau khi mở rộng theo output_count, tổng số tệp đính kèm ảnh tham chiếu trên tất cả các item có ngưỡng bảo vệ nội bộ là 1000, và ảnh tham chiếu inline base64 sau khi giải mã tối đa được 128MB tổng cộng. Con số 1000 đó chỉ là ngưỡng để server từ chối các yêu cầu bất thường, không phải kích thước khuyến nghị; hãy tự tách công việc khi có nhiều ảnh tham chiếu hoặc phần thân yêu cầu trở nên lớn.
6. Ảnh tham chiếu bị tính phí như token đầu vào một lần cho mỗi output_count; với các tác vụ lớn, một ảnh tham chiếu được dùng lại nhiều lần, hoặc khi tổng dung lượng ảnh tham chiếu lớn, hãy ưu tiên dùng gs:// file_uri hoặc tách công việc thành nhiều tác vụ.
7. Chọn API key và model: trước tiên lấy danh sách key/model tạo ảnh hàng loạt hiện đang khả dụng; nếu người dùng nêu tên một model và key đó hỗ trợ model đó, hãy dùng model người dùng nêu; nếu không, dùng model mặc định/đầu tiên trong số các model khả dụng của key đó. Không bao giờ hiển thị hay hỏi về tên nhà cung cấp nội bộ.
8. Gọi API tạo ảnh hàng loạt để gửi, thăm dò và tải xuống; không yêu cầu người dùng tự gõ bất cứ gì vào trang.

Hợp đồng API:
- Model: GET ${endpoint}/v1/images/batches/models
- Gửi: POST ${endpoint}/v1/images/batches
- Trạng thái: GET ${endpoint}/v1/images/batches/{id}
- Item: GET ${endpoint}/v1/images/batches/{id}/items
- Tải xuống: GET ${endpoint}/v1/images/batches/{id}/download
- Hủy: POST ${endpoint}/v1/images/batches/{id}/cancel

Nội dung gửi:
{
  "model": "<một trong các model khả dụng của key đã chọn>",
  "task_name": "<suy ra từ cuộc trò chuyện; dùng thời gian hiện tại nếu để trống>",
  "image_size": "1K",
  "response_mime_type": "image/png",
  "items": [
    {
      "custom_id": "img_001",
      "prompt": "<toàn bộ văn bản của prompt đầu tiên>",
      "output_count": 1,
      "reference_images": [
        {
          "id": "face",
          "type": "subject",
          "mime_type": "image/png",
          "data": "<base64, không kèm tiền tố data:image/png;base64, >"
        }
      ]
    }
  ]
}

Bạn phải:
- Không bao giờ ghi API key vào repository, vào log, vào commit, hoặc vào phản hồi cuối cùng của bạn.
- Không bao giờ ghi base64 của ảnh tham chiếu vào phản hồi cuối cùng, vào log, hoặc vào tệp công khai. Bản ghi resume chỉ giữ tên tệp, mục đích và số lượng của các ảnh tham chiếu cùng đường dẫn tới tệp JSON của request; nếu JSON đó chứa base64, hãy giữ nó trong thư mục đầu ra người dùng đã chọn và không commit nó.
- output_count là số ảnh mà cùng một prompt và ảnh tham chiếu tạo ra; mặc định là 1 và tối đa 4 mỗi item. Nó không dựa vào việc Gemini trả về nhiều ảnh từ một request — hệ thống mở rộng nó thành nhiều item tác vụ thật sự. Trước khi gửi bạn phải xác nhận tổng đầu ra dự kiến không vượt quá 200, và tách công việc thành nhiều tác vụ khi vượt quá. Không bao giờ gửi một tác vụ sẽ tạo ra hơn 200 ảnh chỉ vì tệp đính kèm ảnh tham chiếu có ngưỡng bảo vệ nội bộ cao hơn.
- Tạo ảnh hàng loạt vẫn được tính phí cho người dùng theo số ảnh tạo thành công; ảnh tham chiếu không được tính giá riêng. Bạn có thể giải thích cho người dùng rằng ảnh tham chiếu làm tăng thêm một lượng nhỏ token đầu vào phía thượng nguồn và chi phí lưu trữ tạm thời, được tính lại cho mỗi output_count, và số tiền tạm giữ/đã quyết toán hiển thị trên trang được tính từ số ảnh đầu ra.
- Ngay sau khi gửi thành công, bạn phải ghi một bản ghi resume cục bộ vào thư mục đầu ra, ví dụ batch-image-resume.json. Không bao giờ lưu API key trong bản ghi resume.
- Bản ghi resume phải chứa ít nhất: endpoint, task_name, batch_id, model, output_dir, request_file, submitted_at, last_status, status_url, items_url, download_url, prompt_count, expected_output_count, cùng với một bản đồ custom_id-sang-prompt hoặc đường dẫn tới tệp JSON của request để có thể thử lại các item thất bại.
- Cập nhật bản ghi resume sau mỗi lần kiểm tra trạng thái với last_checked_at, last_status, số lượng thành công, số lượng thất bại, số tiền thực tế bị tính phí và một bản tóm tắt các thất bại. Nếu phiên làm việc bị gián đoạn hoặc tạm dừng, chỉ riêng tệp đó cũng phải đủ để tiếp tục truy vấn, tải xuống hoặc thử lại vào lần sau.
- Đừng thăm dò quá dồn dập. Đợi khoảng 20 đến 30 giây trước lần kiểm tra trạng thái đầu tiên; thăm dò một tác vụ đang queued mỗi 60 đến 120 giây; nếu vẫn còn queued sau 3 lần kiểm tra liên tiếp, hãy dừng thăm dò tạm thời, báo cho người dùng biết tác vụ vẫn đang trong hàng đợi, giữ lại bản ghi resume, và tiếp tục công việc khác hoặc chờ người dùng yêu cầu bạn tiếp tục sau.
- Thăm dò một tác vụ đang running khoảng mỗi 60 giây, và thưa hơn với các tác vụ lớn hoặc khi server đang tải cao; các trạng thái gần hoàn tất như processing_results có thể được thăm dò mỗi 20 đến 45 giây.
- Khi tác vụ hoàn tất, báo cáo tên tác vụ, id tác vụ, số lượng thành công, số lượng thất bại, số tiền thực tế bị tính phí và nơi các tệp đã được lưu.
- Chỉ tải xuống các ảnh thành công. Khi thất bại một phần, trước tiên hãy hiển thị các giá trị custom_id thất bại, mã lỗi, nguồn gốc của mỗi lỗi và một lý do ngắn gọn.
- Chỉ thử lại các item thất bại; không bao giờ gửi lại một item đã thành công. Nếu một tác vụ cũ không lưu lại prompt của các item thất bại, bạn phải báo cho người dùng biết rằng nó không thể được tự động thử lại và hỏi xem họ có cung cấp lại prompt gốc hay không.
- Trước khi hủy một tác vụ, bạn phải cảnh báo người dùng rằng các ảnh đã được đánh dấu thành công vẫn sẽ bị tính phí như các item thành công, và phần tạm giữ còn lại sẽ được giải phóng.
- Chỉ tải xem trước ảnh khi cần; không bao giờ tải hàng loạt nội dung ảnh chỉ để xem một danh sách.`
}

export default {
  batchImage: {
    columns: {
      taskName: 'Tên tác vụ',
      model: 'Model',
      apiKey: 'API key',
      result: 'Kết quả',
      cost: 'Chi phí',
      downloadStatus: 'Trạng thái tải xuống',
    },
    status: {
      queued: 'Đang chờ',
      running: 'Đang tạo',
      processingResults: 'Đang xử lý kết quả',
      settling: 'Đang quyết toán',
      completed: 'Hoàn tất',
      failed: 'Thất bại',
      cancelled: 'Đã hủy',
      outputDeleted: 'Kết quả đã bị xóa',
      partialSuccess: 'Thành công một phần',
      allFailed: 'Tất cả thất bại',
    },
    itemStatus: {
      pending: 'Đang chờ',
      succeeded: 'Thành công',
      failed: 'Thất bại',
      cancelled: 'Đã hủy',
      recovered: 'Đã khôi phục bằng thử lại',
    },
    filters: {
      searchTaskName: 'Tìm kiếm tên tác vụ',
      allApiKeys: 'Tất cả API key',
      allStatuses: 'Tất cả trạng thái',
      allDownloadStates: 'Tất cả trạng thái tải xuống',
      downloaded: 'Đã tải xuống',
      notDownloaded: 'Chưa tải xuống',
    },
    actions: {
      usageGuide: 'Hướng dẫn sử dụng',
      createJob: 'Tạo tác vụ hàng loạt',
      downloadSelected: 'Tải xuống mục đã chọn',
      deleteRecords: 'Xóa bản ghi',
      retryFailedItems: 'Thử lại các item thất bại',
      cancelJob: 'Hủy tác vụ',
      downloadZip: 'Tải xuống ZIP',
      viewDetail: 'Xem chi tiết',
      download: 'Tải xuống',
      moreActions: 'Thêm hành động',
      copyInstruction: 'Sao chép hướng dẫn',
      submitJob: 'Gửi tác vụ',
    },
    list: {
      selectedJobs: 'Đã chọn {count} tác vụ | Đã chọn {count} tác vụ',
      expandChildren: 'Mở rộng {n} tác vụ con | Mở rộng {n} tác vụ con',
      collapseChildren: 'Thu gọn tác vụ con',
      childCount: '{n} tác vụ con | {n} tác vụ con',
      childBadge: 'Tác vụ con',
      keyNotRecorded: 'Không được ghi nhận',
      totalCount: 'trên {n}',
      notDownloaded: 'Chưa tải xuống',
      empty: 'Chưa có tác vụ hàng loạt nào',
      emptyHint: 'Dùng nút ở góc trên bên phải để tạo tác vụ hàng loạt.',
    },
    pagination: {
      pageNumber: 'Trang {page}',
      pageItems: '{count} mục trên trang này',
    },
    promptPopover: {
      title: 'Prompt đầy đủ',
      copied: 'Đã sao chép prompt',
    },
    detail: {
      title: 'Chi tiết tác vụ',
      aggregatedResult: 'Kết quả tổng hợp',
      result: 'Kết quả',
      cost: 'Chi phí',
      downloadStatus: 'Trạng thái tải xuống',
      items: 'Danh sách item',
      customId: 'Custom ID',
      prompt: 'Prompt',
      preview: 'Xem trước',
      previewZoom: 'Phóng to bản xem trước nén {id}',
      previewReload: 'Tải lại bản xem trước nén',
      previewLoad: 'Tải bản xem trước nén',
      previewUnavailable: 'Không thể xem trước',
      noImage: 'Không có ảnh',
      loadingItems: 'Đang tải danh sách item...',
      noItems: 'Chưa có item nào',
      noItemsHint: 'Các tác vụ đang chờ hoặc đang tạo sẽ hiển thị prompt đã gửi trước; trạng thái ảnh sẽ cập nhật sau khi kết quả được xử lý.',
      mainTask: 'Tác vụ chính: {name}',
      childTask: 'Tác vụ con: {name}',
      holdCost: 'Tạm giữ {amount}',
    },
    itemResult: {
      recoveredByRetry: 'Lỗi trước đó đã được khôi phục bởi một tác vụ con thử lại',
      readyPreview: 'Ảnh đã được tạo. Nhấp để xem trước.',
      readyDownload: 'Ảnh đã được tạo và sẵn sàng để tải xuống.',
      noUsableImage: 'Không có ảnh nào sử dụng được được tạo ra.',
      cancelled: 'Tác vụ đã bị hủy.',
      waiting: 'Đang chờ kết quả.',
      emptyImageOutput: 'Phía thượng nguồn đã trả về kết quả, nhưng item này không có nội dung ảnh. Điều này thường có nghĩa là một lần tạo Gemini/Vertex đã thất bại hoặc bị chặn bởi chính sách an toàn.',
      providerItemFailed: 'Kết quả từ phía thượng nguồn cho item này không có ảnh sử dụng được.',
    },
    imagePreview: {
      title: 'Xem trước ảnh',
      notice: 'Đây là ảnh thu nhỏ đã nén được lưu đệm cục bộ trong trình duyệt của bạn, nên chất lượng bị giảm. Hãy tải xuống tệp ZIP để xem ảnh gốc.',
    },
    create: {
      title: 'Tạo tác vụ hàng loạt',
      taskName: 'Tên tác vụ',
      taskNamePlaceholder: 'Mặc định dùng thời gian hiện tại nếu để trống',
      loadingKeys: 'Đang tải API key...',
      selectKeyPlaceholder: 'Chọn một API key Gemini',
      noKeysHint: 'Không có API key Gemini nào khả dụng để tạo ảnh hàng loạt. Hãy tạo một key và gắn nó vào một nhóm Gemini đã bật tính năng tạo ảnh hàng loạt trước.',
      model: 'Model',
      imageSize: 'Kích thước ảnh',
      imageSizeHint: 'Các tác vụ hàng loạt hiện đang được gửi với kích thước ảnh cố định 1K.',
      outputFormat: 'Định dạng đầu ra',
      estimatedOutput: 'Ước tính đầu ra',
      estimatedOutputValue: '{images} ảnh / {prompts} prompt',
      promptSection: 'Prompt',
      promptAdded: 'Đã thêm {count}',
      promptPlaceholder: 'Dán một prompt, sau đó thêm nó vào danh sách bên dưới',
      customIdPlaceholder: 'Custom ID (không bắt buộc)',
      outputCountPerPrompt: 'Số ảnh mỗi prompt',
      outputCountOption: '{n} ảnh | {n} ảnh',
      referenceImage: 'Ảnh tham chiếu',
      removeReferenceImage: 'Xóa ảnh tham chiếu',
      limitsHint: 'Tối đa {maxPerItem} ảnh mỗi prompt và {maxPerJob} ảnh mỗi tác vụ. Model hiện tại cho phép tối đa {refLimit} ảnh tham chiếu mỗi prompt; ảnh tham chiếu tiêu tốn token đầu vào một lần cho mỗi ảnh được tạo.',
      referenceCount: '{n} ảnh tham chiếu | {n} ảnh tham chiếu',
      noPrompts: 'Chưa có prompt nào được thêm.',
      cancelNotice: 'Hủy sẽ yêu cầu hủy ở phía thượng nguồn. Các ảnh đã được đánh dấu thành công vẫn sẽ bị tính phí, và phần tạm giữ còn lại sẽ được giải phóng.',
      submittingNotice: 'Đang tạo tác vụ hàng loạt ở phía thượng nguồn. Việc này thường mất vài giây; vui lòng không gửi lại.',
      modelNoReferenceImages: 'Model hiện tại không hỗ trợ ảnh tham chiếu.',
      refLimitReached: 'Model hiện tại cho phép tối đa {limit} ảnh tham chiếu mỗi prompt.',
      refLimitExceededIgnored: 'Model hiện tại cho phép tối đa {limit} ảnh tham chiếu mỗi prompt. Các tệp thừa đã bị bỏ qua.',
      refFormatUnsupported: 'Ảnh tham chiếu phải là PNG, JPEG, hoặc WebP.',
      refFileTooLarge: '{name} vượt quá 10MB và đã bị bỏ qua.',
    },
    guide: {
      title: 'Hướng dẫn tạo ảnh hàng loạt',
      uiTitle: 'Cách sử dụng trang này',
      step1: '1. Chọn một API key Gemini đã bật tính năng tạo ảnh hàng loạt. Danh sách model hiển thị các model khả dụng cho nhóm của key đó.',
      step2: '2. Tên tác vụ có thể để trống; thời gian hiện tại sẽ được dùng tự động khi gửi. Các prompt được thêm vào danh sách từng cái một, và mỗi prompt có thể kèm ảnh tham chiếu và số lần lặp lại.',
      step3: '3. Sau khi gửi, tác vụ sẽ được xếp hàng chờ trước và danh sách item sẽ hiển thị các prompt đã gửi. Bản xem trước ảnh không được tải theo mặc định; nhấp vào nút xem trước trên một item để tải một ảnh.',
      step4: '4. Sau khi hoàn tất bạn có thể tải xuống tệp ZIP. Nếu một số item thất bại, menu Thêm cho phép bạn chỉ thử lại các item thất bại. Việc tính phí vẫn dựa trên số ảnh tạo thành công; ảnh tham chiếu không được tính giá riêng.',
      skillTitle: 'Hướng dẫn skill cho Codex',
      skillDesc: 'Cho Codex biết cách tổ chức prompt, gửi tác vụ, và tải xuống kết quả thay cho người dùng.',
      /*
       * Stands in for the API endpoint when neither a configured base URL nor a
       * `window.location` is available. It is printed inside the skill document
       * below, so it keeps the same angle-bracket "fill this in" shape as the
       * placeholders in that document.
       */
      endpointFallback: '<endpoint API Sub2API của bạn>',
    },
    messages: {
      loadKeysFailed: 'Tải API key thất bại.',
      loadModelsFailed: 'Tải danh sách model khả dụng thất bại.',
      loadJobsFailed: 'Tải danh sách tác vụ hàng loạt thất bại.',
      selectApiKey: 'Chọn một API key Gemini khả dụng.',
      noModelsForKey: 'Key này không có model tạo ảnh hàng loạt khả dụng nào.',
      selectModel: 'Chọn một model.',
      promptRequired: 'Nhập ít nhất một prompt.',
      submitted: 'Đã gửi tác vụ hàng loạt.',
      submitFailed: 'Gửi tác vụ hàng loạt thất bại.',
      refreshFailed: 'Làm mới tác vụ thất bại.',
      cancelConfirm: 'Yêu cầu hủy sẽ được gửi lên phía thượng nguồn. Các ảnh đã được đánh dấu thành công vẫn sẽ bị tính phí, và phần tạm giữ còn lại sẽ được giải phóng. Tiếp tục?',
      cancelled: 'Đã gửi yêu cầu hủy.',
      cancelFailed: 'Hủy tác vụ thất bại.',
      batchDownloadStarted: 'Đã bắt đầu tải xuống các tác vụ đã chọn.',
      downloadFailed: 'Tải xuống kết quả thất bại.',
      retrySubmitted: 'Đã gửi tác vụ thử lại cho các item thất bại.',
      retryFailed: 'Thử lại các item thất bại không thành công.',
      retryMissingPrompts: 'Tác vụ này không có prompt được lưu cho các item thất bại, nên không thể tự động thử lại. Hãy tạo lại với prompt gốc.',
      retryTaskNameSuffix: 'Thử lại các item thất bại',
      deleteConfirm: 'Thao tác này sẽ ẩn tác vụ khỏi danh sách của bạn nhưng vẫn giữ lại bản ghi tính phí. Xóa chứ?',
      deleteSelectedConfirm: 'Thao tác này sẽ ẩn các tác vụ đã chọn khỏi danh sách của bạn nhưng vẫn giữ lại bản ghi tính phí. Xóa chứ?',
      deleted: 'Đã xóa bản ghi tác vụ.',
      deleteFailed: 'Xóa bản ghi tác vụ thất bại.',
      loadItemsFailed: 'Tải chi tiết item thất bại.',
      loadPreviewFailed: 'Tải bản xem trước ảnh thất bại.',
      copiedInstruction: 'Đã sao chép hướng dẫn tạo ảnh hàng loạt.',
      loadingModels: 'Đang tải danh sách model khả dụng...',
      noModels: 'Không có model khả dụng',
      noModelsHint: 'Nhóm của key này chưa có model nào được cấu hình cho tạo ảnh hàng loạt.',
      noCompatibleAccount: 'Không có tài khoản thượng nguồn tạo ảnh hàng loạt nào khả dụng cho nhóm của key này. Liên hệ quản trị viên để kiểm tra API key Gemini hoặc tài khoản dịch vụ Vertex có thể lập lịch của nhóm cũng như hỗ trợ model.',
      unsupportedProvider: 'Nhà cung cấp tạo ảnh hàng loạt cho tác vụ này hiện không khả dụng. Liên hệ quản trị viên để kiểm tra cấu hình nhà cung cấp tạo ảnh hàng loạt.',
      providerSubmitFailed: 'Tác vụ tạo ảnh hàng loạt ở phía thượng nguồn gửi thất bại. Liên hệ quản trị viên để kiểm tra tài khoản thượng nguồn, quyền model, hoặc trạng thái nhà cung cấp.',
      vertexGcsBucketMissing: 'Tạo ảnh hàng loạt bằng Vertex đang thiếu cấu hình GCS bucket được quản lý. Liên hệ quản trị viên để cấu hình BATCH_IMAGE_VERTEX_MANAGED_GCS_BUCKET trước khi gửi lại.',
      queueFailed: 'Hàng đợi tác vụ tạm thời không khả dụng, nên tác vụ hàng loạt chưa được đưa vào hàng đợi. Liên hệ quản trị viên để kiểm tra dịch vụ hàng đợi.',
      billingHoldFailed: 'Tạm giữ chi phí thất bại, nên tác vụ hàng loạt chưa được gửi. Liên hệ quản trị viên để kiểm tra dịch vụ thanh toán hoặc tạm giữ số dư.',
      groupDisabled: 'Tạo ảnh hàng loạt chưa được bật cho nhóm của key này. Hãy chọn một key khác đã được bật hoặc liên hệ quản trị viên.',
      pricingMissing: 'Model đã chọn chưa có bảng giá tạo ảnh hàng loạt được cấu hình. Liên hệ quản trị viên để thêm bảng giá trước.',
      insufficientBalance: 'Số dư không đủ để tạm giữ chi phí ước tính cho tạo ảnh hàng loạt.',
      invalidModel: 'Chọn một model tạo ảnh hàng loạt khả dụng cho key hiện tại.',
      invalidItems: 'Danh sách prompt không hợp lệ. Kiểm tra rằng nó không rỗng, nằm trong giới hạn số item, và vẫn dùng kích thước ảnh 1K.',
      duplicateCustomId: 'Các custom_id trong danh sách prompt phải là duy nhất.',
      promptTooLong: 'Một prompt quá dài. Hãy rút ngắn và thử lại.',
      invalidReferenceImage: 'Một ảnh tham chiếu không hợp lệ. Dùng PNG, JPEG, hoặc WebP dưới 10 MB.',
      tooManyReferenceImages: 'Quá nhiều ảnh tham chiếu. Flash Image cho phép tối đa 3 ảnh mỗi item, Pro Image cho phép tối đa 14 ảnh, và mỗi tác vụ cho phép tối đa 1000 ảnh tổng cộng.',
      referenceImagesTooLarge: 'Ảnh tham chiếu quá lớn. Ảnh tham chiếu inline bị giới hạn ở 128 MB mỗi tác vụ; hãy dùng gs:// file_uri hoặc tách tác vụ cho các lô lớn.',
      tooManyOutputImages: 'Số ảnh đầu ra dự kiến quá nhiều. Mỗi prompt có thể yêu cầu tối đa 4 ảnh, và mỗi tác vụ có thể tạo tối đa 200 ảnh.',
      idempotencyConflict: 'Lượt gửi này xung đột với một request ID trước đó. Hãy làm mới trang và gửi lại.',
      notReady: 'Tác vụ chưa hoàn tất. Tải xuống sẽ khả dụng sau khi hoàn tất.',
      outputDeleted: 'Các tệp kết quả của tác vụ này đã được dọn dẹp.',
      resultMissing: 'Tệp kết quả không khả dụng. Có thể nó đã bị dọn dẹp, quyền lưu trữ bị hỏng, hoặc cấu hình lưu trữ đã thay đổi. Liên hệ quản trị viên để kiểm tra tệp kết quả.',
      itemFailed: 'Item này không có ảnh thành công nào để xem trước.',
      itemImageIndexOutOfRange: 'Item này không có ảnh nào để xem trước.',
      downloadLimited: 'Có quá nhiều yêu cầu tải xuống đang hoạt động. Vui lòng thử lại sau.',
      downloadTooLarge: 'Tệp ZIP này quá lớn cho một lượt tải xuống. Hãy tải xuống ít item hơn mỗi lần hoặc yêu cầu quản trị viên nâng giới hạn tải xuống hàng loạt.',
      deleteNotReady: 'Chỉ có thể xóa bản ghi tác vụ sau khi tác vụ hoàn tất.',
      disabled: 'Tính năng tạo ảnh hàng loạt hiện đang bị tắt.',
      authRequired: 'API key hiện tại không khả dụng hoặc đã hết hạn. Hãy chọn lại key.',
      adminReference: 'Gửi mã lỗi và request ID cho quản trị viên để xử lý sự cố.',
      errorReference: 'Chi tiết lỗi',
      errorCodeRef: 'mã: {code}',
      requestIdRef: 'request ID: {id}',
      httpStatusRef: 'trạng thái HTTP: {status}',
    },
  },
}
