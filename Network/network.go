/*Package Network 基础网络封装
 *处理 TCP 连接
 *发送/接收原始数据包
 *Varint 编解码
 */
package Network

import (
	"fmt"  //写入处理
	"io"   //io.Writer/错误处理
	"net"  //TCP连接
	"sync" //并发发包 锁 队列

	"bufio" //缓冲写入（防止小型包频繁发送）
	"bytes" //字节缓冲/拼接数据包
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
)

// Network 网络的结构体
type Network struct {
	conn          net.Conn
	reader        *bufio.Reader // 持久化的 bufio.Reader
	writer        *bufio.Writer // 持久化的 bufio.Writer
	decryptReader *XORReader    // 底层可切换的解密读取器
	encryptWriter *XORWriter    // 底层可切换的加密写入器
	mu            sync.Mutex
	closed        bool
	encryptor     cipher.Stream
	decryptor     cipher.Stream
	compressionTh int32 // 压缩阈值，-1 表示禁用
}

// EnableCompression 启用 zlib 压缩
func (n *Network) EnableCompression(threshold int32) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.compressionTh = threshold
}

// XORReader 包装 io.Reader 以进行解密
type XORReader struct {
	Reader io.Reader
	Cipher cipher.Stream
}

func (r *XORReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if n > 0 && r.Cipher != nil {
		r.Cipher.XORKeyStream(p[:n], p[:n])
	}
	return
}

// XORWriter 包装 io.Writer 以进行加密
type XORWriter struct {
	Writer io.Writer
	Cipher cipher.Stream
}

func (w *XORWriter) Write(p []byte) (n int, err error) {
	if w.Cipher != nil {
		encrypted := make([]byte, len(p))
		w.Cipher.XORKeyStream(encrypted, p)
		return w.Writer.Write(encrypted)
	}
	return w.Writer.Write(p)
}

// EnableEncryption 启用 AES/CFB8 加密
func (n *Network) EnableEncryption(sharedSecret []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return err
	}

	n.encryptor = &cfb8{
		block: block,
		iv:    append([]byte(nil), sharedSecret...),
		tmp:   make([]byte, block.BlockSize()),
	}
	n.decryptor = &cfb8Decrypt{
		block: block,
		iv:    append([]byte(nil), sharedSecret...),
		tmp:   make([]byte, block.BlockSize()),
	}

	// 切换底层加密/解密器，不更换 bufio 对象以保留缓冲区
	n.decryptReader.Cipher = n.decryptor
	n.encryptWriter.Cipher = n.encryptor

	return nil
}

type cfb8 struct {
	block cipher.Block
	iv    []byte
	tmp   []byte
}

func (x *cfb8) XORKeyStream(dst, src []byte) {
	for i := range src {
		x.block.Encrypt(x.tmp, x.iv)
		dst[i] = src[i] ^ x.tmp[0]
		copy(x.iv, x.iv[1:])
		x.iv[len(x.iv)-1] = dst[i]
	}
}

type cfb8Decrypt struct {
	block cipher.Block
	iv    []byte
	tmp   []byte
}

func (x *cfb8Decrypt) XORKeyStream(dst, src []byte) {
	for i := range src {
		x.block.Encrypt(x.tmp, x.iv)
		c := src[i] // 必须保存原始密文字节，因为 dst 和 src 可能是同一个切片
		dst[i] = c ^ x.tmp[0]
		copy(x.iv, x.iv[1:])
		x.iv[len(x.iv)-1] = c
	}
}

// NewNetwork 从服务器地址创建新的网络实例（客户端模式）
func NewNetwork(addr *ServerAddr) (*Network, error) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		return nil, err
	}

	dr := &XORReader{Reader: conn, Cipher: nil}
	ew := &XORWriter{Writer: conn, Cipher: nil}

	return &Network{
		conn:          conn,
		reader:        bufio.NewReader(dr),
		writer:        bufio.NewWriter(ew),
		decryptReader: dr,
		encryptWriter: ew,
		closed:        false,
		compressionTh: -1,
	}, nil
}

// NewNetworkFromString 从字符串地址创建新的网络实例
func NewNetworkFromString(address string) (*Network, error) {
	addr, err := NewServerAddrFromString(address)
	if err != nil {
		return nil, err
	}
	return NewNetwork(addr)
}

