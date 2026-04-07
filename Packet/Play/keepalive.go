// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// KeepAlivePacket 保持活跃包 - 服务器定期发送以检测客户端连接状态
type KeepAlivePacket struct {
	KeepAliveID int64 // 随机生成的 ID，客户端必须原样返回
}

// BuildKeepAlive 构建保持活跃包
// 参数: keepAliveID - 随机生成的 ID
// 返回: 完整的保持活跃包字节数据（包含长度前缀）
// Minecraft 1.12.2 (协议 340) KeepAlive (Clientbound) 包 ID: 0x1F
func BuildKeepAlive(keepAliveID int64) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x1F) // Packet ID: 0x1F (Clientbound)
	Protocol.WriteLong(buf, keepAliveID)
	return Protocol.AddLengthPrefix(buf)
}

// ParseKeepAlive 解析保持活跃包（客户端->服务器）
// 参数: data - 保持活跃包的原始字节数据
// 返回: KeepAlive ID，或错误信息
func ParseKeepAlive(data []byte) (int64, error) {
	buf := bytes.NewReader(data)
	return Protocol.ReadLong(buf)
}

// BuildKeepAliveKeepAliveResponse 构建 Keep Alive 响应包（客户端发回相同的ID）
// 用于处理客户端发来的 0x00 (Serverbound) KeepAlive 包
// Minecraft 1.12.2 (协议 340) KeepAlive (Serverbound) 包 ID: 0x00
func BuildKeepAliveKeepAliveResponse(keepAliveID []byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x00) // Serverbound KeepAlive Packet ID
	buf.Write(keepAliveID)
	return Protocol.AddLengthPrefix(buf)
}
