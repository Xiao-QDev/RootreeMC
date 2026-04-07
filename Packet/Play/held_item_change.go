// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// HeldItemChangePacket 手持物品变更包 - 设置玩家当前选中的快捷栏槽位
type HeldItemChangePacket struct {
	Slot byte // 快捷栏槽位 (0-8)，Minecraft 1.12.2 使用 Byte 类型
}

// BuildHeldItemChange 构建手持物品变更包
// 参数: slot - 快捷栏槽位 (0-8)
// 返回: 完整的手持物品变更包字节数据（包含长度前缀）
// Minecraft 1.12.2 (协议 340) Held Item Change 包 ID: 0x3A
func BuildHeldItemChange(slot byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x3A) // Packet ID: 0x3A
	buf.WriteByte(slot)
	return Protocol.AddLengthPrefix(buf)
}

// BuildDefaultHeldItem 构建默认的手持物品包（选中第一个槽位）
// 返回: 选中快捷栏第一个槽位的包
func BuildDefaultHeldItem() []byte {
	return BuildHeldItemChange(0)
}
