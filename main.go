package main

import (
	"RootreeMC/Network"
	"RootreeMC/Packet"
	Login "RootreeMC/Packet/Login"
	Play "RootreeMC/Packet/Play"
	"RootreeMC/Protocol"
	"RootreeMC/Tick"
	"RootreeMC/command"
	"RootreeMC/entity"
	"RootreeMC/logger"
	"RootreeMC/player"
	"RootreeMC/serverconfig"
	"RootreeMC/world"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

const configPath = "serverconfig/server_config.yml"

type mcWorldProvider struct{}

func (mcWorldProvider) GetBlock(x, y, z int32) uint16 {
	return world.GlobalWorld.GetBlock(x, y, z)
}

func (mcWorldProvider) IsBlockSolid(x, y, z float64) bool {
	return world.IsBlockSolid(x, y, z)
}

func main() {
	slog.SetDefault(slog.New(logger.NewMCLogHandler(os.Stdout)))

	if err := ensureConfig(configPath); err != nil {
		slog.Error("[Main] 初始化配置失败", "err", err)
		return
	}

	cfg, err := serverconfig.LoadConfig(configPath)
	if err != nil {
		slog.Error("[Main] 读取配置失败", "path", configPath, "err", err)
		return
	}
	applyConfigDefaults(cfg)

	if err := cfg.Validate(); err != nil {
		slog.Error("[Main] 配置校验失败", "err", err)
		return
	}

	world.SetTerrainSeed(cfg.TerrainSeed)
	world.GlobalWorld.RecalculateSpawnPoint()
	slog.Info("[Main] 地形种子已加载", "seed", cfg.TerrainSeed)

	registerRuntimeCallbacks()
	setTickMode(cfg.TickMode)

	go Tick.Start()

	addr := Network.NewServerAddr(cfg.ServerIP, cfg.ServerPort)
	listener, err := Network.ListenOnAddress(addr)
	if err != nil {
		slog.Error("[Main] 监听端口失败", "addr", addr.String(), "err", err)
		return
	}

	slog.Info("[Main] RootreeMC 已启动",
		"addr", addr.String(),
		"onlineMode", cfg.OnlineMode,
		"compressionThreshold", cfg.NetworkCompressionThreshold,
		"tickMode", cfg.TickMode,
	)

	var shuttingDown atomic.Bool
	var wg sync.WaitGroup

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if shuttingDown.CompareAndSwap(false, true) {
			slog.Warn("[Main] 收到退出信号，开始优雅停机")
			Tick.Stop()
			_ = listener.Close()
		}
	}()

	for {
		client, err := Network.NewNetworkFromListener(listener)
		if err != nil {
			if shuttingDown.Load() || isClosedListenerError(err) {
				break
			}
			slog.Warn("[Main] 接受连接失败", "err", err)
			continue
		}

		wg.Add(1)
		go func(c *Network.Network) {
			defer wg.Done()
			handleClient(c, cfg)
		}(client)
	}

	wg.Wait()
	saveAllState()
	slog.Info("[Main] 服务器已停止")
}

func ensureConfig(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	if err := serverconfig.CreateDefaultConfig(path); err != nil {
		return err
	}
	slog.Info("[Main] 已创建默认配置文件", "path", path)
	return nil
}

func applyConfigDefaults(cfg *serverconfig.ServerConfig) {
	def := serverconfig.DefaultConfig

	if strings.TrimSpace(cfg.LevelName) == "" {
		cfg.LevelName = def.LevelName
	}
	if strings.TrimSpace(cfg.LevelType) == "" {
		cfg.LevelType = def.LevelType
	}
	if cfg.MaxPlayers <= 0 {
		cfg.MaxPlayers = def.MaxPlayers
	}
	if strings.TrimSpace(cfg.MOTD) == "" {
		cfg.MOTD = def.MOTD
	}
	if cfg.QueryPort == 0 {
		cfg.QueryPort = def.QueryPort
	}
	if cfg.RCONPort == 0 {
		cfg.RCONPort = def.RCONPort
	}
	if strings.TrimSpace(cfg.ServerIP) == "" {
		cfg.ServerIP = def.ServerIP
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = def.ServerPort
	}
	if strings.TrimSpace(cfg.TickMode) == "" {
		cfg.TickMode = def.TickMode
	}
}

