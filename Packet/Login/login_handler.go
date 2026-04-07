// Package Login 登录阶段协议处理
package Login

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/Uuid"
	"RootreeMC/serverconfig"
	"bytes"
	"crypto/rand"
	//"crypto/rsa"
	//"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Handler 登录处理器 - 管理客户端的整个登录流程
// 负责处理握手、身份验证、压缩设置等，直到玩家进入游戏状态
type Handler struct {
	conn          *Network.Network           // 网络连接对象
	config        *serverconfig.ServerConfig // 服务器配置
	state         Protocol.State             // 当前连接状态 (Handshaking/Login/Play)
	protocolVer   int32                      // 客户端协议版本
	username      string                     // 玩家用户名
	playerUUID    *Uuid.PlayerUUID           // 玩家 UUID
	properties    []Property                 // 玩家属性（包含皮肤）
	compression   bool                       // 是否启用压缩
	compressionTh int32                      // 压缩阈值
	verifyToken   []byte                     // 验证令牌 (在线模式使用)
	authenticated bool                       // 是否已通过身份验证
}

// Property 玩家属性
type Property struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

// NewLoginHandler 创建新的登录处理器
// 参数: conn - 客户端的网络连接对象
// 参数: config - 服务器配置
// 返回: 初始化后的登录处理器实例
func NewLoginHandler(conn *Network.Network, config *serverconfig.ServerConfig) *Handler {
	return &Handler{
		conn:          conn,
		config:        config,
		state:         Protocol.StateHandshaking,
		compression:   false,
		compressionTh: 256,
	}
}

// HandleHandshake 处理握手包
// 解析客户端发送的握手信息，验证协议版本，并切换到对应状态
// 参数: packetData - 握手包的原始数据（不包含包长度和包 ID）
// 返回: 错误信息（如果协议版本不支持或解析失败）
func (h *Handler) HandleHandshake(packetData []byte) error {
	handshake, err := ParseHandshake(packetData)
	if err != nil {
		return fmt.Errorf("failed to parse handshake: %v", err)
	}

	h.protocolVer = handshake.ProtocolVersion

	// 添加详细调试输出
	fmt.Printf("[Handshake] 协议版本: %d\n", handshake.ProtocolVersion)
	fmt.Printf("[Handshake] 服务器地址: %s\n", handshake.ServerAddress)
	fmt.Printf("[Handshake] 服务器端口: %d\n", handshake.ServerPort)
	fmt.Printf("[Handshake] NextState: %d\n", handshake.NextState)

	// 检查协议版本是否支持
	if !Protocol.Version(h.protocolVer).IsSupported() {
		disconnectPkt := BuildLoginDisconnect(fmt.Sprintf("不支持的协议版本: %d", h.protocolVer))
		h.conn.Send(disconnectPkt)
		h.conn.Close()
		return fmt.Errorf("unsupported protocol version: %d", h.protocolVer)
	}

	// 根据 NextState 切换状态
	// NextState=1 表示状态查询（Ping/MOTD）
	// NextState=2 表示登录流程
	switch handshake.NextState {
	case 1:
		h.state = Protocol.StateStatus
		fmt.Printf("[Handshake] 切换到 Status 状态，等待客户端发送 Status Request...\n")
	case 2:
		h.state = Protocol.StateLogin
		fmt.Printf("[Handshake] 切换到 Login 状态，等待 Login Start...\n")
	default:
		return fmt.Errorf("invalid next state: %d", handshake.NextState)
	}

	return nil
}

// HandleLoginStart 处理登录开始包
func (h *Handler) HandleLoginStart(packetData []byte) error {
	loginStart, err := ParseLoginStart(packetData)
	if err != nil {
		return fmt.Errorf("failed to parse login start: %v", err)
	}

	h.username = loginStart.Name
	fmt.Printf("[Login] 玩家尝试登录: %s\n", h.username)

	// 判断是在线模式还是离线模式
	if h.config.OnlineMode {
		// 在线模式：发送加密请求
		fmt.Printf("[Login] 启用在线模式，向 %s 发送加密请求...\n", h.username)

		// 获取服务器公钥
		_, pubKey, err := GetServerKeyPair()
		if err != nil {
			return fmt.Errorf("failed to get server key pair: %v", err)
		}

		// 生成验证令牌 (4 字节)
		h.verifyToken = make([]byte, 4)
		_, _ = rand.Read(h.verifyToken)

		// 发送 Encryption Request
		// ServerID 通常为空
		encryptReqPkt := BuildEncryptionRequest("", pubKey, h.verifyToken)
		if err := h.conn.Send(encryptReqPkt); err != nil {
			return fmt.Errorf("failed to send encryption request: %v", err)
		}

		return nil // 等待 Encryption Response
	}

	// 离线模式：直接生成 UUID
	h.playerUUID = Uuid.NewOfflineUUID(h.username)
	return h.finishLogin()
}

