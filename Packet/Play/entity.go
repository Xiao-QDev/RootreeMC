// Package Play 实体相关包处理
package Play

import (
	"RootreeMC/Network"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// HandleUseEntity 处理使用实体（交互/攻击）
// Packet ID: 0x0E (1.12.2)
func HandleUseEntity(client *Network.Network, data []byte) {
	if len(data) < 10 {
		return
	}

	reader := bytes.NewReader(data)
	targetID, _ := Network.ReadVarint(reader)
	mouseButton, _ := Network.ReadVarint(reader) // 0=交互, 1=攻击

	// 跳过坐标（如果有）
	if mouseButton == 2 {
		reader.Seek(12, 1) // 3个float64 = 24字节
	}

	action := "交互"
	if mouseButton == 1 {
		action = "攻击"
	}
	fmt.Printf("[Play] 使用实体: target=%d, action=%s\n", targetID, action)
}

// HandleEntityMetadata 处理实体元数据更新
// 这是 S2C 包，这里用于参考
// Packet ID: 0x39 (1.12.2)

// HandleVehicleMove 处理载具移动
// Packet ID: 0x16 (1.12.2)
func HandleVehicleMove(client *Network.Network, data []byte) {
	if len(data) < 25 {
		return
	}

	x := math.Float64frombits(binary.BigEndian.Uint64(data[0:8]))
	y := math.Float64frombits(binary.BigEndian.Uint64(data[8:16]))
	z := math.Float64frombits(binary.BigEndian.Uint64(data[16:24]))
	yaw := math.Float32frombits(binary.BigEndian.Uint32(data[24:28]))
	pitch := math.Float32frombits(binary.BigEndian.Uint32(data[28:32]))

	fmt.Printf("[Play] 载具移动: (%.2f, %.2f, %.2f), yaw=%.2f, pitch=%.2f\n", x, y, z, yaw, pitch)
}

// BuildChangeGameState 构建改变游戏状态包 (0x1E)
// reason: 3=Change game mode
func BuildChangeGameState(reason int32, value float32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x1E) // Change Game State Packet ID
	
	buf.WriteByte(byte(reason))
	binary.Write(buf, binary.BigEndian, value)
	
	// 包装长度前缀
	data := buf.Bytes()
	result := &bytes.Buffer{}
	Network.WriteVarint(result, int32(len(data)))
	result.Write(data)
	
	return result.Bytes()
}
