import React from 'react';
import { Container, Button, Stack, Typography, TableContainer, Table, TableHead, TableBody, TableRow, TableCell, Paper, Tab, Tabs, Box } from '@mui/material';
import { showError, showNotice } from 'utils/common';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';


const allModels = [{
    'name': 'gpt-4',
    'inputTokensCost': 0.03,
    'outputTokensCost': 0.06,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-0314',
    'inputTokensCost': 0.03,
    'outputTokensCost': 0.06,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-0613',
    'inputTokensCost': 0.03,
    'outputTokensCost': 0.06,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-32k-0314',
    'inputTokensCost': 0.06,
    'outputTokensCost': 0.12,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-1106-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-0125-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-turbo-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-turbo',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-turbo-2024-04-09',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4-vision-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4o',
    'inputTokensCost': 0.005,
    'outputTokensCost': 0.015,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4o-2024-05-13',
    'inputTokensCost': 0.005,
    'outputTokensCost': 0.015,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4o-mini',
    'inputTokensCost': 0.00015,
    'outputTokensCost': 0.0006,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-4o-mini-2024-07-18',
    'inputTokensCost': 0.00015,
    'outputTokensCost': 0.0006,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-3.5-turbo',
    'inputTokensCost': 0.0005,
    'outputTokensCost': 0.0015,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},
{
    'name': 'gpt-3.5-turbo-16k',
    'inputTokensCost': 0.003,
    'outputTokensCost': 0.004,
    'Platform': 'OpenAI',
    'Type': 'Chat'
},

{
    'name': 'gpt-4o-2024-08-06',
    'inputTokensCost': 0.025,
    'outputTokensCost': 0,
    'Platform': 'OpenAI',
    'Type': 'Fine-Tuning'
},
{
    'name': 'gpt-4o-mini-2024-07-18',
    'inputTokensCost': 0.003,
    'outputTokensCost': 0,
    'Platform': 'OpenAI',
    'Type': 'Fine-Tuning'
},
{
    'name': 'gpt-3.5-turbo',
    'inputTokensCost': 0.008,
    'outputTokensCost': 0,
    'Platform': 'OpenAI',
    'Type': 'Fine-Tuning'
},
{
    'name': 'tts-1',
    'inputTokensCost': 0.015,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
},
{
    'name': 'tts-1-1106',
    'inputTokensCost': 0.015,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
},
{
    'name': 'tts-1-hd',
    'inputTokensCost': 0.03,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
},
{
    'name': 'tts-1-hd-1106',
    'inputTokensCost': 0.03,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
},
{
    'name': 'whisper-1',
    'inputTokensCost': "0.006/minute",
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'STT'
},
{
    'name': 'text-embedding-ada-002',
    'inputTokensCost': 0.0001,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Embedding'
},
{
    'name': 'text-embedding-3-small',
    'inputTokensCost': 2e-05,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Embedding'
},
{
    'name': 'text-embedding-3-large',
    'inputTokensCost': 0.00013,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Embedding'
},
{
    'name': 'dall-e-2',
    'inputTokensCost': 0.02,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Image'
},
{
    'name': 'dall-e-3',
    'inputTokensCost': 0.04,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Image'
},
{
    'name': 'aws-claude3:haiku-20240307',
    'inputTokensCost': 0.00025,
    'outputTokensCost': 0.00125,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-claude3.5:haiku-20241022',
    'inputTokensCost': 0.001,
    'outputTokensCost': 0.005,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-claude3:sonnet-20240229',
    'inputTokensCost': 0.003,
    'outputTokensCost': 0.015,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-claude3.5:sonnet-20240620',
    'inputTokensCost': 0.003,
    'outputTokensCost': 0.015,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-claude3.5:sonnet-20241022',
    'inputTokensCost': 0.003,
    'outputTokensCost': 0.015,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-claude3:opus-20240229',
    'inputTokensCost': 0.015,
    'outputTokensCost': 0.075,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3:8b',
    'inputTokensCost': 0.0003,
    'outputTokensCost': 0.0006,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3:70b',
    'inputTokensCost': 0.00265,
    'outputTokensCost': 0.0035,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.1:8b',
    'inputTokensCost': 0.00022,
    'outputTokensCost': 0.00022,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.1:70b',
    'inputTokensCost': 0.00099,
    'outputTokensCost': 0.00099,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.1:405b',
    'inputTokensCost': 0.00532,
    'outputTokensCost': 0.016,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.2:1b',
    'inputTokensCost': 0.0001,
    'outputTokensCost': 0.0001,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.2:3b',
    'inputTokensCost': 0.00015,
    'outputTokensCost': 0.00015,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.2:11b',
    'inputTokensCost': 0.00035,
    'outputTokensCost': 0.00035,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'aws-llama3.2:90b',
    'inputTokensCost': 0.002,
    'outputTokensCost': 0.002,
    'Platform': 'AWS',
    'Type': 'Chat'
},
{
    'name': 'ollama-gemma2:9b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-gemma2:27b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3:8b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3:8b_lora',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3:70b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3.1:8b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3.1:70b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3.2:1b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llama3.2:3b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llava1.6:7b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llava1.6:13b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-llava1.6:34b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-qwen2.5:3b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-qwen2.5:7b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-qwen2.5:14b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-qwen2.5:32b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-qwen2.5-coder:7b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-granite3-dense:2b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
},
{
    'name': 'ollama-granite3-dense:8b',
    'inputTokensCost': 0.0,
    'outputTokensCost': 0.0,
    'Platform': 'Ollama',
    'Type': 'Chat'
}]

