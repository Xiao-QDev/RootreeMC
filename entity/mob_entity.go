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
	MeleeCooldownTicks  int         // 近战攻击冷却
	RangedCooldownTicks int         // 远程攻击冷却
	FuseTicks           int         // 苦力怕引爆计时
	WanderTicks         int         // 随机游荡计时
	StrafeTicks         int         // 侧移计时
	WanderAngle         float64     // 当前游荡方向
	StrafeDir           float64     // 侧移方向（-1 或 1）
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

const (
	defaultMeleeCooldownTicks = 20
	skeletonRangedMinRangeSq  = 36.0
	skeletonRangedIdealRangeSq = 100.0
	skeletonRangedMaxRangeSq  = 256.0
	creeperFuseMaxTicks       = 30
	creeperExplosionRadius    = 3.5
)

// PlayerDamageHandler 处理玩家伤害（由外层系统注册，避免循环导入）
type PlayerDamageHandler func(target *PlayerEntity, amount float32, cause string) (newHealth float32, dead bool)

var playerDamageHandler PlayerDamageHandler

// RegisterPlayerDamageHandler 注册玩家伤害处理器
func RegisterPlayerDamageHandler(handler PlayerDamageHandler) {
	playerDamageHandler = handler
}

// MobDestroyHandler 处理生物销毁广播（由外层系统注册，避免循环导入）
type MobDestroyHandler func(eid int32)

var mobDestroyHandler MobDestroyHandler

// RegisterMobDestroyHandler 注册生物销毁处理器
func RegisterMobDestroyHandler(handler MobDestroyHandler) {
	mobDestroyHandler = handler
}

// Update AI更新
func (ai *DefaultMobAI) Update(mob *MobEntity) {
	if mob.Health <= 0 {
		return
	}

	ai.tickCooldowns(mob)

	switch mob.MobType {
	case MobTypeZombie:
		ai.updateZombie(mob)
	case MobTypeSkeleton:
		ai.updateSkeleton(mob)
	case MobTypeCreeper:
		ai.updateCreeper(mob)
	case MobTypeSpider:
		ai.updateSpider(mob)
	case MobTypeCow, MobTypePig, MobTypeChicken, MobTypeSheep:
		ai.updatePassiveAnimal(mob)
	default:
		ai.updateZombie(mob)
	}
}

// FindTarget 寻找最近的玩家作为目标
func (ai *DefaultMobAI) FindTarget(mob *MobEntity) {
	if mob.FollowRange <= 0 {
		mob.TargetPlayer = nil
		return
	}

	players := GlobalEntityManager.GetAllPlayers()
	var nearestPlayer *PlayerEntity
	maxDistanceSq := float64(mob.FollowRange) * float64(mob.FollowRange)
	minDistanceSq := maxDistanceSq

	for _, player := range players {
		player.Mu.RLock()
		px, py, pz := player.X, player.Y, player.Z
		gamemode := player.Gamemode
		player.Mu.RUnlock()

		if !isCombatGamemode(gamemode) {
			continue
		}

		dx := px - mob.X
		dy := py - mob.Y
		dz := pz - mob.Z
		distanceSq := dx*dx + dy*dy + dz*dz
		if distanceSq >= minDistanceSq {
			continue
		}

		if !ai.canSeePlayer(mob, px, py, pz) {
			continue
		}

		minDistanceSq = distanceSq
		nearestPlayer = player
	}

	mob.TargetPlayer = nearestPlayer
}

// MoveTowardTarget 向目标移动
func (ai *DefaultMobAI) MoveTowardTarget(mob *MobEntity) {
	tx, ty, tz, ok := ai.getTargetPosition(mob)
	if !ok {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		return
	}

	dx := tx - mob.X
	dy := ty - mob.Y
	dz := tz - mob.Z

	length := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if length <= 0.001 {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		return
	}
	dx /= length
	dz /= length

	mob.VelocityX = dx * float64(mob.MovementSpeed)
	mob.VelocityZ = dz * float64(mob.MovementSpeed)

	if dy > 0.5 && mob.OnGround {
		mob.VelocityY = 0.42
	} else if mob.OnGround && globalWorldProvider != nil {
		frontX := mob.X + dx*0.45
		frontZ := mob.Z + dz*0.45
		if globalWorldProvider.IsBlockSolid(frontX, mob.Y+0.1, frontZ) &&
			!globalWorldProvider.IsBlockSolid(frontX, mob.Y+1.2, frontZ) {
			mob.VelocityY = 0.42
		}
	}

	mob.LastMoveTime = time.Now()
}

