package pkg

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// 尝试转为 gRPC 状态错误
	if st, ok := status.FromError(err); ok {
		code := st.Code()
		// Unavailable 代表服务不可用（连不上）
		// DeadlineExceeded 可能伴随超时，也可视为连接/网络问题
		return code == codes.Unavailable || code == codes.DeadlineExceeded
	}
	return false
}