const curlCode = `curl http://proxy.aiapps.autel.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 你的令牌" \
  -d '{
    "model": "aws-claude3.5:haiku-20241022",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Hello!"
      }
    ]
  }'`

const pythonCode = `import json
import requests

base_url = "http://proxy.aiapps.autel.com/v1"
api_key = "你的令牌"    # 替换为你的令牌
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {api_key}"
}

payload = {
    "model": "aws-claude3.5:haiku-20241022",  # 替换为你想用的模型名称
    "messages": [{
        "role": "user",
        "content": "帮我写一个童话故事，参考安徒生童话"
    }],
    "stream": True,
}
r = requests.post(base_url+"/chat/completions", json=payload, headers=headers)

for line in r.iter_lines():
    line = line.decode("utf-8")
    if line.startswith("data: ") and not line.endswith("[DONE]"):
        data = json.loads(line[len("data: "):])
        chunk = data["choices"][0]["delta"].get("content", "")
        print(chunk, end="", flush=True)`

const openaiCode = `from openai import OpenAI

# 创建 OpenAI 客户端
client = OpenAI(
    api_key="你的令牌",  # 替换为你的令牌
    base_url="http://proxy.aiapps.autel.com/v1"
)

# 创建聊天完成
stream = client.chat.completions.create(
    model="aws-claude3.5:haiku-20241022",  # 替换为你想用的模型名称
    messages=[
        {"role": "user", "content": "帮我写一个童话故事，参考安徒生童话"}
    ],
    stream=True
)

# 处理流式响应
for chunk in stream:
    if chunk.choices[0].delta.content is not None:
        print(chunk.choices[0].delta.content, end="", flush=True)`


const langchainCode = `from langchain_openai.chat_models import ChatOpenAI
from langchain.schema import HumanMessage
from langchain.callbacks.base import BaseCallbackHandler

class StreamHandler(BaseCallbackHandler):
    def __init__(self):
        self.tokens = []

    def on_llm_new_token(self, token: str, **kwargs) -> None:
        self.tokens.append(token)
        print(token, end="", flush=True)

# 创建 ChatOpenAI 实例
stream_handler = StreamHandler()
chat = ChatOpenAI(
    model_name="aws-claude3.5:haiku-20241022",  # 替换为你想用的模型名称
    openai_api_key="你的令牌",  # 替换为你的令牌
    openai_api_base="http://proxy.aiapps.autel.com/v1",
    streaming=True,
    callbacks=[stream_handler]
)

# 创建消息
messages = [
    HumanMessage(content="帮我写一个童话故事，参考安徒生童话")
]

# 生成响应
response = chat(messages)
`