// AttackTarget 攻击目标
func (ai *DefaultMobAI) AttackTarget(mob *MobEntity) {
	if mob.MeleeCooldownTicks > 0 || mob.TargetPlayer == nil {
		return
	}

	target := mob.TargetPlayer
	target.Mu.RLock()
	targetGamemode := target.Gamemode
	target.Mu.RUnlock()
	if !isCombatGamemode(targetGamemode) {
		mob.TargetPlayer = nil
		return
	}

	mob.MeleeCooldownTicks = ai.getMeleeCooldownTicks(mob)

	if playerDamageHandler != nil {
		_, dead := playerDamageHandler(target, mob.AttackDamage, "mob_attack")
		if dead {
			mob.TargetPlayer = nil
		}
		return
	}

	target.Mu.Lock()
	defer target.Mu.Unlock()
	healthMeta, ok := target.Metadata[7]
	if !ok {
		return
	}
	currentHealth, ok := healthMeta.Value.(float32)
	if !ok {
		return
	}
	if currentHealth <= 0 {
		mob.TargetPlayer = nil
		return
	}

	newHealth := currentHealth - mob.AttackDamage
	if newHealth < 0 {
		newHealth = 0
	}

	healthMeta.Value = newHealth
	target.Metadata[7] = healthMeta

	if newHealth <= 0 {
		mob.TargetPlayer = nil
	}
}

func (ai *DefaultMobAI) updateZombie(mob *MobEntity) {
	ai.FindTarget(mob)
	if mob.TargetPlayer == nil {
		ai.Wander(mob)
		return
	}

	distanceSq := ai.targetDistanceSq(mob)
	if distanceSq <= 4.84 {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		ai.AttackTarget(mob)
		return
	}

	ai.MoveTowardTarget(mob)
}

func (ai *DefaultMobAI) updateSkeleton(mob *MobEntity) {
	ai.FindTarget(mob)
	if mob.TargetPlayer == nil {
		ai.Wander(mob)
		return
	}

	tx, ty, tz, ok := ai.getTargetPosition(mob)
	if !ok {
		ai.Wander(mob)
		return
	}

	dx := tx - mob.X
	dz := tz - mob.Z
	distanceSq := dx*dx + dz*dz
	canSee := ai.canSeePlayer(mob, tx, ty, tz)

	switch {
	case !canSee || distanceSq > skeletonRangedIdealRangeSq:
		ai.MoveTowardTarget(mob)
	case distanceSq < skeletonRangedMinRangeSq:
		ai.moveAwayFromTarget(mob, tx, tz, float64(mob.MovementSpeed)*0.85)
	default:
		ai.strafeAroundTarget(mob, dx, dz, float64(mob.MovementSpeed)*0.7)
	}

	if canSee && distanceSq <= skeletonRangedMaxRangeSq && mob.RangedCooldownTicks <= 0 {
		ai.attackRangedTarget(mob)
	}
}

func (ai *DefaultMobAI) updateCreeper(mob *MobEntity) {
	ai.FindTarget(mob)
	if mob.TargetPlayer == nil {
		mob.FuseTicks = 0
		ai.Wander(mob)
		return
	}

	tx, ty, tz, ok := ai.getTargetPosition(mob)
	if !ok {
		mob.FuseTicks = 0
		ai.Wander(mob)
		return
	}

	distance := calculateDistance(mob.X, mob.Y, mob.Z, tx, ty, tz)
	canSee := ai.canSeePlayer(mob, tx, ty, tz)

	if canSee && distance <= 3.0 {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		mob.FuseTicks++
		if mob.FuseTicks >= creeperFuseMaxTicks {
			ai.explodeCreeper(mob)
		}
		return
	}

	mob.FuseTicks = 0
	ai.MoveTowardTarget(mob)
}

