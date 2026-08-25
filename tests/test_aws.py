import json

from test_base import Base


models = [{
    'name': 'aws-claude3:haiku-20240307',
    'inputTokensCost': 0.00025,
    'outputTokensCost': 0.00125,
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
}]

class TestAWSProxy(Base):

    base_url = "http://10.240.1.171:3000/v1"
    # base_url = "http://10.240.3.251:3500/v1"
    api_key = "sk-"
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}"
    }

    def test_chat(self, model="aws-llama3.2:3b"):
        params = {
            "model": model,
            "messages": [{
                "role": "user",
                "content": [
                    {
                        "type":"text",
                        "text": "这是一个测试"
                    }
                ]
            }],
            "stream": True,
        }
        r = self._http_post(self.CHAT_URL, params=params)
        for line in r.iter_lines():
            line = line.decode("utf-8")
            if line.startswith("data: ") and not line.endswith("[DONE]"):
                data = json.loads(line[len("data: "):])
                chunk = data["choices"][0]["delta"].get("content", "")
                print(chunk, end="", flush=True)
        print()

    def test_embedding(self):
        params = {
            "input": "The food was delicious and the waiter...",
            "model": "embed-multilingual-v3.0",
            "encoding_format": "float"
        }
        data = self._http_post(self.EMBEDDING_URL, params=params)
        print(data)

    def test_image(self):
        params = {
            "model": "stable diffusion",
            "prompt": "A cute baby sea otter",
            "n": 1,
            "size": "1024x1024"
        }
        data = self._http_post(self.IMAGE_URL, params=params)
        print(data)

    def test_rerank(self):
        # model: rerank-multilingual-v3.0
        pass

    def test_stt(self):
        """Speech-To-Text 语音转文本"""
        data = self._http_post_formdata(
            url=self.STT_URL,
            data={"model": "transcible"},
            files={"file": open("audio.mp3", "rb")})
        print(data)

    def test_tts(self):
        """Text-To-Speech 文本转语音"""
        params = {
            "model": "polly",
            "input": "The quick brown fox jumped over the lazy dog.",
            "voice": "alloy"
        }
        r = self._http_post(self.TTS_URL, params=params)
        with open("audio.mp3", "wb+") as f:
            f.write(r.content)

    def test_all_models(self):
        for model in models:
            print("model=", model)
            try:
                if model["Type"] == "Chat":
                    self.test_chat(model=model["name"])
                else:
                    print("未处理的类型, type=", model["Type"])
            except Exception as e:
                print(e)
                