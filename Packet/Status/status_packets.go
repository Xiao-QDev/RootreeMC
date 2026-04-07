package Status

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// BuildStatusResponse 构建状态响应包 (Packet ID: 0x00)
// 返回包含 MOTD、玩家数、版本信息的 JSON 数据包
// 返回完整数据包格式：[Length:VarInt] [PacketID:0x00] [JSON:String]
func BuildStatusResponse(motd string, maxPlayers, onlinePlayers int, versionName string, protocolVersion int) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x00) // Packet ID

	// 构建 1.12.2 兼容的 JSON 格式 (使用结构体替代 map 以确保稳定性)
	statusObj := struct {
		Description struct {
			Text string `json:"text"`
		} `json:"description"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
		} `json:"players"`
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
	}{
		Description: struct {
			Text string `json:"text"`
		}{Text: motd},
		Players: struct {
			Max    int `json:"max"`
			Online int `json:"online"`
		}{Max: maxPlayers, Online: onlinePlayers},
		Version: struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		}{Name: versionName, Protocol: protocolVersion},
	}
	jsonBytes, _ := json.Marshal(statusObj)
	jsonStr := string(jsonBytes)

	fmt.Printf("[Status] 发送 Status Response, JSON: %s\n", jsonStr)
	Protocol.WriteString(buf, jsonStr)

	// 构建完整数据包（包含长度前缀）
	packetData := buf.Bytes()
	
	// 计算并显示长度信息
	length := len(packetData)
	fmt.Printf("[Status] Packet ID 长度: 1 字节\n")
	fmt.Printf("[Status] JSON 字符串长度: %d 字节\n", length-1)
	fmt.Printf("[Status] 数据部分十六进制: %s\n", hex.EncodeToString(packetData))
	
	fullPacket := &bytes.Buffer{}
	Network.WriteVarint(fullPacket, int32(length))
	fullPacket.Write(packetData)

	result := fullPacket.Bytes()
	fmt.Printf("[Status] 完整包十六进制: %s\n", hex.EncodeToString(result))
	fmt.Printf("[Status] 完整包长度: %d 字节\n", len(result))
	
	return result
}

// BuildStatusResponseFromConfig 根据配置构建状态响应包
// 使用配置文件中的信息
func BuildStatusResponseFromConfig(motd string, maxPlayers int, versionName string, protocolVersion int) []byte {
	// onlinePlayers 暂时设为 0，后续可以添加玩家计数器
	return BuildStatusResponse(motd, maxPlayers, 0, versionName, protocolVersion)
}

// BuildPongResponse 构建 Pong 响应包 (Packet ID: 0x01)
// 用于响应客户端的延迟测试 (Ping)
func BuildPongResponse(packetData []byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x01) // Pong Packet ID
	buf.Write(packetData)          // 直接写入 payload (8字节 Long)

	// 构建完整数据包（包含长度前缀）
	pktData := buf.Bytes()
	fullPacket := &bytes.Buffer{}
	Network.WriteVarint(fullPacket, int32(len(pktData)))
	fullPacket.Write(pktData)

	return fullPacket.Bytes()
}
