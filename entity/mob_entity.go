// Package entity 生物实体实现
package entity

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/nbt"
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// MobType 生物类型枚举
type MobType int32

const (
	MobTypeZombie   MobType = 54 // 僵尸
	MobTypeSkeleton MobType = 51 // 骷髅
	MobTypeCreeper  MobType = 50 // 爬行者
	MobTypeSpider   MobType = 52 // 蜘蛛
	MobTypeCow      MobType = 92 // 牛
	MobTypePig      MobType = 90 // 猪
	MobTypeChicken  MobType = 93 // 鸡
	MobTypeSheep    MobType = 91 // 羊
)

// MobEntity 生物实体
type MobEntity struct {
	Entity
	MobType       MobType          // 生物类型
	MaxHealth     float32          // 最大生命值
	Health        float32          // 当前生命值
	AttackDamage  float32          // 攻击力
	MovementSpeed float32          // 移动速度
	FollowRange   float32          // 追踪范围
	TargetPlayer  *PlayerEntity    // 攻击目标
	LastMoveTime  time.Time        // 上次移动时间
	MoveDirection Vector3          // 移动方向
	VelocityX     float64          // X速度
	VelocityY     float64          // Y速度
	VelocityZ     float64          // Z速度
	NBTData       *nbt.CompoundTag // NBT数据存储
}

// Vector3 三维向量
type Vector3 struct {
	X, Y, Z float64
}

// MobAI 生物AI接口
type MobAI interface {
	Update(mob *MobEntity)           // 每tick更新
	FindTarget(mob *MobEntity)       // 寻找目标
	MoveTowardTarget(mob *MobEntity) // 向目标移动
	AttackTarget(mob *MobEntity)     // 攻击目标
}

// DefaultMobAI 默认AI实现
type DefaultMobAI struct{}

// Update AI更新
func (ai *DefaultMobAI) Update(mob *MobEntity) {
	if mob.Health <= 0 {
		return
	}

	// 寻找目标
	ai.FindTarget(mob)

	// 如果有目标，向其移动
	if mob.TargetPlayer != nil {
		ai.MoveTowardTarget(mob)

		// 检查距离，如果足够近则攻击
		distance := calculateDistance(mob.X, mob.Y, mob.Z,
			mob.TargetPlayer.X, mob.TargetPlayer.Y, mob.TargetPlayer.Z)

		if distance < 2.0 { // 距离小于2格时攻击
			ai.AttackTarget(mob)
		}
	} else {
		// 没有目标，随机游荡
		ai.Wander(mob)
	}
}

// FindTarget 寻找最近的玩家作为目标
func (ai *DefaultMobAI) FindTarget(mob *MobEntity) {
	// 简化实现：查找所有玩家实体
	players := GlobalEntityManager.GetAllPlayers()
	var nearestPlayer *PlayerEntity
	minDistance := float64(mob.FollowRange)

	for _, player := range players {
		distance := calculateDistance(mob.X, mob.Y, mob.Z,
			player.X, player.Y, player.Z)

		if distance < float64(minDistance) {
			minDistance = distance
			nearestPlayer = player
		}
	}

	if nearestPlayer != nil {
		mob.TargetPlayer = nearestPlayer
	}
}

// MoveTowardTarget 向目标移动
func (ai *DefaultMobAI) MoveTowardTarget(mob *MobEntity) {
	if mob.TargetPlayer == nil {
		return
	}

	// 计算方向向量
	dx := mob.TargetPlayer.X - mob.X
	dy := mob.TargetPlayer.Y - mob.Y
	dz := mob.TargetPlayer.Z - mob.Z

	// 归一化
	length := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if length > 0 {
		dx /= length
		dy /= length
		dz /= length
	}

	// 应用移动
	mob.VelocityX = dx * float64(mob.MovementSpeed)
	// 只有飞行生物才由 AI 控制 Y 轴移动，普通生物由重力控制
	if mob.MobType == 56 || mob.MobType == 61 { // Ghast, Blaze 等
		mob.VelocityY = dy * float64(mob.MovementSpeed)
	} else {
		// 普通地面生物，如果目标在上方，可以尝试跳跃
		if dy > 0.5 && mob.OnGround {
			mob.VelocityY = 0.42 // 基础跳跃高度
		}
	}
	mob.VelocityZ = dz * float64(mob.MovementSpeed)

	mob.LastMoveTime = time.Now()
}

