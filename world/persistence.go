// Package world 世界存档（LINEAR V2 + ANVIL 自动转换）
package world

import (
	"RootreeMC/nbt"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	linearChunkMagic  = "LINEARV2"
	linearChunkVersion uint16 = 2
	linearChunkExt    = ".linear2"
	linearHeaderSize  = 22 // magic(8) + version(2) + x(4) + z(4) + blockCount(4)
	chunkBlockCount   = 16 * 256 * 16

	legacyChunkMagic  = "RTB1"
	legacyHeaderSize  = 12
)

func defaultChunkSaveDir() string {
	return filepath.Join("saves", "world", "rootree", "linear_v2")
}

func defaultBlockTickSavePath() string {
	return filepath.Join("saves", "world", "rootree", "block_ticks.nbt")
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func printYellow(msg string, args ...any) {
	fmt.Printf("\033[33m"+msg+"\033[0m\n", args...)
}

func (wm *WorldManager) chunkFilePath(chunkX, chunkZ int32) string {
	name := fmt.Sprintf("c.%d.%d%s", chunkX, chunkZ, linearChunkExt)
	return filepath.Join(wm.saveDir, name)
}

func (wm *WorldManager) legacyChunkFilePath(chunkX, chunkZ int32) string {
	name := fmt.Sprintf("c.%d.%d.rtb", chunkX, chunkZ)
	legacyDir := filepath.Join("saves", "world", "rootree", "chunks")
	return filepath.Join(legacyDir, name)
}

func (wm *WorldManager) hasLinearChunks() bool {
	matches, err := filepath.Glob(filepath.Join(wm.saveDir, "*"+linearChunkExt))
	if err != nil {
		return false
	}
	return len(matches) > 0
}

func (wm *WorldManager) anvilConversionMarkerPath() string {
	return filepath.Join(wm.saveDir, ".anvil_converted")
}

func (wm *WorldManager) hasCompletedAnvilConversion(regionFiles []string) bool {
	markerInfo, err := os.Stat(wm.anvilConversionMarkerPath())
	if err != nil {
		return false
	}
	markerTime := markerInfo.ModTime()

	for _, regionPath := range regionFiles {
		info, err := os.Stat(regionPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(markerTime) {
			return false
		}
	}
	return true
}

func anvilRegionFiles() ([]string, error) {
	return filepath.Glob(filepath.Join("saves", "world", "region", "*.mca"))
}

func uniqueBackupZipPath() string {
	base := fmt.Sprintf("ANVIL_BACKUP_%s.zip", time.Now().Format("20060102_150405"))
	path := filepath.Join(".", base)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	for i := 1; i < 1000; i++ {
		p := filepath.Join(".", fmt.Sprintf("ANVIL_BACKUP_%s_%d.zip", time.Now().Format("20060102_150405"), i))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
	return filepath.Join(".", fmt.Sprintf("ANVIL_BACKUP_%d.zip", time.Now().UnixNano()))
}

func zipDir(srcDir, zipPath string) error {
	if err := ensureDir(filepath.Dir(zipPath)); err != nil {
		return err
	}

	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(w, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

// ConvertAnvilToLinearV2IfNeeded 检测并自动转换 ANVIL 存档到 LINEAR V2。
func (wm *WorldManager) ConvertAnvilToLinearV2IfNeeded() error {
	regionFiles, err := anvilRegionFiles()
	if err != nil {
		return err
	}
	if len(regionFiles) == 0 {
		return nil
	}

	// 已完成转换时，不重复执行
	if wm.hasCompletedAnvilConversion(regionFiles) {
		return nil
	}

	printYellow("[World] 检测到 ANVIL 存档，正在自动转换为 LINEAR V2 ...")
	if wm.hasLinearChunks() {
		printYellow("[World] 检测到已有 LINEAR V2 区块，将仅转换缺失区块")
	}

	backupZip := uniqueBackupZipPath()
	if err := zipDir(filepath.Join("saves", "world"), backupZip); err != nil {
		slog.Warn("[World] 备份 ANVIL 存档失败", "err", err)
	} else {
		printYellow("[World] 已备份原 ANVIL 存档到 %s", backupZip)
	}

	totalConverted := 0
	for _, regionPath := range regionFiles {
		n, err := wm.convertOneAnvilRegion(regionPath)
		if err != nil {
			return fmt.Errorf("convert region %s failed: %w", regionPath, err)
		}
		totalConverted += n
	}

	if totalConverted > 0 || wm.hasLinearChunks() {
		if err := os.WriteFile(wm.anvilConversionMarkerPath(), []byte(time.Now().Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
			slog.Warn("[World] 写入 ANVIL 转换标记失败", "err", err)
		}
	} else {
		slog.Warn("[World] 未转换到任何区块，将在下次启动继续尝试")
	}

	printYellow("[World] ANVIL -> LINEAR V2 转换完成，共转换 %d 个区块", totalConverted)
	return nil
}

func parseRegionCoords(regionPath string) (int32, int32, error) {
	base := filepath.Base(regionPath) // r.<x>.<z>.mca
	parts := strings.Split(base, ".")
	if len(parts) != 4 || parts[0] != "r" || parts[3] != "mca" {
		return 0, 0, fmt.Errorf("invalid region filename: %s", base)
	}

	rx64, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return 0, 0, err
	}
	rz64, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil {
		return 0, 0, err
	}
	return int32(rx64), int32(rz64), nil
}

func readChunkPayload(file *os.File, sectorOffset uint32) ([]byte, error) {
	chunkPos := int64(sectorOffset) * 4096

	var lengthBuf [4]byte
	if _, err := file.ReadAt(lengthBuf[:], chunkPos); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf[:])
	if length < 1 {
		return nil, fmt.Errorf("invalid chunk length: %d", length)
	}

	raw := make([]byte, length)
	if _, err := file.ReadAt(raw, chunkPos+4); err != nil {
		return nil, err
	}
	return raw, nil
}

func decompressChunkData(raw []byte) ([]byte, error) {
	if len(raw) < 1 {
		return nil, fmt.Errorf("empty compressed chunk payload")
	}
	compressionType := raw[0]
	payload := raw[1:]

	var r io.ReadCloser
	var err error
	switch compressionType {
	case 1:
		r, err = gzip.NewReader(bytes.NewReader(payload))
	case 2:
		r, err = zlib.NewReader(bytes.NewReader(payload))
	case 3:
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported anvil compression type: %d", compressionType)
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

func nibbleAt(data []byte, index int) uint8 {
	if len(data) == 0 {
		return 0
	}
	b := data[index>>1]
	if index&1 == 0 {
		return b & 0x0F
	}
	return (b >> 4) & 0x0F
}

func byteArrayTag(c *nbt.CompoundTag, name string) []byte {
	tag, ok := c.Get(name)
	if !ok {
		return nil
	}
	arr, ok := tag.(*nbt.ByteArrayTag)
	if !ok {
		return nil
	}
	return arr.Value
}

func chunkFromAnvilNBT(doc *nbt.NBT, fallbackX, fallbackZ int32) (*Chunk, error) {
	root, ok := doc.Root.(*nbt.CompoundTag)
	if !ok {
		return nil, fmt.Errorf("invalid root tag type: %T", doc.Root)
	}

	level, ok := root.GetCompound("Level")
	if !ok {
		return nil, fmt.Errorf("missing Level compound")
	}

	chunkX, hasX := level.GetInt("xPos")
	chunkZ, hasZ := level.GetInt("zPos")
	if !hasX || !hasZ {
		chunkX = fallbackX
		chunkZ = fallbackZ
	}

	chunk := NewChunk(chunkX, chunkZ)

	sections, ok := level.GetList("Sections")
	if !ok {
		return chunk, nil
	}

	for _, sectionTag := range sections.Value {
		section, ok := sectionTag.(*nbt.CompoundTag)
		if !ok {
			continue
		}

		sectionYRaw, ok := section.GetByte("Y")
		if !ok {
			continue
		}
		sectionY := int(sectionYRaw)

		blocks := byteArrayTag(section, "Blocks")
		if len(blocks) < 4096 {
			continue
		}
		data := byteArrayTag(section, "Data")
		add := byteArrayTag(section, "Add")

		for idx := 0; idx < 4096; idx++ {
			x := idx & 15
			z := (idx >> 4) & 15
			yInSection := (idx >> 8) & 15
			y := sectionY*16 + yInSection
			if y < 0 || y >= 256 {
				continue
			}

			idLow := uint16(uint8(blocks[idx]))
			idHigh := uint16(nibbleAt(add, idx))
			id := (idHigh << 8) | idLow

			meta := nibbleAt(data, idx)
			chunk.Blocks[x][y][z] = ToState(id, meta)
		}
	}

	return chunk, nil
}

func (wm *WorldManager) convertOneAnvilRegion(regionPath string) (int, error) {
	rx, rz, err := parseRegionCoords(regionPath)
	if err != nil {
		return 0, err
	}

	file, err := os.Open(regionPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	header := make([]byte, 8192)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, err
	}

	converted := 0
	for idx := 0; idx < 1024; idx++ {
		entry := binary.BigEndian.Uint32(header[idx*4 : idx*4+4])
		sectorOffset := entry >> 8
		sectorCount := entry & 0xFF
		if sectorOffset == 0 || sectorCount == 0 {
			continue
		}

		localX := int32(idx % 32)
		localZ := int32(idx / 32)
		chunkX := rx*32 + localX
		chunkZ := rz*32 + localZ

		target := wm.chunkFilePath(chunkX, chunkZ)
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return converted, err
		}

		raw, err := readChunkPayload(file, sectorOffset)
		if err != nil {
			slog.Warn("[World] 读取 ANVIL 区块失败", "region", regionPath, "chunkX", chunkX, "chunkZ", chunkZ, "err", err)
			continue
		}

		nbtBytes, err := decompressChunkData(raw)
		if err != nil {
			slog.Warn("[World] 解压 ANVIL 区块失败", "region", regionPath, "chunkX", chunkX, "chunkZ", chunkZ, "err", err)
			continue
		}

		doc, err := nbt.ReadBytes(nbtBytes)
		if err != nil {
			slog.Warn("[World] 解析 ANVIL 区块 NBT 失败", "region", regionPath, "chunkX", chunkX, "chunkZ", chunkZ, "err", err)
			continue
		}

		chunk, err := chunkFromAnvilNBT(doc, chunkX, chunkZ)
		if err != nil {
			slog.Warn("[World] 转换 ANVIL 区块失败", "region", regionPath, "chunkX", chunkX, "chunkZ", chunkZ, "err", err)
			continue
		}

		if err := wm.writeChunkToDisk(chunk); err != nil {
			return converted, err
		}
		converted++
	}

	return converted, nil
}

func decodeLegacyChunk(data []byte, chunkX, chunkZ int32) (*Chunk, error) {
	expectedSize := legacyHeaderSize + chunkBlockCount*2
	if len(data) != expectedSize {
		return nil, fmt.Errorf("invalid legacy chunk file size %d, expected %d", len(data), expectedSize)
	}
	if string(data[:4]) != legacyChunkMagic {
		return nil, fmt.Errorf("invalid legacy chunk magic %q", string(data[:4]))
	}

	fileChunkX := int32(binary.LittleEndian.Uint32(data[4:8]))
	fileChunkZ := int32(binary.LittleEndian.Uint32(data[8:12]))
	if fileChunkX != chunkX || fileChunkZ != chunkZ {
		return nil, fmt.Errorf("legacy chunk coordinate mismatch: file(%d,%d) request(%d,%d)", fileChunkX, fileChunkZ, chunkX, chunkZ)
	}

	chunk := NewChunk(chunkX, chunkZ)
	offset := legacyHeaderSize
	for y := 0; y < 256; y++ {
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				chunk.Blocks[x][y][z] = binary.LittleEndian.Uint16(data[offset : offset+2])
				offset += 2
			}
		}
	}
	return chunk, nil
}

func (wm *WorldManager) loadChunkFromDisk(chunkX, chunkZ int32) (*Chunk, error) {
	// 1) 优先读取 LINEAR V2
	path := wm.chunkFilePath(chunkX, chunkZ)
	data, err := os.ReadFile(path)
	if err == nil {
		expectedSize := linearHeaderSize + chunkBlockCount*2
		if len(data) != expectedSize {
			return nil, fmt.Errorf("invalid linear chunk file size %d, expected %d", len(data), expectedSize)
		}
		if string(data[:8]) != linearChunkMagic {
			return nil, fmt.Errorf("invalid linear chunk magic %q", string(data[:8]))
		}

		version := binary.LittleEndian.Uint16(data[8:10])
		if version != linearChunkVersion {
			return nil, fmt.Errorf("unsupported linear chunk version %d", version)
		}

		fileChunkX := int32(binary.LittleEndian.Uint32(data[10:14]))
		fileChunkZ := int32(binary.LittleEndian.Uint32(data[14:18]))
		if fileChunkX != chunkX || fileChunkZ != chunkZ {
			return nil, fmt.Errorf("linear chunk coordinate mismatch: file(%d,%d) request(%d,%d)", fileChunkX, fileChunkZ, chunkX, chunkZ)
		}

		count := binary.LittleEndian.Uint32(data[18:22])
		if count != uint32(chunkBlockCount) {
			return nil, fmt.Errorf("invalid linear chunk block count %d", count)
		}

		chunk := NewChunk(chunkX, chunkZ)
		offset := linearHeaderSize
		for y := 0; y < 256; y++ {
			for z := 0; z < 16; z++ {
				for x := 0; x < 16; x++ {
					chunk.Blocks[x][y][z] = binary.LittleEndian.Uint16(data[offset : offset+2])
					offset += 2
				}
			}
		}
		return chunk, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	// 2) 兼容读取历史 RTB1，并自动迁移到 LINEAR V2
	legacyPath := wm.legacyChunkFilePath(chunkX, chunkZ)
	legacyData, legacyErr := os.ReadFile(legacyPath)
	if legacyErr != nil {
		if os.IsNotExist(legacyErr) {
			return nil, nil
		}
		return nil, legacyErr
	}

	chunk, err := decodeLegacyChunk(legacyData, chunkX, chunkZ)
	if err != nil {
		return nil, err
	}
	if err := wm.writeChunkToDisk(chunk); err != nil {
		slog.Warn("[World] 迁移 RTB1 到 LINEAR V2 失败", "x", chunkX, "z", chunkZ, "err", err)
	}
	return chunk, nil
}

func (wm *WorldManager) writeChunkToDisk(chunk *Chunk) error {
	if chunk == nil {
		return nil
	}

	if err := ensureDir(wm.saveDir); err != nil {
		return err
	}

	buf := make([]byte, linearHeaderSize+chunkBlockCount*2)
	copy(buf[:8], []byte(linearChunkMagic))
	binary.LittleEndian.PutUint16(buf[8:10], linearChunkVersion)
	binary.LittleEndian.PutUint32(buf[10:14], uint32(chunk.X))
	binary.LittleEndian.PutUint32(buf[14:18], uint32(chunk.Z))
	binary.LittleEndian.PutUint32(buf[18:22], uint32(chunkBlockCount))

	offset := linearHeaderSize
	for y := 0; y < 256; y++ {
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				binary.LittleEndian.PutUint16(buf[offset:offset+2], chunk.Blocks[x][y][z])
				offset += 2
			}
		}
	}

	target := wm.chunkFilePath(chunk.X, chunk.Z)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, buf, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// SaveDirtyChunks 将脏区块写入磁盘。
func (wm *WorldManager) SaveDirtyChunks() error {
	type saveTask struct {
		key   [2]int32
		chunk *Chunk
	}

	wm.mu.Lock()
	if len(wm.dirtyChunks) == 0 {
		wm.mu.Unlock()
		return nil
	}

	tasks := make([]saveTask, 0, len(wm.dirtyChunks))
	for key := range wm.dirtyChunks {
		if chunk, ok := wm.chunks[key]; ok {
			tasks = append(tasks, saveTask{key: key, chunk: chunk})
		}
	}
	wm.dirtyChunks = make(map[[2]int32]bool)
	wm.mu.Unlock()

	var firstErr error
	for _, task := range tasks {
		if err := wm.writeChunkToDisk(task.chunk); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Warn("[World] 保存区块失败", "x", task.key[0], "z", task.key[1], "err", err)
			wm.mu.Lock()
			wm.dirtyChunks[task.key] = true // 回滚脏标记，下一次重试
			wm.mu.Unlock()
			continue
		}
	}

	return firstErr
}

// SaveAllChunks 保存所有已加载区块。
func (wm *WorldManager) SaveAllChunks() error {
	wm.mu.Lock()
	for key := range wm.chunks {
		wm.dirtyChunks[key] = true
	}
	wm.mu.Unlock()
	return wm.SaveDirtyChunks()
}

// SaveBlockTickState 保存方块 Tick 队列到 NBT。
func (wm *WorldManager) SaveBlockTickState() error {
	if GlobalWorldSimulation == nil {
		return nil
	}

	state := GlobalWorldSimulation.ExportPendingTicksNBT()
	if state == nil {
		return nil
	}

	data, err := state.WriteBytes()
	if err != nil {
		return err
	}

	path := defaultBlockTickSavePath()
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadBlockTickState 读取方块 Tick 队列 NBT。
func (wm *WorldManager) LoadBlockTickState() error {
	path := defaultBlockTickSavePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	doc, err := nbt.ReadBytes(data)
	if err != nil {
		return err
	}

	if GlobalWorldSimulation == nil {
		GlobalWorldSimulation = NewWorldSimulation()
	}
	return GlobalWorldSimulation.ImportPendingTicksNBT(doc)
}
