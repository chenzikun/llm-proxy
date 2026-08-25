import json

from test_base import Base


class TestOpenAIProxy(Base):

    base_url = "https://api.openai.com/v1"
    api_key = "sk-*"
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}"
    }

    def test_chat(self):
        params = {
            "model": "gpt-4-32k",
            "messages": [{
                "role": "user",
                "content": "Say this is a test"
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

    def test_stt(self, model="whisper-1"):
        """Speech-To-Text 语音转文本"""
        data = self._http_post_formdata(
            url=self.STT_URL,
            data={"model": model},
            files={"file": ("./outputs/tts-1.mp3", open("./outputs/tts-1.mp3", "rb"), "audio/mp3")})
        print(data)
    
    def test_file_upload(self):
        with open("C:\\Users\\A24619\\Downloads\\sql_data.jsonl", "r") as f:
            r = self._http_post_formdata(
                url=self.FILE_UPLOAD,
                data={"purpose": "fine-tune"},
                files={"file": f})
            print(r)