// NewNetworkFromListener 从监听器接受连接创建网络实例（服务器模式）
func NewNetworkFromListener(listener net.Listener) (*Network, error) {
	conn, err := listener.Accept()
	if err != nil {
		return nil, err
	}
	
	// 优化：启用 TCP_NODELAY，禁用 Nagle 算法，减少延迟
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetLinger(0) // 立即关闭连接，不等待缓冲区发送
	}
	
	dr := &XORReader{Reader: conn, Cipher: nil}
	ew := &XORWriter{Writer: conn, Cipher: nil}

	return &Network{
		conn:          conn,
		reader:        bufio.NewReader(dr),
		writer:        bufio.NewWriter(ew),
		decryptReader: dr,
		encryptWriter: ew,
		closed:        false,
		compressionTh: -1,
	}, nil
}

// ListenOnAddress 在指定地址监听（服务器模式）
func ListenOnAddress(addr *ServerAddr) (net.Listener, error) {
	return addr.Listen()
}

// ListenOnPort 在指定端口监听（默认 IP 0.0.0.0）
func ListenOnPort(port int) (net.Listener, error) {
	addr := NewServerAddr("0.0.0.0", port)
	return addr.Listen()
}

// ListenDefault 在默认地址监听（0.0.0.0:25565）
func ListenDefault() (net.Listener, error) {
	return DefaultServerAddr.Listen()
}

// Close 关闭网络链接
func (n *Network) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return nil
	}
	n.closed = true
	return n.conn.Close()
}

// WriteVarint 写入 Varint 数据
func WriteVarint(buf *bytes.Buffer, value int32) {
	uvalue := uint32(value)
	for uvalue >= 0x80 {
		buf.WriteByte(byte(uvalue) | 0x80)
		uvalue >>= 7
	}
	buf.WriteByte(byte(uvalue))
}

// ReadVarint 读取 Varint 数据
func ReadVarint(reader io.Reader) (int32, error) {
	var result int32
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
		result |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 35 {
			return 0, io.ErrUnexpectedEOF
		}
	}
	return result, nil
}

// Send 发送数据包
// 如果启用了压缩，会自动重构成压缩包格式
func (n *Network) Send(data []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return io.ErrClosedPipe
	}

	if n.compressionTh >= 0 {
		// 启用压缩模式：读取数据包长度前缀，解析出 Payload 并重新按照压缩格式封包
		buf := bytes.NewReader(data)
		length, err := ReadVarint(buf)
		if err == nil && int(length) == buf.Len() {
			// 只有在数据包符合 [Length][Payload] 格式且长度匹配时才重构
			payload := data[len(data)-buf.Len():]
			return n.sendPacketPayloadUnsafe(payload)
		}
		// 如果长度不匹配，则作为原始数据直接发送
	}

	// 未启用压缩，直接透传
	_, err := n.writer.Write(data)
	if err != nil {
		return err
	}
	return n.writer.Flush()
}

// SendPacketPayload 发送已经拼接好 PacketID 的数据包负载
// 会根据当前的压缩和加密状态进行正确封包
func (n *Network) SendPacketPayload(payload []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.sendPacketPayloadUnsafe(payload)
}

func (n *Network) sendPacketPayloadUnsafe(payload []byte) error {
	if n.closed {
		return io.ErrClosedPipe
	}

	finalBuf := &bytes.Buffer{}

	if n.compressionTh >= 0 {
		// 启用压缩模式
		if int32(len(payload)) >= n.compressionTh {
			// 需要压缩
			var compressed bytes.Buffer
			zw := zlib.NewWriter(&compressed)
			_, _ = zw.Write(payload)
			_ = zw.Close()

			compressedBytes := compressed.Bytes()
			uncompressedLen := int32(len(payload))

			// 计算 Data Length 的 VarInt 长度
			dataLenBuf := &bytes.Buffer{}
			WriteVarint(dataLenBuf, uncompressedLen)
			dataLenBytes := dataLenBuf.Bytes()

			// Packet Length = len(Data Length) + len(Compressed Data)
			packetLen := int32(len(dataLenBytes) + len(compressedBytes))
			WriteVarint(finalBuf, packetLen)
			finalBuf.Write(dataLenBytes)
			finalBuf.Write(compressedBytes)
		} else {
			// 小于阈值，不压缩，但 Data Length 设置为 0
			// Packet Length = len(Data Length的VarInt) + len(Payload)
			dataLenBuf := &bytes.Buffer{}
			WriteVarint(dataLenBuf, 0) // Data Length = 0 (表示未压缩)
			dataLenBytes := dataLenBuf.Bytes()
			
			packetLen := int32(len(dataLenBytes) + len(payload))
			WriteVarint(finalBuf, packetLen)
			finalBuf.Write(dataLenBytes)
			finalBuf.Write(payload)
		}
	} else {
		// 未启用压缩模式
		// Packet Length = len(Payload)
		WriteVarint(finalBuf, int32(len(payload)))
		finalBuf.Write(payload)
	}

	_, err := n.writer.Write(finalBuf.Bytes())
	if err != nil {
		return err
	}
	return n.writer.Flush()
}

