package response

import "github.com/gin-gonic/gin"

const (
	CodeSuccess            = 200
	CodeParamErr           = 400
	CodeNotFound           = 404
	CodeServiceUnavailable = 503
	CodeServerErr          = 500
)

// Response 统一返回格式
type Response struct {
	Code  int         `json:"code"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// Success 成功返回
func Success(c *gin.Context, data interface{}, msg ...string) {
	message := "success"
	if len(msg) > 0 {
		message = msg[0]
	}
	c.JSON(200, Response{
		Code: CodeSuccess,
		Msg:  message,
		Data: data,
	})
}

// Fail 失败返回
func Fail(c *gin.Context, code int, msg string, err ...error) {
	errMsg := ""
	if len(err) > 0 && err[0] != nil {
		errMsg = err[0].Error()
	}
	c.JSON(200, Response{
		Code:  code,
		Msg:   msg,
		Error: errMsg,
	})
}
