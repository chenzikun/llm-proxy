import json
import base64

from test_base import Base

models = [{
    'name': 'gpt-4',
    'inputTokensCost': 0.03,
    'outputTokensCost': 0.06,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-0314',
    'inputTokensCost': 0.03,
    'outputTokensCost': 0.06,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-0613',
    'inputTokensCost': 0.03,
    'outputTokensCost': 0.06,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-32k-0314',
    'inputTokensCost': 0.06,
    'outputTokensCost': 0.12,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-1106-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-0125-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-turbo-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-turbo',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-turbo-2024-04-09',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4-vision-preview',
    'inputTokensCost': 0.01,
    'outputTokensCost': 0.03,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4o',
    'inputTokensCost': 0.005,
    'outputTokensCost': 0.015,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4o-2024-05-13',
    'inputTokensCost': 0.005,
    'outputTokensCost': 0.015,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4o-mini',
    'inputTokensCost': 0.00015,
    'outputTokensCost': 0.0006,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-4o-mini-2024-07-18',
    'inputTokensCost': 0.00015,
    'outputTokensCost': 0.0006,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-3.5-turbo',
    'inputTokensCost': 0.0005,
    'outputTokensCost': 0.0015,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'gpt-3.5-turbo-16k',
    'inputTokensCost': 0.003,
    'outputTokensCost': 0.004,
    'Platform': 'OpenAI',
    'Type': 'Chat'
}, {
    'name': 'tts-1',
    'inputTokensCost': 0.015,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
}, {
    'name': 'tts-1-1106',
    'inputTokensCost': 0.015,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
}, {
    'name': 'tts-1-hd',
    'inputTokensCost': 0.03,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
}, {
    'name': 'tts-1-hd-1106',
    'inputTokensCost': 0.03,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'TTS'
}, {
    'name': 'whisper-1',
    'inputTokensCost': 0.03,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'STT'
}, {
    'name': 'text-embedding-ada-002',
    'inputTokensCost': 0.0001,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Embedding'
}, {
    'name': 'text-embedding-3-small',
    'inputTokensCost': 2e-05,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Embedding'
}, {
    'name': 'text-embedding-3-large',
    'inputTokensCost': 0.00013,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Embedding'
}, {
    'name': 'dall-e-2',
    'inputTokensCost': 0.02,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Image'
}, {
    'name': 'dall-e-3',
    'inputTokensCost': 0.04,
    'outputTokensCost': '',
    'Platform': 'OpenAI',
    'Type': 'Image'
}]


#图片转base64函数
def encode_image(image_path):
    with open(image_path, "rb") as image_file:
        return base64.b64encode(image_file.read()).decode("utf8")


