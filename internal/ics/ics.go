// Package ics 生成 iCalendar(.ics) 文件，格式与 Bugaoshan 的导出兼容
//（VTIMEZONE 固定 Asia/Shanghai +0800，UID 沿用 @bugaoshan 域以便日历应用去重）。
package ics

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Geo 是结构化地理位置（校区坐标）。
type Geo struct {
	Lat float64
	Lng float64
}

// Event 是一条日历事件。普通事件使用 DTSTART;TZID=Asia/Shanghai:yyyyMMddTHHmm00；
// 全天事件（AllDay）使用 DTSTART;VALUE=DATE:yyyyMMdd，End 为不含尾日的下一日。
type Event struct {
	Title       string
	Location    string
	Description string
	UID         string
	Start       time.Time
	End         time.Time
	Geo         *Geo
	AllDay      bool
}

// shanghai 是固定的 +08:00 时区（中国无夏令时），避免依赖时区数据库。
var shanghai = time.FixedZone("Asia/Shanghai", 8*3600)

// Shanghai 返回固定 +08:00 时区，供调用方构造事件时间。
func Shanghai() *time.Location { return shanghai }

const vtimezoneBlock = `BEGIN:VTIMEZONE
TZID:Asia/Shanghai
BEGIN:STANDARD
TZOFFSETFROM:+0800
TZOFFSETTO:+0800
TZNAME:CST
DTSTART:19700101T000000
RRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=3
END:STANDARD
BEGIN:DAYLIGHT
TZOFFSETFROM:+0800
TZOFFSETTO:+0800
TZNAME:CST
DTSTART:19700101T000000
RRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=11
END:DAYLIGHT
END:VTIMEZONE
`

// Build 生成完整 ICS 文本（CRLF 行尾，RFC 5545）。
func Build(productName string, events []Event) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	fmt.Fprintf(&b, "PRODID:-//SCU-CLI//%s//EN\r\n", productName)
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("X-WR-TIMEZONE:Asia/Shanghai\r\n")
	b.WriteString(strings.ReplaceAll(vtimezoneBlock, "\n", "\r\n"))
	for _, e := range events {
		writeEvent(&b, e)
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func writeEvent(b *strings.Builder, e Event) {
	b.WriteString("BEGIN:VEVENT\r\n")
	if e.AllDay {
		fmt.Fprintf(b, "DTSTART;VALUE=DATE:%s\r\n", e.Start.Format("20060102"))
		fmt.Fprintf(b, "DTEND;VALUE=DATE:%s\r\n", e.End.Format("20060102"))
	} else {
		fmt.Fprintf(b, "DTSTART;TZID=Asia/Shanghai:%s\r\n", formatLocal(e.Start))
		fmt.Fprintf(b, "DTEND;TZID=Asia/Shanghai:%s\r\n", formatLocal(e.End))
	}
	fmt.Fprintf(b, "SUMMARY:%s\r\n", EscapeText(e.Title))
	if e.Location != "" {
		fmt.Fprintf(b, "LOCATION:%s\r\n", EscapeText(e.Location))
	}
	if e.Geo != nil {
		fmt.Fprintf(b, "GEO:%v;%v\r\n", e.Geo.Lat, e.Geo.Lng)
	}
	if e.Description != "" {
		fmt.Fprintf(b, "DESCRIPTION:%s\r\n", EscapeText(e.Description))
	}
	fmt.Fprintf(b, "UID:%s\r\n", e.UID)
	b.WriteString("END:VEVENT\r\n")
}

// formatLocal 以本地墙钟格式输出（秒固定 00，与 Bugaoshan 一致）。
func formatLocal(t time.Time) string {
	t = t.In(shanghai)
	return fmt.Sprintf("%04d%02d%02dT%02d%02d00", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
}

// EscapeText 按 RFC 5545 转义文本值（反斜杠优先）。
func EscapeText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, ";", `\;`)
	return s
}

var (
	safeNameRe       = regexp.MustCompile(`[^\w一-鿿.]`)
	safeNameHyphenRe = regexp.MustCompile(`[^\w一-鿿.-]`)
)

// SafeFileName 将任意字符串转为安全文件名（保留中英文、数字、点；可选保留连字符）。
func SafeFileName(s string, allowHyphen bool) string {
	re := safeNameRe
	if allowHyphen {
		re = safeNameHyphenRe
	}
	out := re.ReplaceAllString(s, "_")
	if out == "" {
		return "calendar"
	}
	return out
}

// StableUID 从任意种子字符串生成稳定 UID（sha1 前 24 位十六进制）。
func StableUID(seed, domain string) string {
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:24] + "@" + domain
}
