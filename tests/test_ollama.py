import json
import base64

from test_base import Base


models = [{
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
}]


#图片转base64函数
def encode_image(image_path):
  with open(image_path, "rb") as image_file:
    return base64.b64encode(image_file.read()).decode("utf8")


class TestOllamaProxy(Base):

    # base_url = "http://10.240.1.171:3000/v1"
    # base_url = "http://10.240.3.251:3500/v1"
    base_url = "http://127.0.0.1:3000/v1"
    api_key = "sk-"     # http://10.240.1.171:3000
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}"
    }

    def test_chat(self, model="ollama-llama3:8b"):
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

    def test_chat_with_image(self):
        base64_image = encode_image("C:\\Users\\A24619\\Downloads\\field-8544288_1280.jpg")
        params = {
            "model": "ollama-llava1.6:13b",
            "messages": [{
                "role": "user",
                "content": [
                    {
                        "type":"text",
                        "text": "描述一下这个图片"
                    },
                    {
                        "type":"image_url",
                        "image_url":{
                            "url": f"data:image/jpeg;base64,{base64_image}"
                        }
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
        pass

    def test_image(self):
        pass

    def test_rerank(self):
        pass

    def test_stt(self):
        pass

    def test_tts(self):
        pass

    def test_all_models(self):
        for model in models:
            print("model=", model)
            if model["Type"] == "Chat":
                self.test_chat(model=model["name"])
            else:
                print("未处理的类型, type=", model["Type"])