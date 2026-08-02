package telegram

import "testing"

// TestMarkdownToTelegramHTML 转换器主路径：各构造的转换结果。
func TestMarkdownToTelegramHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"纯文本不变", "你好，世界", "你好，世界"},
		{"转义特殊字符", "a < b & c > d", "a &lt; b &amp; c &gt; d"},
		{"加粗", "**加粗**文本", "<b>加粗</b>文本"},
		{"下划线加粗", "__加粗__", "<b>加粗</b>"},
		{"斜体", "*斜体* 文本", "<i>斜体</i> 文本"},
		{"下划线斜体", "这是 _斜体_ 哦", "这是 <i>斜体</i> 哦"},
		{"词中下划线不解析", "snake_case_name 变量", "snake_case_name 变量"},
		{"星号运算不解析", "2 * 3 = 6", "2 * 3 = 6"},
		{"未配对加粗保留", "**未闭合", "**未闭合"},
		{"未配对星号保留", "a * b", "a * b"},
		{"删除线", "~~删除~~", "<s>删除</s>"},
		{"行内代码", "使用 `fmt.Println` 输出", "使用 <code>fmt.Println</code> 输出"},
		{"行内代码转义", "`a < b`", "<code>a &lt; b</code>"},
		{"链接", "[官网](https://example.com)", `<a href="https://example.com">官网</a>`},
		{"链接含加粗文本", "[**重要**链接](https://example.com)", `<a href="https://example.com"><b>重要</b>链接</a>`},
		{"非法协议链接保留", "[文件](file 路径)", "[文件](file 路径)"},
		{"无协议链接保留", "[页面](example.com)", "[页面](example.com)"},
		{"标题一级", "# 大标题", "<b>大标题</b>"},
		{"标题三级", "### 小节", "<b>小节</b>"},
		{"标题内需空格", "#不是标题", "#不是标题"},
		{"引用", "> 引用内容", "<blockquote>引用内容</blockquote>"},
		{"多行引用合并", "> 第一行\n> 第二行", "<blockquote>第一行\n第二行</blockquote>"},
		{"行内大于号非引用", "a > b", "a &gt; b"},
		{"嵌套格式", "**加粗与 `代码`**", "<b>加粗与 <code>代码</code></b>"},
		{"围栏代码块", "```go\nfmt.Println(\"hi\")\n```", "<pre><code class=\"language-go\">fmt.Println(\"hi\")</code></pre>"},
		{"围栏代码块无语言", "```\ncode\n```", "<pre><code>code</code></pre>"},
		{"代码块内不解析标记", "```\n**不是加粗** <tag>\n```", "<pre><code>**不是加粗** &lt;tag&gt;</code></pre>"},
		{"围栏未闭合按文本", "```go\nfmt.Println()", "```go\nfmt.Println()"},
		{"多行混合", "# 标题\n\n普通 **加粗** 文本", "<b>标题</b>\n\n普通 <b>加粗</b> 文本"},
		{"列表原样保留", "- 项目一\n- 项目二", "- 项目一\n- 项目二"},
		{"编号列表原样保留", "1. 第一\n2. 第二", "1. 第一\n2. 第二"},
		{"MarkdownV2 敏感字符无需转义", "好的。没问题！(真的) 1+1=2", "好的。没问题！(真的) 1+1=2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := markdownToTelegramHTML(c.in); got != c.want {
				t.Fatalf("markdownToTelegramHTML(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

// TestMarkdownToTelegramHTMLTypicalAIOutput 典型 AI 回复整体转换：
// 含标题/列表/加粗/代码块的完整段落。
func TestMarkdownToTelegramHTMLTypicalAIOutput(t *testing.T) {
	in := "## 答案\n\n当然可以！步骤如下：\n\n1. 打开 **设置** 页面\n2. 点击 `保存` 按钮\n\n```python\nprint(\"hello\")\n```\n\n> 注意：详见 [文档](https://example.com/docs)。"
	want := "<b>答案</b>\n\n当然可以！步骤如下：\n\n1. 打开 <b>设置</b> 页面\n2. 点击 <code>保存</code> 按钮\n\n<pre><code class=\"language-python\">print(\"hello\")</code></pre>\n\n<blockquote>注意：详见 <a href=\"https://example.com/docs\">文档</a>。</blockquote>"
	if got := markdownToTelegramHTML(in); got != want {
		t.Fatalf("整体转换不符:\n got: %q\nwant: %q", got, want)
	}
}