func (ai *DefaultMobAI) updateSpider(mob *MobEntity) {
	darkEnough := ai.isSpiderAggressive(mob)
	if !darkEnough {
		if mob.TargetPlayer != nil && rand.Intn(100) == 0 {
			mob.TargetPlayer = nil
		}
		if mob.TargetPlayer == nil {
			ai.Wander(mob)
			return
		}
	}

	if mob.TargetPlayer == nil {
		ai.FindTarget(mob)
	}
	if mob.TargetPlayer == nil {
		ai.Wander(mob)
		return
	}

	tx, ty, tz, ok := ai.getTargetPosition(mob)
	if !ok {
		ai.Wander(mob)
		return
	}

	distance := calculateDistance(mob.X, mob.Y, mob.Z, tx, ty, tz)
	if distance > 2.2 && distance < 6.0 && mob.OnGround && rand.Intn(20) == 0 {
		ai.leapToTarget(mob, tx, tz)
		return
	}

	if distance <= 2.2 {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		ai.AttackTarget(mob)
		return
	}

	ai.MoveTowardTarget(mob)
}

func (ai *DefaultMobAI) updatePassiveAnimal(mob *MobEntity) {
	mob.TargetPlayer = nil
	ai.Wander(mob)
}

func (ai *DefaultMobAI) attackRangedTarget(mob *MobEntity) {
	if mob.TargetPlayer == nil {
		return
	}

	target := mob.TargetPlayer
	target.Mu.RLock()
	targetGamemode := target.Gamemode
	target.Mu.RUnlock()
	if !isCombatGamemode(targetGamemode) {
		mob.TargetPlayer = nil
		return
	}

	if playerDamageHandler != nil {
		_, dead := playerDamageHandler(target, mob.AttackDamage, "mob_attack")
		if dead {
			mob.TargetPlayer = nil
		}
	}

	mob.RangedCooldownTicks = 25 + rand.Intn(16)
}

func (ai *DefaultMobAI) explodeCreeper(mob *MobEntity) {
	if playerDamageHandler != nil {
		players := GlobalEntityManager.GetAllPlayers()
		for _, p := range players {
			p.Mu.RLock()
			px, py, pz := p.X, p.Y, p.Z
			gamemode := p.Gamemode
			p.Mu.RUnlock()

			if !isCombatGamemode(gamemode) {
				continue
			}

			distance := calculateDistance(mob.X, mob.Y, mob.Z, px, py, pz)
			if distance > creeperExplosionRadius {
				continue
			}

			scale := 1.0 - distance/creeperExplosionRadius
			damage := mob.AttackDamage * float32(scale)
			if damage < 1 {
				damage = 1
			}
			_, _ = playerDamageHandler(p, damage, "mob_attack")
		}
	}

	eid := mob.EID
	GlobalEntityManager.RemoveMob(eid)
	if mobDestroyHandler != nil {
		mobDestroyHandler(eid)
	}
}

func (ai *DefaultMobAI) moveAwayFromTarget(mob *MobEntity, tx, tz, speed float64) {
	dx := mob.X - tx
	dz := mob.Z - tz
	length := math.Sqrt(dx*dx + dz*dz)
	if length <= 0.001 {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		return
	}
	mob.VelocityX = (dx / length) * speed
	mob.VelocityZ = (dz / length) * speed
}

func (ai *DefaultMobAI) strafeAroundTarget(mob *MobEntity, dx, dz, speed float64) {
	length := math.Sqrt(dx*dx + dz*dz)
	if length <= 0.001 {
		return
	}

	if mob.StrafeTicks <= 0 {
		mob.StrafeTicks = 12 + rand.Intn(20)
		if rand.Intn(2) == 0 {
			mob.StrafeDir = 1
		} else {
			mob.StrafeDir = -1
		}
	}

	perpX := -dz / length
	perpZ := dx / length
	mob.VelocityX = perpX * mob.StrafeDir * speed
	mob.VelocityZ = perpZ * mob.StrafeDir * speed
}

func (ai *DefaultMobAI) leapToTarget(mob *MobEntity, tx, tz float64) {
	dx := tx - mob.X
	dz := tz - mob.Z
	length := math.Sqrt(dx*dx + dz*dz)
	if length <= 0.001 {
		return
	}
	dx /= length
	dz /= length
	mob.VelocityX = dx * 0.45
	mob.VelocityZ = dz * 0.45
	mob.VelocityY = 0.42
}

func (ai *DefaultMobAI) getTargetPosition(mob *MobEntity) (x, y, z float64, ok bool) {
	if mob.TargetPlayer == nil {
		return 0, 0, 0, false
	}

	target := mob.TargetPlayer
	target.Mu.RLock()
	x = target.X
	y = target.Y
	z = target.Z
	gamemode := target.Gamemode
	target.Mu.RUnlock()

	if !isCombatGamemode(gamemode) {
		mob.TargetPlayer = nil
		return 0, 0, 0, false
	}

	return x, y, z, true
}

