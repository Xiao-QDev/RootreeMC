// Package Play 玩家列表包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/binary"
	"fmt"
)

// PlayerListAction 玩家列表操作
const (
	PlayerListActionAdd    = 0 // 添加玩家
	PlayerListActionRemove = 4 // 移除玩家 (1.12.2)
)

// PlayerListEntry 玩家列表条目
type PlayerListEntry struct {
	UUID        []byte // 玩家 UUID（16字节二进制格式）
	Name        string // 玩家名称
	Properties  []PlayerProperty
	Gamemode    int32  // 游戏模式
	Ping        int32  // 延迟（毫秒）
	DisplayName string // 显示名称（可选）
}

// PlayerProperty 玩家属性
type PlayerProperty struct {
	Name      string
	Value     string
	IsSigned  bool
	Signature string
}

// BuildPlayerListAdd 添加玩家到列表
// Minecraft 1.12.2 (协议 340) Player List Item 包 ID: 0x2E
func BuildPlayerListAdd(entries []PlayerListEntry) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x2E) // Packet ID: 0x2E (Player List Item for 1.12.2)
	Network.WriteVarint(buf, PlayerListActionAdd)
	Network.WriteVarint(buf, int32(len(entries)))

	for _, entry := range entries {
		// UUID: 16字节，高位字节在前（网络字节序）
		if len(entry.UUID) == 16 {
			buf.Write(entry.UUID)
		} else {
			fmt.Printf("[PlayerList] UUID 长度错误: %d，期望 16\n", len(entry.UUID))
			buf.Write(make([]byte, 16))
		}
		// 名称
		Protocol.WriteString(buf, entry.Name)
		// 属性数量
		Network.WriteVarint(buf, int32(len(entry.Properties)))
		// 属性
		for _, prop := range entry.Properties {
			Protocol.WriteString(buf, prop.Name)
			Protocol.WriteString(buf, prop.Value)
			Protocol.WriteBoolean(buf, prop.IsSigned)
			if prop.IsSigned {
				Protocol.WriteString(buf, prop.Signature)
			}
		}
		// 游戏模式
		Network.WriteVarint(buf, entry.Gamemode)
		// 延迟
		Network.WriteVarint(buf, entry.Ping)
		// 显示名称（可选）
		if entry.DisplayName != "" {
			Protocol.WriteBoolean(buf, true) // 有显示名称
			Protocol.WriteString(buf, entry.DisplayName)
		} else {
			Protocol.WriteBoolean(buf, false) // 无显示名称
		}
	}

	return Protocol.AddLengthPrefix(buf)
}

// BuildPlayerListAddDebug 添加玩家到列表（带调试输出）
func BuildPlayerListAddDebug(entries []PlayerListEntry) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x2E)
	Network.WriteVarint(buf, PlayerListActionAdd)
	Network.WriteVarint(buf, int32(len(entries)))

	for _, entry := range entries {
		// UUID: 16字节，高位字节在前（网络字节序）
		if len(entry.UUID) == 16 {
			buf.Write(entry.UUID)
		} else {
			fmt.Printf("[PlayerList] UUID 长度错误: %d，期望 16\n", len(entry.UUID))
			buf.Write(make([]byte, 16))
		}
		// 名称
		Protocol.WriteString(buf, entry.Name)
		// 属性数量
		Network.WriteVarint(buf, int32(len(entry.Properties)))
		// 属性
		for _, prop := range entry.Properties {
			Protocol.WriteString(buf, prop.Name)
			Protocol.WriteString(buf, prop.Value)
			Protocol.WriteBoolean(buf, prop.IsSigned)
			if prop.IsSigned {
				Protocol.WriteString(buf, prop.Signature)
			}
		}
		// 游戏模式
		Network.WriteVarint(buf, entry.Gamemode)
		// 延迟
		Network.WriteVarint(buf, entry.Ping)
		// 显示名称（可选）
		if entry.DisplayName != "" {
			Protocol.WriteBoolean(buf, true)
			Protocol.WriteString(buf, entry.DisplayName)
		} else {
			Protocol.WriteBoolean(buf, false)
		}
	}

	packet := Protocol.AddLengthPrefix(buf)

	// 调试输出
	fmt.Printf("[PlayerList] 包长度: %d 字节\n", len(packet))
	fmt.Printf("[PlayerList] 包十六进制: %x\n", packet)
	if len(entries) > 0 && len(entries[0].UUID) == 16 {
		fmt.Printf("[PlayerList] UUID 字节: %x\n", entries[0].UUID)
		// 验证 UUID 结构
		high := binary.BigEndian.Uint64(entries[0].UUID[0:8])
		low := binary.BigEndian.Uint64(entries[0].UUID[8:16])
		fmt.Printf("[PlayerList] UUID High: %016x, Low: %016x\n", high, low)
	}

	return packet
}

// BuildPlayerListAddFromUUID 使用 Uuid.PlayerUUID 构建玩家列表
func BuildPlayerListAddFromUUID(name string, uuidBytes []byte, gamemode int32, ping int32, displayName string) []byte {
	entry := PlayerListEntry{
		UUID:        uuidBytes,
		Name:        name,
		Properties:  []PlayerProperty{},
		Gamemode:    gamemode,
		Ping:        ping,
		DisplayName: displayName,
	}
	return BuildPlayerListAdd([]PlayerListEntry{entry})
}

// BuildPlayerListRemove 从列表移除玩家
// Minecraft 1.12.2 (协议 340) Player List Item 包 ID: 0x2E
func BuildPlayerListRemove(uuids [][]byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x2E)
	Network.WriteVarint(buf, PlayerListActionRemove)
	Network.WriteVarint(buf, int32(len(uuids)))

	for _, uuid := range uuids {
		if len(uuid) == 16 {
			buf.Write(uuid)
		} else {
			buf.Write(make([]byte, 16))
		}
	}

	return Protocol.AddLengthPrefix(buf)
}

// UUIDFromHighLow 从高64位和低64位构建 UUID
func UUIDFromHighLow(high int64, low int64) []byte {
	bytes := make([]byte, 16)
	binary.BigEndian.PutUint64(bytes[0:8], uint64(high))
	binary.BigEndian.PutUint64(bytes[8:16], uint64(low))
	return bytes
}
