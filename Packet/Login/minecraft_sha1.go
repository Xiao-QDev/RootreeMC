package Login

import (
	"crypto/sha1"
	"math/big"
)

// MinecraftSha1 计算 Minecraft 协议要求的 SHA-1 哈希
// 用于服务器身份验证
// 逻辑: sha1(ServerID + SharedSecret + PublicKey)
// 返回: 带负号的十六进制哈希字符串
func MinecraftSha1(serverID string, sharedSecret []byte, publicKey []byte) string {
	hasher := sha1.New()
	hasher.Write([]byte(serverID))
	hasher.Write(sharedSecret)
	hasher.Write(publicKey)
	hash := hasher.Sum(nil)

	// 将字节数组视为大整数
	num := new(big.Int).SetBytes(hash)

	// 检查是否为负数（最高位为 1）
	if hash[0]&0x80 != 0 {
		// 补码逻辑: 负数 = -(2^(8*len) - val)
		neg := new(big.Int).Lsh(big.NewInt(1), uint(len(hash)*8))
		num.Sub(num, neg)
	}

	return num.Text(16)
}
