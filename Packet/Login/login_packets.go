// Package Login Minecraft 登录阶段协议包
//
// 登录流程:
// 1. 客户端发送 Handshake (NextState=2) 切换到登录状态
// 2. 客户端发送 Login Start (包含用户名)
// 3. [可选] 服务器发送 Encryption Request (在线模式)
// 4. [可选] 客户端发送 Encryption Response
// 5. [可选] 服务器发送 Set Compression (启用压缩)
// 6. 服务器发送 Login Success (登录成功，切换到 Play 状态)
//
// 离线模式下，步骤 3-4 被跳过，直接发送 Login Success
package Login

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/Uuid"
	"bytes"
	"encoding/json"
	"fmt"
)

// === Serverbound Packets (客户端 -> 服务器) ===
// 这些包由客户端发送，服务器接收并处理

// HandshakePacket 握手包 - 客户端连接时发送的第一个包
// 用于告知服务器客户端的协议版本和目标状态（Status 或 Login）
type HandshakePacket struct {
	ProtocolVersion int32  // 客户端协议版本 (1.12.2 = 340)
	ServerAddress   string // 服务器地址 (主机名或 IP)
	ServerPort      uint16 // 服务器端口
	NextState       int32  // 下一个状态: 1=Status(服务器列表Ping), 2=Login(登录)
}

// ParseHandshake 解析握手包
// 从原始字节数据中解析出握手信息
// 参数: data - 握手包的原始字节数据（不包含包长度和包 ID）
// 返回: 解析后的握手包结构体，或错误信息
func ParseHandshake(data []byte) (*HandshakePacket, error) {
	fmt.Printf("[DEBUG] 握手包原始数据长度: %d 字节\n", len(data))
	fmt.Printf("[DEBUG] 握手包十六进制: %x\n", data)

	buf := bytes.NewReader(data)

	protocolVersion, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, fmt.Errorf("读取协议版本失败: %v", err)
	}
	fmt.Printf("[DEBUG] 协议版本: %d\n", protocolVersion)

	// 读取服务器地址 (String 格式: [Length:VarInt][Data])
	addrLen, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, fmt.Errorf("读取地址长度失败: %v", err)
	}
	fmt.Printf("[DEBUG] 地址长度: %d\n", addrLen)

	serverAddr := make([]byte, addrLen)
	_, err = buf.Read(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("读取地址失败: %v", err)
	}
	fmt.Printf("[DEBUG] 地址: %s\n", string(serverAddr))

	// 读取端口 (Unsigned Short, 2 bytes big-endian)
	portBytes := make([]byte, 2)
	_, err = buf.Read(portBytes)
	if err != nil {
		return nil, fmt.Errorf("读取端口失败: %v", err)
	}
	serverPort := uint16(portBytes[0])<<8 | uint16(portBytes[1])
	fmt.Printf("[DEBUG] 端口: %d\n", serverPort)

	// 读取下一个状态
	nextState, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, fmt.Errorf("读取 NextState 失败: %v", err)
	}
	fmt.Printf("[DEBUG] NextState: %d\n", nextState)

	return &HandshakePacket{
		ProtocolVersion: protocolVersion,
		ServerAddress:   string(serverAddr),
		ServerPort:      serverPort,
		NextState:       nextState,
	}, nil
}

// StartPacket 登录开始包 - 客户端请求登录时发送
// 包含玩家的用户名，服务器根据此用户名进行身份验证
type StartPacket struct {
	Name string // 玩家用户名 (最大 16 字符)
}

// ParseLoginStart 解析登录开始包
// 从原始字节数据中解析出玩家用户名
// 参数: data - 登录开始包的原始字节数据
// 返回: 解析后的登录开始包结构体，或错误信息
func ParseLoginStart(data []byte) (*StartPacket, error) {
	buf := bytes.NewReader(data)

	// 读取用户名长度
	nameLen, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	nameBytes := make([]byte, nameLen)
	_, err = buf.Read(nameBytes)
	if err != nil {
		return nil, err
	}

	return &StartPacket{
		Name: string(nameBytes),
	}, nil
}

