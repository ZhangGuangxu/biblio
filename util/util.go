// Package util contains utility tools
package util

// Reasons of closing client
const (
	InvalidReason          = 0
	HeartbeatTimeout       = 1 // 心跳包超时
	AnotherClientConnected = 2 // 账号在其它客户端登录
	ServerClosed           = 3 // 服务器关闭
)
