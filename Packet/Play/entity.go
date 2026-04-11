// Package Play 实体相关包处理
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/entity"
	"RootreeMC/player"
	"bytes"
	"encoding/binary"
	"math"
)

// HandleUseEntity 处理使用实体（交互/攻击）
// Packet ID: 0x0A (1.12.2)
func HandleUseEntity(client *Network.Network, data []byte) {
	if len(data) < 2 {
		return
	}

	reader := bytes.NewReader(data)
	targetID, _ := Network.ReadVarint(reader)
	actionType, _ := Network.ReadVarint(reader) // 0=交互, 1=攻击, 2=交互(带坐标)

	// 只处理攻击动作
	if actionType != 1 {
		return
	}

	attacker := player.GlobalPlayerManager.GetPlayerByClient(client)
	if attacker == nil || attacker.PlayerEntity == nil {
		return
	}

	// 仅支持攻击生物实体
	mob := entity.GlobalEntityManager.GetMob(targetID)
	if mob == nil || mob.Health <= 0 {
		return
	}

	// 简化近战伤害：每次 4 点
	const baseAttackDamage = float32(4.0)
	mob.Health -= baseAttackDamage
	if mob.Health < 0 {
		mob.Health = 0
	}
	mob.Metadata[7] = entity.EntityMetadata{Index: 7, Type: 2, Value: mob.Health}

	if mob.Health <= 0 {
		entity.GlobalEntityManager.RemoveMob(mob.EID)
		destroyPkt := entity.BuildDestroyEntities([]int32{mob.EID})
		for _, p := range player.GlobalPlayerManager.GetAllOnlinePlayers() {
			_ = p.Client.Send(destroyPkt)
		}
	}
}

// HandleEntityMetadata 处理实体元数据更新
// 这是 S2C 包，这里用于参考
// Packet ID: 0x39 (1.12.2)

// HandleVehicleMove 处理载具移动
// Packet ID: 0x16 (1.12.2)
func HandleVehicleMove(client *Network.Network, data []byte) {
	if len(data) < 32 {
		return
	}

	x := math.Float64frombits(binary.BigEndian.Uint64(data[0:8]))
	y := math.Float64frombits(binary.BigEndian.Uint64(data[8:16]))
	z := math.Float64frombits(binary.BigEndian.Uint64(data[16:24]))
	yaw := math.Float32frombits(binary.BigEndian.Uint32(data[24:28]))
	pitch := math.Float32frombits(binary.BigEndian.Uint32(data[28:32]))

	_ = x
	_ = y
	_ = z
	_ = yaw
	_ = pitch
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