const jsCode = `const axios = require('axios');
const EventSource = require('eventsource');

// llm-proxy 服务地址
const baseUrl = "http://proxy.aiapps.autel.com/v1";
const apiKey = "你的令牌";  // 替换为你的令牌
const headers = {
    "Content-Type": "application/json",
    "Authorization": \`Bearer \${apiKey}\`
};

// 请求数据
const payload = {
    model: "aws-claude3.5:haiku-20241022",  // 填写你想用的模型名称
    messages: [{
        role: "user",
        content: "帮我写一个童话故事，参考安徒生童话"
    }],
    stream: true,
};

// 发送 POST 请求
axios.post(\`\${baseUrl}/chat/completions\`, payload, { 
    headers,
    responseType: 'stream'
})
.then(response => {
    response.data.on('data', (chunk) => {
        const lines = chunk.toString().split('\\n').filter(line => line.trim() !== '');
        for (const line of lines) {
            if (line.startsWith("data: ") && !line.includes("[DONE]")) {
                const data = JSON.parse(line.substring("data: ".length));
                const content = data.choices[0].delta?.content || "";
                process.stdout.write(content);
            }
        }
    });

    response.data.on('end', () => {
        console.log('\\n流式响应结束');
    });
})
.catch(error => {
    console.error('Error:', error.message);
});`

const Document = () => {
    const [tabValue, setTabValue] = React.useState(0);

    const copyToClipboard = (text) => {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(text).then(() => {
                showNotice('复制成功！', true);
            }, () => {
                showError('复制失败！');
            });
        } else {
            const textArea = document.createElement("textarea");
            textArea.value = text;
            document.body.appendChild(textArea);
            textArea.select();
            try {
                document.execCommand('copy');
                showNotice('复制成功！', true);
            } catch (err) {
                showError('复制失败！');
            }
            document.body.removeChild(textArea);
        }
    };

    const codeTabs = [
        { label: 'Curl', code: curlCode, language: 'shell' },
        { label: 'Python', code: pythonCode, language: 'python' },
        { label: 'OpenAI', code: openaiCode, language: 'python' },
        { label: 'LangChain', code: langchainCode, language: 'python' },
        { label: 'NodeJS', code: jsCode, language: 'javascript' },
    ];

    const handleTabChange = (event, newValue) => {
        setTabValue(newValue);
    };

    return (
        <Container>
        <Stack spacing={4}>
            <Stack>
                <Typography variant="h4">支持的模型</Typography>
                <TableContainer component={Paper} sx={{ height: '400px', overflowY: 'auto' }}>
                    <Table stickyHeader aria-label="模型信息表">
                        <TableHead>
                            <TableRow>
                                <TableCell>模型名称</TableCell>
                                <TableCell>平台</TableCell>
                                <TableCell>类型</TableCell>
                                <TableCell>输入令牌成本</TableCell>
                                <TableCell>输出令牌成本</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {allModels.map((model, index) => (
                                <TableRow key={index}>
                                    <TableCell align="left">{model.name}</TableCell>
                                    <TableCell>{model.Platform}</TableCell>
                                    <TableCell>{model.Type}</TableCell>
                                    <TableCell>{model.inputTokensCost}</TableCell>
                                    <TableCell>{model.outputTokensCost}</TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </TableContainer>
            </Stack>

            <Stack>
                <Typography variant="h4">代码示例</Typography>
                <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                    <Tabs value={tabValue} onChange={handleTabChange} aria-label="code examples">
                        {codeTabs.map((tab, index) => (
                            <Tab label={tab.label} key={index} />
                        ))}
                    </Tabs>
                </Box>
                {codeTabs.map((tab, index) => (
                    <TabPanel value={tabValue} index={index} key={index}>
                        <Button onClick={() => copyToClipboard(tab.code)}>复制</Button>
                        <SyntaxHighlighter language={tab.language} style={oneDark}>
                            {tab.code}
                        </SyntaxHighlighter>
                    </TabPanel>
                ))}
            </Stack>
        </Stack>
        </Container>
    );
};

function TabPanel(props) {
    const { children, value, index, ...other } = props;

    return (
        <div
            role="tabpanel"
            hidden={value !== index}
            id={`simple-tabpanel-${index}`}
            aria-labelledby={`simple-tab-${index}`}
            {...other}
        >
            {value === index && (
                <Box sx={{ p: 3 }}>
                    {children}
                </Box>
            )}
        </div>
    );
}

export default Document;