// AttackTarget 攻击目标
func (ai *DefaultMobAI) AttackTarget(mob *MobEntity) {
	if mob.TargetPlayer == nil {
		return
	}

	// 减少目标生命值
	currentHealth := mob.TargetPlayer.Metadata[7].Value.(float32)
	newHealth := currentHealth - mob.AttackDamage

	// 更新元数据（需要先读取，修改，再赋值）
	metadata := mob.TargetPlayer.Metadata[7]
	metadata.Value = newHealth
	mob.TargetPlayer.Metadata[7] = metadata

	fmt.Printf("[MobAI] %s 攻击了 %s, 剩余生命值: %.1f\n",
		mob.getDisplayName(), mob.TargetPlayer.Username, newHealth)

	// 如果目标死亡，清空目标
	if newHealth <= 0 {
		mob.TargetPlayer = nil
	}
}

// Wander 随机游荡
func (ai *DefaultMobAI) Wander(mob *MobEntity) {
	// 每5秒改变一次方向
	if time.Since(mob.LastMoveTime) > 5*time.Second {
		// 随机方向
		angle := rand.Float64() * 2 * math.Pi
		speed := float64(mob.MovementSpeed * 0.3) // 游荡速度较慢

		mob.VelocityX = math.Cos(angle) * speed
		mob.VelocityZ = math.Sin(angle) * speed
		mob.VelocityY = 0

		mob.LastMoveTime = time.Now()
	}
}

