package channeltype

var ChannelBaseURLs = []string{
	"",                              // 0
	"https://api.openai.com",        // 1
	"https://oa.api2d.net",          // 2
	"",                              // 3
	"https://api.closeai-proxy.xyz", // 4
	"https://api.openai-sb.com",     // 5
	"https://api.openaimax.com",     // 6
	"https://api.ohmygpt.com",       // 7
	"",                              // 8
	"https://api.caipacity.com",     // 9
	"https://api.aiproxy.io",        // 10
	"https://generativelanguage.googleapis.com", // 11
	"https://api.api2gpt.com",                   // 12
	"https://api.aigc2d.com",                    // 13
	"https://api.anthropic.com",                 // 14
	"https://aip.baidubce.com",                  // 15
	"https://open.bigmodel.cn",                  // 16
	"https://dashscope.aliyuncs.com",            // 17
	"",                                          // 18
	"https://ai.360.cn",                         // 19
	"https://openrouter.ai/api",                 // 20
	"https://api.aiproxy.io",                    // 21
	"https://fastgpt.run/api/openapi",           // 22
	"https://hunyuan.tencentcloudapi.com",       // 23
	"https://generativelanguage.googleapis.com", // 24
	"https://api.moonshot.cn",                   // 25
	"https://api.baichuan-ai.com",               // 26
	"https://api.minimax.chat",                  // 27
	"https://api.mistral.ai",                    // 28
	"https://api.groq.com/openai",               // 29
	"http://localhost:11434",                    // 30
	"https://api.lingyiwanwu.com",               // 31
	"https://api.stepfun.com",                   // 32
	"",                                          // 33
	"https://api.coze.com",                      // 34
	"https://api.cohere.ai",                     // 35
	"https://api.deepseek.com",                  // 36
	"https://api.cloudflare.com",                // 37
	"https://api-free.deepl.com",                // 38
	"https://api.together.xyz",                  // 39
	"https://ark.cn-beijing.volces.com",         // 40
	"https://api.novita.ai/v3/openai",           // 41
	"",                                          // 42
	"",                                          // 43
	"https://api.siliconflow.cn",                // 44
	// 45 Seedance：只到 host。/api/v3 由入站路径带上来，
	// 因此渠道里填自定义 BaseURL 时也只能填到 host，多写 /api/v3 会拼成双份。
	"https://ark.ap-southeast.bytepluses.com", //
	"https://api.sagemaker.endpoint",          // 46
	// 47 Wan3：**故意留空**。百炼的端点是按 Workspace 与 Region 拼的
	// （https://{WorkspaceId}.{region}.maas.aliyuncs.com），没有一个能通用的默认值。
	// 留空 ⇒ 管理员不填 BaseURL 时 GetRequestURL 会直接报错，而不是把请求发到
	// dashscope.aliyuncs.com 上收到一个看不懂的 404。
	"", // 47
}

func init() {
	if len(ChannelBaseURLs) != Dummy {
		panic("channel base urls length not match")
	}
}
