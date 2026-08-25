# LLM-Proxy Tutorial

支持的模型：
```shell
# OpenAI
gpt-3.5-turbo
gpt-4
gpt-4-turbo-preview
gpt-4-turbo
gpt-4-turbo-2024-04-09
gpt-4o
gpt-4o-2024-05-13
gpt-4o-mini
gpt-4o-mini-2024-07-18
text-embedding-ada-002
text-embedding-3-small
text-embedding-3-large
text-curie-001
text-babbage-001
text-ada-001
text-davinci-002
text-davinci-003
text-moderation-latest
text-moderation-stable
text-davinci-edit-001
davinci-002
babbage-002
dall-e-2
dall-e-3
whisper-1
tts-1
tts-1-1106
tts-1-hd
tts-1-hd-1106

# Ollama
gemma2:9b
gemma2:27b
llama3:8b
llama3:8b_lora
llama3:70b
llama3.1:8b
llama3.1:70b

# AWS bedrock
claude-3-5-sonnet-20240620
claude-3-5-sonnet-20240620
llama3-8b-8192
llama3-1-8b
claude-3-opus-20240229
claude-3-sonnet-20240229
llama3-70b-8192
llama3-1-70b
llama3-1-405b
claude-3-haiku-20240307
```

curl
```shell
# 这里使用的是 llm-proxy 服务的地址
curl https://10.240.3.251:3500/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 你的令牌" \
  -d '{
    "model": "gpt-4o",
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
  }'
```

Python
```python
import json
import requests

# llm-proxy 服务地址
base_url = "http://10.240.3.251:3500/v1"
api_key = "你的令牌"    # 替换为你的令牌
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {api_key}"
}

payload = {
    "model": "gpt-4o-mini",  # 替换为你想用的模型名称
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
        print(chunk, end="", flush=True)
```

 NodeJS
 ```js
 const axios = require('axios');
const EventSource = require('eventsource');

// llm-proxy 服务地址
const baseUrl = "http://10.240.3.251:3500/v1";
const apiKey = "你的令牌";  // 替换为你的令牌
const headers = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${apiKey}`
};

// 请求数据
const payload = {
    model: "gpt-4o-mini",  // 填写你想用的模型名称
    messages: [{
        role: "user",
        content: "帮我写一个童话故事，参考安徒生童话"
    }],
    stream: true,
};

// 发送 POST 请求
axios.post(`${baseUrl}/chat/completions`, payload, { headers })
    .then(response => {
        const eventSourceUrl = response.data.url; // 假设响应中包含 stream URL
        const eventSource = new EventSource(eventSourceUrl, { headers });

        eventSource.onmessage = (event) => {
            const line = event.data;

            if (line.startsWith("data: ") && !line.endsWith("[DONE]")) {
                const data = JSON.parse(line.substring("data: ".length));
                const chunk = data.choices[0].delta?.content || "";
                process.stdout.write(chunk);
            }
        };

        eventSource.onerror = (error) => {
            console.error('Error:', error);
            eventSource.close();
        };
    })
    .catch(error => {
        console.error('Error:', error.message);
    });
 ```