class TestOpenAIProxy(Base):

    # base_url = "http://10.240.3.251:3500/v1"
    # base_url = "http://127.0.0.1:3000/v1"
    base_url = "http://10.240.1.171:3000/v1"
    api_key = "sk-*"
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}"
    }

    def test_chat(self, model="gpt-4o-mini"):
        params = {
            "model":
            model,
            "messages": [{
                "role": "user",
                "content": [{
                    "type": "text",
                    "text": "这是一个测试"
                }]
            }],
            "stream":
            True,
        }
        try:
            r = self._http_post(self.CHAT_URL, params=params)
        except Exception as e:
            print(e)
            return
        for line in r.iter_lines():
            line = line.decode("utf-8")
            if line.startswith("data: ") and not line.endswith("[DONE]"):
                data = json.loads(line[len("data: "):])
                chunk = data["choices"][0]["delta"].get("content", "")
                print(chunk, end="", flush=True)
        print()

    def test_chat_with_image(self, model="gpt-4o-mini"):
        base64_image = encode_image(
            "C:\\Users\\A24619\\Downloads\\field-8544288_1280.jpg")
        params = {
            "model":
            model,
            "messages": [{
                "role":
                "user",
                "content": [{
                    "type": "text",
                    "text": "描述一下这个图片"
                }, {
                    "type": "image_url",
                    "image_url": {
                        "url": f"data:image/jpeg;base64,{base64_image}"
                    }
                }]
            }],
            "stream":
            True,
        }
        r = self._http_post(self.CHAT_URL, params=params)
        for line in r.iter_lines():
            line = line.decode("utf-8")
            if line.startswith("data: ") and not line.endswith("[DONE]"):
                data = json.loads(line[len("data: "):])
                chunk = data["choices"][0]["delta"].get("content", "")
                print(chunk, end="", flush=True)

    def test_embedding(self, model="text-embedding-ada-002"):
        params = {
            "input": "The food was delicious and the waiter...",
            "model": model,
            "encoding_format": "float"
        }
        data = self._http_post(self.EMBEDDING_URL, params=params)
        print("model=", model, "result= ", data)

    def test_image(self, model="dall-e-3"):
        params = {
            "model": model,
            "prompt": "A cute baby sea otter",
            "n": 1,
            "size": "1024x1024"
        }
        data = self._http_post(self.IMAGE_URL, params=params)
        print("model=", model, "image= ", data)

    def test_rerank(self):
        pass

    def test_stt(self, model="whisper-1"):
        """Speech-To-Text 语音转文本"""
        data = self._http_post_formdata(url=self.STT_URL,
                                        data={"model": model},
                                        files={
                                            "file":
                                            ("./outputs/tts-1.mp3",
                                             open("./outputs/tts-1.mp3",
                                                  "rb"), "audio/mp3")
                                        })
        print(data)

    def test_tts(self, model="tts-1"):
        """Text-To-Speech 文本转语音"""
        params = {
            "model": model,
            "input": "The quick brown fox jumped over the lazy dog.",
            "voice": "alloy"
        }
        r = self._http_post(self.TTS_URL, params=params)
        with open(f"./outputs/{model}.mp3", "wb+") as f:
            f.write(r.content)

    def test_moderation(self, model="text-moderation-stable"):
        base64_image = encode_image(
            "C:\\Users\\A24619\\Downloads\\field-8544288_1280.jpg")
        params = {
            "model":
            model,
            "input": [{
                "type": "text",
                "text": "描述一下这个图片"
            }, {
                "type": "image_url",
                "image_url": {
                    "url": f"data:image/jpeg;base64,{base64_image}"
                }
            }],
            "stream":
            True,
        }
        r = self._http_post(self.CHAT_URL, params=params)

    def test_file_upload(self):
        with open("C:\\Users\\A24619\\Downloads\\sql_data.jsonl", "r") as f:
            r = self._http_post_formdata(url=self.FILE_UPLOAD,
                                         data={"purpose": "fine-tune"},
                                         files={"file": f})
            print(r)
            return r["id"]

    def test_file_list(self):
        r = self._http_get(self.FILE_LIST)
        print(r)
    
    def test_file_retrieve(self, file_id="file-fxV8rkufdWd0XG443IaU4XH9"):
        r = self._http_get(self.FILE_RETRIEVE.format(file_id=file_id))
        print(r)

    def test_file_retrieve_content(self, file_id="file-fxV8rkufdWd0XG443IaU4XH9"):
        r = self._http_get(self.FILE_RETRIEVE_CONTENT.format(file_id=file_id))
        print(r)

    def test_file_delete(self, file_id="file-fxV8rkufdWd0XG443IaU4XH9"):
        r = self._http_delete(self.FILE_DELETE.format(file_id=file_id))
        print(r)

    def test_finetuning(self, file_id="file-fxV8rkufdWd0XG443IaU4XH9"):
        params = {
            "training_file": file_id,
            "model": "gpt-4o-mini-2024-07-18",
            "hyperparameters": {
                "n_epochs": 2
            }
        }
        r = self._http_post(self.FINE_TUNING, params=params)
        print(r)
        return r["id"]
    
    def test_finetuning_list(self):
        r = self._http_get(self.FINE_TUNING_LIST)
        print(r)

    def test_finetuning_retrieve(self, job_id="job-1"):
        r = self._http_get(self.FINE_TUNING_RETRIEVE.format(job_id=job_id))
        print(r)

    def test_finetuning_events_list(self, job_id="job-1"):
        r = self._http_get(self.FINE_TUNING_EVENTS_LIST.format(job_id=job_id))
        print(r)
    
    def test_finetuning_cancel(self, job_id="job-1"):
        r = self._http_post(self.FINE_TUNING_CANCEL.format(job_id=job_id))
        print(r)
    
    def test_files(self):
        file_id = self.test_file_upload()
        self.test_file_list()
        self.test_file_retrieve(file_id=file_id)
        self.test_file_retrieve_content(file_id=file_id)
        self.test_file_delete(file_id=file_id)
    
    def test_finetuning_api(self):
        file_id = self.test_file_upload()
        print("file_id=", file_id)
        job_id = self.test_finetuning(file_id=file_id)
        self.test_finetuning_list()
        self.test_finetuning_retrieve(job_id=job_id)
        self.test_finetuning_events_list(job_id=job_id)
        self.test_finetuning_cancel(job_id=job_id)
        self.test_file_delete(file_id=file_id)

    def test_all_models(self):
        for model in models:
            print("model=", model)
            if model["Type"] == "Chat":
                self.test_chat(model=model["name"])
            elif model["Type"] == "Embedding":
                self.test_embedding(model=model["name"])
            elif model["Type"] == "Image":
                self.test_image(model=model["name"])
            # elif model["Type"] == "STT":
            #     self.test_stt(model=model["name"])
            elif model["Type"] == "TTS":
                self.test_tts(model=model["name"])
            elif model["Type"] == "Moderation":
                self.test_tts(model=model["name"])
            else:
                print("未处理的类型, type=", model["Type"])
