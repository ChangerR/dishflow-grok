package apperr

import "fmt"

type Error struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func WithDetails(status int, code, message string, details any) *Error {
	return &Error{Status: status, Code: code, Message: message, Details: details}
}

var (
	Unauthorized          = New(401, "UNAUTHORIZED", "请先登录")
	Forbidden             = New(403, "FORBIDDEN", "没有权限执行该操作")
	NotFound              = New(404, "NOT_FOUND", "资源不存在")
	Conflict              = New(409, "CONFLICT", "资源冲突")
	IdempotencyConflict   = New(409, "IDEMPOTENCY_CONFLICT", "幂等键已用于不同请求")
	StateConflict         = New(409, "STATE_CONFLICT", "状态已变化，请刷新后重试")
	QuoteExpired          = New(409, "QUOTE_EXPIRED", "报价已过期，请重新确认金额")
	QuoteMismatch         = New(409, "QUOTE_MISMATCH", "购物车与报价不一致")
	TableNotFound         = New(404, "TABLE_NOT_FOUND", "桌码无效或已轮换")
	TableDisabled         = New(409, "TABLE_DISABLED", "桌台已停用，请重新扫码")
	PickupSlotFull        = New(409, "PICKUP_SLOT_FULL", "该时段刚刚约满，请重新选择")
	PickupTimeInvalid     = New(400, "PICKUP_TIME_INVALID", "预约时间不合法")
	PaymentUnavailable    = New(409, "PAYMENT_UNAVAILABLE", "支付暂不可用")
	RefundConflict        = New(409, "REFUND_CONFLICT", "已存在有效退款")
	InsufficientPoints    = New(409, "INSUFFICIENT_POINTS", "积分不足")
	WechatAppIDConflict   = New(409, "WECHAT_APPID_CONFLICT", "该微信 AppID 已被其他门店占用")
	StoreClosed           = New(409, "STORE_CLOSED", "门店休息中，暂不可结算")
	Validation            = func(msg string) *Error { return New(400, "VALIDATION_ERROR", msg) }
	RateLimited           = New(429, "RATE_LIMITED", "尝试过于频繁，请稍后再试")
	Internal              = New(500, "INTERNAL_ERROR", "服务暂时不可用")
)
