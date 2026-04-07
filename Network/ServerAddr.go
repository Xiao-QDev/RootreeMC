// Package Network 处理端口和IP地址连接的工具
package Network

import (
	"bufio"
	"fmt"
	"net"
)

// ServerAddr 服务器地址结构体
type ServerAddr struct {
	IP   string //IP 地址
	Port int    //端口号
}

// DefaultServerAddr 默认服务器地址
var DefaultServerAddr = &ServerAddr{
	IP:   "0.0.0.0",
	Port: 25565,
}

// NewServerAddr 创建新的服务器地址
func NewServerAddr(ip string, port int) *ServerAddr {
	return &ServerAddr{
		IP:   ip,
		Port: port,
	}
}

// NewServerAddrFromString 从字符串创建新的服务器地址
func NewServerAddrFromString(address string) (*ServerAddr, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	portInt, err := net.LookupPort("tcp", port)
	if err != nil {
		return nil, err
	}

	return &ServerAddr{
		IP:   host,
		Port: portInt,
	}, nil
}

// String 返回地址字符串（IP:Port）
func (s *ServerAddr) String() string {
	return fmt.Sprintf("%s:%d", s.IP, s.Port)
}

// TCPAddr 转换为 net.TCPAddr
func (s *ServerAddr) TCPAddr() *net.TCPAddr {
	return &net.TCPAddr{
		IP:   net.ParseIP(s.IP),
		Port: s.Port,
	}
}

// Listen 监听此地址
func (s *ServerAddr) Listen() (net.Listener, error) {
	return net.ListenTCP("tcp", s.TCPAddr())
}

// Dial 连接到此地址
func (s *ServerAddr) Dial() (*Network, error) {
	conn, err := net.Dial("tcp", s.String())
	if err != nil {
		return nil, err
	}
	return &Network{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		closed: false,
	}, nil
}

// IsValid 检查地址是否有效
func (s *ServerAddr) IsValid() bool {
	if s.Port < 1 || s.Port > 65535 {
		return false
	}

	ip := net.ParseIP(s.IP)
	if ip == nil {
		return false
	}

	return true
}

// IsLocalhost 检查是否是本地地址
func (s *ServerAddr) IsLocalhost() bool {
	return s.IP == "localhost" ||
		s.IP == "127.0.0.1" ||
		s.IP == "::1" ||
		s.IP == "0.0.0.0"
}

// Clone 克隆地址对象
func (s *ServerAddr) Clone() *ServerAddr {
	return &ServerAddr{
		IP:   s.IP,
		Port: s.Port,
	}
}

// WithIP 设置 IP 地址（链式调用）
func (s *ServerAddr) WithIP(ip string) *ServerAddr {
	s.IP = ip
	return s
}

// WithPort 设置端口（链式调用）
func (s *ServerAddr) WithPort(port int) *ServerAddr {
	s.Port = port
	return s
}
