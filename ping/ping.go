// Package ping 处理 Minecraft Status 阶段的 Ping/Pong 协议
// 用于服务器列表显示延迟值 (ms)
package ping

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// BuildPongResponse 构建 Pong 响应包 (Packet ID: 0x01)
// 用于响应客户端的延迟测试 (Ping)
// 返回完整数据包格式：[Length:VarInt] [PacketID:0x01] [Payload:Long]
// 注意：Status 阶段 Pong 的 Payload 是 Long（8字节固定长度），不是 VarLong
func BuildPongResponse(payload int64) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x01) // Pong Packet ID
	Protocol.WriteLong(buf, payload) // 使用 Long（8字节固定长度）

	// 构建完整数据包（包含长度前缀）
	packetData := buf.Bytes()
	fullPacket := &bytes.Buffer{}
	Network.WriteVarint(fullPacket, int32(len(packetData)))
	fullPacket.Write(packetData)
	
	return fullPacket.Bytes()
}

// ParsePingRequest 解析 Ping Request 数据包中的 payload
// Ping Request 数据包格式：[Payload:Long]
func ParsePingRequest(packetData []byte) (int64, error) {
	return Protocol.ReadLong(bytes.NewReader(packetData))
}
