package repository

import "errors"

// 定义Repository层的通用错误类型
// 这些错误类型用于在不同Repository实现之间保持一致的错误处理

var (
	// ErrNotFound 记录不存在错误
	// 当查询的记录在数据库中不存在时返回此错误
	ErrNotFound = errors.New("记录不存在")
	
	// ErrDuplicateEntry 重复条目错误
	// 当插入或更新数据违反唯一性约束时返回此错误
	ErrDuplicateEntry = errors.New("数据重复")
	
	// ErrInvalidCredentials 无效凭据错误
	// 当用户认证失败时返回此错误
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	
	// ErrInvalidInput 无效输入错误
	// 当输入参数不符合要求时返回此错误
	ErrInvalidInput = errors.New("输入参数无效")
	
	// ErrPermissionDenied 权限拒绝错误
	// 当用户没有权限执行某个操作时返回此错误
	ErrPermissionDenied = errors.New("权限不足")
	
	// ErrInternalError 内部错误
	// 当发生意外的内部错误时返回此错误
	ErrInternalError = errors.New("内部错误")
	
	// ErrConnectionFailed 连接失败错误
	// 当数据库连接失败时返回此错误
	ErrConnectionFailed = errors.New("数据库连接失败")
	
	// ErrTimeout 超时错误
	// 当操作超时时返回此错误
	ErrTimeout = errors.New("操作超时")
	
	// ErrTooManyRequests 请求过多错误
	// 当请求频率超过限制时返回此错误
	ErrTooManyRequests = errors.New("请求过于频繁")
)

// IsNotFound 检查错误是否为记录不存在错误
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsDuplicateEntry 检查错误是否为重复条目错误
func IsDuplicateEntry(err error) bool {
	return errors.Is(err, ErrDuplicateEntry)
}

// IsInvalidCredentials 检查错误是否为无效凭据错误
func IsInvalidCredentials(err error) bool {
	return errors.Is(err, ErrInvalidCredentials)
}

// IsInvalidInput 检查错误是否为无效输入错误
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsPermissionDenied 检查错误是否为权限拒绝错误
func IsPermissionDenied(err error) bool {
	return errors.Is(err, ErrPermissionDenied)
}

// IsInternalError 检查错误是否为内部错误
func IsInternalError(err error) bool {
	return errors.Is(err, ErrInternalError)
}

// IsConnectionFailed 检查错误是否为连接失败错误
func IsConnectionFailed(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}

// IsTimeout 检查错误是否为超时错误
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsTooManyRequests 检查错误是否为请求过多错误
func IsTooManyRequests(err error) bool {
	return errors.Is(err, ErrTooManyRequests)
}