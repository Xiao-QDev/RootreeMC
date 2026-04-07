 // Package entity 实体系统
 package entity
 
import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
)
 
 // EntityType 实体类型
 type EntityType int32
 
 const (
 	EntityTypePlayer EntityType = 0
 	EntityTypeItem   EntityType = 1
 	EntityTypeMob    EntityType = 2
 )
 
 // Entity 基础实体
 type Entity struct {
 	EID      int32      // 实体ID
 	Type     EntityType // 实体类型
 	X, Y, Z  float64    // 位置
 	Yaw, Pitch float32  // 朝向
 	OnGround bool       // 是否在地面上
 	Metadata map[byte]EntityMetadata // 实体元数据
 }
 
 // EntityMetadata 实体元数据
 type EntityMetadata struct {
 	Index byte
 	Type  byte
 	Value interface{}
 }
 
 // PlayerEntity 玩家实体（继承Entity）
 type PlayerEntity struct {
 	Entity
 	UUID       []byte
 	Username   string
 	Gamemode   int32
 	Properties []PlayerProperty
 	Mu         sync.RWMutex // 保护元数据并发访问
 }
 
 // PlayerProperty 玩家属性
 type PlayerProperty struct {
 	Name      string
 	Value     string
 	IsSigned  bool
 	Signature string
 }
 
// EntityManager 实体管理器
type EntityManager struct {
	nextEID  int32
	entities map[int32]*Entity
	players  map[int32]*PlayerEntity
	mobs     map[int32]*MobEntity // 生物实体
	mu       sync.RWMutex
}

// GlobalEntityManager 全局实体管理器
var GlobalEntityManager *EntityManager

func init() {
	GlobalEntityManager = NewEntityManager()
}

// NewEntityManager 创建新的实体管理器
func NewEntityManager() *EntityManager {
	return &EntityManager{
		nextEID:  1,
		entities: make(map[int32]*Entity),
		players:  make(map[int32]*PlayerEntity),
		mobs:     make(map[int32]*MobEntity),
	}
}
 
 // CreatePlayer 创建玩家实体
 func (em *EntityManager) CreatePlayer(username string, uuid []byte, x, y, z float64) *PlayerEntity {
 	em.mu.Lock()
 	defer em.mu.Unlock()
 	
 	eid := em.nextEID
 	em.nextEID++
 	
 	player := &PlayerEntity{
 		Entity: Entity{
 			EID:      eid,
 			Type:     EntityTypePlayer,
 			X:        x,
 			Y:        y,
 			Z:        z,
 			Yaw:      0,
 			Pitch:    0,
 			OnGround: true,
 			Metadata: make(map[byte]EntityMetadata),
 		},
 		UUID:     uuid,
 		Username: username,
 		Gamemode: 1, // Creative
 		Properties: []PlayerProperty{},
 	}
 	
 	// 1.12.2 (Protocol 340) Player Metadata - 最小必需集合以提高稳定性
 	player.Metadata[0] = EntityMetadata{Index: 0, Type: 0, Value: byte(0)}      // Flags (Byte)
 	player.Metadata[7] = EntityMetadata{Index: 7, Type: 2, Value: float32(20.0)} // Health (Float)
 	player.Metadata[13] = EntityMetadata{Index: 13, Type: 0, Value: byte(127)}    // Skin Parts (Byte)
 	player.Metadata[14] = EntityMetadata{Index: 14, Type: 0, Value: byte(1)}      // Main Hand (Byte)
 	
 	em.entities[eid] = &player.Entity
 	em.players[eid] = player
 	
 	fmt.Printf("[Entity] 创建玩家实体: EID=%d, Username=%s\n", eid, username)
 	return player
 }
 
 // BuildSpawnPlayer 生成玩家包 (0x05)
 func BuildSpawnPlayer(player *PlayerEntity) []byte {
 	player.Mu.RLock()
 	defer player.Mu.RUnlock()
 	
 	buf := &bytes.Buffer{}
 	Network.WriteVarint(buf, 0x05) 
 	
 	Network.WriteVarint(buf, player.EID)
 	Protocol.WriteUUID(buf, player.UUID)
	Protocol.WriteDouble(buf, player.X)
	Protocol.WriteDouble(buf, player.Y)
	Protocol.WriteDouble(buf, player.Z)
	// yaw和pitch都使用相同转换方式
	yawByte := byte(int(player.Yaw*256.0/360.0) & 0xFF)
	pitchByte := byte(int(player.Pitch*256.0/360.0) & 0xFF)
	buf.WriteByte(yawByte)
	buf.WriteByte(pitchByte)
	
	BuildEntityMetadata(buf, player.Metadata)
 	
 	return Protocol.AddLengthPrefix(buf)
 }
 
 // BuildEntityMetadata 构建实体元数据
 func BuildEntityMetadata(buf *bytes.Buffer, metadata map[byte]EntityMetadata) {
 	var indices []int
 	for index := range metadata {
 		indices = append(indices, int(index))
 	}
 	for i := 0; i < len(indices); i++ {
 		for j := i + 1; j < len(indices); j++ {
 			if indices[i] > indices[j] {
 				indices[i], indices[j] = indices[j], indices[i]
 			}
 		}
 	}
 
 	for _, indexInt := range indices {
 		index := byte(indexInt)
 		meta := metadata[index]
 		
 		buf.WriteByte(index)
 		Network.WriteVarint(buf, int32(meta.Type)) // 1.12.2 Type IS VarInt
 		
 		switch meta.Type {
 		case 0: // Byte
 			buf.WriteByte(meta.Value.(byte))
 		case 1: // VarInt
 			Network.WriteVarint(buf, meta.Value.(int32))
 		case 2: // Float
 			binary.Write(buf, binary.BigEndian, meta.Value.(float32))
 		case 3: // String
 			Protocol.WriteString(buf, meta.Value.(string))
 		case 4: // Chat
 			Protocol.WriteString(buf, meta.Value.(string))
 		case 5: // Slot
 			binary.Write(buf, binary.BigEndian, int16(-1))
 		case 6: // Boolean
 			if meta.Value.(bool) { buf.WriteByte(1) } else { buf.WriteByte(0) }
 		case 7: // Rotation
 			val := [3]float32{0, 0, 0}
 			if v, ok := meta.Value.([3]float32); ok { val = v }
 			binary.Write(buf, binary.BigEndian, val[0])
 			binary.Write(buf, binary.BigEndian, val[1])
 			binary.Write(buf, binary.BigEndian, val[2])
 		case 8: // Position
 			binary.Write(buf, binary.BigEndian, meta.Value.(uint64))
 		case 9: // OptPosition
 			if meta.Value != nil {
 				buf.WriteByte(1)
 				binary.Write(buf, binary.BigEndian, meta.Value.(uint64))
 			} else {
 				buf.WriteByte(0)
 			}
 		case 10: // Direction
 			Network.WriteVarint(buf, meta.Value.(int32))
 		case 11: // OptUUID
 			if meta.Value != nil {
 				buf.WriteByte(1)
 				buf.Write(meta.Value.([]byte))
 			} else {
 				buf.WriteByte(0)
 			}
 		case 12: // BlockID
 			Network.WriteVarint(buf, meta.Value.(int32))
 		case 13: // NBT
 			buf.WriteByte(0)
 		case 14: // Particle
 			Network.WriteVarint(buf, 0)
 		}
 	}
 	buf.WriteByte(0xFF)
 }
 
 // BuildDestroyEntities 销毁实体包 (0x32)
 func BuildDestroyEntities(eids []int32) []byte {
 	buf := &bytes.Buffer{}
 	Network.WriteVarint(buf, 0x32)
 	Network.WriteVarint(buf, int32(len(eids)))
 	for _, eid := range eids {
 		Network.WriteVarint(buf, eid)
 	}
 	return Protocol.AddLengthPrefix(buf)
 }
 
