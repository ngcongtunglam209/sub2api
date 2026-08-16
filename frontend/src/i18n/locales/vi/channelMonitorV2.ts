/** Channel Monitor V2 (user + admin passive monitor UI) */
export default {
  channelMonitorV2: {
    title: 'Giám sát kênh',
    updating: 'Đang cập nhật dữ liệu',
    updatedTo: 'Đã cập nhật đến {time}',
    partialCoverage: 'Dữ liệu lịch sử chưa đầy đủ',
    bootstrap: {
      title: 'Đang xây dựng dữ liệu giám sát lịch sử',
      description:
        'Khi bật lần đầu, tổng hợp thụ động sẽ âm thầm lấp đầy các khung 90 phút, 24 giờ, 7 ngày và 30 ngày ở nền. Mọi khoảng thời gian sẽ đầy đủ khi quá trình này hoàn tất.',
      progress: 'Hoàn thành {percent}%',
      working: 'Đang tổng hợp ở nền…',
    },
    timeRange: 'Khoảng thời gian',
    clearFilters: 'Đặt lại',
    summaryAria: 'Tóm tắt khoảng đã chọn',
    loadFailed: 'Không tải được dữ liệu giám sát kênh',
    detailLoadFailed: 'Không tải được chi tiết giám sát kênh',
    otherModels: 'Mô hình khác',
    ignored: 'Đã bỏ qua',
    currentUser: 'Người dùng hiện tại',
    ranges: { '90m': '90p', '24h': '24g', '7d': '7n', '30d': '30n' },
    filters: {
      platform: 'Nền tảng', allPlatforms: 'Tất cả', group: 'Nhóm', allGroups: 'Tất cả', model: 'Mô hình', allModels: 'Tất cả',
      empty: 'Không có tùy chọn', selectedCount: '{count}', labelValue: '{label}: {value}'
    },
    groupBy: {
      label: 'Nhóm theo', platform: 'Nền tảng', platformGroup: 'Nền tảng / Nhóm', platformModel: 'Nền tảng / Mô hình', platformGroupModel: 'Nền tảng / Nhóm / Mô hình'
    },
    trendView: { label: 'Chế độ xu hướng', pulse: 'Ma trận nhịp', line: 'Biểu đồ đường' },
    healthMode: { label: 'Hiển thị sức khỏe', overall: 'Tổng thể', success: 'Tỷ lệ lỗi', ttft: 'Token đầu tiên', cache: 'Tỷ lệ cache' },
    tabs: { aria: 'Chiều dữ liệu chi tiết', models: 'Mô hình', errors: 'Nguyên nhân lỗi', users: 'Xếp hạng người dùng' },
    metrics: {
      rpm: 'RPM',
      tps: 'Token/giây',
      rpmDetail: 'Số yêu cầu mỗi phút',
      tpsDetail: 'Tính bằng TPM ÷ 60',
      ttft: 'Token đầu tiên',
      ttftP50: 'Token đầu tiên P50',
      cacheRate: 'Tỷ lệ cache',
      cacheDetail: 'Tỷ lệ đọc cache',
      successRate: 'Tỷ lệ thành công',
      successRateValue: 'Tỷ lệ thành công {value}',
      errorRateValue: 'Tỷ lệ lỗi {value}',
      rpmValue: 'RPM {value}',
      tpsValue: 'Token/giây {value}',
      ttftValue: 'Token đầu tiên {value}',
      durationValue: 'Thời lượng {value}',
      cacheRateValue: 'Tỷ lệ cache {value}',
    },
    table: { platformModel: 'Nền tảng / Mô hình', rank: 'Hạng', user: 'Người dùng' },
    empty: { title: 'Không có dữ liệu để hiển thị', description: 'Hãy thử thay đổi khoảng thời gian hoặc bộ lọc' },
    bucket: { minutes: 'Nhóm {count} phút', hours: 'Nhóm {count} giờ', days: 'Nhóm {count} ngày' },
    matrix: {
      title: 'Xu hướng khả dụng', description: 'Mỗi hàng là một chiều kênh và mỗi ô là một khoảng tổng hợp; di chuột để xem chi tiết', wheelZoomX: 'Cuộn chuột trên các ô để phóng to (khoảng hẹp hơn, ô rộng hơn)', dimension: 'Chiều kênh', emptyTitle: 'Không có dữ liệu ma trận cho khoảng đã chọn', legendAria: 'Chú giải điểm sức khỏe', bad: 'Kém', good: 'Tốt', healthyLegend: 'Khỏe mạnh (≥80)', warningLegend: 'Cần theo dõi (50–79)', criticalLegend: 'Nghiêm trọng (<50)', unknownLegend: 'Không có lưu lượng / mẫu chưa đủ', noTraffic: 'Không có lưu lượng trong khoảng này', noTrafficAt: '{time} · không có lưu lượng', scoreLine: 'Điểm sức khỏe {score}', resetZoom: 'Đặt lại thu phóng'
    },
    chart: {
      title: 'Xu hướng khả dụng', description: 'Tỷ lệ lỗi · token đầu tiên P50 · tỷ lệ cache', emptyTitle: 'Không có dữ liệu xu hướng cho khoảng đã chọn', errorLegend: 'Tỷ lệ lỗi (trục trái %)', cacheLegend: 'Tỷ lệ cache (trục trái %)', ttftLegend: 'Token đầu tiên P50 (trục phải)', errorDataset: 'Xu hướng tỷ lệ lỗi %', cacheDataset: 'Xu hướng tỷ lệ cache %', ttftDataset: 'Xu hướng token đầu tiên P50 (ms)', percentAxis: 'Tỷ lệ %', resetZoom: 'Đặt lại thu phóng'
    },
    errorDetail: { http: 'HTTP {code}', upstream: 'Upstream {code}', noMessage: 'Không có thông báo lỗi', empty: 'Chỉ hiển thị tỷ lệ theo danh mục (mẫu thông báo chỉ dành cho quản trị viên)' },
    errorCategories: {
      content_policy: 'Chính sách nội dung', authentication: 'Xác thực', context_limit: 'Giới hạn ngữ cảnh', invalid_request: 'Yêu cầu không hợp lệ', model_unsupported: 'Mô hình không hỗ trợ', group_access: 'Quyền truy cập nhóm', quota_or_balance: 'Hạn mức hoặc số dư', account_pool_unavailable: 'Nhóm tài khoản không khả dụng', rate_or_capacity: 'Tốc độ hoặc dung lượng', timeout: 'Hết thời gian chờ', transport_or_stream: 'Truyền tải hoặc luồng dữ liệu', upstream_forbidden: 'Upstream từ chối', not_found: 'Không tìm thấy', client_cancelled: 'Client đã hủy', upstream_5xx: 'Upstream 5xx', internal: 'Nội bộ', other: 'Khác'
    },
    rank: {
      gold: 'Hạng 1 vàng',
      silver: 'Hạng 2 bạc',
      bronze: 'Hạng 3 đồng',
      place: 'Hạng {n}',
      unranked: 'Chưa xếp hạng',
    },
    settings: {
      title: 'Cấu hình giám sát dữ liệu V2',
      description:
        'Cấu hình các chiều tổng hợp sử dụng thụ động (nền tảng / mô hình / nhóm) và chu kỳ làm mới. Màu sắc sức khỏe và chi tiết trên trang /monitor của người dùng hiển thị tỷ lệ, RPM và TPM — không phải khối lượng yêu cầu tuyệt đối.',
      save: 'Lưu',
      loadFailed: 'Không tải được cấu hình V2',
      saveSuccess: 'Đã lưu cấu hình giám sát V2',
      saveFailed: 'Không lưu được cấu hình V2',
      modeBanner:
        'Chế độ hệ thống hiện là {mode}. Tổng hợp theo phút V2 sẽ không chạy; cấu hình này có thể chuẩn bị sẵn ngay bây giờ và có hiệu lực sau khi chuyển sang {modeV2}. Đổi chế độ tại Cài đặt hệ thống → Công tắc tính năng.',
      modeClosed: 'Giám sát kênh đã tắt',
      modeV1: 'V1 kiểm tra chủ động',
      modeV2: 'V2 giám sát thụ động',
      enableTitle: 'Bật tổng hợp V2',
      enableHint:
        'Áp dụng khi chế độ hệ thống là V2. Tắt tùy chọn này chỉ dừng tổng hợp của cấu hình này; công tắc chế độ hệ thống vẫn nằm ở Công tắc tính năng.',
      refreshTitle: 'Chu kỳ tổng hợp',
      refreshHint: 'Ảnh hưởng đến độ chi tiết thời gian của ma trận và chu kỳ làm mới',
      refreshAria: 'Chu kỳ tổng hợp',
      platformsTitle: 'Nền tảng và mô hình',
      platformsHint:
        'Để trống = hiển thị mọi tên mô hình thực; nếu điền, chỉ các mô hình được liệt kê có hàng riêng, phần còn lại gộp vào “Khác”',
      modelsPlaceholder: 'Trống = mọi mô hình thực; hoặc liệt kê các mô hình phổ biến (còn lại → Khác)',
      badgeAllModels: 'Mọi mô hình',
      badgeOther: '+ Khác',
      groupsTitle: 'Nhóm được giám sát',
      groupsSelected: 'Đã chọn {count} nhóm',
      groupsAll: 'Mọi nhóm',
      groupsEmpty: 'Không có nhóm nào',
      errorsTitle: 'Danh mục lỗi và bỏ qua',
      errorsHint:
        'Các danh mục được đánh dấu “bỏ qua” bị loại khỏi tỷ lệ lỗi và điểm sức khỏe, nhưng vẫn hiển thị mờ trong phân tích lỗi. Lỗi không khớp sẽ gộp vào “Khác”.',
      ignoredSummary: 'Đã bỏ qua {ignored} danh mục · tính vào tỷ lệ lỗi {counted} danh mục',
      healthTitle: 'Ngưỡng sức khỏe',
      healthHint:
        'Kiểm soát dải màu hiển thị cho người dùng và điểm tổng thể. Mặc định khá khoan dung để tỷ lệ lỗi nhỏ hoặc cache thấp không lập tức bị coi là không khỏe mạnh.',
      fields: {
        minimumSample: 'Số mẫu tối thiểu',
        warningError: 'Ngưỡng theo dõi tỷ lệ lỗi %',
        criticalError: 'Ngưỡng nghiêm trọng tỷ lệ lỗi %',
        targetTtft: 'Mục tiêu TTFT (ms)',
        warningTtft: 'Ngưỡng theo dõi TTFT (ms)',
        criticalTtft: 'Ngưỡng nghiêm trọng TTFT (ms)',
        warningCache: 'Ngưỡng theo dõi tỷ lệ cache %',
        criticalCache: 'Ngưỡng nghiêm trọng tỷ lệ cache %',
      },
      namedModelsEmpty: 'Danh sách mô hình theo nền tảng đang trống: mọi tên mô hình thực sẽ được hiển thị (không gộp vào “Khác”).',
      namedModelsCount: 'Đang hiển thị {count} chiều mô hình được đặt tên; các mô hình chưa liệt kê sẽ gộp vào “Khác” theo từng nền tảng.',
      userContractTitle: 'Cam kết hiển thị cho người dùng',
      userContract: {
        health: 'Trọng số màu sức khỏe: tỷ lệ lỗi 60% + token đầu tiên P50 20% + tỷ lệ cache 20% (ngưỡng có thể cấu hình ở trên)',
        trend: 'Xu hướng có thể chuyển giữa ma trận nhịp và biểu đồ đường (lỗi · cache · token đầu tiên)',
        latency: 'Độ trễ hiển thị AVG · P50 · P90; không hiển thị số yêu cầu / lỗi tuyệt đối',
        models: 'Danh sách mô hình trống sẽ hiển thị tên thật và không bao giờ dồn hết vào “Khác”',
      },
    },
    admin: {
      descriptionV1:
        'Chế độ hệ thống là V1 kiểm tra chủ động: quản lý các probe giám sát và chạy kiểm tra ngay bây giờ; tổng hợp V2 không chạy.',
      descriptionV2:
        'Chế độ hệ thống là V2 giám sát thụ động: cấu hình các chiều tổng hợp; probe chủ động V1 không chạy.',
      tabAria: 'Quản lý giám sát',
      tabV2: 'Cấu hình giám sát dữ liệu V2',
      tabV1Active: 'Probe chủ động V1',
      tabV1History: 'Lịch sử V1 (probe không hoạt động ở chế độ hiện tại)',
    },
  },
}
