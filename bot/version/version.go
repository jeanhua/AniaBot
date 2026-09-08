// Package version 提供 AniaBot 的构建版本号。
//
// 版本号在 CI 构建时通过 -ldflags "-X 注入"（release workflow 用 git tag，
// docker workflow 无 tag 时用短 commit SHA）；go run / 本地直接编译不注入，
// 保持默认值 dev。面板「自动更新」在容器内重新编译时会把当前运行版本透传下去。
package version

// ldflagsPath -X 注入的目标变量路径（各构建入口共用，避免路径写散）。
const ldflagsPath = "github.com/jeanhua/AniaBot/bot/version.Version"

// Version 当前版本号。CI 打 tag 构建注入形如 v4.6.6；docker 手动触发
// （容器滚动更新拉 latest）注入短 commit SHA；未注入时为 dev。
var Version = "dev"

// UserAgent 返回 LLM API 请求使用的 UA 标识（如 "AniaBot/v4.6.6"），
// 便于上游识别请求来源。
func UserAgent() string {
	return "AniaBot/" + Version
}

// Ldflags 返回注入当前版本号的 ldflags 片段（形如 "-s -w -X <path>=<version>"），
// 供 CI workflow 与面板自动更新编译时拼接，保证重新编译不丢失版本标识。
func Ldflags(extra string) string {
	if extra == "" {
		return "-X " + ldflagsPath + "=" + Version
	}
	return extra + " -X " + ldflagsPath + "=" + Version
}