// calculateDistance 计算两点间距离
func calculateDistance(x1, y1, z1, x2, y2, z2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// getDisplayName 获取显示名称
func (mob *MobEntity) getDisplayName() string {
	switch mob.MobType {
	case MobTypeZombie:
		return "Zombie"
	case MobTypeSkeleton:
		return "Skeleton"
	case MobTypeCreeper:
		return "Creeper"
	case MobTypeSpider:
		return "Spider"
	case MobTypeCow:
		return "Cow"
	case MobTypePig:
		return "Pig"
	case MobTypeChicken:
		return "Chicken"
	case MobTypeSheep:
		return "Sheep"
	default:
		return "UnknownMob"
	}
}

// CreateMob 创建生物
func (em *EntityManager) CreateMob(mobType MobType, x, y, z float64) int32 {
	em.mu.Lock()
	defer em.mu.Unlock()

	eid := em.nextEID
	em.nextEID++

	mob := &MobEntity{
		Entity: Entity{
			EID:      eid,
			Type:     EntityTypeMob,
			X:        x,
			Y:        y,
			Z:        z,
			Yaw:      0,
			Pitch:    0,
			OnGround: false,
			Metadata: make(map[byte]EntityMetadata),
		},
		MobType:     mobType,
		VelocityX:   0,
		VelocityY:   0,
		VelocityZ:   0,
	}

	// 根据生物类型设置属性
	em.setupMobAttributes(mob)

	// 设置默认元数据 (1.12.2 标准)
	mob.Metadata[0] = EntityMetadata{Index: 0, Type: 0, Value: byte(0)}    // Flags
	mob.Metadata[1] = EntityMetadata{Index: 1, Type: 1, Value: int32(300)} // Air
	mob.Metadata[2] = EntityMetadata{Index: 2, Type: 3, Value: ""}         // Custom Name (Type 3 = String)
	mob.Metadata[3] = EntityMetadata{Index: 3, Type: 6, Value: false}      // Custom Name Visible (Type 6 = Boolean)
	mob.Metadata[4] = EntityMetadata{Index: 4, Type: 6, Value: false}      // Silent
	mob.Metadata[5] = EntityMetadata{Index: 5, Type: 6, Value: false}      // No Gravity

	// 1.12.2 生物 (Living Entity) 额外元数据
	mob.Metadata[6] = EntityMetadata{Index: 6, Type: 0, Value: byte(0)}    // Hand States (Type 0 = Byte)
	mob.Metadata[7] = EntityMetadata{Index: 7, Type: 2, Value: mob.Health} // Health (Type 2 = Float)
	mob.Metadata[8] = EntityMetadata{Index: 8, Type: 1, Value: int32(0)}   // Potion Effect Color
	mob.Metadata[9] = EntityMetadata{Index: 9, Type: 6, Value: false}      // Potion Effect Ambient
	mob.Metadata[10] = EntityMetadata{Index: 10, Type: 1, Value: int32(0)} // Arrows

	// 针对 Ageable 实体 (如牛、羊、猪)
	if mobType == MobTypeCow || mobType == MobTypeSheep || mobType == MobTypePig || mobType == MobTypeChicken {
		mob.Metadata[12] = EntityMetadata{Index: 12, Type: 6, Value: false} // Is Child (Boolean)
	}

	// 存储到EntityManager
	em.mobs[eid] = mob
	em.entities[eid] = &mob.Entity

	fmt.Printf("[Mob] 创建生物: EID=%d, Type=%s, Pos=(%.1f, %.1f, %.1f)\n",
		eid, mob.getDisplayName(), x, y, z)

	return eid
}

// setupMobAttributes 设置生物属性
func (em *EntityManager) setupMobAttributes(mob *MobEntity) {
	switch mob.MobType {
	case MobTypeZombie:
		mob.MaxHealth = 20
		mob.Health = 20
		mob.AttackDamage = 3
		mob.MovementSpeed = 0.23
		mob.FollowRange = 35
	case MobTypeSkeleton:
		mob.MaxHealth = 20
		mob.Health = 20
		mob.AttackDamage = 2
		mob.MovementSpeed = 0.25
		mob.FollowRange = 15
	case MobTypeCreeper:
		mob.MaxHealth = 20
		mob.Health = 20
		mob.AttackDamage = 22 // 爆炸伤害
		mob.MovementSpeed = 0.3
		mob.FollowRange = 16
	case MobTypeSpider:
		mob.MaxHealth = 16
		mob.Health = 16
		mob.AttackDamage = 2
		mob.MovementSpeed = 0.35
		mob.FollowRange = 16
	case MobTypeCow, MobTypePig, MobTypeChicken, MobTypeSheep:
		mob.MaxHealth = 10
		mob.Health = 10
		mob.AttackDamage = 0
		mob.MovementSpeed = 0.2
		mob.FollowRange = 0 // 被动生物不主动攻击
	default:
		mob.MaxHealth = 10
		mob.Health = 10
		mob.AttackDamage = 1
		mob.MovementSpeed = 0.2
		mob.FollowRange = 16
	}
}

// GetMob 获取生物实体
func (em *EntityManager) GetMob(eid int32) *MobEntity {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.mobs[eid]
}

// GetAllMobs 获取所有生物
func (em *EntityManager) GetAllMobs() []*MobEntity {
	em.mu.RLock()
	defer em.mu.RUnlock()

	mobs := make([]*MobEntity, 0, len(em.mobs))
	for _, mob := range em.mobs {
		mobs = append(mobs, mob)
	}
	return mobs
}

// RemoveMob 移除生物
func (em *EntityManager) RemoveMob(eid int32) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if mob, ok := em.mobs[eid]; ok {
		delete(em.mobs, eid)
		delete(em.entities, eid)
		fmt.Printf("[Mob] 移除生物: EID=%d, Type=%s\n", eid, mob.getDisplayName())
	}
}

// BuildSpawnMobPacket 生成生物生成包 (0x03)
func BuildSpawnMobPacket(mob *MobEntity) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x03) // Packet ID: Spawn Mob

	Network.WriteVarint(buf, mob.EID)
	Protocol.WriteUUID(buf, make([]byte, 16)) // 临时UUID
	Network.WriteVarint(buf, int32(mob.MobType)) // 1.12.2 生物类型是 VarInt

	Protocol.WriteDouble(buf, mob.X)
	Protocol.WriteDouble(buf, mob.Y)
	Protocol.WriteDouble(buf, mob.Z)

	// Yaw和Pitch (角度转字节)
	yawByte := byte(int(mob.Yaw*256.0/360.0) & 0xFF)
	pitchByte := byte(int(mob.Pitch*256.0/360.0) & 0xFF)
	buf.WriteByte(yawByte)
	buf.WriteByte(pitchByte)
	buf.WriteByte(yawByte) // Head Yaw

	// Velocity (Short)
	Protocol.WriteShort(buf, int16(mob.VelocityX*8000))
	Protocol.WriteShort(buf, int16(mob.VelocityY*8000))
	Protocol.WriteShort(buf, int16(mob.VelocityZ*8000))

	// 元数据
	BuildEntityMetadata(buf, mob.Metadata)

	return Protocol.AddLengthPrefix(buf)
}
