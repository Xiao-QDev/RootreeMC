 // Package nbt Minecraft NBT (Named Binary Tag) 格式解析器
 // 参考: https://minecraft.fandom.com/wiki/NBT_format
 package nbt
 
 import (
 	"bytes"
 	"encoding/binary"
 	"fmt"
 	"io"
 )
 
 // TagType NBT标签类型
 type TagType byte
 
 const (
 	TagEnd        TagType = 0 // 结束标记
 	TagByte       TagType = 1 // 有符号字节 (8位)
 	TagShort      TagType = 2 // 有符号短整型 (16位, 大端)
 	TagInt        TagType = 3 // 有符号整型 (32位, 大端)
 	TagLong       TagType = 4 // 有符号长整型 (64位, 大端)
 	TagFloat      TagType = 5 // 单精度浮点 (32位, 大端, IEEE 754)
 	TagDouble     TagType = 6 // 双精度浮点 (64位, 大端, IEEE 754)
 	TagByteArray  TagType = 7 // 字节数组 (长度前缀)
 	TagString     TagType = 8 // UTF-8字符串 (长度前缀)
 	TagList       TagType = 9 // 同类型标签列表
 	TagCompound   TagType = 10 // 复合标签 (类似字典)
 	TagIntArray   TagType = 11 // 整型数组 (长度前缀)
 	TagLongArray  TagType = 12 // 长整型数组 (长度前缀)
 )
 
 // Tag NBT标签接口
 type Tag interface {
 	Type() TagType
 	String() string
 }
 
 // ByteTag 字节标签
 type ByteTag struct {
 	Value int8
 }
 
 func (t *ByteTag) Type() TagType { return TagByte }
 func (t *ByteTag) String() string { return fmt.Sprintf("%db", t.Value) }
 
 // ShortTag 短整型标签
 type ShortTag struct {
 	Value int16
 }
 
 func (t *ShortTag) Type() TagType { return TagShort }
 func (t *ShortTag) String() string { return fmt.Sprintf("%ds", t.Value) }
 
 // IntTag 整型标签
 type IntTag struct {
 	Value int32
 }
 
 func (t *IntTag) Type() TagType { return TagInt }
 func (t *IntTag) String() string { return fmt.Sprintf("%d", t.Value) }
 
 // LongTag 长整型标签
 type LongTag struct {
 	Value int64
 }
 
 func (t *LongTag) Type() TagType { return TagLong }
 func (t *LongTag) String() string { return fmt.Sprintf("%dl", t.Value) }
 
 // FloatTag 浮点标签
 type FloatTag struct {
 	Value float32
 }
 
 func (t *FloatTag) Type() TagType { return TagFloat }
 func (t *FloatTag) String() string { return fmt.Sprintf("%ff", t.Value) }
 
 // DoubleTag 双精度浮点标签
 type DoubleTag struct {
 	Value float64
 }
 
 func (t *DoubleTag) Type() TagType { return TagDouble }
 func (t *DoubleTag) String() string { return fmt.Sprintf("%f", t.Value) }
 
 // ByteArrayTag 字节数组标签
 type ByteArrayTag struct {
 	Value []byte
 }
 
 func (t *ByteArrayTag) Type() TagType { return TagByteArray }
 func (t *ByteArrayTag) String() string { return fmt.Sprintf("[B;%d]", len(t.Value)) }
 
 // StringTag 字符串标签
 type StringTag struct {
 	Value string
 }
 
 func (t *StringTag) Type() TagType { return TagString }
 func (t *StringTag) String() string { return fmt.Sprintf("\"%s\"", t.Value) }
 
 // ListTag 列表标签 (所有元素类型相同)
 type ListTag struct {
 	TagType TagType
 	Value   []Tag
 }
 
 func (t *ListTag) Type() TagType { return TagList }
 func (t *ListTag) String() string { return fmt.Sprintf("[%d elements]", len(t.Value)) }
 
 // CompoundTag 复合标签 (类似字典)
 type CompoundTag struct {
 	Value map[string]Tag
 }
 
 func (t *CompoundTag) Type() TagType { return TagCompound }
 func (t *CompoundTag) String() string { return fmt.Sprintf("{%d entries}", len(t.Value)) }
 
 // IntArrayTag 整型数组标签
 type IntArrayTag struct {
 	Value []int32
 }
 
 func (t *IntArrayTag) Type() TagType { return TagIntArray }
 func (t *IntArrayTag) String() string { return fmt.Sprintf("[I;%d]", len(t.Value)) }
 
 // LongArrayTag 长整型数组标签
 type LongArrayTag struct {
 	Value []int64
 }
 
 func (t *LongArrayTag) Type() TagType { return TagLongArray }
 func (t *LongArrayTag) String() string { return fmt.Sprintf("[L;%d]", len(t.Value)) }
 
 // EndTag 结束标签
 type EndTag struct{}
 
 func (t *EndTag) Type() TagType { return TagEnd }
 func (t *EndTag) String() string { return "END" }
 
 // NBT NBT数据根结构
 type NBT struct {
 	Name string
 	Root Tag
 }
 
 // Read 从io.Reader读取NBT数据
 func Read(reader io.Reader) (*NBT, error) {
 	buf := &bytes.Buffer{}
 	_, err := buf.ReadFrom(reader)
 	if err != nil {
 		return nil, err
 	}
 	return ReadBytes(buf.Bytes())
 }
 
 // ReadBytes 从字节数组读取NBT数据
 func ReadBytes(data []byte) (*NBT, error) {
 	reader := bytes.NewReader(data)
 	
 	// 读取根标签类型
 	var tagType TagType
 	if err := binary.Read(reader, binary.BigEndian, &tagType); err != nil {
 		return nil, err
 	}
 	
 	if tagType != TagCompound {
 		return nil, fmt.Errorf("NBT root must be a compound tag, got %d", tagType)
 	}
 	
 	// 读取名称
 	name, err := readString(reader)
 	if err != nil {
 		return nil, err
 	}
 	
 	// 读取根复合标签
 	tag, err := readTag(reader, TagCompound)
 	if err != nil {
 		return nil, err
 	}
 	
 	return &NBT{
 		Name: name,
 		Root: tag,
 	}, nil
 }
 
 // Write 写入NBT到io.Writer
 func (nbt *NBT) Write(writer io.Writer) error {
 	buf := &bytes.Buffer{}
 	
 	// 写入根标签类型
 	if err := binary.Write(buf, binary.BigEndian, TagCompound); err != nil {
 		return err
 	}
 	
 	// 写入名称
 	if err := writeString(buf, nbt.Name); err != nil {
 		return err
 	}
 	
 	// 写入根标签
 	if err := writeTag(buf, nbt.Root); err != nil {
 		return err
 	}
 	
 	_, err := writer.Write(buf.Bytes())
 	return err
 }
 
 // WriteAnonymous 写入不带根名称的 NBT (用于网络协议中的 Slot Data)
 func (nbt *NBT) WriteAnonymous(writer io.Writer) error {
 	buf := &bytes.Buffer{}
 	
 	// 1. 写入根标签类型 (必须是 Compound)
 	if err := binary.Write(buf, binary.BigEndian, TagCompound); err != nil {
 		return err
 	}
 	
 	// 2. 跳过名称 (不写入任何名称字节)
 	
 	// 3. 直接写入根标签内容
 	if err := writeTag(buf, nbt.Root); err != nil {
 		return err
 	}
 	
 	_, err := writer.Write(buf.Bytes())
 	return err
 }
 
 // WriteAnonymousBytes 写入不带根名称的 NBT 到字节数组
 func (nbt *NBT) WriteAnonymousBytes() ([]byte, error) {
 	buf := &bytes.Buffer{}
 	err := nbt.WriteAnonymous(buf)
 	return buf.Bytes(), err
 }
 
 // ReadAnonymousBytes 从字节数组读取不带根名称的 NBT
 func ReadAnonymousBytes(data []byte) (*NBT, error) {
 	reader := bytes.NewReader(data)
 	
 	// 读取根标签类型
 	var tagType TagType
 	if err := binary.Read(reader, binary.BigEndian, &tagType); err != nil {
 		return nil, err
 	}
 	
 	if tagType != TagCompound {
 		return nil, fmt.Errorf("NBT root must be a compound tag, got %d", tagType)
 	}
 	
 	// 直接读取内容 (名称为空)
 	tag, err := readTag(reader, TagCompound)
 	if err != nil {
 		return nil, err
 	}
 	
 	return &NBT{
 		Name: "",
 		Root: tag,
 	}, nil
 }
 
 // WriteBytes 写入NBT到字节数组
 func (nbt *NBT) WriteBytes() ([]byte, error) {
 	buf := &bytes.Buffer{}
 	err := nbt.Write(buf)
 	return buf.Bytes(), err
 }
 
 // 读取标签
 func readTag(reader io.Reader, expectedType TagType) (Tag, error) {
 	switch expectedType {
 	case TagByte:
 		var value int8
 		err := binary.Read(reader, binary.BigEndian, &value)
 		return &ByteTag{Value: value}, err
 		
 	case TagShort:
 		var value int16
 		err := binary.Read(reader, binary.BigEndian, &value)
 		return &ShortTag{Value: value}, err
 		
 	case TagInt:
 		var value int32
 		err := binary.Read(reader, binary.BigEndian, &value)
 		return &IntTag{Value: value}, err
 		
 	case TagLong:
 		var value int64
 		err := binary.Read(reader, binary.BigEndian, &value)
 		return &LongTag{Value: value}, err
 		
 	case TagFloat:
 		var value float32
 		err := binary.Read(reader, binary.BigEndian, &value)
 		return &FloatTag{Value: value}, err
 		
 	case TagDouble:
 		var value float64
 		err := binary.Read(reader, binary.BigEndian, &value)
 		return &DoubleTag{Value: value}, err
 		
 	case TagByteArray:
 		length, err := readInt(reader)
 		if err != nil {
 			return nil, err
 		}
 		value := make([]byte, length)
 		_, err = io.ReadFull(reader, value)
 		return &ByteArrayTag{Value: value}, err
 		
 	case TagString:
 		value, err := readString(reader)
 		return &StringTag{Value: value}, err
 		
 	case TagList:
 		// 读取元素类型
 		var elemType TagType
 		if err := binary.Read(reader, binary.BigEndian, &elemType); err != nil {
 			return nil, err
 		}
 		
 		// 读取长度
 		length, err := readInt(reader)
 		if err != nil {
 			return nil, err
 		}
 		
 		// 读取元素
 		list := &ListTag{
 			TagType: elemType,
 			Value:   make([]Tag, length),
 		}
 		
 		for i := int32(0); i < length; i++ {
 			elem, err := readTag(reader, elemType)
 			if err != nil {
 				return nil, err
 			}
 			list.Value[i] = elem
 		}
 		
 		return list, nil
 		
 	case TagCompound:
 		compound := &CompoundTag{
 			Value: make(map[string]Tag),
 		}
 		
 		for {
 			// 读取下一个标签的类型
 			var nextTagType TagType
 			if err := binary.Read(reader, binary.BigEndian, &nextTagType); err != nil {
 				return nil, err
 			}
 			
 			if nextTagType == TagEnd {
 				break
 			}
 			
 			// 读取名称
 			name, err := readString(reader)
 			if err != nil {
 				return nil, err
 			}
 			
 			// 读取标签
 			tag, err := readTag(reader, nextTagType)
 			if err != nil {
 				return nil, err
 			}
 			
 			compound.Value[name] = tag
 		}
 		
 		return compound, nil
 		
 	case TagIntArray:
 		length, err := readInt(reader)
 		if err != nil {
 			return nil, err
 		}
 		
 		arr := &IntArrayTag{
 			Value: make([]int32, length),
 		}
 		
 		for i := int32(0); i < length; i++ {
 			value, err := readInt(reader)
 			if err != nil {
 				return nil, err
 			}
 			arr.Value[i] = value
 		}
 		
 		return arr, nil
 		
 	case TagLongArray:
 		length, err := readInt(reader)
 		if err != nil {
 			return nil, err
 		}
 		
 		arr := &LongArrayTag{
 			Value: make([]int64, length),
 		}
 		
 		for i := int32(0); i < length; i++ {
 			value, err := readLong(reader)
 			if err != nil {
 				return nil, err
 			}
 			arr.Value[i] = value
 		}
 		
 		return arr, nil
 		
 	case TagEnd:
 		return &EndTag{}, nil
 		
 	default:
 		return nil, fmt.Errorf("unknown tag type: %d", expectedType)
 	}
 }
 
 // 写入标签
 func writeTag(writer io.Writer, tag Tag) error {
 	switch t := tag.(type) {
 	case *ByteTag:
 		return binary.Write(writer, binary.BigEndian, t.Value)
 		
 	case *ShortTag:
 		return binary.Write(writer, binary.BigEndian, t.Value)
 		
 	case *IntTag:
 		return binary.Write(writer, binary.BigEndian, t.Value)
 		
 	case *LongTag:
 		return binary.Write(writer, binary.BigEndian, t.Value)
 		
 	case *FloatTag:
 		return binary.Write(writer, binary.BigEndian, t.Value)
 		
 	case *DoubleTag:
 		return binary.Write(writer, binary.BigEndian, t.Value)
 		
 	case *ByteArrayTag:
 		if err := writeInt(writer, int32(len(t.Value))); err != nil {
 			return err
 		}
 		_, err := writer.Write(t.Value)
 		return err
 		
 	case *StringTag:
 		return writeString(writer, t.Value)
 		
 	case *ListTag:
 		// 写入元素类型
 		if err := binary.Write(writer, binary.BigEndian, t.TagType); err != nil {
 			return err
 		}
 		// 写入长度
 		if err := writeInt(writer, int32(len(t.Value))); err != nil {
 			return err
 		}
 		// 写入元素
 		for _, elem := range t.Value {
 			if err := writeTag(writer, elem); err != nil {
 				return err
 			}
 		}
 		return nil
 		
 	case *CompoundTag:
 		for name, tag := range t.Value {
 			// 写入标签类型
 			if err := binary.Write(writer, binary.BigEndian, tag.Type()); err != nil {
 				return err
 			}
 			// 写入名称
 			if err := writeString(writer, name); err != nil {
 				return err
 			}
 			// 写入标签
 			if err := writeTag(writer, tag); err != nil {
 				return err
 			}
 		}
 		// 写入结束标记
 		return binary.Write(writer, binary.BigEndian, TagEnd)
 		
 	case *IntArrayTag:
 		if err := writeInt(writer, int32(len(t.Value))); err != nil {
 			return err
 		}
 		for _, value := range t.Value {
 			if err := writeInt(writer, value); err != nil {
 				return err
 			}
 		}
 		return nil
 		
 	case *LongArrayTag:
 		if err := writeInt(writer, int32(len(t.Value))); err != nil {
 			return err
 		}
 		for _, value := range t.Value {
 			if err := writeLong(writer, value); err != nil {
 				return err
 			}
 		}
 		return nil
 		
 	case *EndTag:
 		return nil // 不需要写入任何数据
 		
 	default:
 		return fmt.Errorf("unknown tag type: %T", tag)
 	}
 }
 
 // 读取字符串（Minecraft格式：长度前缀 + UTF-8字符串）
 func readString(reader io.Reader) (string, error) {
 	length, err := readUShort(reader)
 	if err != nil {
 		return "", err
 	}
 	
 	if length == 0 {
 		return "", nil
 	}
 	
 	bytes := make([]byte, length)
 	_, err = io.ReadFull(reader, bytes)
 	if err != nil {
 		return "", err
 	}
 	
 	return string(bytes), nil
 }
 
 // 写入字符串
 func writeString(writer io.Writer, value string) error {
 	if err := writeUShort(writer, uint16(len(value))); err != nil {
 		return err
 	}
 	_, err := writer.Write([]byte(value))
 	return err
 }
 
 // 读取Int
 func readInt(reader io.Reader) (int32, error) {
 	var value int32
 	err := binary.Read(reader, binary.BigEndian, &value)
 	return value, err
 }
 
 // 写入Int
 func writeInt(writer io.Writer, value int32) error {
 	return binary.Write(writer, binary.BigEndian, value)
 }
 
 // 读取Long
 func readLong(reader io.Reader) (int64, error) {
 	var value int64
 	err := binary.Read(reader, binary.BigEndian, &value)
 	return value, err
 }
 
 // 写入Long
 func writeLong(writer io.Writer, value int64) error {
 	return binary.Write(writer, binary.BigEndian, value)
 }
 
 // 读取无符号短整型
 func readUShort(reader io.Reader) (uint16, error) {
 	var value uint16
 	err := binary.Read(reader, binary.BigEndian, &value)
 	return value, err
 }
 
 // 写入无符号短整型
 func writeUShort(writer io.Writer, value uint16) error {
 	return binary.Write(writer, binary.BigEndian, value)
 }
 
 // NewCompoundTag 创建新的复合标签
 func NewCompoundTag() *CompoundTag {
 	return &CompoundTag{
 		Value: make(map[string]Tag),
 	}
 }
 
 // NewListTag 创建新的列表标签
 func NewListTag(elemType TagType) *ListTag {
 	return &ListTag{
 		TagType: elemType,
 		Value:   make([]Tag, 0),
 	}
 }
 
 // Get 从CompoundTag获取子标签
 func (t *CompoundTag) Get(name string) (Tag, bool) {
 	tag, ok := t.Value[name]
 	return tag, ok
 }
 
 // Set 向CompoundTag设置子标签
 func (t *CompoundTag) Set(name string, tag Tag) {
 	t.Value[name] = tag
 }
 
 // GetByte 从CompoundTag获取字节值
 func (t *CompoundTag) GetByte(name string) (int8, bool) {
 	if tag, ok := t.Get(name); ok {
 		if byteTag, ok := tag.(*ByteTag); ok {
 			return byteTag.Value, true
 		}
 	}
 	return 0, false
 }
 
 // GetInt 从CompoundTag获取整型值
 func (t *CompoundTag) GetInt(name string) (int32, bool) {
 	if tag, ok := t.Get(name); ok {
 		if intTag, ok := tag.(*IntTag); ok {
 			return intTag.Value, true
 		}
 	}
 	return 0, false
 }
 
 // GetString 从CompoundTag获取字符串值
 func (t *CompoundTag) GetString(name string) (string, bool) {
 	if tag, ok := t.Get(name); ok {
 		if stringTag, ok := tag.(*StringTag); ok {
 			return stringTag.Value, true
 		}
 	}
 	return "", false
 }
 
 // GetCompound 从CompoundTag获取子复合标签
 func (t *CompoundTag) GetCompound(name string) (*CompoundTag, bool) {
 	if tag, ok := t.Get(name); ok {
 		if compoundTag, ok := tag.(*CompoundTag); ok {
 			return compoundTag, true
 		}
 	}
 	return nil, false
 }
 
 // GetList 从CompoundTag获取列表标签
 func (t *CompoundTag) GetList(name string) (*ListTag, bool) {
 	if tag, ok := t.Get(name); ok {
 		if listTag, ok := tag.(*ListTag); ok {
 			return listTag, true
 		}
 	}
 	return nil, false
 }
 
 // GetShort 从CompoundTag获取短整型值
 func (t *CompoundTag) GetShort(name string) (int16, bool) {
 	if tag, ok := t.Get(name); ok {
 		if shortTag, ok := tag.(*ShortTag); ok {
 			return shortTag.Value, true
 		}
 	}
 	return 0, false
 }
 
 // Append 向ListTag添加元素
 func (t *ListTag) Append(tag Tag) {
 	t.Value = append(t.Value, tag)
 }