// EncryptionResponsePacket 加密响应包 - 客户端响应服务器的加密请求
// 包含用服务器公钥加密的共享密钥和验证令牌
// 仅在在线模式下使用
type EncryptionResponsePacket struct {
	SharedSecret []byte // 用服务器公钥加密的共享密钥 (用于后续通信加密)
	VerifyToken  []byte // 用服务器公钥加密的验证令牌 (用于身份验证)
}

// ParseEncryptionResponse 解析加密响应包
// 从原始字节数据中解析出加密响应信息
// 参数: data - 加密响应包的原始字节数据
// 返回: 解析后的加密响应包结构体，或错误信息
func ParseEncryptionResponse(data []byte) (*EncryptionResponsePacket, error) {
	buf := bytes.NewReader(data)

	// 读取共享密钥长度
	ssLen, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	sharedSecret := make([]byte, ssLen)
	_, err = buf.Read(sharedSecret)
	if err != nil {
		return nil, err
	}

	// 读取验证令牌长度
	vtLen, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	verifyToken := make([]byte, vtLen)
	_, err = buf.Read(verifyToken)
	if err != nil {
		return nil, err
	}

	return &EncryptionResponsePacket{
		SharedSecret: sharedSecret,
		VerifyToken:  verifyToken,
	}, nil
}

// === Clientbound Packets (服务器 -> 客户端) ===
// 这些包由服务器发送，客户端接收并处理

// DisconnectPacket 登录断开包 - 服务器拒绝客户端登录时发送
// 包含断开连接的原因，客户端会显示此原因并断开连接
type DisconnectPacket struct {
	Reason string // 断开原因 (JSON 格式的聊天组件)
}

// BuildLoginDisconnect 构建登录断开包
// 创建一个断开连接的响应包，发送给客户端
// 参数: reason - 断开原因文本 (会自动转换为 JSON 格式)
// 返回: 完整的断开连接包字节数据
func BuildLoginDisconnect(reason string) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x00) // Packet ID

	// 写入 JSON 格式的断开原因
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: reason,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	reasonJSON := string(jsonBytes)
	reasonLen := int32(len(reasonJSON))
	Network.WriteVarint(buf, reasonLen)
	buf.WriteString(reasonJSON)

	return Protocol.AddLengthPrefix(buf)
}

// EncryptionRequestPacket 加密请求包 - 服务器要求客户端进行加密认证
// 仅在在线模式下使用，用于启动加密握手流程
type EncryptionRequestPacket struct {
	ServerID    string // 服务器 ID (通常为空字符串)
	PublicKey   []byte // 服务器的 RSA 公钥 (用于加密共享密钥和验证令牌)
	VerifyToken []byte // 随机生成的验证令牌 (4 字节，用于防止中间人攻击)
	ShouldAuth  bool   // 是否需要认证 (true=在线模式, false=离线模式)
}

// BuildEncryptionRequest 构建加密请求包
// 创建加密请求包，发送给客户端以启动加密握手
// 参数:
//   - serverID: 服务器 ID (通常为空)
//   - publicKey: 服务器 RSA 公钥
//   - verifyToken: 随机验证令牌 (4 字节)
//
// 返回: 完整的加密请求包字节数据
func BuildEncryptionRequest(serverID string, publicKey []byte, verifyToken []byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x01) // Packet ID

	// 服务器 ID
	serverIDLen := int32(len(serverID))
	Network.WriteVarint(buf, serverIDLen)
	buf.WriteString(serverID)

	// 公钥
	pubKeyLen := int32(len(publicKey))
	Network.WriteVarint(buf, pubKeyLen)
	buf.Write(publicKey)

	// 验证令牌
	verifyTokenLen := int32(len(verifyToken))
	Network.WriteVarint(buf, verifyTokenLen)
	buf.Write(verifyToken)

	return Protocol.AddLengthPrefix(buf)
}

// SuccessPacket 登录成功包 - 服务器确认客户端登录成功
// 发送此包后，连接状态从 Login 切换到 Play
type SuccessPacket struct {
	UUID     *Uuid.PlayerUUID // 玩家的 UUID (在线模式从 Mojang 获取，离线模式本地生成)
	Username string           // 玩家用户名
}

// BuildLoginSuccess 构建登录成功包
// 注意: Minecraft 1.12.2 协议只包含 UUID 和 Username 两个字段
func BuildLoginSuccess(uuid *Uuid.PlayerUUID, username string) []byte {
	return Protocol.AddLengthPrefix(BuildLoginSuccessPayload(uuid, username))
}

