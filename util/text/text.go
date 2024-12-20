package text

import (
	"unicode"
	"unicode/utf16"
)

const TruncateEllipsis = " …"

func TruncateEllipsized(text string, length int) string {
	return Truncate(text, length, TruncateEllipsis)
}

func Truncate(text string, length int, ending string) string {
	length -= UTF16RuneCountString(ending)
	if UTF16RuneCountString(text) <= length {
		return text
	}
	utf16Text := StrToUTF16(text)
	var lastWordIndex, lastNonSpace, currentLen, endTextPos int
	for i, r := range utf16Text {
		currentLen++
		if unicode.IsSpace(rune(r)) {
			lastWordIndex = lastNonSpace
		} else if unicode.In(rune(r), unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana) {
			lastWordIndex = i
		} else {
			lastNonSpace = i + 1
		}
		if currentLen > length {
			if lastWordIndex == 0 {
				endTextPos = i
			} else {
				endTextPos = lastWordIndex
			}
			out := utf16Text[0:endTextPos]
			return UTF16ToStr(out) + ending
		}
	}
	return UTF16ToStr(utf16Text)
}

func UTF16RuneCountString(str string) int {
	buf := make([]uint16, 0, 2)
	var n int
	for _, s := range str {
		buf = utf16.AppendRune(buf, s)
		n += len(buf)
		buf = buf[:0]
	}
	return n
}

func UTF16RuneCount(bStr []byte) int {
	str := string(bStr)
	return UTF16RuneCountString(str)
}

func StrToUTF16(str string) []uint16 {
	return utf16.Encode([]rune(str))
}

func UTF16ToStr(b []uint16) string {
	return string(utf16.Decode(b))
}
