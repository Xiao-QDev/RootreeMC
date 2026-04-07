package Login

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"sync"
)

var (
	serverKey    *rsa.PrivateKey
	serverPubKey []byte
	keyOnce      sync.Once
	keyErr       error
)

// GetServerKeyPair 获取服务器 RSA 密钥对
func GetServerKeyPair() (*rsa.PrivateKey, []byte, error) {
	keyOnce.Do(func() {
		// 生成 1024 位 RSA 密钥（Minecraft 1.12.2 标准）
		serverKey, keyErr = rsa.GenerateKey(rand.Reader, 1024)
		if keyErr != nil {
			return
		}

		// 将公钥转换为 DER 格式（Minecraft 协议要求）
		serverPubKey, keyErr = x509.MarshalPKIXPublicKey(&serverKey.PublicKey)
		if keyErr != nil {
			return
		}
	})

	return serverKey, serverPubKey, keyErr
}

// GenerateVerifyToken 生成随机验证令牌
func GenerateVerifyToken() []byte {
	token := make([]byte, 4)
	_, _ = rand.Read(token)
	return token
}

// DecryptSharedSecret 使用私钥解密共享密钥
func DecryptSharedSecret(encryptedSecret []byte) ([]byte, error) {
	key, _, err := GetServerKeyPair()
	if err != nil {
		return nil, err
	}

	return rsa.DecryptPKCS1v15(rand.Reader, key, encryptedSecret)
}

// DecryptVerifyToken 使用私钥解密验证令牌
func DecryptVerifyToken(encryptedToken []byte) ([]byte, error) {
	key, _, err := GetServerKeyPair()
	if err != nil {
		return nil, err
	}

	return rsa.DecryptPKCS1v15(rand.Reader, key, encryptedToken)
}

// GenerateServerIDHash 生成用于 Mojang 验证的 Server ID 哈希
// Minecraft 1.12.2 使用 sha1(ServerID + SharedSecret + PublicKey)
// 注意：ServerID 传入通常为空字符串
func GenerateServerIDHash(serverID string, sharedSecret []byte, publicKey []byte) string {
	// 这是一个特殊的 Minecraft sha1 变体（支持负值十六进制）
	// 这里我们需要手动实现该逻辑或调用已有的实现
	// 由于 Go 标准库没有直接支持，我们暂时先定义接口
	return MinecraftSha1(serverID, sharedSecret, publicKey)
}
