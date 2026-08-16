export default {
  audit: {
    title: 'Nhật ký kiểm toán',
    description: 'Ghi lại các thao tác trên mặt phẳng quản trị của quản trị viên và người dùng. Thông tin xác thực trong tiêu đề chỉ giữ lại ký tự đầu/cuối và nội dung yêu cầu bị che. Không thể xóa từng mục riêng lẻ; xóa toàn bộ yêu cầu xác thực hai yếu tố.',
    clearAll: 'Xóa tất cả',
    empty: 'Chưa có nhật ký kiểm toán',
    loadFailed: 'Không tải được nhật ký kiểm toán',
    filters: {
      all: 'Tất cả',
      q: 'Từ khóa',
      qPlaceholder: 'Đường dẫn / hành động / email người thực hiện',
      actorEmail: 'Email người thực hiện',
      action: 'Hành động',
      clientIp: 'IP client',
      method: 'Phương thức',
      authMethod: 'Phương thức xác thực',
      result: 'Kết quả',
      resultSuccess: 'Thành công',
      resultFailure: 'Thất bại',
    },
    columns: {
      time: 'Thời gian',
      actor: 'Người thực hiện',
      action: 'Hành động',
      result: 'Kết quả',
      clientIp: 'IP client',
      detail: 'Chi tiết'
    },
    detail: {
      title: 'Chi tiết nhật ký kiểm toán',
      latency: 'Độ trễ',
      requestId: 'Mã yêu cầu',
      userAgent: 'User-Agent',
      requestBody: 'Nội dung yêu cầu (đã che)',
      extra: 'Bổ sung'
    },
    clearConfirm: {
      title: 'Xóa tất cả nhật ký kiểm toán',
      message: 'Thao tác này sẽ xóa vĩnh viễn toàn bộ nhật ký kiểm toán và không thể hoàn tác. Hành động xóa cũng sẽ được ghi lại. Tiếp tục?',
      totpTitle: 'Nhập mã xác thực hai yếu tố',
      totpHint: 'Xóa nhật ký kiểm toán yêu cầu xác minh TOTP mới.',
      success: 'Đã xóa {count} nhật ký kiểm toán',
      failed: 'Không xóa được nhật ký kiểm toán'
    }
  }
}
