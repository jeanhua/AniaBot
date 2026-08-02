package telegram

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// markdownToTelegramHTML 把 AI 输出的常见 Markdown 转换为 Telegram HTML
// （parse_mode=HTML 的 text）。Telegram 原生 Markdown/MarkdownV2 对 AI 输出
// 几乎必然解析失败（未转义特殊字符/未配对标记 → 400 降级纯文本，消息最终
// 只能以纯文本展示），HTML 模式只需转义 & < > 且标签由本函数生成——
// 对任意输入都产出合法 HTML，未配对/不支持的标记原样保留为文本，不会触发 400。
//
// 支持的构造：```围栏代码块```、`行内代码`、**加粗**/__加粗__、*斜体*/_斜体_
// （词中下划线不解析）、~~删除线~~、[链接](url)、# 标题（转加粗）、> 引用。
func markdownToTelegramHTML(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + len(s)/4)
	writeMarkdownBlocks(&sb, s)
	return sb.String()
}

// writeMarkdownBlocks 按行处理块级结构：围栏代码块 / 标题 / 引用 / 普通行。
func writeMarkdownBlocks(sb *strings.Builder, s string) {
	lines := strings.Split(s, "\n")
	first := true
	// newline 在相邻输出单元之间补换行（代码块/引用块跨行消费，不能按行简单拼接）
	newline := func() {
		if !first {
			sb.WriteByte('\n')
		}
		first = false
	}
	i := 0
	for i < len(lines) {
		line := lines[i]
		// 围栏代码块：```lang 开始、``` 结束；无闭合围栏时开始行按普通文本处理
		if lang, ok := fenceOpen(line); ok {
			end := i + 1
			for end < len(lines) && !isFenceClose(lines[end]) {
				end++
			}
			if end < len(lines) {
				newline()
				writeCodeBlock(sb, lang, lines[i+1:end])
				i = end + 1
				continue
			}
		}
		// 引用块：连续 > 行合并为一个 blockquote
		if _, ok := quoteText(line); ok {
			newline()
			sb.WriteString("<blockquote>")
			firstLine := true
			for i < len(lines) {
				rest, ok2 := quoteText(lines[i])
				if !ok2 {
					break
				}
				if !firstLine {
					sb.WriteByte('\n')
				}
				firstLine = false
				writeInline(sb, rest, 0)
				i++
			}
			sb.WriteString("</blockquote>")
			continue
		}
		// ATX 标题（# ~ ###### + 空格）：Telegram 无标题，转加粗
		if rest, ok := headerText(line); ok {
			newline()
			sb.WriteString("<b>")
			writeInline(sb, rest, 0)
			sb.WriteString("</b>")
			i++
			continue
		}
		newline()
		writeInline(sb, line, 0)
		i++
	}
}

// fenceOpen 判断围栏代码块开始行（``` 或 ```lang），返回语言标识。
// 语言标识只保留字母数字与 +-#（防止注入 class 属性），其余整行按普通文本处理。
func fenceOpen(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "```") || strings.HasPrefix(t, "````") {
		return "", false
	}
	lang := t[3:]
	for _, r := range lang {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '-' && r != '#' {
			return "", false
		}
	}
	return lang, true
}

// isFenceClose 判断围栏代码块闭合行（```）。
func isFenceClose(line string) bool {
	return strings.TrimSpace(line) == "```"
}

// writeCodeBlock 输出围栏代码块：<pre><code class="language-lang">内容</code></pre>。
func writeCodeBlock(sb *strings.Builder, lang string, codeLines []string) {
	sb.WriteString("<pre><code")
	if lang != "" {
		sb.WriteString(` class="language-`)
		sb.WriteString(lang)
		sb.WriteByte('"')
	}
	sb.WriteByte('>')
	writeEscaped(sb, strings.Join(codeLines, "\n"))
	sb.WriteString("</code></pre>")
}

// quoteText 判断引用行并返回去掉 > 前缀的内容。
func quoteText(line string) (string, bool) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, ">") {
		return "", false
	}
	return strings.TrimPrefix(t[1:], " "), true
}

// headerText 判断 ATX 标题行（1~6 个 # + 空格），返回标题文本。
func headerText(line string) (string, bool) {
	t := strings.TrimLeft(line, " \t")
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(t) || t[n] != ' ' {
		return "", false
	}
	return strings.TrimSpace(t[n+1:]), true
}

// maxInlineDepth 行内格式嵌套上限（防御病态输入的递归深度）。
const maxInlineDepth = 8

