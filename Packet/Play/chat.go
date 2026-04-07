// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/Uuid"
	"bytes"
	"encoding/json"
	"fmt"
)

// ChatMessagePacket 聊天消息包 - 服务器发送聊天消息给客户端
type ChatMessagePacket struct {
	Message  string           // JSON 格式的聊天组件文本
	Position byte             // 消息位置: 0=聊天框, 1=系统消息, 2=物品栏上方
	Sender   *Uuid.PlayerUUID // 发送者 UUID
}

// BuildChatMessage 构建聊天消息包
// 参数:
//   - message: JSON 格式的聊天消息文本
//   - position: 消息显示位置 (0=聊天框, 1=系统消息, 2=物品栏上方)
//   - sender: 发送者 UUID
//
// 返回: 完整的聊天消息包字节数据（包含长度前缀）
func BuildChatMessage(message string, position byte, sender *Uuid.PlayerUUID) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x0F) // Packet ID (Play 状态, 1.12.2 = 0x0F)
	Protocol.WriteString(buf, message)
	buf.WriteByte(position)

	return Protocol.AddLengthPrefix(buf)
}

// BuildSystemMessage 构建系统消息包
// 用于发送服务器提示、警告或错误信息
func BuildSystemMessage(text string) []byte {
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	return BuildChatMessage(string(jsonBytes), 1, nil) // Position=1，不需要 UUID
}

// BuildSimpleChatMessage 构建普通玩家聊天消息
func BuildSimpleChatMessage(username, message string, sender *Uuid.PlayerUUID) []byte {
	fullText := fmt.Sprintf("<%s> %s", username, message)
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: fullText,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	return BuildChatMessage(string(jsonBytes), 0, sender)
}

// BuildActionBarMessage 构建物品栏上方消息
// 参数: text - 要显示的文本
// 返回: 完整的 ActionBar 消息包字节数据（ActionBar 不需要 Sender UUID）
func BuildActionBarMessage(text string) []byte {
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	return BuildChatMessage(string(jsonBytes), 2, nil) // Position=2，不需要 UUID
}