// BuildLoginSuccessPayload 构建登录成功包负载 (不包含外层长度)
func BuildLoginSuccessPayload(uuid *Uuid.PlayerUUID, username string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x02) // Packet ID

	// UUID (带连字符的字符串，36字符)
	uuidStr := uuid.String()
	uuidLen := int32(len(uuidStr))
	Network.WriteVarint(buf, uuidLen)
	buf.WriteString(uuidStr)

	// 用户名
	usernameLen := int32(len(username))
	Network.WriteVarint(buf, usernameLen)
	buf.WriteString(username)

	return buf
}

// SetCompressionPacket 设置压缩包 - 服务器启用数据包压缩
// 发送此包后，所有后续数据包都将使用 zlib 压缩
// 必须在 Login Success 之前发送（如果启用压缩）
type SetCompressionPacket struct {
	Threshold int32 // 压缩阈值: 未压缩数据包大小 >= 此值时才压缩，0 表示总是压缩，负数表示禁用压缩
}

// BuildSetCompression 构建设置压缩包
// 创建压缩设置包，通知客户端启用数据包压缩
// 参数: threshold - 压缩阈值 (推荐值: 256)
// 返回: 完整的设置压缩包字节数据
func BuildSetCompression(threshold int32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x03) // Packet ID
	Network.WriteVarint(buf, threshold)

	return Protocol.AddLengthPrefix(buf)
}

// PluginRequestPacket 登录插件请求包 - 服务器发送自定义插件握手请求
// 用于实现自定义的登录验证流程（如 BungeeCord、Velocity 等代理）
type PluginRequestPacket struct {
	MessageID int32  // 消息 ID (服务器生成，客户端应在响应中返回相同的 ID)
	Channel   string // 插件通道名称 (例如: "bungeecord:main")
	Data      []byte // 自定义数据 (由插件定义格式)
}

// BuildLoginPluginRequest 构建登录插件请求包
// 创建插件请求包，发送给客户端以启动自定义握手流程
// 参数:
//   - messageID: 唯一的消息 ID
//   - channel: 插件通道名称
//   - data: 自定义数据
//
// 返回: 完整的插件请求包字节数据
func BuildLoginPluginRequest(messageID int32, channel string, data []byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x04) // Packet ID
	Network.WriteVarint(buf, messageID)

	// Channel (Identifier)
	channelLen := int32(len(channel))
	Network.WriteVarint(buf, channelLen)
	buf.WriteString(channel)

	// Data
	buf.Write(data)

	return Protocol.AddLengthPrefix(buf)
}

// === Serverbound Packets (继续) ===

// PluginResponsePacket 登录插件响应包 - 客户端响应服务器的插件请求
// 客户端必须响应每个收到的 Login Plugin Request
type PluginResponsePacket struct {
	MessageID  int32  // 消息 ID (必须与请求中的 ID 匹配)
	Successful bool   // 是否成功理解请求 (false 表示不支持该通道)
	Data       []byte // 响应数据 (仅在 Successful=true 时存在)
}

// ParseLoginPluginResponse 解析登录插件响应包
// 从原始字节数据中解析出插件响应信息
// 注意: Data 字段仅在 Successful=true 时存在，长度从包总长度推断
// 参数: data - 插件响应包的原始字节数据
// 返回: 解析后的插件响应包结构体，或错误信息
func ParseLoginPluginResponse(data []byte) (*PluginResponsePacket, error) {
	buf := bytes.NewReader(data)

	// 读取 Message ID
	messageID, err := Network.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	// 读取 Successful
	successfulByte, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	successful := successfulByte != 0

	// 读取 Data (可选，仅在 successful 为 true 时存在)
	var responseData []byte
	if successful {
		// 从剩余数据长度推断
		remaining := buf.Len()
		if remaining > 0 {
			responseData = make([]byte, remaining)
			_, err = buf.Read(responseData)
			if err != nil {
				return nil, err
			}
		}
	}

	return &PluginResponsePacket{
		MessageID:  messageID,
		Successful: successful,
		Data:       responseData,
	}, nil
}
