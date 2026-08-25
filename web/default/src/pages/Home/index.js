import React, { useContext, useEffect, useState } from 'react';
import { Card, Grid, Header, Segment, Button, Dropdown, Tab, Table } from 'semantic-ui-react';
import { API, showError, showNotice, timestamp2string } from '../../helpers';
import { StatusContext } from '../../context/Status';
import { marked } from 'marked';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';

const models = [
  {
    name: 'GPT-4',
    contextWindow: '8192 tokens',
    inputTokensCost: '$0.03 per 1k tokens',
    outputTokensCost: '$0.06 per 1k tokens',
    description: 'GPT-4 is a large multimodal model that can accept image and text inputs and produce text outputs.',
  },
  {
    name: 'Claude 3.5',
    contextWindow: '4096 tokens',
    inputTokensCost: '$0.02 per 1k tokens',
    outputTokensCost: '$0.04 per 1k tokens',
    description: 'Claude 3.5 is a conversational AI model designed for natural language understanding and generation.',
  },
  {
    name: 'Llama 3',
    contextWindow: '8192 tokens',
    inputTokensCost: '$0.01 per 1k tokens',
    outputTokensCost: '$0.02 per 1k tokens',
    description: 'Llama 3 is a state-of-the-art language model optimized for various natural language tasks.',
  },
];

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
  'name': 'gpt-4-32k',
  'inputTokensCost': 0.06,
  'outputTokensCost': 0.12,
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
  'name': 'gpt-4-32k-0613',
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
  'name': 'gpt-3.5-turbo-instruct',
  'inputTokensCost': 0.0015,
  'outputTokensCost': 0.002,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'davinci-002',
  'inputTokensCost': 0.002,
  'outputTokensCost': 0.002,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'babbage-002',
  'inputTokensCost': 0.0004,
  'outputTokensCost': 0.0004,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'text-ada-001',
  'inputTokensCost': 0.0004,
  'outputTokensCost': 0.0004,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'text-babbage-001',
  'inputTokensCost': 0.0005,
  'outputTokensCost': 0.0005,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'text-curie-001',
  'inputTokensCost': 0.002,
  'outputTokensCost': 0.002,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'text-davinci-002',
  'inputTokensCost': 0.02,
  'outputTokensCost': 0.02,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'text-davinci-003',
  'inputTokensCost': 0.02,
  'outputTokensCost': 0.02,
  'Platform': 'OpenAI',
  'Type': 'Chat'
},
{
  'name': 'whisper-1',
  'inputTokensCost': 0.03,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Audio'
},
{
  'name': 'tts-1',
  'inputTokensCost': 0.015,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Audio'
},
{
  'name': 'tts-1-1106',
  'inputTokensCost': 0.015,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Audio'
},
{
  'name': 'tts-1-hd',
  'inputTokensCost': 0.03,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Audio'
},
{
  'name': 'tts-1-hd-1106',
  'inputTokensCost': 0.03,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Audio'
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
  'name': 'text-search-ada-doc-001',
  'inputTokensCost': 0.02,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Embedding'
},
{
  'name': 'text-moderation-stable',
  'inputTokensCost': 0.0002,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Moderation'
},
{
  'name': 'text-moderation-latest',
  'inputTokensCost': 0.0002,
  'outputTokensCost': '',
  'Platform': 'OpenAI',
  'Type': 'Moderation'
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
  'name': 'claude-3-haiku-20240307',
  'inputTokensCost': 0.00025,
  'outputTokensCost': 0.00125,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'claude-3-sonnet-20240229',
  'inputTokensCost': 0.003,
  'outputTokensCost': 0.015,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'claude-3-5-sonnet-20240620',
  'inputTokensCost': 0.003,
  'outputTokensCost': 0.015,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'claude-3-opus-20240229',
  'inputTokensCost': 0.015,
  'outputTokensCost': 0.075,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'llama3-8b-8192',
  'inputTokensCost': 0.0003,
  'outputTokensCost': 0.0006,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'llama3-70b-8192',
  'inputTokensCost': 0.00265,
  'outputTokensCost': 0.0035,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'llama3-1-8b',
  'inputTokensCost': 0.00022,
  'outputTokensCost': 0.00022,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'llama3-1-70b',
  'inputTokensCost': 0.00099,
  'outputTokensCost': 0.00099,
  'Platform': 'AWS',
  'Type': 'Chat'
},
{
  'name': 'llama3-1-405b',
  'inputTokensCost': 0.00532,
  'outputTokensCost': 0.016,
  'Platform': 'AWS',
  'Type': 'Chat'
}]

const curlCode = `curl http://10.240.3.251:3500/v1/chat/completions \
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
  }'`

const pythonCode = `import json
import requests

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
        print(chunk, end="", flush=True)`