// HandleEncryptionResponse 处理加密响应包 (在线模式)
func (h *Handler) HandleEncryptionResponse(packetData []byte) error {
	resp, err := ParseEncryptionResponse(packetData)
	if err != nil {
		return fmt.Errorf("failed to parse encryption response: %v", err)
	}

	// 1. 解密共享密钥和验证令牌
	sharedSecret, err := DecryptSharedSecret(resp.SharedSecret)
	if err != nil {
		return fmt.Errorf("failed to decrypt shared secret: %v", err)
	}

	verifyToken, err := DecryptVerifyToken(resp.VerifyToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt verify token: %v", err)
	}

	// 2. 验证令牌
	if !bytes.Equal(h.verifyToken, verifyToken) {
		disconnectPkt := BuildLoginDisconnect("验证令牌不匹配，身份验证失败")
		_ = h.conn.Send(disconnectPkt)
		return fmt.Errorf("verify token mismatch")
	}

	// 3. 启用连接加密
	fmt.Printf("[Login] 为玩家 %s 启用连接加密...\n", h.username)
	if err := h.conn.EnableEncryption(sharedSecret); err != nil {
		return fmt.Errorf("failed to enable encryption: %v", err)
	}

	// 4. 向 Mojang 验证身份
	fmt.Printf("[Login] 正在通过 Mojang 验证玩家 %s...\n", h.username)
	_, pubKey, _ := GetServerKeyPair()
	serverHash := MinecraftSha1("", sharedSecret, pubKey)

	verified, err := h.verifyWithMojang(serverHash)
	if err != nil {
		return fmt.Errorf("mojang authentication error: %v", err)
	}

	if !verified {
		disconnectPkt := BuildLoginDisconnect("身份验证失败，请确保您已登录 Mojang 账户")
		_ = h.conn.Send(disconnectPkt)
		return fmt.Errorf("mojang authentication failed")
	}

	fmt.Printf("[Login] 玩家 %s 身份验证成功\n", h.username)
	return h.finishLogin()
}

// verifyWithMojang 向 Mojang 验证玩家身份
func (h *Handler) verifyWithMojang(serverHash string) (bool, error) {
	url := fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/hasJoined?username=%s&serverId=%s", h.username, serverHash)

	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return false, nil // 身份验证失败
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code from Mojang: %d", resp.StatusCode)
	}

	// 解析响应以获取 UUID 和属性
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		Properties []Property `json:"properties"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	h.properties = result.Properties

	// 在线模式 UUID 是 16 进制字符串（无连字符），需要转换为 PlayerUUID
	h.playerUUID, err = Uuid.ParseHex(result.ID)
	if err != nil {
		return false, err
	}

	return true, nil
}

// finishLogin 完成登录流程
func (h *Handler) finishLogin() error {
	// 如果设置了压缩，在 Login Success 之前发送 Set Compression
	if h.config.NetworkCompressionThreshold >= 0 {
		threshold := int32(h.config.NetworkCompressionThreshold)
		setCompressionPkt := BuildSetCompression(threshold)
		if err := h.conn.Send(setCompressionPkt); err != nil {
			return fmt.Errorf("failed to send set compression: %v", err)
		}
		// 在网络层启用压缩
		h.conn.EnableCompression(threshold)
	}

	// 发送登录成功包
	loginSuccessPkt := BuildLoginSuccess(h.playerUUID, h.username)
	if err := h.conn.Send(loginSuccessPkt); err != nil {
		return fmt.Errorf("failed to send login success: %v", err)
	}

	fmt.Printf("[Login] %s join the game(UUID: %s)\n", h.username, h.playerUUID.String())

	// 登录完成，切换到游戏状态
	h.state = Protocol.StatePlay
	return nil
}

// GetPlayerInfo 获取玩家信息
// 返回登录成功后存储的玩家用户名、UUID 和属性（皮肤等）
// 返回: username - 玩家用户名, uuid - 玩家 UUID 对象, props - 玩家属性
func (h *Handler) GetPlayerInfo() (string, *Uuid.PlayerUUID, []Property) {
	return h.username, h.playerUUID, h.properties
}

// IsLoginFinished 返回登录流程是否已完全结束
func (h *Handler) IsLoginFinished() bool {
	return h.state == Protocol.StatePlay
}

// GetState 获取当前连接状态
// 用于判断登录流程是否完成，是否可以切换到 Play 阶段
// 返回: 当前的协议状态 (StateHandshaking/StateLogin/StatePlay)
func (h *Handler) GetState() Protocol.State {
	return h.state
}

// sendPacketWithLength 发送带有长度前缀的数据包
// packetData 格式: [PacketID:VarInt] [Data]
func sendPacketWithLength(conn *Network.Network, packetData []byte) error {
	buf := &bytes.Buffer{}
	// 写入长度 (PacketID + Data 的总长度)
	Network.WriteVarint(buf, int32(len(packetData)))
	// 写入 PacketID + Data
	buf.Write(packetData)
	return conn.Send(buf.Bytes())
}
