package Protocol

type Version int32

const Minecraft1_12_2 Version = 340

// State ProtocolState 连接状态
type State int32

const (
	StateHandshaking State = iota // 0 - 握手阶段
	StateStatus                   // 1 - 状态查询阶段
	StateLogin                    // 2 - 登录阶段
	StatePlay                     // 3 - 游戏阶段
)

func (pv Version) String() string {
	switch pv {
	case Minecraft1_12_2:
		return "1.12.2"
	default:
		return "Unknown"
	}
}

func (pv Version) IsSupported() bool {
	for _, supportedVersion := range []Version{Minecraft1_12_2} {
		if pv == supportedVersion {
			return true
		}
	}
	return false
}