const jsCode = `const axios = require('axios');
const EventSource = require('eventsource');

// llm-proxy 服务地址
const baseUrl = "http://10.240.3.251:3500/v1";
const apiKey = "你的令牌";  // 替换为你的令牌
const headers = {
    "Content-Type": "application/json",
    "Authorization": 'Bearer \${apiKey}'
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
axios.post('\${baseUrl}/chat/completions', payload, { headers })
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
    });`

const Home = () => {
  const [statusState] = useContext(StatusContext);
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [selectedModel, setSelectedModel] = useState(models[0]); // 默认选择第一个模型

  const displayNotice = async () => {
    const res = await API.get('/api/notice');
    const { success, message, data } = res.data;
    if (success) {
      let oldNotice = localStorage.getItem('notice');
      if (data !== oldNotice && data !== '') {
        const htmlNotice = marked(data);
        showNotice(htmlNotice, true);
        localStorage.setItem('notice', data);
      }
    } else {
      showError(message);
    }
  };

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);
    } else {
      showError(message);
      setHomePageContent('加载首页内容失败...');
    }
    setHomePageContentLoaded(true);
  };

  const getStartTimeString = () => {
    const timestamp = statusState?.status?.start_time;
    return timestamp2string(timestamp);
  };

  const copyToClipboard = (text) => {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => {
        showNotice('复制成功！', true);
      }, () => {
        showError('复制失败！');
      });
    } else {
      // 备选方案：创建一个临时文本区域来复制文本
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

  const codePanes = [
    {
      menuItem: 'Curl',
      render: () => (
        <Tab.Pane>
          <Button onClick={() => copyToClipboard(curlCode)}>复制</Button>
          <SyntaxHighlighter language="shell" style={oneDark}>
            {curlCode}
          </SyntaxHighlighter>
        </Tab.Pane>
      ),
    },
    {
      menuItem: 'Python',
      render: () => (
        <Tab.Pane>
          <Button onClick={() => copyToClipboard(pythonCode)}>复制</Button>
          <SyntaxHighlighter language="python" style={oneDark}>
            {pythonCode}
          </SyntaxHighlighter>
        </Tab.Pane>
      ),
    },
    {
      menuItem: 'NodeJS',
      render: () => (
        <Tab.Pane>
          <Button onClick={() => copyToClipboard(jsCode)}>复制</Button>
          <SyntaxHighlighter language="javascript" style={oneDark}>
            {jsCode}
          </SyntaxHighlighter>
        </Tab.Pane>
      ),
    },
  ];

  useEffect(() => {
    displayNotice().then();
    displayHomePageContent().then();
  }, []);

  return (
    <>
      {
        homePageContentLoaded ? (
          <>
            {
              homePageContent === '' ? (
                ''
              ) : (
                <div style={{ fontSize: 'larger' }} dangerouslySetInnerHTML={{ __html: homePageContent }}></div>
              )
            }

            {/* 新增的模型选择和描述部分 */}
            {/* <Segment>
              <Header as='h3'>支持的模型</Header>
              <Dropdown
                placeholder='选择模型'
                fluid
                selection
                options={models.map(model => ({
                  key: model.name,
                  text: model.name,
                  value: model.name,
                }))}
                onChange={(e, { value }) => setSelectedModel(models.find(model => model.name === value))}
              />
              <Segment>
                <Header as='h4'>模型信息</Header>
                <p><strong>描述:</strong> {selectedModel.description}</p>
                <p><strong>上下文窗口:</strong> {selectedModel.contextWindow}</p>
                <p><strong>输入令牌成本:</strong> {selectedModel.inputTokensCost}</p>
                <p><strong>输出令牌成本:</strong> {selectedModel.outputTokensCost}</p>
              </Segment>
            </Segment> */}

            <Segment>
              <Header as='h3'>支持的模型</Header>
              <div style={{ height: '400px', overflowY: 'auto' }}>
                <Table celled>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell>模型名称</Table.HeaderCell>
                      <Table.HeaderCell>平台</Table.HeaderCell>
                      <Table.HeaderCell>类型</Table.HeaderCell>
                      <Table.HeaderCell>输入令牌成本</Table.HeaderCell>
                      <Table.HeaderCell>输出令牌成本</Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {allModels.map((model, index) => (
                      <Table.Row key={index}>
                        <Table.Cell>{model.name}</Table.Cell>
                        <Table.Cell>{model.Platform}</Table.Cell>
                        <Table.Cell>{model.Type}</Table.Cell>
                        <Table.Cell>{model.inputTokensCost}</Table.Cell>
                        <Table.Cell>{model.outputTokensCost}</Table.Cell>
                      </Table.Row>
                    ))}
                    {/* 可以根据需要添加更多行 */}
                  </Table.Body>
                </Table>
              </div>
            </Segment>

            <Segment>
              <Header as='h3'>代码示例</Header>
              <Tab panes={codePanes} />
            </Segment>
          </>
        ) : (
          <div>加载中...</div> // 可以显示加载状态
        )
      }
    </>
  );
};

export default Home;
