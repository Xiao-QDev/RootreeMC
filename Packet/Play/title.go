// Package Play 标题包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/json"
)

// TitleAction 标题动作
const (
	TitleActionSetTitle    = 0 // 设置标题文本
	TitleActionSetSubtitle = 1 // 设置副标题文本
	TitleActionSetTimes    = 2 // 设置显示时间
	TitleActionHide        = 3 // 隐藏标题
	TitleActionReset       = 4 // 重置为默认
)

// BuildTitle 设置主标题
func BuildTitle(text string) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x48) // 1.12.2 Title ID: 0x48
	Network.WriteVarint(buf, TitleActionSetTitle)

	// 1.12.2 标题必须是 JSON 格式
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	Protocol.WriteString(buf, string(jsonBytes))

	return Protocol.AddLengthPrefix(buf)
}

// BuildSubtitle 设置副标题
func BuildSubtitle(text string) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x48)
	Network.WriteVarint(buf, TitleActionSetSubtitle)

	// 1.12.2 标题必须是 JSON 格式
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	Protocol.WriteString(buf, string(jsonBytes))

	return Protocol.AddLengthPrefix(buf)
}

// BuildTitleTimes 设置显示时间
// fadeIn: 淡入时间（刻）
// stay: 停留时间（刻）
// fadeOut: 淡出时间（刻）
func BuildTitleTimes(fadeIn, stay, fadeOut int32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x48)
	Network.WriteVarint(buf, TitleActionSetTimes)
	Protocol.WriteInt(buf, fadeIn)
	Protocol.WriteInt(buf, stay)
	Protocol.WriteInt(buf, fadeOut)
	return Protocol.AddLengthPrefix(buf)
}

// BuildTitleHide 隐藏标题
func BuildTitleHide() []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x48)
	Network.WriteVarint(buf, TitleActionHide)
	return Protocol.AddLengthPrefix(buf)
}

// BuildTitleReset 重置标题
func BuildTitleReset() []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x48)
	Network.WriteVarint(buf, TitleActionReset)
	return Protocol.AddLengthPrefix(buf)
}
