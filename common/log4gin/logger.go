package log4gin

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"time"
)

// LoggerConfig defines the config for Logger middleware.
type LoggerConfig struct {
	// Optional. Default value is gin.defaultLogFormatter
	Formatter LogFormatter

	// Output is a writer where logs are written.
	// Optional. Default value is gin.DefaultWriter.
	Output io.Writer

	// SkipPaths is an url path array which logs are not written.
	// Optional.
	SkipPaths []string
}

// LogFormatter gives the signature of the formatter function passed to LoggerWithFormatter
type LogFormatter func(params LogFormatterParams) string

// LogFormatterParams is the structure any formatter will be handed when time to log4gin comes
type LogFormatterParams struct {
	Request *http.Request

	// TimeStamp shows the time after the server returns a response.
	TimeStamp time.Time

	// ClientIP equals Context's ClientIP method.
	ClientIP string
	// Method is the HTTP method given to the request.
	Method string
	// Path is a path the client requests.
	Path string
}

// defaultLogFormatter is the default log4gin format function Logger middleware uses.
var defaultLogFormatter = func(param LogFormatterParams) string {

	return fmt.Sprintf("[GIN-REQ] %v | %15s | %-7s %#v\n",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		param.ClientIP,
		param.Method, param.Path,
	)
}

// LoggerWithConfig instance a Logger middleware with config.
func LoggerWithConfig(conf LoggerConfig) gin.HandlerFunc {
	formatter := conf.Formatter
	if formatter == nil {
		formatter = defaultLogFormatter
	}

	out := conf.Output
	if out == nil {
		out = gin.DefaultWriter
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		param := LogFormatterParams{
			Request: c.Request,
		}
		param.TimeStamp = time.Now()

		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		if raw != "" {
			path = path + "?" + raw
		}

		param.Path = path

		fmt.Fprint(out, formatter(param))

		// Process request
		c.Next()

	}
}
