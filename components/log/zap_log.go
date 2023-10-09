package log

import (
	"github.com/gin-gonic/gin"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

/*
	@func: zap_log的使用
	@author: Andy_文铎
	@time: 2023/10/09
*/

var (
	logger         *zap.Logger
	sugarLogger    *zap.SugaredLogger
	errLogger      *zap.Logger
	sugarErrLogger *zap.SugaredLogger
)

func InitLogger(env string) error {
	var (
		allCore      []zapcore.Core
		allErrorCore []zapcore.Core
	)
	writer := getLogWriter(".log")
	errWriter := getLogWriter("-error.log")
	encoder := getConsoleEncoder()
	var l = new(zapcore.Level)
	l.Set("Debug")
	allCore = append(allCore, zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), zapcore.DebugLevel))
	allErrorCore = append(allErrorCore, zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), zapcore.DebugLevel))
	if env == "prod" {
		allCore = append(allCore, zapcore.NewCore(encoder, writer, zapcore.InfoLevel))
		allErrorCore = append(allErrorCore, zapcore.NewCore(encoder, errWriter, zapcore.ErrorLevel))
	} else if env == "test" {
		allCore = append(allCore, zapcore.NewCore(encoder, writer, zapcore.DebugLevel))
		allErrorCore = append(allErrorCore, zapcore.NewCore(encoder, errWriter, zapcore.ErrorLevel))
	}
	core := zapcore.NewTee(allCore...)
	logger = zap.New(core, zap.AddCaller())
	defer logger.Sync()
	sugarLogger = logger.Sugar()
	zap.ReplaceGlobals(logger)
	errCore := zapcore.NewTee(allErrorCore...)
	errLogger = zap.New(errCore, zap.AddCaller())
	sugarErrLogger = errLogger.Sugar()
	return nil
}

func GetLogInstance() *zap.Logger {
	return logger
}

func GetSugarLogInstance() *zap.SugaredLogger {
	return sugarLogger
}

func GetErrorLogInstance() *zap.Logger {
	return errLogger
}

func GetSugarErrorLogInstance() *zap.SugaredLogger {
	return sugarErrLogger
}

// GinLogger 接收gin框架的默认日志
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		ref, _ := c.GetQuery("ref")
		c.Next()

		cost := time.Since(start)
		logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
			zap.String("ref", ref),
		)
	}
}

// GinRecovery recover掉项目可能出现的panic
func GinRecovery(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, sok := ne.Err.(*os.SyscallError); sok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					errLogger.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					errLogger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					errLogger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func getConsoleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = customTimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getJsonEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = customTimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(suffix string) zapcore.WriteSyncer {
	writer, err := getWriter(suffix)
	if err != nil {
		return nil
	}
	return zapcore.AddSync(writer)
}

// getWriter 日志文件分割，按小时
func getWriter(suffix string) (io.Writer, error) {
	//hook, err := rotatelogs.New(
	//	"/opt/logs/eva-inquire/log/zap-%Y%m%d-%H"+suffix,
	//	rotatelogs.WithLinkName("zap"+suffix),
	//	rotatelogs.WithMaxAge(time.Hour*24*7),
	//	rotatelogs.WithRotationTime(time.Hour),
	//)

	hook, err := rotatelogs.New(
		"./log/zap-%Y%m%d-%H%M"+suffix,
		rotatelogs.WithLinkName("zap"+suffix),
		rotatelogs.WithMaxAge(time.Hour*24*7),
		rotatelogs.WithRotationTime(time.Minute),
	)
	if err != nil {
		return nil, err
	}
	return hook, nil
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}