// SendPacket 发送数据包
func (n *Network) SendPacket(packetID int32, data []byte) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 1. 准备原始负载 (PacketID + Data)
	payloadBuf := &bytes.Buffer{}
	WriteVarint(payloadBuf, packetID)
	payloadBuf.Write(data)
	payload := payloadBuf.Bytes()

	return n.sendPacketPayloadUnsafe(payload)
}

// Read 读取数据包
func (n *Network) Read(length int) ([]byte, error) {
	if n.closed {
		return nil, io.ErrClosedPipe
	}
	data := make([]byte, length)
	_, err := io.ReadFull(n.reader, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ReadPacket 读取数据包
func (n *Network) ReadPacket() (packetID int32, data []byte, err error) {
	if n.closed {
		return 0, nil, io.ErrClosedPipe
	}

	// 1. 读取包长度
	length, err := ReadVarint(n.reader)
	if err != nil {
		return 0, nil, err
	}

	// 2. 读取完整数据包内容
	fullData := make([]byte, length)
	_, err = io.ReadFull(n.reader, fullData)
	if err != nil {
		return 0, nil, err
	}

	buf := bytes.NewReader(fullData)

	if n.compressionTh >= 0 {
		// 启用压缩模式：读取解压长度
		uncompressedLen, err := ReadVarint(buf)
		if err != nil {
			return 0, nil, err
		}

		if uncompressedLen > 0 {
			// 数据被压缩，需要解压
			zr, err := zlib.NewReader(buf)
			if err != nil {
				return 0, nil, err
			}
			defer zr.Close()

			decompressed := make([]byte, uncompressedLen)
			_, err = io.ReadFull(zr, decompressed)
			if err != nil {
				return 0, nil, err
			}

			// 从解压后的数据中读取 PacketID
			dBuf := bytes.NewReader(decompressed)
			packetID, err = ReadVarint(dBuf)
			if err != nil {
				return 0, nil, err
			}

			data = make([]byte, dBuf.Len())
			_, _ = dBuf.Read(data)
			return packetID, data, nil
		}
		// uncompressedLen == 0 表示数据未压缩，直接读取 PacketID 和 Data
	}

	// 未启用压缩，或者 Data Length 为 0 (未达到阈值)
	packetID, err = ReadVarint(buf)
	if err != nil {
		return 0, nil, err
	}

	data = make([]byte, buf.Len())
	_, _ = buf.Read(data)

	return packetID, data, nil
}

// IsClosed 是否已关闭
func (n *Network) IsClosed() bool {
	return n.closed
}

// GetConn 获取网络连接
func (n *Network) GetConn() net.Conn {
	return n.conn
}

// RemoteAddr 获取远程地址
func (n *Network) RemoteAddr() string {
	if n.conn == nil {
		return "unknown"
	}
	return n.conn.RemoteAddr().String()
}

// SendPacketWithLength 发送带有长度前缀的数据包
// 直接使用 SendPacket 方法，它已经正确处理了包长度计算
func (n *Network) SendPacketWithLength(packetData []byte) error {
	// packetData 格式: [PacketID:VarInt] [Data]
	// 我们需要提取 PacketID 和 Data
	buf := bytes.NewReader(packetData)

	// 读取 PacketID
	packetID, err := ReadVarint(buf)
	if err != nil {
		return fmt.Errorf("failed to read packet ID: %v", err)
	}

	// 剩余的就是 Data
	data := make([]byte, buf.Len())
	_, err = buf.Read(data)
	if err != nil {
		return fmt.Errorf("failed to read packet data: %v", err)
	}

	// 使用 SendPacket 发送（会自动添加正确的长度前缀）
	return n.SendPacket(packetID, data)
}