// writeInline 处理行内 Markdown：行内代码、链接、加粗、斜体、删除线。
// 未配对标记原样保留（转义后输出），保证输出恒为合法 HTML。
func writeInline(sb *strings.Builder, s string, depth int) {
	if depth > maxInlineDepth {
		writeEscaped(sb, s)
		return
	}
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '`':
			if end := strings.IndexByte(s[i+1:], '`'); end > 0 {
				sb.WriteString("<code>")
				writeEscaped(sb, s[i+1:i+1+end])
				sb.WriteString("</code>")
				i += end + 2
				continue
			}
			writeEscapedChar(sb, c)
			i++
		case c == '[':
			if text, url, n, ok := parseLink(s[i:]); ok {
				sb.WriteString(`<a href="`)
				writeEscapedAttr(sb, url)
				sb.WriteString(`">`)
				writeInline(sb, text, depth+1)
				sb.WriteString("</a>")
				i += n
				continue
			}
			writeEscapedChar(sb, c)
			i++
		case c == '*' || c == '_':
			double := i+1 < len(s) && s[i+1] == c
			delim := s[i : i+1]
			if double {
				delim = s[i : i+2]
			}
			// 下划线类标记要求左侧非字母数字（词中下划线不解析，如 snake_case）
			if c == '_' && i > 0 && isAlnumByte(s[i-1]) {
				writeEscapedChar(sb, c)
				i++
				continue
			}
			tag := "i"
			if double {
				tag = "b"
			}
			if inner, n, ok := parseSpan(s[i+len(delim):], delim, c == '_'); ok {
				sb.WriteByte('<')
				sb.WriteString(tag)
				sb.WriteByte('>')
				writeInline(sb, inner, depth+1)
				sb.WriteString("</")
				sb.WriteString(tag)
				sb.WriteByte('>')
				i += len(delim) + n
				continue
			}
			// 未配对：标记字符原样输出
			writeEscaped(sb, delim)
			i += len(delim)
		case c == '~':
			if i+1 < len(s) && s[i+1] == '~' {
				if inner, n, ok := parseSpan(s[i+2:], "~~", false); ok {
					sb.WriteString("<s>")
					writeInline(sb, inner, depth+1)
					sb.WriteString("</s>")
					i += 2 + n
					continue
				}
				writeEscaped(sb, "~~")
				i += 2
				continue
			}
			writeEscapedChar(sb, c)
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r < utf8.RuneSelf {
				writeEscapedChar(sb, c)
			} else {
				sb.WriteRune(r)
			}
			i += size
		}
	}
}

// parseSpan 在 s 中查找定界符 delim 的闭合位置（行内格式 * _ ** __ ~~）。
// 内容不能为空、首尾不能是空白；下划线类定界符（wordBound）要求闭合后
// 不是字母数字（词边界）。返回内容与其后闭合符的总消耗长度。
func parseSpan(s, delim string, wordBound bool) (string, int, bool) {
	search := 0
	for search <= len(s) {
		idx := strings.Index(s[search:], delim)
		if idx < 0 {
			return "", 0, false
		}
		end := search + idx
		inner := s[:end]
		if inner != "" && !isSpaceByte(inner[0]) && !isSpaceByte(inner[len(inner)-1]) {
			after := end + len(delim)
			if !wordBound || after >= len(s) || !isAlnumByte(s[after]) {
				return inner, after, true
			}
		}
		search = end + len(delim)
	}
	return "", 0, false
}

// parseLink 解析 [text](url)（s[0] 为 '['）：返回文本、URL 与总消耗长度。
// URL 需带合法 scheme（防 Telegram 400 "unsupported URL protocol"），
// 否则整体按普通文本处理。
func parseLink(s string) (string, string, int, bool) {
	closeIdx := strings.Index(s, "](")
	if closeIdx <= 1 {
		return "", "", 0, false
	}
	rest := s[closeIdx+2:]
	endIdx := strings.IndexByte(rest, ')')
	if endIdx <= 0 {
		return "", "", 0, false
	}
	url := rest[:endIdx]
	if !validLinkURL(url) {
		return "", "", 0, false
	}
	return s[1:closeIdx], url, closeIdx + 2 + endIdx + 1, true
}

// validLinkURL 校验链接 URL：scheme:... 形式（字母开头的 scheme），
// 不含空白与控制字符。
func validLinkURL(u string) bool {
	scheme, rest, ok := strings.Cut(u, ":")
	if !ok || rest == "" || scheme == "" || !isAlphaByte(scheme[0]) {
		return false
	}
	for i := 1; i < len(scheme); i++ {
		if c := scheme[i]; !isAlnumByte(c) && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	for i := 0; i < len(u); i++ {
		if u[i] <= ' ' {
			return false
		}
	}
	return true
}

// writeEscaped 输出转义 & < > 的文本（HTML 文本节点安全；多字节字符按字节
// 透传，UTF-8 非 ASCII 字节不会与 & < > 冲突）。
func writeEscaped(sb *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		writeEscapedChar(sb, s[i])
	}
}

func writeEscapedChar(sb *strings.Builder, c byte) {
	switch c {
	case '&':
		sb.WriteString("&amp;")
	case '<':
		sb.WriteString("&lt;")
	case '>':
		sb.WriteString("&gt;")
	default:
		sb.WriteByte(c)
	}
}

// writeEscapedAttr 输出转义属性值（href 等）：在文本转义基础上额外转义双引号。
func writeEscapedAttr(sb *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			sb.WriteString("&quot;")
		} else {
			writeEscapedChar(sb, s[i])
		}
	}
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func isAlnumByte(c byte) bool { return isAlphaByte(c) || c >= '0' && c <= '9' }
func isAlphaByte(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
