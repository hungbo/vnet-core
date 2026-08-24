export const REG_USER_NAME = /^[\u4E00-\u9FA5a-zA-Z0-9_-]{4,16}$/;

/** Phone reg */
export const REG_PHONE =
  /^[1](([3][0-9])|([4][01456789])|([5][012356789])|([6][2567])|([7][0-8])|([8][0-9])|([9][012356789]))[0-9]{8}$/;

/**
 * Password reg
 *
 * 6-18 characters, including letters, numbers, and underscores
 */
// 6-18 ký tự, cấm dấu cách, còn lại cho hết.
//
// Bản cũ là /^\w{6,18}$/ — \w chỉ có [A-Za-z0-9_], nên mọi mật khẩu chứa ký
// tự đặc biệt đều bị chặn. Rule này dùng chung cho CẢ form đăng nhập, nên nó
// khoá luôn người dùng ra ngoài với mật khẩu hợp lệ đặt từ nơi khác: máy chủ
// nhận admin@123 bình thường mà form không cho bấm, và không có request nào
// rời khỏi trình duyệt để mà đi tìm nguyên nhân.
//
// Cấm dấu cách thì giữ: khoảng trắng thừa ở đầu/cuối là thứ không nhìn thấy
// được trong ô mật khẩu.
export const REG_PWD = /^\S{6,18}$/;

/** Email reg */
export const REG_EMAIL = /^\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*$/;

/** Six digit code reg */
export const REG_CODE_SIX = /^\d{6}$/;

/** Four digit code reg */
export const REG_CODE_FOUR = /^\d{4}$/;

/** Url reg */
export const REG_URL =
  /(((^https?:(?:\/\/)?)(?:[-;:&=+$,\w]+@)?[A-Za-z0-9.-]+(?::\d+)?|(?:www.|[-;:&=+$,\w]+@)[A-Za-z0-9.-]+)((?:\/[+~%/.\w-_]*)?\??(?:[-+=&;%@.\w_]*)#?(?:[\w]*))?)$/;
