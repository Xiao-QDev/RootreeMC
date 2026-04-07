// Package Protocol Minecraft 协议工具函数
package Protocol

import (
	"RootreeMC/Network"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// ReadString 读取 Minecraft 字符串
func ReadString(reader io.Reader) (string, error) {
	length, err := Network.ReadVarint(reader)
	if err != nil {
		return "", err
	}

	if length < 0 || length > 32767 {
		return "", fmt.Errorf("string length out of range: %d", length)
	}

	data := make([]byte, length)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// WriteString 写入 Minecraft 字符串
func WriteString(buf *bytes.Buffer, str string) {
	if len(str) > 32767 {
		str = str[:32767] // 截断过长的字符串
	}

	Network.WriteVarint(buf, int32(len(str)))
	buf.WriteString(str)
}

// ReadBoolean 读取布尔值
func ReadBoolean(reader io.Reader) (bool, error) {
	br, ok := reader.(interface{ ReadByte() (byte, error) })
	if !ok {
		return false, fmt.Errorf("reader does not support ReadByte")
	}

	b, err := br.ReadByte()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// WriteBoolean 写入布尔值
func WriteBoolean(buf *bytes.Buffer, value bool) {
	if value {
		buf.WriteByte(0x01)
	} else {
		buf.WriteByte(0x00)
	}
}

// ReadShort 读取短整数（大端）
func ReadShort(reader io.Reader) (int16, error) {
	var value int16
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

// WriteShort 写入短整数（大端）
func WriteShort(buf *bytes.Buffer, value int16) {
	err := binary.Write(buf, binary.BigEndian, value)
	if err != nil {
		return
	}
}

// WriteAngle 写入角度（字节，角度/256）
func WriteAngle(buf *bytes.Buffer, value float32) {
	angle := byte(int(value * 256.0 / 360.0) & 0xFF)
	buf.WriteByte(angle)
}

// ReadUnsignedShort 读取无符号短整数（大端）
func ReadUnsignedShort(reader io.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

// WriteUnsignedShort 写入无符号短整数（大端）
func WriteUnsignedShort(buf *bytes.Buffer, value uint16) {
	err := binary.Write(buf, binary.BigEndian, value)
	if err != nil {
		return
	}
}

// ReadInt 读取整数（大端）
func ReadInt(reader io.Reader) (int32, error) {
	var value int32
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

// WriteInt 写入整数（大端）
func WriteInt(buf *bytes.Buffer, value int32) {
	err := binary.Write(buf, binary.BigEndian, value)
	if err != nil {
		return
	}
}

// ReadLong 读取长整数（大端）
func ReadLong(reader io.Reader) (int64, error) {
	var value int64
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

// WriteLong 写入长整数（大端）
func WriteLong(buf *bytes.Buffer, value int64) {
	err := binary.Write(buf, binary.BigEndian, value)
	if err != nil {
		return
	}
}

// ReadFloat 读取浮点数（大端）
func ReadFloat(reader io.Reader) (float32, error) {
	var value float32
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

// WriteFloat 写入浮点数（大端）
func WriteFloat(buf *bytes.Buffer, value float32) {
	err := binary.Write(buf, binary.BigEndian, value)
	if err != nil {
		return
	}
}

// ReadDouble 读取双精度浮点数（大端）
func ReadDouble(reader io.Reader) (float64, error) {
	var value float64
	err := binary.Read(reader, binary.BigEndian, &value)
	return value, err
}

// WriteDouble 写入双精度浮点数（大端）
func WriteDouble(buf *bytes.Buffer, value float64) {
	err := binary.Write(buf, binary.BigEndian, value)
	if err != nil {
		return
	}
}

// ReadVarlong 读取 VarLong 数据
func ReadVarlong(reader io.Reader) (int64, error) {
	var result int64
	var shift uint

	br, ok := reader.(interface{ ReadByte() (byte, error) })
	if !ok {
		return 0, fmt.Errorf("reader does not support ReadByte")
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 70 {
			return 0, io.ErrUnexpectedEOF
		}
	}
	return result, nil
}

// WriteVarlong 写入 VarLong 数据
func WriteVarlong(buf *bytes.Buffer, value int64) {
	for value >= 0x80 || value < 0 {
		buf.WriteByte(byte(value&0x7F) | 0x80)
		value >>= 7
	}
	buf.WriteByte(byte(value))
}

// ReadByteArray 读取字节数组
func ReadByteArray(reader io.Reader, length int32) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("negative byte array length: %d", length)
	}

	data := make([]byte, length)
	_, err := io.ReadFull(reader, data)
	return data, err
}

// WriteByteArray 写入字节数组
func WriteByteArray(buf *bytes.Buffer, data []byte) {
	buf.Write(data)
}

// EncodePosition 编码 Position (x, y, z) 为 8 字节 - Minecraft 1.12.2 格式
func EncodePosition(x, y, z int32) [8]byte {
	var position int64

	// 1.12.2 格式: x: 26 bits, y: 12 bits, z: 26 bits
	// ((x & 0x3FFFFFF) << 38) | ((y & 0xFFF) << 26) | (z & 0x3FFFFFF)
	position = (int64(x&0x3FFFFFF) << 38) | (int64(y&0xFFF) << 26) | int64(z&0x3FFFFFF)

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(position))
	return buf
}

// DecodePosition 解码 8 字节为 Position (x, y, z) - Minecraft 1.12.2 格式
func DecodePosition(data [8]byte) (int32, int32, int32) {
	val := int64(binary.BigEndian.Uint64(data[:]))

	x := val >> 38
	y := (val >> 26) & 0xFFF
	z := val << 38 >> 38 // 提取低 26 位并处理符号位

	// 处理符号位
	if x >= 1<<25 {
		x -= 1 << 26
	}
	if y >= 1<<11 {
		y -= 1 << 12
	}
	if z >= 1<<25 {
		z -= 1 << 26
	}

	return int32(x), int32(y), int32(z)
}

// WriteUUID 写入 UUID (16 bytes)
func WriteUUID(buf *bytes.Buffer, uuid []byte) {
	if len(uuid) == 16 {
		buf.Write(uuid)
	} else {
		buf.Write(make([]byte, 16))
	}
}

// WriteByte 写入单个字节
func WriteByte(buf *bytes.Buffer, value int8) {
	buf.WriteByte(byte(value))
}


// AddLengthPrefix 为数据包添加 VarInt 长度前缀
func AddLengthPrefix(buf *bytes.Buffer) []byte {
	fullPacketBytes := buf.Bytes()
	if len(fullPacketBytes) == 0 {
		return []byte{}
	}
	
	lengthBuf := &bytes.Buffer{}
	Network.WriteVarint(lengthBuf, int32(len(fullPacketBytes)))
	lengthBuf.Write(fullPacketBytes)
	return lengthBuf.Bytes()
}

// BuildAbsoluteTeleport 构建玩家传送包 (0x2F) - Minecraft 1.12.2
func BuildAbsoluteTeleport(x, y, z float64, yaw, pitch float32, teleportID int32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x2F)
	WriteDouble(buf, x)
	WriteDouble(buf, y)
	WriteDouble(buf, z)
	WriteFloat(buf, yaw)
	WriteFloat(buf, pitch)
	buf.WriteByte(0)
	Network.WriteVarint(buf, teleportID)
	return AddLengthPrefix(buf)
}

// BuildChangeGameState 构建改变游戏状态包 (0x1E)
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

// BuildSystemMessage 构建系统消息包 (0x0F)
// 参数 jsonMsg 必须是符合 Minecraft 规范的 JSON 聊天组件字符串
// 注意：该函数直接接收 JSON 字符串。若需自动包装纯文本，请优先使用 Play.BuildSystemMessage
func BuildSystemMessage(jsonMsg string) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x0F) // Chat Message Packet ID
	
	WriteString(buf, jsonMsg)
	buf.WriteByte(1) // Position: 1 = 系统消息
	
	return AddLengthPrefix(buf)
}