func (ai *DefaultMobAI) targetDistanceSq(mob *MobEntity) float64 {
	tx, ty, tz, ok := ai.getTargetPosition(mob)
	if !ok {
		return math.MaxFloat64
	}
	dx := tx - mob.X
	dy := ty - mob.Y
	dz := tz - mob.Z
	return dx*dx + dy*dy + dz*dz
}

func (ai *DefaultMobAI) getMeleeCooldownTicks(mob *MobEntity) int {
	switch mob.MobType {
	case MobTypeSpider:
		return 20
	case MobTypeZombie:
		return 20
	default:
		return defaultMeleeCooldownTicks
	}
}

func (ai *DefaultMobAI) isSpiderAggressive(mob *MobEntity) bool {
	x := int32(math.Floor(mob.X))
	y := int32(math.Floor(mob.Y))
	z := int32(math.Floor(mob.Z))
	return isLightZero(x, y, z)
}

func (ai *DefaultMobAI) tickCooldowns(mob *MobEntity) {
	if mob.MeleeCooldownTicks > 0 {
		mob.MeleeCooldownTicks--
	}
	if mob.RangedCooldownTicks > 0 {
		mob.RangedCooldownTicks--
	}
	if mob.StrafeTicks > 0 {
		mob.StrafeTicks--
	}
	if mob.WanderTicks > 0 {
		mob.WanderTicks--
	}
}

// canSeePlayer 简化版视线检测（有方块遮挡则不锁定目标）
func (ai *DefaultMobAI) canSeePlayer(mob *MobEntity, px, py, pz float64) bool {
	if globalWorldProvider == nil {
		return true
	}

	startX, startY, startZ := mob.X, mob.Y+1.62, mob.Z
	endX, endY, endZ := px, py+1.62, pz
	dx := endX - startX
	dy := endY - startY
	dz := endZ - startZ

	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if distance <= 0.001 {
		return true
	}

	// 每格采样一次并限制上限，避免视线检测拖慢性能
	steps := int(distance)
	if steps < 1 {
		steps = 1
	}
	if steps > 48 {
		steps = 48
	}

	invSteps := 1.0 / float64(steps)
	for i := 1; i < steps; i++ {
		t := float64(i) * invSteps
		sx := startX + dx*t
		sy := startY + dy*t
		sz := startZ + dz*t
		if globalWorldProvider.IsBlockSolid(sx, sy, sz) {
			return false
		}
	}

	return true
}

func isCombatGamemode(gamemode int32) bool {
	return gamemode == 0 || gamemode == 2
}

// Wander 随机游荡
func (ai *DefaultMobAI) Wander(mob *MobEntity) {
	if mob.WanderTicks <= 0 {
		mob.WanderTicks = 40 + rand.Intn(80)
		mob.WanderAngle = rand.Float64() * 2 * math.Pi
	}

	if rand.Intn(100) < 15 {
		mob.VelocityX = 0
		mob.VelocityZ = 0
		return
	}

	speed := float64(mob.MovementSpeed) * 0.3
	if mob.MobType == MobTypeChicken {
		speed = float64(mob.MovementSpeed) * 0.25
	}
	mob.VelocityX = math.Cos(mob.WanderAngle) * speed
	mob.VelocityZ = math.Sin(mob.WanderAngle) * speed
	mob.VelocityY = 0
	mob.LastMoveTime = time.Now()
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
		StrafeDir:   1,
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
		mob.FollowRange = 32
	case MobTypeCreeper:
		mob.MaxHealth = 20
		mob.Health = 20
		mob.AttackDamage = 22 // 爆炸伤害
		mob.MovementSpeed = 0.25
		mob.FollowRange = 16
	case MobTypeSpider:
		mob.MaxHealth = 16
		mob.Health = 16
		mob.AttackDamage = 2
		mob.MovementSpeed = 0.30
		mob.FollowRange = 16
	case MobTypeCow, MobTypePig, MobTypeChicken, MobTypeSheep:
		mob.MaxHealth = 10
		mob.Health = 10
		mob.AttackDamage = 0
		if mob.MobType == MobTypeChicken {
			mob.MovementSpeed = 0.25
		} else {
			mob.MovementSpeed = 0.20
		}
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