// BuildEntityTeleport 实体传送包 (0x4C)
func BuildEntityTeleport(eid int32, x, y, z float64, yaw, pitch float32, onGround bool) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x4C)
	Network.WriteVarint(buf, eid)
	Protocol.WriteDouble(buf, x)
	Protocol.WriteDouble(buf, y)
	Protocol.WriteDouble(buf, z)
	// yaw和pitch都使用相同转换方式
	yawByte := byte(int(yaw*256.0/360.0) & 0xFF)
	pitchByte := byte(int(pitch*256.0/360.0) & 0xFF)
	buf.WriteByte(yawByte)
	buf.WriteByte(pitchByte)
	if onGround { buf.WriteByte(1) } else { buf.WriteByte(0) }
	return Protocol.AddLengthPrefix(buf)
}
 
// BuildEntityHeadLook 实体头部朝向包 (0x36)
func BuildEntityHeadLook(eid int32, yaw float32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x36)
	Network.WriteVarint(buf, eid)
	// 正确转换yaw：0-360度 -> 0-255字节值
	yawByte := byte(int(yaw*256.0/360.0) & 0xFF)
	buf.WriteByte(yawByte)
	return Protocol.AddLengthPrefix(buf)
}

// BuildEntityAnimation 实体动画包 (0x06)
// animationID: 0=摆动主臂, 1=受到伤害, 2=离开床, 3=摆动副手, 4=临界效应, 5=魔法临界效应
func BuildEntityAnimation(eid int32, animationID byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x06) // Packet ID
	Network.WriteVarint(buf, eid)  // Entity ID
	buf.WriteByte(animationID)     // Animation ID
	return Protocol.AddLengthPrefix(buf)
}

// RemoveEntity 移除实体
 func (em *EntityManager) RemoveEntity(eid int32) {
 	em.mu.Lock()
 	defer em.mu.Unlock()
 	if player, ok := em.players[eid]; ok {
 		delete(em.players, eid)
 		delete(em.entities, eid)
 		fmt.Printf("[Entity] 移除玩家实体: EID=%d, Username=%s\n", eid, player.Username)
 	} else if entity, ok := em.entities[eid]; ok {
 		delete(em.entities, eid)
 		fmt.Printf("[Entity] 移除实体: EID=%d, Type=%d\n", eid, entity.Type)
 	}
 }
 
 func (em *EntityManager) GetPlayer(eid int32) *PlayerEntity {
 	em.mu.RLock()
 	defer em.mu.RUnlock()
 	return em.players[eid]
 }
 
 func (em *EntityManager) GetEntity(eid int32) *Entity {
 	em.mu.RLock()
 	defer em.mu.RUnlock()
 	return em.entities[eid]
 }
 
func (em *EntityManager) GetAllPlayers() []*PlayerEntity {
	em.mu.RLock()
	defer em.mu.RUnlock()
	players := make([]*PlayerEntity, 0, len(em.players))
	for _, p := range em.players { players = append(players, p) }
	return players
}

// GetAllItems 获取所有掉落物实体
func (em *EntityManager) GetAllItems() []*ItemEntity {
	em.mu.RLock()
	defer em.mu.RUnlock()
	
	items := make([]*ItemEntity, 0)
	for _, ent := range em.entities {
		// 检查实体类型
		if ent.Type == EntityTypeItem {
			// 这里需要更安全的类型转换方式
			// 由于entities map存储的是*Entity，我们需要找到ItemEntity
			// 临时解决方案：在ItemEntity中存储引用
		}
	}
	return items
}
