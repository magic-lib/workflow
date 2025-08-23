package errorflow

import "fmt"
import "errors"

// TimeoutError 表示超时错误
type TimeoutError struct {
	Msg string
}

// Error 实现error接口
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("超时错误: %s", e.Msg)
}

// BusinessError 表示业务逻辑错误
type BusinessError struct {
	Code    int    // 业务错误码
	Message string // 业务错误信息
}

// Error 实现error接口
func (e *BusinessError) Error() string {
	return fmt.Sprintf("业务错误 (代码: %d): %s", e.Code, e.Message)
}

// IsTimeoutError 辅助函数：判断错误是否为超时错误
func IsTimeoutError(err error) bool {
	var te *TimeoutError
	return errors.As(err, &te)
}

// IsBusinessError 辅助函数：判断错误是否为业务错误
func IsBusinessError(err error) bool {
	var be *BusinessError
	return errors.As(err, &be)
}