func setTickMode(mode string) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "multithread":
		Tick.SetTickMode(Tick.TickModeMultiThread)
	default:
		Tick.SetTickMode(Tick.TickModeSingleThread)
	}
}

func registerRuntimeCallbacks() {
	entity.RegisterWorldProvider(mcWorldProvider{})
	entity.RegisterPlayerDamageHandler(Tick.ApplyPlayerDamage)
	entity.RegisterMobDestroyHandler(func(eid int32) {
		pkt := entity.BuildDestroyEntities([]int32{eid})
		allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
		for _, p := range allPlayers {
			_ = p.Client.Send(pkt)
		}
	})

	world.RegisterBroadcastCallback(func(pkt []byte) {
		allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
		for _, p := range allPlayers {
			_ = p.Client.Send(pkt)
		}
	})

	command.RegisterLightUpdateCallback(func(chunkX, chunkZ int32) {
		world.GlobalLightingEngine.CalculateNaturalLight(chunkX, chunkZ)
		chunk := world.GlobalWorld.GetOrCreateChunk(chunkX, chunkZ)
		packet := world.BuildMapChunk(chunk)
		allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
		for _, p := range allPlayers {
			_ = p.Client.Send(packet)
		}
	})
}

func handleClient(client *Network.Network, cfg *serverconfig.ServerConfig) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[Conn] 连接处理发生 panic", "remote", client.RemoteAddr(), "panic", r)
		}
		cleanupClient(client)
	}()

	handler := Login.NewLoginHandler(client, cfg)
	state := Protocol.StateHandshaking

	for {
		packetID, packetData, err := client.ReadPacket()
		if err != nil {
			return
		}

		switch state {
		case Protocol.StateHandshaking:
			if packetID != 0x00 {
				return
			}
			if err := handler.HandleHandshake(packetData); err != nil {
				return
			}
			state = handler.GetState()

		case Protocol.StateStatus:
			if keep := Packet.HandleStatusPackets(client, packetID, packetData, cfg.MOTD, cfg.MaxPlayers); !keep {
				return
			}

		case Protocol.StateLogin:
			switch packetID {
			case 0x00:
				err = handler.HandleLoginStart(packetData)
			case 0x01:
				err = handler.HandleEncryptionResponse(packetData)
			default:
				continue
			}
			if err != nil {
				return
			}

			state = handler.GetState()
			if handler.IsLoginFinished() {
				username, pUUID, props := handler.GetPlayerInfo()
				Packet.SendGameInitialization(client, username, pUUID, toEntityProperties(props))
				Packet.StartKeepAliveLoop(client)

				if p := player.GlobalPlayerManager.GetPlayerByClient(client); p != nil {
					Play.UpdateChunksForPlayer(client, p.PlayerEntity.X, p.PlayerEntity.Z)
				}

				state = Protocol.StatePlay
			}

		case Protocol.StatePlay:
			Packet.HandlePlayPackets(client, packetID, packetData)
		}
	}
}

func toEntityProperties(props []Login.Property) []entity.PlayerProperty {
	if len(props) == 0 {
		return nil
	}
	out := make([]entity.PlayerProperty, 0, len(props))
	for _, p := range props {
		out = append(out, entity.PlayerProperty{
			Name:      p.Name,
			Value:     p.Value,
			IsSigned:  strings.TrimSpace(p.Signature) != "",
			Signature: p.Signature,
		})
	}
	return out
}

func cleanupClient(client *Network.Network) {
	if client == nil {
		return
	}

	if p := player.GlobalPlayerManager.GetPlayerByClient(client); p != nil {
		Packet.BroadcastPlayerLeave(p)
		Tick.RemovePlayerPhysics(p.PlayerEntity.EID)
		player.RemoveKeepAliveRecord(p.PlayerEntity.EID)
		player.GlobalPlayerManager.PlayerLeave(client)
	}

	Packet.CleanupClientUsername(client)
	_ = client.Close()
}

func saveAllState() {
	player.GlobalPlayerManager.SaveAllPlayers()

	if err := world.GlobalWorld.SaveAllChunks(); err != nil {
		slog.Warn("[Main] 保存区块失败", "err", err)
	}
	if err := world.GlobalWorld.SaveBlockTickState(); err != nil {
		slog.Warn("[Main] 保存方块 Tick 队列失败", "err", err)
	}
}

func isClosedListenerError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}
