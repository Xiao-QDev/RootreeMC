// Package Uuid 处理玩家的UUID
package Uuid

import (
	"crypto/md5"      //MD5哈希算法
	"encoding/binary" //二进制编码/解码（大端/小端）
	"encoding/hex"    //十六进制编码/解码
	"fmt"             //格式化输入输出（打印、错误信息等）
	"strings"         //字符串操作（替换、修剪空格等）

	"github.com/google/uuid" //UUID 生成和处理库
)

// PlayerUUID 玩家 UUID 结构体
type PlayerUUID struct {
	uuid uuid.UUID
}

// NewPlayerUUID 从字符串创建玩家 UUID
func NewPlayerUUID(uuidStr string) (*PlayerUUID, error) {
	//去除连字符和空格
	cleaned := strings.ReplaceAll(uuidStr, "-", "")
	cleaned = strings.TrimSpace(cleaned)

	//解析 UUID
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		//如果不是标准格式，尝试从用户名生成离线模式 UUID
		return NewOfflineUUID(uuidStr), nil
	}

	return &PlayerUUID{uuid: id}, nil
}

// NewOnlineUUID 从用户名生成在线模式 UUID（通过 Mojang 服务器验证的）
func NewOnlineUUID(username string) (*PlayerUUID, error) {
	// Minecraft 在线模式使用 MD5 哈希生成 UUID
	hash := md5.Sum([]byte(username))

	//设置 UUID 版本为 3（基于 MD5 的命名 UUID）
	hash[6] = (hash[6] & 0x0f) | 0x30
	//设置 UUID 变体为 RFC 4122
	hash[8] = (hash[8] & 0x3f) | 0x80

	id, err := uuid.FromBytes(hash[:])
	if err != nil {
		return nil, err
	}

	return &PlayerUUID{uuid: id}, nil
}

// NewOfflineUUID 生成离线模式 UUID（用于盗版服务器）
func NewOfflineUUID(username string) *PlayerUUID {
	//离线模式使用 "OfflinePlayer:" + 用户名的 MD5
	offlineName := "OfflinePlayer:" + username
	hash := md5.Sum([]byte(offlineName))

	//设置 UUID 版本为 3
	hash[6] = (hash[6] & 0x0f) | 0x30
	//设置 UUID 变体为 RFC 4122
	hash[8] = (hash[8] & 0x3f) | 0x80

	id, _ := uuid.FromBytes(hash[:])
	return &PlayerUUID{uuid: id}
}

// NewRandomUUID 生成随机 UUID
func NewRandomUUID() *PlayerUUID {
	return &PlayerUUID{uuid: uuid.New()}
}

// String 返回带连字符的 UUID 字符串
func (p *PlayerUUID) String() string {
	return p.uuid.String()
}

// StringNoDash 返回不带连字符的 UUID 字符串
func (p *PlayerUUID) StringNoDash() string {
	return strings.ReplaceAll(p.uuid.String(), "-", "")
}

// Bytes 返回 UUID 的字节数组
func (p *PlayerUUID) Bytes() []byte {
	bytes := make([]byte, 16)
	copy(bytes, p.uuid[:])
	return bytes
}

// HighBits 获取 UUID 的高 64 位
func (p *PlayerUUID) HighBits() int64 {
	return int64(binary.BigEndian.Uint64(p.uuid[:8]))
}

// LowBits 获取 UUID 的低 64 位
func (p *PlayerUUID) LowBits() int64 {
	return int64(binary.BigEndian.Uint64(p.uuid[8:]))
}

// Equal 比较两个 UUID 是否相等
func (p *PlayerUUID) Equal(other *PlayerUUID) bool {
	if other == nil {
		return false
	}
	return p.uuid == other.uuid
}

// IsZero 检查是否是零值 UUID
func (p *PlayerUUID) IsZero() bool {
	return p.uuid == uuid.Nil
}

// Version 返回 UUID 版本
func (p *PlayerUUID) Version() byte {
	return byte(p.uuid.Version())
}

// Variant 返回 UUID 变体
func (p *PlayerUUID) Variant() byte {
	return byte(p.uuid.Variant())
}

// MarshalBinary 二进制序列化
func (p *PlayerUUID) MarshalBinary() ([]byte, error) {
	return p.uuid.MarshalBinary()
}

// UnmarshalBinary 二进制反序列化
func (p *PlayerUUID) UnmarshalBinary(data []byte) error {
	return p.uuid.UnmarshalBinary(data)
}

// MarshalText 文本序列化
func (p *PlayerUUID) MarshalText() ([]byte, error) {
	return p.uuid.MarshalText()
}

// UnmarshalText 文本反序列化
func (p *PlayerUUID) UnmarshalText(text []byte) error {
	return p.uuid.UnmarshalText(text)
}

// ToNetworkID 转换为 Minecraft 网络包中的 UUID 格式（16 字节）
func (p *PlayerUUID) ToNetworkID() []byte {
	return p.Bytes()
}

// FromNetworkID 从 Minecraft 网络包中解析 UUID
func FromNetworkID(data []byte) (*PlayerUUID, error) {
	if len(data) != 16 {
		return nil, fmt.Errorf("invalid UUID length: expected 16 bytes, got %d", len(data))
	}

	id, err := uuid.FromBytes(data)
	if err != nil {
		return nil, err
	}

	return &PlayerUUID{uuid: id}, nil
}

// ParseHex 从十六进制字符串解析 UUID
func ParseHex(hexStr string) (*PlayerUUID, error) {
	//清理字符串
	hexStr = strings.ReplaceAll(hexStr, "-", "")
	hexStr = strings.TrimSpace(hexStr)

	//解码十六进制
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}

	return FromNetworkID(data)
}

// Must 如果错误则 panic，否则返回 UUID
func Must(p *PlayerUUID, err error) *PlayerUUID {
	if err != nil {
		panic(err)
	}
	return p
}
